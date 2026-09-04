package service

import (
	"errors"
	"fmt"
	"time"

	"blacklist-index/internal/model"
	"blacklist-index/internal/repository"
)

var (
	// ErrEmailExists 表示邮箱已被标记为不可信。
	ErrEmailExists = errors.New("该邮箱已被标记为不可信")
	// ErrInvalidEmail 表示邮箱格式非法。
	ErrInvalidEmail = errors.New("请输入有效的邮箱地址")
	// ErrReasonTooLong 表示标记原因超出长度限制。
	ErrReasonTooLong = fmt.Errorf("标记原因过长，最多 %d 字", MaxReasonLen)
	// ErrEventLinkRequired 表示事件链接为必填项。
	ErrEventLinkRequired = errors.New("事件链接为必填项，请提供有效的证据链接")
	// ErrInTrash 表示记录在回收站中不可编辑。
	ErrInTrash = errors.New("回收站中的记录不可编辑，请先还原")
)

// BlacklistService 负责黑名单业务逻辑。
type BlacklistService struct {
	repo  *repository.BlacklistRepo
	audit *AuditService
	loc   *time.Location
}

func NewBlacklistService(repo *repository.BlacklistRepo, audit *AuditService, loc *time.Location) *BlacklistService {
	return &BlacklistService{repo: repo, audit: audit, loc: loc}
}

// Check 查询邮箱是否在黑名单中。
func (s *BlacklistService) Check(email string) (*model.Blacklist, error) {
	email = NormalizeEmail(email)
	if !ValidEmail(email) {
		return nil, ErrInvalidEmail
	}
	rec, err := s.repo.Check(email)
	if err != nil {
		return nil, err
	}
	if err := s.attachImages(rec, "/api/v1/image/"); err != nil {
		return nil, err
	}
	return rec, nil
}

// CheckSimilar 判断是否存在「账户名一致、域名不同」的不可信记录。
// 仅返回状态，不暴露任何相似邮箱数据。
func (s *BlacklistService) CheckSimilar(email string) (bool, error) {
	email = NormalizeEmail(email)
	if !ValidEmail(email) {
		return false, ErrInvalidEmail
	}
	return s.repo.HasSimilarAccount(email)
}

// Add 新增一条黑名单记录；若该邮箱已被标记，则更新覆盖为最新内容。
func (s *BlacklistService) Add(email, banReason, eventLink, relatedPeople, bannedAt string, domains []string, submissionID int64, ip, ua string) (*model.Blacklist, error) {
	email = NormalizeEmail(email)
	if !ValidEmail(email) {
		return nil, ErrInvalidEmail
	}
	if ReasonTooLong(banReason) {
		return nil, ErrReasonTooLong
	}

	eventLink = NormalizeLink(eventLink)
	if eventLink != "" {
		if err := ValidateLinkWithDomains(eventLink, domains); err != nil {
			return nil, err
		}
	}

	bannedAtParsed, err := ParseBannedAt(bannedAt, s.loc)
	if err != nil {
		return nil, err
	}

	// 已存在未删除记录时，以最新内容覆盖更新。
	if existing, err := s.repo.Check(email); err == nil {
		updated := &model.Blacklist{
			Email:              email,
			BanReason:          banReason,
			BanReasonRaw:       MarkdownToPlain(banReason),
			EventLink:          eventLink,
			EventRelatedPeople: relatedPeople,
			BannedAt:           bannedAtParsed,
			SubmissionID:       submissionID,
		}
		if err := s.repo.Update(existing.ID, updated); err != nil {
			return nil, err
		}
		updated.ID = existing.ID
		s.audit.Log("admin", ActionUpdate, email, ip, ua)
		return updated, nil
	} else if !errors.Is(err, repository.ErrNotFound) {
		return nil, err
	}

	rec := &model.Blacklist{
		Email:              email,
		BanReason:          banReason,
		BanReasonRaw:       MarkdownToPlain(banReason),
		EventLink:          eventLink,
		EventRelatedPeople: relatedPeople,
		BannedAt:           bannedAtParsed,
		CreatedBy:          "admin",
		CreatedAt:          NowStr(s.loc),
		SubmissionID:       submissionID,
	}
	if err := s.repo.Create(rec); err != nil {
		return nil, err
	}
	s.audit.Log("admin", ActionAdd, email, ip, ua)
	return rec, nil
}

