package handler

import (
	"encoding/json"
	"errors"
	"log"
	"net"
	"net/http"
	"strings"

	"blacklist-index/internal/repository"
	"blacklist-index/internal/service"
)

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeSuccess(w http.ResponseWriter, extra map[string]any) {
	out := map[string]any{"success": true}
	for k, v := range extra {
		out[k] = v
	}
	writeJSON(w, http.StatusOK, out)
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	if err := dec.Decode(dst); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"success": false, "message": "请求体格式错误"})
		return false
	}
	return true
}

// writeError 将业务错误映射为 HTTP 状态码与统一提示。
func (h *Handler) writeError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	msg := "服务器内部错误"

	switch {
	case errors.Is(err, service.ErrInvalidEmail):
		status, msg = http.StatusBadRequest, "请输入有效的邮箱地址"
	case errors.Is(err, service.ErrEmailExists):
		status, msg = http.StatusConflict, "该邮箱已被标记为不可信"
	case errors.Is(err, service.ErrReasonTooLong):
		status, msg = http.StatusBadRequest, err.Error()
	case errors.Is(err, service.ErrReportReasonTooLong):
		status, msg = http.StatusBadRequest, err.Error()
	case errors.Is(err, service.ErrAppealReasonTooLong):
		status, msg = http.StatusBadRequest, err.Error()
	case errors.Is(err, service.ErrRejectReasonTooLong):
		status, msg = http.StatusBadRequest, err.Error()
	case errors.Is(err, service.ErrEventLinkRequired):
		status, msg = http.StatusBadRequest, err.Error()
	case errors.Is(err, service.ErrInTrash):
		status, msg = http.StatusBadRequest, err.Error()
	case errors.Is(err, service.ErrBadCredentials):
		status, msg = http.StatusUnauthorized, "用户名或动态口令错误"
	case errors.Is(err, service.ErrWrongPassword):
		status, msg = http.StatusBadRequest, "当前密码错误"
	case errors.Is(err, service.ErrWeakPassword):
		status, msg = http.StatusBadRequest, "新密码需至少 8 位，且同时包含大写字母、小写字母和数字"
	case errors.Is(err, service.ErrInvalidUsername):
		status, msg = http.StatusBadRequest, err.Error()
	case errors.Is(err, service.ErrUsernameExists):
		status, msg = http.StatusConflict, err.Error()
	case errors.Is(err, service.ErrCannotDeleteSelf):
		status, msg = http.StatusBadRequest, err.Error()
	case errors.Is(err, service.ErrNeedAtLeastOneSuper):
		status, msg = http.StatusBadRequest, err.Error()
	case errors.Is(err, service.ErrTooManyAttempts):
		status, msg = http.StatusTooManyRequests, "尝试次数过多，请 15 分钟后再试"
	case errors.Is(err, service.ErrSubmissionNotFound):
		status, msg = http.StatusNotFound, "申请不存在，请检查查询码"
	case errors.Is(err, service.ErrSubmissionNotPending):
		status, msg = http.StatusBadRequest, "该申请已处理，无法重复审核"
	case errors.Is(err, service.ErrReasonRequired):
		status, msg = http.StatusBadRequest, "请填写标记原因"
	case errors.Is(err, service.ErrEventLinkRequired):
		status, msg = http.StatusBadRequest, err.Error()
	case errors.Is(err, service.ErrAppealReasonRequired):
		status, msg = http.StatusBadRequest, "请填写申诉理由"
	case errors.Is(err, service.ErrAppealEvidenceRequired):
		status, msg = http.StatusBadRequest, "反驳证据链接为必填项，请提供有效链接"
	case errors.Is(err, service.ErrRejectReasonRequired):
		status, msg = http.StatusBadRequest, "请填写驳回原因"
	case errors.Is(err, service.ErrLinkHTTPSRequired):
		status, msg = http.StatusBadRequest, "链接必须使用 HTTPS 协议"
	case errors.Is(err, service.ErrLinkDomainUnsupported):
		status, msg = http.StatusBadRequest, "为了用户安全暂不支持该网站，仅支持 tieba.baidu.com，后续将逐步适配"
	case errors.Is(err, repository.ErrNotFound):
		status, msg = http.StatusNotFound, "记录不存在"
	default:
		log.Printf("internal error: %v", err)
	}
	writeJSON(w, status, map[string]any{"success": false, "message": msg})
}

// clientIP 返回客户端 IP，优先取反向代理头。
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if first := strings.TrimSpace(strings.Split(xff, ",")[0]); first != "" {
			return first
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
