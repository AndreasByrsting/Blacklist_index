package handler

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"blacklist-index/internal/service"
)

const cookieName = "token"

var sessionTTL = 8 * time.Hour

type loginRequest struct {
	Password string `json:"password"`
	TOTP     string `json:"totp"`
}

// Login 管理员登录。
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	ip := clientIP(r)
	ua := r.UserAgent()

	token, err := h.auth.Login(req.Password, req.TOTP, ip, ua)
	if err != nil {
		h.writeError(w, err)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   int(sessionTTL.Seconds()),
		HttpOnly: true,
		Secure:   secureCookie(r),
		SameSite: http.SameSiteLaxMode,
	})
	writeSuccess(w, nil)
}

// Logout 管理员退出登录。
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	h.auth.Logout(clientIP(r), r.UserAgent())
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   secureCookie(r),
		SameSite: http.SameSiteLaxMode,
	})
	writeSuccess(w, nil)
}

// secureCookie 在 HTTPS 连接或反向代理注入 https 时返回 true，从而启用 Secure 标记。
func secureCookie(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	return strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}

// Status 查询当前登录状态。
func (h *Handler) Status(w http.ResponseWriter, r *http.Request) {
	c, err := r.Cookie(cookieName)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"logged_in": false, "username": ""})
		return
	}
	claims, err := service.ParseJWT(c.Value, h.jwtSecret)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"logged_in": false, "username": ""})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"logged_in": true, "username": claims.Username})
}

// ListBlacklist 分页列出黑名单（含回收站筛选）。
func (h *Handler) ListBlacklist(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page := intParam(q.Get("page"), 1)
	pageSize := clampInt(intParam(q.Get("page_size"), 10), 1, 100)
	deleted := q.Get("deleted") == "1" || strings.EqualFold(q.Get("deleted"), "true")

	list, total, err := h.blacklist.List(q.Get("q"), deleted, page, pageSize)
	if err != nil {
		h.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"total": total, "list": list})
}

type addRequest struct {
	Email              string `json:"email"`
	BanReason          string `json:"ban_reason"`
	EventLink          string `json:"event_link"`
	EventRelatedPeople string `json:"event_related_people"`
	BannedAt           string `json:"banned_at"`
}

// AddBlacklist 新增黑名单记录。
func (h *Handler) AddBlacklist(w http.ResponseWriter, r *http.Request) {
	var req addRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	rec, err := h.blacklist.Add(req.Email, req.BanReason, req.EventLink, req.EventRelatedPeople, req.BannedAt, clientIP(r), r.UserAgent())
	if err != nil {
		h.writeError(w, err)
		return
	}
	writeSuccess(w, map[string]any{"email": rec.Email})
}

// DeleteBlacklist 软删除（移入回收站）。
func (h *Handler) DeleteBlacklist(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if err := h.blacklist.Delete(id, clientIP(r), r.UserAgent()); err != nil {
		h.writeError(w, err)
		return
	}
	writeSuccess(w, nil)
}

// RestoreBlacklist 从回收站恢复。
func (h *Handler) RestoreBlacklist(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if err := h.blacklist.Restore(id, clientIP(r), r.UserAgent()); err != nil {
		h.writeError(w, err)
		return
	}
	writeSuccess(w, nil)
}

// PermanentDeleteBlacklist 永久删除。
func (h *Handler) PermanentDeleteBlacklist(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if err := h.blacklist.PermanentDelete(id, clientIP(r), r.UserAgent()); err != nil {
		h.writeError(w, err)
		return
	}
	writeSuccess(w, nil)
}

type announcementRequest struct {
	Content string `json:"content"`
}

// UpdateAnnouncement 保存公告。
func (h *Handler) UpdateAnnouncement(w http.ResponseWriter, r *http.Request) {
	var req announcementRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := h.announcement.Save(req.Content, "admin"); err != nil {
		h.writeError(w, err)
		return
	}
	h.audit.Log("admin", service.ActionEditAnnouncement, "", clientIP(r), r.UserAgent())
	writeSuccess(w, nil)
}

// GetAnnouncement 获取公告原始 Markdown 内容（后台编辑预填）。
func (h *Handler) GetAnnouncement(w http.ResponseWriter, r *http.Request) {
	a, err := h.announcement.GetActive()
	if err != nil {
		h.writeError(w, err)
		return
	}
	content := ""
	if a != nil {
		content = a.Content
	}
	writeJSON(w, http.StatusOK, map[string]any{"content": content})
}

// AuditLogs 分页查询审计日志。
func (h *Handler) AuditLogs(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page := intParam(q.Get("page"), 1)
	pageSize := clampInt(intParam(q.Get("page_size"), 20), 1, 100)

	list, total, err := h.audit.List(q.Get("action"), page, pageSize)
	if err != nil {
		h.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"total": total, "list": list})
}

// —— 辅助函数 ——

func pathID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"success": false, "message": "无效的 ID"})
		return 0, false
	}
	return id, true
}

func intParam(s string, def int) int {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return def
	}
	return n
}

func clampInt(n, min, max int) int {
	if n < min {
		return min
	}
	if n > max {
		return max
	}
	return n
}