// Update 按 ID 修改黑名单记录。
func (s *BlacklistService) Update(id int64, email, banReason, eventLink, relatedPeople, bannedAt string, domains []string, ip, ua string) (*model.Blacklist, error) {
	rec, err := s.repo.GetByID(id)
	if err != nil {
		return nil, err
	}
	if rec.DeletedAt != nil {
		return nil, ErrInTrash
	}

	email = NormalizeEmail(email)
	if !ValidEmail(email) {
		return nil, ErrInvalidEmail
	}
	if ReasonTooLong(banReason) {
		return nil, ErrReasonTooLong
	}
	exists, err := s.repo.ExistsEmail(email, id)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrEmailExists
	}

	eventLink = NormalizeLink(eventLink)
	if eventLink != "" {
		if err := ValidateLinkWithDomains(eventLink, domains); err != nil {
			return nil, err
		}
	}

	bannedAtParsed, err := ParseBannedAt(bannedAt, s.loc)
	if err != nil {
		return nil, err
	}

	updated := &model.Blacklist{
		Email:              email,
		BanReason:          banReason,
		BanReasonRaw:       MarkdownToPlain(banReason),
		EventLink:          eventLink,
		EventRelatedPeople: relatedPeople,
		BannedAt:           bannedAtParsed,
		SubmissionID:       rec.SubmissionID,
	}
	if err := s.repo.Update(id, updated); err != nil {
		return nil, err
	}
	s.audit.Log("admin", ActionUpdate, email, ip, ua)
	return updated, nil
}

// List 分页列表。deleted=true 表示回收站。
func (s *BlacklistService) List(query string, deleted bool, page, pageSize int) ([]*model.Blacklist, int, error) {
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * pageSize
	list, total, err := s.repo.List(query, offset, pageSize, deleted)
	if err != nil {
		return nil, 0, err
	}
	for _, b := range list {
		if err := s.attachImages(b, "/api/v1/image/"); err != nil {
			return nil, 0, err
		}
	}
	return list, total, nil
}

// attachImages 关联黑名单记录的证据图片元数据并拼接可加载的 URL。
func (s *BlacklistService) attachImages(b *model.Blacklist, urlPrefix string) error {
	images, err := s.repo.ListImages(b.SubmissionID)
	if err != nil {
		return err
	}
	for _, img := range images {
		img.URL = urlPrefix + img.FileHash + "." + img.Ext
	}
	b.Images = images
	return nil
}

// Delete 软删除（移入回收站）。
func (s *BlacklistService) Delete(id int64, ip, ua string) error {
	rec, err := s.repo.GetByID(id)
	if err != nil {
		return err
	}
	if err := s.repo.SoftDelete(id, NowStr(s.loc)); err != nil {
		return err
	}
	s.audit.Log("admin", ActionDelete, rec.Email, ip, ua)
	return nil
}

// Restore 从回收站恢复。
func (s *BlacklistService) Restore(id int64, ip, ua string) error {
	rec, err := s.repo.GetByID(id)
	if err != nil {
		return err
	}
	if err := s.repo.Restore(id); err != nil {
		return err
	}
	s.audit.Log("admin", ActionRestore, rec.Email, ip, ua)
	return nil
}

// PermanentDelete 永久删除。
func (s *BlacklistService) PermanentDelete(id int64, ip, ua string) error {
	rec, err := s.repo.GetByID(id)
	if err != nil {
		return err
	}
	if err := s.repo.PermanentDelete(id); err != nil {
		return err
	}
	s.audit.Log("admin", ActionDelete, rec.Email, ip, ua)
	return nil
}
