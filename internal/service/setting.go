package service

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"blacklist-index/internal/repository"
)

const (
	ConfigKeyInbox                  = "inbox_email"
	ConfigKeyReportEvidenceRequired = "report_evidence_required"
	ConfigKeyReportLinkDomains      = "report_link_domains"
	ConfigKeyAppealEvidenceRequired = "appeal_evidence_required"
	ConfigKeyAppealLinkDomains      = "appeal_link_domains"
	ConfigKeyReportImageRequired    = "report_image_required"
	ConfigKeyReportImageMax         = "report_image_max"
	ConfigKeyAppealImageRequired    = "appeal_image_required"
	ConfigKeyAppealImageMax         = "appeal_image_max"
	ConfigKeyQueryCount             = "query_count"
	ConfigKeyStatsEnabled           = "stats_enabled"
)

// LinkPolicy 保存某一类提交（举报或申诉）的证据链接校验策略。
type LinkPolicy struct {
	EvidenceRequired bool
	Domains          []string
}

// ImagePolicy 保存某一类提交（举报或申诉）的证据图片校验策略。
type ImagePolicy struct {
	Required bool
	MaxCount int
}

// SettingService 负责动态站点设置（收件箱等）的读写。
type SettingService struct {
	repo  *repository.ConfigRepo
	audit *AuditService
}

func NewSettingService(repo *repository.ConfigRepo, audit *AuditService) *SettingService {
	return &SettingService{repo: repo, audit: audit}
}

// GetInbox 返回收件箱邮箱，未配置时返回空字符串。
func (s *SettingService) GetInbox() (string, error) {
	v, err := s.repo.Get(ConfigKeyInbox)
	if errors.Is(err, repository.ErrNotFound) {
		return "", nil
	}
	return v, err
}

// SetInbox 保存收件箱邮箱（空字符串表示清除，隐藏首页提报按钮）。
func (s *SettingService) SetInbox(email, ip, ua string) error {
	email = NormalizeEmail(email)
	if email != "" && !ValidEmail(email) {
		return ErrInvalidEmail
	}
	if err := s.repo.Set(ConfigKeyInbox, email); err != nil {
		return err
	}
	s.audit.Log("admin", ActionEditSettings, email, ip, ua)
	return nil
}

// ——— 链接策略 ———

func (s *SettingService) getBool(key string, def bool) bool {
	v, err := s.repo.Get(key)
	if err != nil {
		return def
	}
	return v == "true" || v == "1"
}

func (s *SettingService) getString(key, def string) string {
	v, err := s.repo.Get(key)
	if err != nil {
		return def
	}
	return v
}

func (s *SettingService) getInt(key string, def int) int {
	v, err := s.repo.Get(key)
	if err != nil {
		return def
	}
	var n int
	if _, err := fmt.Sscanf(v, "%d", &n); err != nil {
		return def
	}
	return n
}

