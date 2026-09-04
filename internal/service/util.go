package service

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"blacklist-index/internal/model"
)

var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

// MaxReasonLen 标记原因的最大字符数。
const MaxReasonLen = 500

// MaxAppealReasonLen 申诉理由的最大字符数。
const MaxAppealReasonLen = 500

// 链接校验相关错误。
var (
	// ErrLinkHTTPSRequired 表示链接必须使用 HTTPS。
	ErrLinkHTTPSRequired = errors.New("链接必须使用 HTTPS 协议")
	// ErrLinkDomainUnsupported 表示链接域名不在白名单内。
	ErrLinkDomainUnsupported = errors.New("为了用户安全暂不支持该网站")
)

// NormalizeEmail 统一转小写并去除首尾空白。
func NormalizeEmail(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

// ValidEmail 校验邮箱格式。
func ValidEmail(s string) bool {
	return len(s) <= 254 && emailRegex.MatchString(s)
}

// NormalizeLink 规范化事件链接：去除首尾空白，无协议时自动补全 https://。
func NormalizeLink(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	lower := strings.ToLower(s)
	if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") {
		return s
	}
	return "https://" + s
}

// ValidateLinkWithDomains 校验链接（需先经 NormalizeLink 规范化）。
// 要求使用 HTTPS；domains 非空时还要求域名命中白名单（含子域名）。
// domains 为空表示不限制域名。
func ValidateLinkWithDomains(s string, domains []string) error {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	u, err := url.Parse(s)
	if err != nil || u.Host == "" {
		return fmt.Errorf("链接格式无效")
	}
	if !strings.EqualFold(u.Scheme, "https") {
		return ErrLinkHTTPSRequired
	}
	if len(domains) > 0 {
		host := strings.ToLower(u.Hostname())
		allowed := false
		for _, d := range domains {
			if host == d || strings.HasSuffix(host, "."+d) {
				allowed = true
				break
			}
		}
		if !allowed {
			return fmt.Errorf("%w，仅支持 %s", ErrLinkDomainUnsupported, strings.Join(domains, "、"))
		}
	}
	return nil
}

// SplitRelatedPeople 按中英文逗号拆分相关人并去除空白项。
func SplitRelatedPeople(s string) []string {
	fields := strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == '，' || r == '、' })
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		if f = strings.TrimSpace(f); f != "" {
			out = append(out, f)
		}
	}
	return out
}

// ReasonTooLong 判断拉黑原因是否超出长度限制。
func ReasonTooLong(s string) bool {
	return utf8.RuneCountInString(s) > MaxReasonLen
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

// NormalizeTime 将历史遗留的 "2006-01-02T15:04:05Z"（含字面 Z / 时区后缀）等格式
// 统一规范为 model.TimeFormat。历史数据的时间数值本身已是目标时区，
// 因此仅做格式规范化、不做时区偏移转换，避免时间被错误偏移（如 01:17 变 09:17）。
func NormalizeTime(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if _, err := time.Parse(model.TimeFormat, s); err == nil {
		return s
	}
	for _, layout := range []string{
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05",
		time.RFC3339,
		time.RFC3339Nano,
	} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.Format(model.TimeFormat)
		}
	}
	return s
}
