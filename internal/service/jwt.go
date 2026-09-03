package service

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

// ErrInvalidToken 表示 JWT 无效或已过期。
var ErrInvalidToken = errors.New("invalid token")

// Claims JWT 载荷。
type Claims struct {
	Username string `json:"username"`
	IsSuper  bool   `json:"is_super"`
	Exp      int64  `json:"exp"`
}

func b64(raw []byte) string { return base64.RawURLEncoding.EncodeToString(raw) }

// GenerateJWT 签发 HS256 JWT。
func GenerateJWT(username string, isSuper bool, secret []byte, ttl time.Duration) (string, error) {
	header := b64([]byte(`{"alg":"HS256","typ":"JWT"}`))
	payload, err := json.Marshal(Claims{Username: username, IsSuper: isSuper, Exp: time.Now().Add(ttl).Unix()})
	if err != nil {
		return "", err
	}
	unsigned := header + "." + b64(payload)
	return unsigned + "." + b64(signJWT([]byte(unsigned), secret)), nil
}

func signJWT(data, secret []byte) []byte {
	m := hmac.New(sha256.New, secret)
	m.Write(data)
	return m.Sum(nil)
}

// ParseJWT 校验签名与有效期，返回载荷。
func ParseJWT(token string, secret []byte) (*Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, ErrInvalidToken
	}
	unsigned := parts[0] + "." + parts[1]
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || !hmac.Equal(sig, signJWT([]byte(unsigned), secret)) {
		return nil, ErrInvalidToken
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, ErrInvalidToken
	}
	var claims Claims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, ErrInvalidToken
	}
	if claims.Exp < time.Now().Unix() {
		return nil, ErrInvalidToken
	}
	return &claims, nil
}
