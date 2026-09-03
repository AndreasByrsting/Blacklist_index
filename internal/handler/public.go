package handler

import (
	"errors"
	"net/http"
	"time"

	"blacklist-index/internal/repository"
	"blacklist-index/internal/service"
)

// Health 健康检查接口。
func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	dbStatus := "up"
	if err := h.db.PingContext(r.Context()); err != nil {
		dbStatus = "down"
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"db":     dbStatus,
		"uptime": int64(time.Since(h.started).Seconds()),
	})
}

// Check 公开黑名单查询。
func (h *Handler) Check(w http.ResponseWriter, r *http.Request) {
	if !h.queryLimiter.Allow(clientIP(r)) {
		writeJSON(w, http.StatusTooManyRequests, map[string]any{"blocked": false, "message": "查询过于频繁，请 5 分钟后再试"})
		return
	}
	email := r.URL.Query().Get("email")
	rec, err := h.blacklist.Check(email)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidEmail):
			writeJSON(w, http.StatusBadRequest, map[string]any{"blocked": false, "message": "请输入有效的邮箱地址"})
		case errors.Is(err, repository.ErrNotFound):
			// 未精确命中时，检查是否存在同名账户（不同域名）的不可信记录，仅返回状态。
			similar, serr := h.blacklist.CheckSimilar(email)
			if serr != nil {
				h.writeError(w, serr)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"blocked": false, "similar": similar})
		default:
			h.writeError(w, err)
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"blocked":             true,
		"reason_html":         service.RenderMarkdown(rec.BanReason),
		"event_link":          rec.EventLink,
		"related_people":      rec.EventRelatedPeople,
		"related_people_list": service.SplitRelatedPeople(rec.EventRelatedPeople),
		"banned_at":           rec.BannedAt,
	})
}

// SiteConfig 公开站点配置（用于首页提报按钮等展示逻辑）。
func (h *Handler) SiteConfig(w http.ResponseWriter, r *http.Request) {
	inbox, err := h.setting.GetInbox()
	if err != nil {
		h.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"inbox_email": inbox})
}

// Announcement 公开公告查询。
func (h *Handler) Announcement(w http.ResponseWriter, r *http.Request) {
	htmlContent, err := h.announcement.GetActiveHTML()
	if err != nil {
		h.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"content_html": htmlContent})
}
