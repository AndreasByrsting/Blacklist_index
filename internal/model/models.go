package model

// TimeFormat 是数据库与 API 之间统一的时间字符串格式。
// 所有时间均以配置时区（默认 Asia/Shanghai）存储与展示。
const TimeFormat = "2006-01-02 15:04:05"

// Blacklist 表示一条不可信邮箱记录。
type Blacklist struct {
	ID                 int64              `json:"id"`
	Email              string             `json:"email"`
	BanReason          string             `json:"ban_reason"`
	BanReasonRaw       string             `json:"ban_reason_raw"`
	EventLink          string             `json:"event_link"`
	EventRelatedPeople string             `json:"event_related_people"`
	BannedAt           string             `json:"banned_at"`
	CreatedBy          string             `json:"created_by"`
	CreatedAt          string             `json:"created_at"`
	DeletedAt          *string            `json:"deleted_at,omitempty"`
	SubmissionID       int64              `json:"submission_id"`
	Images             []*SubmissionImage `json:"images,omitempty"`
}

// AdminUser 表示管理员账号。
type AdminUser struct {
	ID           int64  `json:"id"`
	Username     string `json:"username"`
	PasswordHash string `json:"-"`
	TOTPSecret   string `json:"-"`
	IsSuper      bool   `json:"is_super"`
	CreatedAt    string `json:"created_at"`
	CreatedBy    string `json:"created_by"`
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

// SubmissionStatus 提交审核状态。
type SubmissionStatus string

const (
	StatusPending  SubmissionStatus = "pending"  // 待审核
	StatusApproved SubmissionStatus = "approved" // 已通过
	StatusRejected SubmissionStatus = "rejected" // 已驳回
)

// SubmissionType 提交类型。
type SubmissionType string

const (
	TypeReport  SubmissionType = "report"  // 举报（标记为不可信）
	TypeAppeal  SubmissionType = "appeal"  // 申诉
)

// Submission 表示一条用户提交的举报/申诉申请。
type Submission struct {
	ID                 int64            `json:"id"`
	QueryCode          string           `json:"query_code"`
	Type               SubmissionType   `json:"type"`
	Email              string           `json:"email"`
	BanReason          string           `json:"ban_reason"`
	EventLink          string           `json:"event_link"`
	EventRelatedPeople string           `json:"event_related_people"`
	AppealReason       string           `json:"appeal_reason"`   // 申诉理由（仅申诉类型）
	AppealEvidence     string           `json:"appeal_evidence"` // 申诉反驳证据链接（仅申诉类型）
	Status             SubmissionStatus `json:"status"`
	RejectReason       string           `json:"reject_reason"`
	SubmitterIP        string           `json:"-"`
	SubmitterUA        string           `json:"-"`
	CreatedAt          string           `json:"created_at"`
	ReviewedAt         string           `json:"reviewed_at"`
	ReviewedBy         string           `json:"reviewed_by"`
	Images             []*SubmissionImage `json:"images,omitempty"`
}

// SubmissionImage 表示一笔提交关联的证据图片元数据。
// URL 在服务层拼接，供后台审核页直接以 <img> 加载。
type SubmissionImage struct {
	ID           int64  `json:"id"`
	SubmissionID int64  `json:"submission_id"`
	FileHash     string `json:"file_hash"`
	Ext          string `json:"ext"`
	Size         int64  `json:"size"`
	SortOrder    int    `json:"sort_order"`
	URL          string `json:"url,omitempty"`
}
