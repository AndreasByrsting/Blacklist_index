package middleware

import (
	"net/http"

	"blacklist-index/internal/service"
)

// RequireAuth 校验 JWT Cookie，失败返回 401。
func RequireAuth(jwtSecret []byte, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie("token")
		if err != nil || c.Value == "" {
			writeUnauthorized(w)
			return
		}
		if _, err := service.ParseJWT(c.Value, jwtSecret); err != nil {
			writeUnauthorized(w)
			return
		}
		next(w, r)
	}
}

func writeUnauthorized(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusUnauthorized)
	_, _ = w.Write([]byte(`{"success":false,"message":"未登录或登录已过期"}`))
}
