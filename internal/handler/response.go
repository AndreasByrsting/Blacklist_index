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
		status, msg = http.StatusConflict, "该邮箱已在黑名单中"
	case errors.Is(err, service.ErrBadCredentials):
		status, msg = http.StatusUnauthorized, "用户名或动态口令错误"
	case errors.Is(err, service.ErrTooManyAttempts):
		status, msg = http.StatusTooManyRequests, "尝试次数过多，请 15 分钟后再试"
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