// parseDomains 将逗号分隔的域名列表拆分为去重后的小写数组。
func parseDomains(s string) []string {
	parts := strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == '，' || r == ' ' })
	var out []string
	for _, p := range parts {
		p = strings.TrimSpace(strings.ToLower(p))
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// GetReportLinkConfig 返回举报证据链接的校验策略。
func (s *SettingService) GetReportLinkConfig() LinkPolicy {
	return LinkPolicy{
		EvidenceRequired: s.getBool(ConfigKeyReportEvidenceRequired, true),
		Domains:          parseDomains(s.getString(ConfigKeyReportLinkDomains, "tieba.baidu.com")),
	}
}

// GetAppealLinkConfig 返回申诉证据链接的校验策略。
func (s *SettingService) GetAppealLinkConfig() LinkPolicy {
	return LinkPolicy{
		EvidenceRequired: s.getBool(ConfigKeyAppealEvidenceRequired, true),
		Domains:          parseDomains(s.getString(ConfigKeyAppealLinkDomains, "tieba.baidu.com")),
	}
}

// SetReportLinkConfig 保存举报证据链接策略。
func (s *SettingService) SetReportLinkConfig(required bool, domains, ip, ua string) error {
	if err := s.repo.Set(ConfigKeyReportEvidenceRequired, boolStr(required)); err != nil {
		return err
	}
	if err := s.repo.Set(ConfigKeyReportLinkDomains, strings.TrimSpace(domains)); err != nil {
		return err
	}
	s.audit.Log("admin", ActionEditSettings, "report_link_config", ip, ua)
	return nil
}

// SetAppealLinkConfig 保存申诉证据链接策略。
func (s *SettingService) SetAppealLinkConfig(required bool, domains, ip, ua string) error {
	if err := s.repo.Set(ConfigKeyAppealEvidenceRequired, boolStr(required)); err != nil {
		return err
	}
	if err := s.repo.Set(ConfigKeyAppealLinkDomains, strings.TrimSpace(domains)); err != nil {
		return err
	}
	s.audit.Log("admin", ActionEditSettings, "appeal_link_config", ip, ua)
	return nil
}

// ——— 图片策略 ———

// GetReportImageConfig 返回举报证据图片的校验策略。
func (s *SettingService) GetReportImageConfig() ImagePolicy {
	return ImagePolicy{
		Required: s.getBool(ConfigKeyReportImageRequired, false),
		MaxCount: clampImageMax(s.getInt(ConfigKeyReportImageMax, 3)),
	}
}

// GetAppealImageConfig 返回申诉证据图片的校验策略。
func (s *SettingService) GetAppealImageConfig() ImagePolicy {
	return ImagePolicy{
		Required: s.getBool(ConfigKeyAppealImageRequired, false),
		MaxCount: clampImageMax(s.getInt(ConfigKeyAppealImageMax, 3)),
	}
}

// SetReportImageConfig 保存举报证据图片策略。
func (s *SettingService) SetReportImageConfig(required bool, maxCount int, ip, ua string) error {
	if err := s.repo.Set(ConfigKeyReportImageRequired, boolStr(required)); err != nil {
		return err
	}
	if err := s.repo.Set(ConfigKeyReportImageMax, intStr(clampImageMax(maxCount))); err != nil {
		return err
	}
	s.audit.Log("admin", ActionEditSettings, "report_image_config", ip, ua)
	return nil
}

// SetAppealImageConfig 保存申诉证据图片策略。
func (s *SettingService) SetAppealImageConfig(required bool, maxCount int, ip, ua string) error {
	if err := s.repo.Set(ConfigKeyAppealImageRequired, boolStr(required)); err != nil {
		return err
	}
	if err := s.repo.Set(ConfigKeyAppealImageMax, intStr(clampImageMax(maxCount))); err != nil {
		return err
	}
	s.audit.Log("admin", ActionEditSettings, "appeal_image_config", ip, ua)
	return nil
}

// ——— 首页统计（查询次数） ———

// GetQueryCount 返回累计查询次数，未初始化时返回 0。
func (s *SettingService) GetQueryCount() (int64, error) {
	v, err := s.repo.Get(ConfigKeyQueryCount)
	if errors.Is(err, repository.ErrNotFound) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	n, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
	if err != nil {
		return 0, nil
	}
	return n, nil
}

// IncrementQueryCount 将查询计数加一。
func (s *SettingService) IncrementQueryCount() error {
	return s.repo.Increment(ConfigKeyQueryCount)
}

// GetStatsEnabled 返回是否在首页展示查询统计。
func (s *SettingService) GetStatsEnabled() bool {
	return s.getBool(ConfigKeyStatsEnabled, false)
}

// SetStatsEnabled 保存首页统计展示开关。
func (s *SettingService) SetStatsEnabled(enabled bool, ip, ua string) error {
	if err := s.repo.Set(ConfigKeyStatsEnabled, boolStr(enabled)); err != nil {
		return err
	}
	s.audit.Log("admin", ActionEditSettings, "stats_enabled", ip, ua)
	return nil
}

// clampImageMax 将图片上限收敛到 [0, 9]，0 表示不限制。
func clampImageMax(n int) int {
	if n <= 0 {
		return 0
	}
	if n > 9 {
		return 9
	}
	return n
}

func intStr(n int) string {
	return strconv.Itoa(n)
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
