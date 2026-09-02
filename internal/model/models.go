package model

// TimeFormat 是数据库与 API 之间统一的时间字符串格式。
// 所有时间均以配置时区（默认 Asia/Shanghai）存储与展示。
const TimeFormat = "2006-01-02 15:04:05"

// Blacklist 表示一条黑名单记录。
type Blacklist struct {
	ID                 int64   `json:"id"`
	Email              string  `json:"email"`
	BanReason          string  `json:"ban_reason"`
	BanReasonRaw       string  `json:"ban_reason_raw"`
	EventLink          string  `json:"event_link"`
	EventRelatedPeople string  `json:"event_related_people"`
	BannedAt           string  `json:"banned_at"`
	CreatedBy          string  `json:"created_by"`
	CreatedAt          string  `json:"created_at"`
	DeletedAt          *string `json:"deleted_at,omitempty"`
}

// AdminUser 表示唯一的管理员账号。
type AdminUser struct {
	ID           int64  `json:"id"`
	PasswordHash string `json:"-"`
	TOTPSecret   string `json:"-"`
	CreatedAt    string `json:"created_at"`
}

// Announcement 表示站点公告（单行）。
type Announcement struct {
	ID         int64  `json:"id"`
	Content    string `json:"content"`
	ContentRaw string `json:"content_raw"`
	IsActive   bool   `json:"is_active"`
	UpdatedAt  string `json:"updated_at"`
	UpdatedBy  string `json:"updated_by"`
}

// AuditLog 表示一条审计日志。
type AuditLog struct {
	ID        int64  `json:"id"`
	User      string `json:"user"`
	Action    string `json:"action"`
	Target    string `json:"target"`
	IP        string `json:"ip"`
	UserAgent string `json:"user_agent"`
	CreatedAt string `json:"created_at"`
}
