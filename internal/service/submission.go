package service

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"time"
	"unicode/utf8"

	"blacklist-index/internal/model"
	"blacklist-index/internal/repository"
)

var (
	ErrSubmissionNotFound     = errors.New("申请不存在")
	ErrSubmissionNotPending   = errors.New("该申请已处理，无法重复审核")
	ErrReasonRequired         = errors.New("请填写标记原因")
	ErrAppealReasonRequired   = errors.New("请填写申诉理由")
	ErrRejectReasonRequired   = errors.New("请填写驳回原因")
	ErrAppealEvidenceRequired = errors.New("反驳证据链接为必填项，请提供有效链接")
	ErrReportReasonTooLong    = errors.New("标记原因过长，最多 500 字")
	ErrAppealReasonTooLong    = errors.New("申诉理由过长，最多 500 字")
	ErrRejectReasonTooLong    = errors.New("驳回原因过长，最多 500 字")
	ErrImageRequired          = errors.New("请上传证据图片")
	ErrImageTooMany           = errors.New("上传图片数量超出限制")
	ErrImageTooLarge          = errors.New("单张图片大小不能超过 5MB")
)

type SubmissionService struct {
	repo      *repository.SubmissionRepo
	blacklist *BlacklistService
	audit     *AuditService
	images    *ImageService
	location  *time.Location
}

func NewSubmissionService(repo *repository.SubmissionRepo, blacklist *BlacklistService, audit *AuditService, images *ImageService, loc *time.Location) *SubmissionService {
	return &SubmissionService{repo: repo, blacklist: blacklist, audit: audit, images: images, location: loc}
}

// GenerateQueryCode 生成 12 位查询码（8 位随机十六进制 + 4 位时间戳后缀）。
func GenerateQueryCode() string {
	b := make([]byte, 5)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%s%s", hex.EncodeToString(b), fmt.Sprintf("%04d", time.Now().Unix()%10000))
}

// SubmitReport 提交举报（标记为不可信申请）。
func (s *SubmissionService) SubmitReport(email, reason, link, relatedPeople string, policy LinkPolicy, imgPolicy ImagePolicy, imageDatas [][]byte, ip, ua string) (string, error) {
	email = NormalizeEmail(email)
	if !ValidEmail(email) {
		return "", ErrInvalidEmail
	}
	if len(reason) == 0 {
		return "", ErrReasonRequired
	}
	if ReasonTooLong(reason) {
		return "", ErrReportReasonTooLong
	}
	eventLink := NormalizeLink(link)
	if policy.EvidenceRequired && eventLink == "" {
		return "", ErrEventLinkRequired
	}
	if eventLink != "" {
		if err := ValidateLinkWithDomains(eventLink, policy.Domains); err != nil {
			return "", err
		}
	}

	images, err := s.processImages(imageDatas, imgPolicy)
	if err != nil {
		return "", err
	}

	code := GenerateQueryCode()
	now := NowStr(s.location)
	sub := &model.Submission{
		QueryCode:          code,
		Type:               model.TypeReport,
		Email:              email,
		BanReason:          reason,
		EventLink:          eventLink,
		EventRelatedPeople: relatedPeople,
		Status:             model.StatusPending,
		SubmitterIP:        ip,
		SubmitterUA:        ua,
		CreatedAt:          now,
	}
	if err := s.repo.Create(sub, images); err != nil {
		return "", err
	}
	return code, nil
}

// SubmitAppeal 提交申诉。
func (s *SubmissionService) SubmitAppeal(email, appealReason, appealEvidence string, policy LinkPolicy, imgPolicy ImagePolicy, imageDatas [][]byte, ip, ua string) (string, error) {
	email = NormalizeEmail(email)
	if !ValidEmail(email) {
		return "", ErrInvalidEmail
	}
	if len(appealReason) == 0 {
		return "", ErrAppealReasonRequired
	}
	if utf8.RuneCountInString(appealReason) > MaxAppealReasonLen {
		return "", ErrAppealReasonTooLong
	}
	evidence := NormalizeLink(appealEvidence)
	if policy.EvidenceRequired && evidence == "" {
		return "", ErrAppealEvidenceRequired
	}
	if evidence != "" {
		if err := ValidateLinkWithDomains(evidence, policy.Domains); err != nil {
			return "", err
		}
	}

	images, err := s.processImages(imageDatas, imgPolicy)
	if err != nil {
		return "", err
	}

	code := GenerateQueryCode()
	now := NowStr(s.location)
	sub := &model.Submission{
		QueryCode:      code,
		Type:           model.TypeAppeal,
		Email:          email,
		AppealReason:   appealReason,
		AppealEvidence: evidence,
		Status:         model.StatusPending,
		SubmitterIP:    ip,
		SubmitterUA:    ua,
		CreatedAt:      now,
	}
	if err := s.repo.Create(sub, images); err != nil {
		return "", err
	}
	return code, nil
}

// processImages 校验并落盘证据图片，返回待关联的图片元数据。
func (s *SubmissionService) processImages(datas [][]byte, policy ImagePolicy) ([]*model.SubmissionImage, error) {
	if policy.Required && len(datas) == 0 {
		return nil, ErrImageRequired
	}
	if policy.MaxCount > 0 && len(datas) > policy.MaxCount {
		return nil, ErrImageTooMany
	}

	images := make([]*model.SubmissionImage, 0, len(datas))
	for i, data := range datas {
		if len(data) == 0 {
			continue
		}
		if len(data) > MaxImageSize {
			return nil, ErrImageTooLarge
		}
		hash, ext, err := s.images.Save(data)
		if err != nil {
			return nil, err
		}
		images = append(images, &model.SubmissionImage{
			FileHash:  hash,
			Ext:       ext,
			Size:      int64(len(data)),
			SortOrder: i,
		})
	}
	return images, nil
}

