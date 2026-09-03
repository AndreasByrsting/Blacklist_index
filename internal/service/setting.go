package service

import (
	"errors"

	"blacklist-index/internal/repository"
)

// ConfigKeyInbox 是收件箱邮箱在 app_config 表中的键名。
const ConfigKeyInbox = "inbox_email"

// SettingService 负责动态站点设置（收件箱等）的读写。
type SettingService struct {
	repo  *repository.ConfigRepo
	audit *AuditService
}

func NewSettingService(repo *repository.ConfigRepo, audit *AuditService) *SettingService {
	return &SettingService{repo: repo, audit: audit}
}

// GetInbox 返回收件箱邮箱，未配置时返回空字符串。
func (s *SettingService) GetInbox() (string, error) {
	v, err := s.repo.Get(ConfigKeyInbox)
	if errors.Is(err, repository.ErrNotFound) {
		return "", nil
	}
	return v, err
}

// SetInbox 保存收件箱邮箱（空字符串表示清除，隐藏首页提报按钮）。
func (s *SettingService) SetInbox(email, ip, ua string) error {
	email = NormalizeEmail(email)
	if email != "" && !ValidEmail(email) {
		return ErrInvalidEmail
	}
	if err := s.repo.Set(ConfigKeyInbox, email); err != nil {
		return err
	}
	s.audit.Log("admin", ActionEditSettings, email, ip, ua)
	return nil
}
