package service

import (
	"log"
	"sync"
	"time"

	"blacklist-index/internal/model"
	"blacklist-index/internal/repository"
)

// 审计操作类型常量（存储为英文编码，前端展示时映射为中文）。
const (
	ActionLogin            = "login"
	ActionLogout           = "logout"
	ActionAdd              = "add"
	ActionUpdate           = "update"
	ActionDelete           = "delete"
	ActionRestore          = "restore"
	ActionEditAnnouncement = "edit_announcement"
	ActionEditSettings     = "edit_settings"
	ActionChangePassword   = "change_password"
	ActionCreateAdmin      = "create_admin"
	ActionResetPassword    = "reset_password"
	ActionResetTOTP        = "reset_totp"
	ActionDeleteAdmin      = "delete_admin"
	ActionApproveSub       = "approve_submission"
	ActionRejectSub        = "reject_submission"
	ActionCleanupImages    = "cleanup_images"
)

// AuditService 负责审计日志的写入与查询。
type AuditService struct {
	repo    *repository.AuditRepo
	loc     *time.Location
	mu      sync.Mutex
	counter int
}

func NewAuditService(repo *repository.AuditRepo, loc *time.Location) *AuditService {
	return &AuditService{repo: repo, loc: loc}
}

// Log 写入一条审计日志（尽力而为，失败仅记录日志而不中断请求）。
func (s *AuditService) Log(user, action, target, ip, ua string) {
	a := &model.AuditLog{
		User:      user,
		Action:    action,
		Target:    target,
		IP:        ip,
		UserAgent: ua,
		CreatedAt: NowStr(s.loc),
	}
	if err := s.repo.Log(a); err != nil {
		log.Printf("audit log write failed: %v", err)
		return
	}

	// 每 100 条执行一次保留策略，把总量控制在 10000 条附近。
	s.mu.Lock()
	s.counter++
	if s.counter%100 == 0 {
		if err := s.repo.Prune(); err != nil {
			log.Printf("audit log prune failed: %v", err)
		}
	}
	s.mu.Unlock()
}

// List 分页查询审计日志，action 为空表示全部。
func (s *AuditService) List(action string, page, pageSize int) ([]*model.AuditLog, int, error) {
	offset := (page - 1) * pageSize
	return s.repo.List(action, offset, pageSize)
}
