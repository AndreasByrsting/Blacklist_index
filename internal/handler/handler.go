package handler

import (
	"database/sql"
	"time"

	"blacklist-index/internal/config"
	"blacklist-index/internal/service"
)

// Handler 聚合所有服务，对外提供 HTTP 处理方法。
type Handler struct {
	cfg          *config.Config
	db           *sql.DB
	blacklist    *service.BlacklistService
	auth         *service.AuthService
	announcement *service.AnnouncementService
	audit        *service.AuditService
	jwtSecret    []byte
	started      time.Time
}

// New 构造 Handler。
func New(cfg *config.Config, db *sql.DB, blacklist *service.BlacklistService, auth *service.AuthService,
	announcement *service.AnnouncementService, audit *service.AuditService, jwtSecret []byte) *Handler {
	return &Handler{
		cfg:          cfg,
		db:           db,
		blacklist:    blacklist,
		auth:         auth,
		announcement: announcement,
		audit:        audit,
		jwtSecret:    jwtSecret,
		started:      time.Now(),
	}
}

// JWTSecret 暴露 JWT 密钥供中间件使用。
func (h *Handler) JWTSecret() []byte { return h.jwtSecret }
