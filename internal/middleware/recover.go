package middleware

import (
	"log/slog"
	"net/http"
	"runtime/debug"
)

// Recover 捕获 panic，记录堆栈并返回 500。
func Recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				slog.Error("panic recovered", "error", rec, "stack", string(debug.Stack()))
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"success":false,"message":"服务器内部错误"}`))
			}
		}()
		next.ServeHTTP(w, r)
	})
}
