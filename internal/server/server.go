package server

import (
	"io/fs"
	"net/http"
	"path"
	"strings"

	"blacklist-index/internal/handler"
	"blacklist-index/internal/middleware"
)

// Server 组织路由与静态资源。
type Server struct {
	h             *handler.Handler
	dashboardPath string
	siteName      string
	assets        fs.FS
}

// New 构建完整的 HTTP Handler。
func New(h *handler.Handler, dashboardPath, siteName string, assets fs.FS) http.Handler {
	s := &Server{h: h, dashboardPath: dashboardPath, siteName: siteName, assets: assets}

	mux := http.NewServeMux()

	// —— 公开接口 ——
	mux.HandleFunc("GET /health", h.Health)
	mux.HandleFunc("GET /api/v1/check", h.Check)
	mux.HandleFunc("GET /api/v1/announcement", h.Announcement)
	mux.HandleFunc("GET /api/v1/site-config", h.SiteConfig)
	mux.HandleFunc("POST /api/v1/admin/login", h.Login)
	mux.HandleFunc("GET /api/v1/admin/status", h.Status)
	mux.HandleFunc("POST /api/v1/submit/report", h.SubmitReport)
	mux.HandleFunc("POST /api/v1/submit/appeal", h.SubmitAppeal)
	mux.HandleFunc("GET /api/v1/submission", h.QuerySubmission)
	mux.HandleFunc("GET /api/v1/image/{file}", h.ServeImage)

	// —— 需登录接口 ——
	auth := func(next http.HandlerFunc) http.HandlerFunc {
		return middleware.RequireAuth(h.JWTSecret(), next)
	}
	superAuth := func(next http.HandlerFunc) http.HandlerFunc {
		return middleware.RequireSuper(h.JWTSecret(), next)
	}
	mux.HandleFunc("GET /api/v1/admin/image/{file}", auth(h.ServeImage))
	mux.HandleFunc("POST /api/v1/admin/logout", auth(h.Logout))
	mux.HandleFunc("GET /api/v1/admin/list", auth(h.ListBlacklist))
	mux.HandleFunc("POST /api/v1/admin/add", auth(h.AddBlacklist))
	mux.HandleFunc("PUT /api/v1/admin/update/{id}", auth(h.UpdateBlacklist))
	mux.HandleFunc("DELETE /api/v1/admin/delete/{id}", auth(h.DeleteBlacklist))
	mux.HandleFunc("POST /api/v1/admin/restore/{id}", auth(h.RestoreBlacklist))
	mux.HandleFunc("DELETE /api/v1/admin/permanent/{id}", auth(h.PermanentDeleteBlacklist))
	mux.HandleFunc("PUT /api/v1/admin/announcement", auth(h.UpdateAnnouncement))
	mux.HandleFunc("GET /api/v1/admin/announcement", auth(h.GetAnnouncement))
	mux.HandleFunc("GET /api/v1/admin/audit-logs", auth(h.AuditLogs))
	mux.HandleFunc("POST /api/v1/admin/password", auth(h.ChangePassword))
	mux.HandleFunc("GET /api/v1/admin/settings", auth(h.GetSettings))
	mux.HandleFunc("PUT /api/v1/admin/settings", superAuth(h.SaveSettings))
	mux.HandleFunc("GET /api/v1/admin/submissions", auth(h.ListSubmissions))
	mux.HandleFunc("POST /api/v1/admin/submissions/{id}/approve", auth(h.ApproveSubmission))
	mux.HandleFunc("POST /api/v1/admin/submissions/{id}/reject", auth(h.RejectSubmission))
	mux.HandleFunc("POST /api/v1/admin/cleanup-images", superAuth(h.CleanupImages))

	// —— 超级管理员：用户管理 ——
	mux.HandleFunc("GET /api/v1/admin/users", superAuth(h.ListAdmins))
	mux.HandleFunc("POST /api/v1/admin/users", superAuth(h.CreateAdmin))
	mux.HandleFunc("GET /api/v1/admin/users/{id}/totp", superAuth(h.GetAdminTOTP))
	mux.HandleFunc("POST /api/v1/admin/users/{id}/password", superAuth(h.ResetAdminPassword))
	mux.HandleFunc("POST /api/v1/admin/users/{id}/totp", superAuth(h.ResetAdminTOTP))
	mux.HandleFunc("DELETE /api/v1/admin/users/{id}", superAuth(h.DeleteAdmin))

	// —— 静态页面 ——
	mux.HandleFunc("GET /{$}", s.serveIndex)
	mux.HandleFunc("GET /assets/", s.serveAssets)
	mux.HandleFunc("GET /favicon.ico", s.serveFavicon)
	mux.HandleFunc("GET /"+dashboardPath, s.serveAdmin)
	mux.HandleFunc("GET /"+dashboardPath+"/", s.serveAdmin)

	// 未匹配的路径（404 等）统一重定向回首页。
	mux.HandleFunc("/", s.redirectHome)

	var root http.Handler = mux
	root = middleware.Recover(root)
	root = middleware.Logging(root)
	return root
}

func (s *Server) serveIndex(w http.ResponseWriter, r *http.Request) {
	s.serveHTML(w, r, "index.html")
}

// redirectHome 将未匹配的路径重定向回首页。
func (s *Server) redirectHome(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/", http.StatusFound)
}

func (s *Server) serveAdmin(w http.ResponseWriter, r *http.Request) {
	s.serveHTML(w, r, "admin.html")
}

func (s *Server) serveHTML(w http.ResponseWriter, r *http.Request, name string) {
	data, err := fs.ReadFile(s.assets, name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	html := string(data)
	html = strings.ReplaceAll(html, "{{SITE_NAME}}", htmlEscape(s.siteName))
	html = strings.ReplaceAll(html, "{{DASHBOARD_PATH}}", s.dashboardPath)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write([]byte(html))
}

func htmlEscape(s string) string {
	repl := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&#34;", "'", "&#39;")
	return repl.Replace(s)
}

func (s *Server) serveAssets(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/assets/")
	if name == "" || !fs.ValidPath(name) {
		http.NotFound(w, r)
		return
	}
	data, err := fs.ReadFile(s.assets, "assets/"+name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", contentType(name))
	w.Header().Set("Cache-Control", "public, max-age=3600")
	_, _ = w.Write(data)
}

func (s *Server) serveFavicon(w http.ResponseWriter, r *http.Request) {
	data, err := fs.ReadFile(s.assets, "assets/favicon.ico")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "image/x-icon")
	_, _ = w.Write(data)
}

func contentType(name string) string {
	switch strings.ToLower(path.Ext(name)) {
	case ".css":
		return "text/css; charset=utf-8"
	case ".js":
		return "application/javascript; charset=utf-8"
	case ".svg":
		return "image/svg+xml"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".ico":
		return "image/x-icon"
	case ".woff2":
		return "font/woff2"
	case ".html":
		return "text/html; charset=utf-8"
	default:
		return "application/octet-stream"
	}
}