// CleanupOrphanedImages 删除磁盘上不可达（无有效引用）的图片文件。
// 「可达」指图片属于待审核申请，或仍属于黑名单（含回收站）记录；黑名单被彻底删除或申请
// 已驳回后，其图片才视为不可达而被删除。返回删除的文件数与文件名（file_hash.ext）。
func (s *SubmissionService) CleanupOrphanedImages() (int, []string, error) {
	referenced, err := s.repo.ListReferencedImages()
	if err != nil {
		return 0, nil, err
	}
	files, err := s.images.ListStoredImages()
	if err != nil {
		return 0, nil, err
	}

	deleted := 0
	names := make([]string, 0)
	for _, f := range files {
		if referenced[f.Hash+"."+f.Ext] {
			continue
		}
		if err := os.Remove(f.Path); err != nil {
			return deleted, names, err
		}
		deleted++
		names = append(names, f.Hash+"."+f.Ext)
	}
	return deleted, names, nil
}

// GetByQueryCode 根据查询码获取申请详情。
func (s *SubmissionService) GetByQueryCode(code string) (*model.Submission, error) {
	sub, err := s.repo.GetByQueryCode(code)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrSubmissionNotFound
		}
		return nil, err
	}
	s.normalizeTimes(sub)
	return sub, nil
}

// List 分页列出申请（管理员后台）。
func (s *SubmissionService) List(status string, typ string, page int, pageSize int) ([]*model.Submission, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	list, total, err := s.repo.List(status, typ, page, pageSize)
	if err != nil {
		return nil, 0, err
	}
	for _, sub := range list {
		s.normalizeTimes(sub)
		if err := s.attachImages(sub); err != nil {
			return nil, 0, err
		}
	}
	return list, total, nil
}

// attachImages 关联图片元数据并拼接后台可加载的 URL。
func (s *SubmissionService) attachImages(sub *model.Submission) error {
	images, err := s.repo.ListImages(sub.ID)
	if err != nil {
		return err
	}
	for _, img := range images {
		img.URL = "/api/v1/admin/image/" + img.FileHash + "." + img.Ext
	}
	sub.Images = images
	return nil
}

// normalizeTimes 统一提交时间/审核时间为目标格式，兼容历史带 Z 后缀的数据。
func (s *SubmissionService) normalizeTimes(sub *model.Submission) {
	sub.CreatedAt = NormalizeTime(sub.CreatedAt)
	sub.ReviewedAt = NormalizeTime(sub.ReviewedAt)
}

// Approve 通过申请（举报通过则加入黑名单；申诉通过则从黑名单移除）。
func (s *SubmissionService) Approve(id int64, reviewer string, domains []string, ip, ua string) (*model.Submission, error) {
	sub, err := s.repo.GetByID(id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrSubmissionNotFound
		}
		return nil, err
	}
	if sub.Status != model.StatusPending {
		return nil, ErrSubmissionNotPending
	}
	s.normalizeTimes(sub)

	now := NowStr(s.location)
	if sub.Type == model.TypeReport {
		// 举报通过 → 加入黑名单（以审核通过时间作为标记时间）
		_, err := s.blacklist.Add(sub.Email, sub.BanReason, sub.EventLink, sub.EventRelatedPeople, now, domains, sub.ID, ip, ua)
		if err != nil {
			return nil, err
		}
	} else if sub.Type == model.TypeAppeal {
		// 申诉通过 → 从黑名单删除（软删）
		bl, err := s.blacklist.Check(sub.Email)
		if err == nil && bl != nil {
			_ = s.blacklist.Delete(bl.ID, ip, ua)
		}
		// 如果没找到也不报错，可能已经被处理
	}

	if err := s.repo.Approve(id, reviewer, now); err != nil {
		return nil, err
	}

	sub.Status = model.StatusApproved
	sub.ReviewedAt = now
	sub.ReviewedBy = reviewer

	s.audit.Log(reviewer, ActionApproveSub, fmt.Sprintf("%s (ID:%d)", sub.Email, sub.ID), ip, ua)
	return sub, nil
}

// Reject 驳回申请。
func (s *SubmissionService) Reject(id int64, reason string, reviewer, ip, ua string) (*model.Submission, error) {
	if len(reason) == 0 {
		return nil, ErrRejectReasonRequired
	}
	if utf8.RuneCountInString(reason) > 500 {
		return nil, ErrRejectReasonTooLong
	}

	sub, err := s.repo.GetByID(id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrSubmissionNotFound
		}
		return nil, err
	}
	if sub.Status != model.StatusPending {
		return nil, ErrSubmissionNotPending
	}
	s.normalizeTimes(sub)

	now := NowStr(s.location)
	if err := s.repo.Reject(id, reason, reviewer, now); err != nil {
		return nil, err
	}

	sub.Status = model.StatusRejected
	sub.RejectReason = reason
	sub.ReviewedAt = now
	sub.ReviewedBy = reviewer

	s.audit.Log(reviewer, ActionRejectSub, fmt.Sprintf("%s (ID:%d)", sub.Email, sub.ID), ip, ua)
	return sub, nil
}
