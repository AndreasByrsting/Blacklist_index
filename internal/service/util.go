package service

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"blacklist-index/internal/model"
)

var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

// NormalizeEmail 统一转小写并去除首尾空白。
func NormalizeEmail(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

// ValidEmail 校验邮箱格式。
func ValidEmail(s string) bool {
	return len(s) <= 254 && emailRegex.MatchString(s)
}

// NowStr 返回配置时区下的当前时间字符串。
func NowStr(loc *time.Location) string {
	return time.Now().In(loc).Format(model.TimeFormat)
}

// ParseBannedAt 解析封禁时间输入。空字符串返回当前时间。
// 支持 "2006-01-02T15:04"、"2006-01-02T15:04:05"、"2006-01-02 15:04:05"。
func ParseBannedAt(s string, loc *time.Location) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return NowStr(loc), nil
	}
	layouts := []string{"2006-01-02T15:04:05", "2006-01-02T15:04", "2006-01-02 15:04:05", "2006-01-02 15:04"}
	for _, l := range layouts {
		if t, err := time.ParseInLocation(l, s, loc); err == nil {
			return t.Format(model.TimeFormat), nil
		}
	}
	return "", fmt.Errorf("无法解析时间 %q", s)
}
