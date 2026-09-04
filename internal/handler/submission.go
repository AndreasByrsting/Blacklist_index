package handler

import (
	"io"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"

	"blacklist-index/internal/service"
)

// —— 公开接口 ——

// SubmitReport 用户提交举报（标记为不可信申请）。
func (h *Handler) SubmitReport(w http.ResponseWriter, r *http.Request) {
	fields, datas, ok := h.readSubmissionForm(w, r)
	if !ok {
		return
	}
	policy := h.setting.GetReportLinkConfig()
	imgPolicy := h.setting.GetReportImageConfig()
	code, err := h.submission.SubmitReport(
		fields["email"], fields["ban_reason"], fields["event_link"], fields["event_related_people"],
		policy, imgPolicy, datas, clientIP(r), r.UserAgent(),
	)
	if err != nil {
		h.writeError(w, err)
		return
	}
	writeSuccess(w, map[string]any{"query_code": code})
}

// SubmitAppeal 用户提交申诉。
func (h *Handler) SubmitAppeal(w http.ResponseWriter, r *http.Request) {
	fields, datas, ok := h.readSubmissionForm(w, r)
	if !ok {
		return
	}
	policy := h.setting.GetAppealLinkConfig()
	imgPolicy := h.setting.GetAppealImageConfig()
	code, err := h.submission.SubmitAppeal(
		fields["email"], fields["appeal_reason"], fields["appeal_evidence"],
		policy, imgPolicy, datas, clientIP(r), r.UserAgent(),
	)
	if err != nil {
		h.writeError(w, err)
		return
	}
	writeSuccess(w, map[string]any{"query_code": code})
}

// readSubmissionForm 解析 multipart 表单，返回文本字段与图片原始字节。
// 解析或文件校验失败时直接写出错误响应并返回 ok=false。
func (h *Handler) readSubmissionForm(w http.ResponseWriter, r *http.Request) (map[string]string, [][]byte, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, 32<<20)
	if err := r.ParseMultipartForm(16 << 20); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"success": false, "message": "上传内容过大或格式错误"})
		return nil, nil, false
	}

	fields := make(map[string]string)
	for _, key := range []string{"email", "ban_reason", "event_link", "event_related_people", "appeal_reason", "appeal_evidence"} {
		if vs := r.MultipartForm.Value[key]; len(vs) > 0 {
			fields[key] = strings.TrimSpace(vs[0])
		}
	}

	var datas [][]byte
	for _, fh := range r.MultipartForm.File["images"] {
		if fh.Size > service.MaxImageSize {
			writeJSON(w, http.StatusBadRequest, map[string]any{"success": false, "message": "单张图片大小不能超过 5MB"})
			return nil, nil, false
		}
		f, err := fh.Open()
		if err != nil {
			continue
		}
		data, err := io.ReadAll(io.LimitReader(f, service.MaxImageSize+1))
		f.Close()
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"success": false, "message": "图片读取失败"})
			return nil, nil, false
		}
		if len(data) > service.MaxImageSize {
			writeJSON(w, http.StatusBadRequest, map[string]any{"success": false, "message": "单张图片大小不能超过 5MB"})
			return nil, nil, false
		}
		datas = append(datas, data)
	}

	return fields, datas, true
}

// ServeImage 后台加载证据图片（需登录）。
func (h *Handler) ServeImage(w http.ResponseWriter, r *http.Request) {
	file := r.PathValue("file")
	if file == "" {
		http.NotFound(w, r)
		return
	}
	ext := strings.ToLower(path.Ext(file))
	if ext == "" {
		http.NotFound(w, r)
		return
	}
	hash := strings.TrimSuffix(file, ext)
	ext = strings.TrimPrefix(ext, ".")
	if len(hash) != 64 || !isHexHash(hash) || !service.IsImageExtAllowed(ext) {
		http.NotFound(w, r)
		return
	}

	data, err := os.ReadFile(h.image.Path(hash, ext))
	if err != nil {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", imageContentType(ext))
	w.Header().Set("Cache-Control", "private, max-age=3600")
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	_, _ = w.Write(data)
}

func isHexHash(s string) bool {
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}
	return true
}

func imageContentType(ext string) string {
	switch ext {
	case "jpg", "jpeg":
		return "image/jpeg"
	case "png":
		return "image/png"
	case "webp":
		return "image/webp"
	default:
		return "application/octet-stream"
	}
}

// QuerySubmission 用户按查询码查询申请状态。
func (h *Handler) QuerySubmission(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	if code == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"success": false, "message": "请输入查询码"})
		return
	}
	sub, err := h.submission.GetByQueryCode(code)
	if err != nil {
		h.writeError(w, err)
		return
	}
	writeSuccess(w, map[string]any{
		"id":              sub.ID,
		"type":            sub.Type,
		"email":           sub.Email,
		"status":          sub.Status,
		"ban_reason":      sub.BanReason,
		"event_link":      sub.EventLink,
		"related_people":  sub.EventRelatedPeople,
		"appeal_reason":   sub.AppealReason,
		"appeal_evidence": sub.AppealEvidence,
		"reject_reason":   sub.RejectReason,
		"created_at":      sub.CreatedAt,
		"reviewed_at":     sub.ReviewedAt,
	})
}

// —— 管理员接口 ——

// ListSubmissions 分页列出申请（后台审核页）。
func (h *Handler) ListSubmissions(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	status := q.Get("status")
	typ := q.Get("type")
	page := intParam(q.Get("page"), 1)
	pageSize := clampInt(intParam(q.Get("page_size"), 20), 1, 100)

	list, total, err := h.submission.List(status, typ, page, pageSize)
	if err != nil {
		h.writeError(w, err)
		return
	}
	writeSuccess(w, map[string]any{"total": total, "list": list, "page": page, "page_size": pageSize})
}

type rejectRequest struct {
	Reason string `json:"reason"`
}

// ApproveSubmission 通过申请。
func (h *Handler) ApproveSubmission(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	domains := h.setting.GetReportLinkConfig().Domains
	sub, err := h.submission.Approve(id, h.currentUser(r), domains, clientIP(r), r.UserAgent())
	if err != nil {
		h.writeError(w, err)
		return
	}
	writeSuccess(w, map[string]any{"submission": sub})
}

// RejectSubmission 驳回申请。
func (h *Handler) RejectSubmission(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var req rejectRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	sub, err := h.submission.Reject(id, req.Reason, h.currentUser(r), clientIP(r), r.UserAgent())
	if err != nil {
		h.writeError(w, err)
		return
	}
	writeSuccess(w, map[string]any{"submission": sub})
}
