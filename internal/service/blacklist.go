package service

import (
	"errors"
	"time"

	"blacklist-index/internal/model"
	"blacklist-index/internal/repository"
)

var (
	// ErrEmailExists 表示邮箱已存在于黑名单。
	ErrEmailExists = errors.New("该邮箱已在黑名单中")
	// ErrInvalidEmail 表示邮箱格式非法。
	ErrInvalidEmail = errors.New("请输入有效的邮箱地址")
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
	return s.repo.Check(email)
}

// Add 新增一条黑名单记录。
func (s *BlacklistService) Add(email, banReason, eventLink, relatedPeople, bannedAt, ip, ua string) (*model.Blacklist, error) {
	email = NormalizeEmail(email)
	if !ValidEmail(email) {
		return nil, ErrInvalidEmail
	}
	if _, err := s.repo.Check(email); err == nil {
		return nil, ErrEmailExists
	} else if !errors.Is(err, repository.ErrNotFound) {
		return nil, err
	}

	bannedAtParsed, err := ParseBannedAt(bannedAt, s.loc)
	if err != nil {
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
	}
	if err := s.repo.Create(rec); err != nil {
		return nil, err
	}
	s.audit.Log("admin", ActionAdd, email, ip, ua)
	return rec, nil
}

// List 分页列表。deleted=true 表示回收站。
func (s *BlacklistService) List(query string, deleted bool, page, pageSize int) ([]*model.Blacklist, int, error) {
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * pageSize
	return s.repo.List(query, offset, pageSize, deleted)
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
