package middleware

import (
	"context"
	"net/http"

	"blacklist-index/internal/service"
)

type ctxKey string

const (
	claimsKey ctxKey = "claims"
)

// RequireAuth 校验 JWT Cookie，失败返回 401。成功后将 Claims 放入请求上下文。
func RequireAuth(jwtSecret []byte, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie("token")
		if err != nil || c.Value == "" {
			writeUnauthorized(w)
			return
		}
		claims, err := service.ParseJWT(c.Value, jwtSecret)
		if err != nil {
			writeUnauthorized(w)
			return
		}
		ctx := context.WithValue(r.Context(), claimsKey, claims)
		next(w, r.WithContext(ctx))
	}
}

// RequireSuper 额外要求当前用户是超级管理员。
func RequireSuper(jwtSecret []byte, next http.HandlerFunc) http.HandlerFunc {
	return RequireAuth(jwtSecret, func(w http.ResponseWriter, r *http.Request) {
		claims := ClaimsFrom(r)
		if claims == nil || !claims.IsSuper {
			writeForbidden(w)
			return
		}
		next(w, r)
	})
}

// ClaimsFrom 从请求上下文取出 JWT Claims，没有则返回 nil。
func ClaimsFrom(r *http.Request) *service.Claims {
	v := r.Context().Value(claimsKey)
	if v == nil {
		return nil
	}
	c, _ := v.(*service.Claims)
	return c
}

func writeUnauthorized(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusUnauthorized)
	_, _ = w.Write([]byte(`{"success":false,"message":"未登录或登录已过期"}`))
}

func writeForbidden(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusForbidden)
	_, _ = w.Write([]byte(`{"success":false,"message":"权限不足，需要超级管理员身份"}`))
}
