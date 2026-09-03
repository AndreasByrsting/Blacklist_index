package handler

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"blacklist-index/internal/middleware"
	"blacklist-index/internal/service"
)

const cookieName = "token"

var sessionTTL = 8 * time.Hour

type loginRequest struct {
	Username string `json:"username"`
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

	token, admin, err := h.auth.Login(req.Username, req.Password, req.TOTP, ip, ua)
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
	writeSuccess(w, map[string]any{
		"username":  admin.Username,
		"is_super":  admin.IsSuper,
	})
}

// Logout 管理员退出登录。
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	claims := middleware.ClaimsFrom(r)
	username := ""
	if claims != nil {
		username = claims.Username
	}
	h.auth.Logout(username, clientIP(r), r.UserAgent())
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
		writeJSON(w, http.StatusOK, map[string]any{"logged_in": false, "username": "", "is_super": false})
		return
	}
	claims, err := service.ParseJWT(c.Value, h.jwtSecret)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"logged_in": false, "username": "", "is_super": false})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"logged_in": true,
		"username":  claims.Username,
		"is_super":  claims.IsSuper,
	})
}

// currentUser 从请求中提取当前登录用户名。
func (h *Handler) currentUser(r *http.Request) string {
	claims := middleware.ClaimsFrom(r)
	if claims == nil {
		return ""
	}
	return claims.Username
}

// isSuper 从请求中判断当前是否为超级管理员。
func (h *Handler) isSuper(r *http.Request) bool {
	claims := middleware.ClaimsFrom(r)
	return claims != nil && claims.IsSuper
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

// UpdateBlacklist 按 ID 修改黑名单记录。
func (h *Handler) UpdateBlacklist(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var req addRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	rec, err := h.blacklist.Update(id, req.Email, req.BanReason, req.EventLink, req.EventRelatedPeople, req.BannedAt, clientIP(r), r.UserAgent())
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
	if err := h.announcement.Save(req.Content, h.currentUser(r)); err != nil {
		h.writeError(w, err)
		return
	}
	h.audit.Log(h.currentUser(r), service.ActionEditAnnouncement, "", clientIP(r), r.UserAgent())
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

type changePasswordRequest struct {
	OldPassword string `json:"old_password"`
	NewPassword string `json:"new_password"`
}

// ChangePassword 修改管理员自己的密码。
func (h *Handler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	var req changePasswordRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := h.auth.ChangePassword(h.currentUser(r), req.OldPassword, req.NewPassword, clientIP(r), r.UserAgent()); err != nil {
		h.writeError(w, err)
		return
	}
	writeSuccess(w, nil)
}

type inboxRequest struct {
	InboxEmail string `json:"inbox_email"`
}

// GetSettings 获取站点设置（收件箱等）。
func (h *Handler) GetSettings(w http.ResponseWriter, r *http.Request) {
	inbox, err := h.setting.GetInbox()
	if err != nil {
		h.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"inbox_email": inbox})
}

// SaveSettings 保存站点设置。
func (h *Handler) SaveSettings(w http.ResponseWriter, r *http.Request) {
	var req inboxRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := h.setting.SetInbox(req.InboxEmail, clientIP(r), r.UserAgent()); err != nil {
		h.writeError(w, err)
		return
	}
	writeSuccess(w, nil)
}

// ========== 用户管理（仅超级管理员）==========

// ListAdmins 列出所有管理员。
func (h *Handler) ListAdmins(w http.ResponseWriter, r *http.Request) {
	list, err := h.auth.ListAdmins()
	if err != nil {
		h.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"list": list})
}

type createAdminRequest struct {
	Username string `json:"username"`
	IsSuper  bool   `json:"is_super"`
}

// CreateAdmin 新增管理员（仅超级管理员）。
func (h *Handler) CreateAdmin(w http.ResponseWriter, r *http.Request) {
	var req createAdminRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	password, totpSecret, err := h.auth.CreateAdmin(h.currentUser(r), req.Username, req.IsSuper, clientIP(r), r.UserAgent())
	if err != nil {
		h.writeError(w, err)
		return
	}
	writeSuccess(w, map[string]any{
		"password":    password,
		"totp_secret": totpSecret,
		"totp_setup_url":   service.TOTPSetupURL(totpSecret, h.cfg.SiteName, req.Username),
	})
}

// ResetAdminPassword 重置指定管理员密码（仅超级管理员）。
func (h *Handler) ResetAdminPassword(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	password, err := h.auth.ResetPassword(h.currentUser(r), id, clientIP(r), r.UserAgent())
	if err != nil {
		h.writeError(w, err)
		return
	}
	writeSuccess(w, map[string]any{"password": password})
}

// GetAdminTOTP 查看指定管理员当前 TOTP 密钥（仅超级管理员）。
func (h *Handler) GetAdminTOTP(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	admin, err := h.auth.GetAdminByID(id)
	if err != nil {
		h.writeError(w, err)
		return
	}
	writeSuccess(w, map[string]any{
		"totp_secret": admin.TOTPSecret,
		"totp_setup_url":   service.TOTPSetupURL(admin.TOTPSecret, h.cfg.SiteName, admin.Username),
	})
}

// ResetAdminTOTP 重置指定管理员 TOTP 密钥（仅超级管理员）。
func (h *Handler) ResetAdminTOTP(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	secret, err := h.auth.ResetTOTP(h.currentUser(r), id, clientIP(r), r.UserAgent())
	if err != nil {
		h.writeError(w, err)
		return
	}
	// 需要用户名生成 setup URL
	admin, err := h.auth.GetAdminByID(id)
	if err != nil {
		h.writeError(w, err)
		return
	}
	writeSuccess(w, map[string]any{
		"totp_secret": secret,
		"totp_setup_url":   service.TOTPSetupURL(secret, h.cfg.SiteName, admin.Username),
	})
}

// DeleteAdmin 删除管理员（仅超级管理员）。
func (h *Handler) DeleteAdmin(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if err := h.auth.DeleteAdmin(h.currentUser(r), id, clientIP(r), r.UserAgent()); err != nil {
		h.writeError(w, err)
		return
	}
	writeSuccess(w, nil)
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
