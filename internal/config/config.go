package config

import (
	"encoding/base32"
	"fmt"
	"os"
	"strings"
	"time"
)

// Config 保存所有运行时配置。
type Config struct {
	Timezone      string
	Location      *time.Location
	AdminPassword string
	TOTPSecret    string
	Port          string
	SiteName      string
	DataDir       string
}

// Load 从环境变量读取配置并校验。
func Load() (*Config, error) {
	cfg := &Config{
		Timezone:      getEnv("TIMEZONE", "Asia/Shanghai"),
		AdminPassword: os.Getenv("ADMIN_PASSWORD"),
		TOTPSecret:    strings.TrimSpace(os.Getenv("TOTP_SECRET")),
		Port:          getEnv("PORT", "8080"),
		SiteName:      getEnv("SITE_NAME", "邮箱黑名单查询"),
		DataDir:       getEnv("DATA_DIR", "./data"),
	}

	loc, err := time.LoadLocation(cfg.Timezone)
	if err != nil {
		return nil, fmt.Errorf("invalid TIMEZONE %q: %w", cfg.Timezone, err)
	}
	cfg.Location = loc

	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (c *Config) validate() error {
	if len(c.AdminPassword) < 8 {
		return fmt.Errorf("ADMIN_PASSWORD 必须至少 8 位")
	}
	if !isStrongPassword(c.AdminPassword) {
		return fmt.Errorf("ADMIN_PASSWORD 需同时包含大写字母、小写字母和数字")
	}

	secret, err := normalizeBase32(c.TOTPSecret)
	if err != nil {
		return fmt.Errorf("TOTP_SECRET 非法: %w", err)
	}
	if len(secret) < 16 {
		return fmt.Errorf("TOTP_SECRET 长度必须 >= 16 个 Base32 字符")
	}
	c.TOTPSecret = secret
	return nil
}

// normalizeBase32 将用户输入的密钥规范化：大写、去空白与连字符、去 padding。
func normalizeBase32(s string) (string, error) {
	s = strings.Map(func(r rune) rune {
		switch r {
		case ' ', '\t', '\r', '\n', '-':
			return -1
		}
		return r
	}, strings.ToUpper(s))
	s = strings.TrimRight(s, "=")
	if s == "" {
		return "", fmt.Errorf("不能为空")
	}
	padded := s + strings.Repeat("=", (8-len(s)%8)%8)
	if _, err := base32.StdEncoding.DecodeString(padded); err != nil {
		return "", fmt.Errorf("不是合法的 Base32 编码: %v", err)
	}
	return s, nil
}

func isStrongPassword(p string) bool {
	var hasLower, hasUpper, hasDigit bool
	for _, r := range p {
		switch {
		case r >= 'a' && r <= 'z':
			hasLower = true
		case r >= 'A' && r <= 'Z':
			hasUpper = true
		case r >= '0' && r <= '9':
			hasDigit = true
		}
	}
	return hasLower && hasUpper && hasDigit
}

func getEnv(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}
