package repository

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"blacklist-index/internal/model"
)

// ErrNotFound 表示记录不存在。
var ErrNotFound = errors.New("记录不存在")

// BlacklistRepo 负责 blacklist 表的数据访问。
type BlacklistRepo struct {
	db *sql.DB
}

func NewBlacklistRepo(db *sql.DB) *BlacklistRepo { return &BlacklistRepo{db: db} }

// Check 按邮箱查询未删除的黑名单记录。
func (r *BlacklistRepo) Check(email string) (*model.Blacklist, error) {
	row := r.db.QueryRow(`
		SELECT id, email, ban_reason, ban_reason_raw, event_link, event_related_people,
		       COALESCE(banned_at,''), created_by, COALESCE(created_at,''), COALESCE(deleted_at,''),
		       COALESCE(submission_id,0)
		FROM blacklist WHERE email = ? AND deleted_at IS NULL`, email)
	b, err := scanBlacklist(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return b, err
}

// HasSimilarAccount 判断是否存在「@ 前账户名完全一致、但域名不同」的未删除记录。
// 仅返回布尔状态，不暴露具体相似记录，避免泄露无关数据。
func (r *BlacklistRepo) HasSimilarAccount(email string) (bool, error) {
	local := email
	if i := strings.Index(email, "@"); i >= 0 {
		local = email[:i]
	}
	var one int
	err := r.db.QueryRow(`
		SELECT 1 FROM blacklist
		WHERE deleted_at IS NULL
		  AND email <> ?
		  AND substr(email, 1, instr(email, '@') - 1) = ?
		LIMIT 1`, email, local).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// GetByID 按 ID 查询（含已软删除记录），用于回收站操作。
func (r *BlacklistRepo) GetByID(id int64) (*model.Blacklist, error) {
	row := r.db.QueryRow(`
		SELECT id, email, ban_reason, ban_reason_raw, event_link, event_related_people,
		       COALESCE(banned_at,''), created_by, COALESCE(created_at,''), COALESCE(deleted_at,''),
		       COALESCE(submission_id,0)
		FROM blacklist WHERE id = ?`, id)
	b, err := scanBlacklist(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return b, err
}

// Create 插入一条黑名单记录。
func (r *BlacklistRepo) Create(b *model.Blacklist) error {
	b.Email = strings.ToLower(strings.TrimSpace(b.Email))
	_, err := r.db.Exec(`
		INSERT INTO blacklist (email, ban_reason, ban_reason_raw, event_link, event_related_people, banned_at, created_by, created_at, submission_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		b.Email, b.BanReason, b.BanReasonRaw, b.EventLink, b.EventRelatedPeople, b.BannedAt, b.CreatedBy, b.CreatedAt, b.SubmissionID)
	return err
}

// ExistsEmail 判断邮箱是否已被其他未删除记录占用（excludeID 用于编辑时排除自身）。
func (r *BlacklistRepo) ExistsEmail(email string, excludeID int64) (bool, error) {
	var one int
	err := r.db.QueryRow(
		`SELECT 1 FROM blacklist WHERE email = ? AND deleted_at IS NULL AND id <> ? LIMIT 1`,
		email, excludeID).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// Update 按 ID 更新黑名单记录（仅限未删除记录）。
func (r *BlacklistRepo) Update(id int64, b *model.Blacklist) error {
	b.Email = strings.ToLower(strings.TrimSpace(b.Email))
	res, err := r.db.Exec(`
		UPDATE blacklist
		SET email = ?, ban_reason = ?, ban_reason_raw = ?, event_link = ?, event_related_people = ?, banned_at = ?, submission_id = ?
		WHERE id = ? AND deleted_at IS NULL`,
		b.Email, b.BanReason, b.BanReasonRaw, b.EventLink, b.EventRelatedPeople, b.BannedAt, b.SubmissionID, id)
	return checkAffected(res, err)
}

// List 分页列出记录。query 为空时列出全部；deleted=true 仅列回收站记录。
func (r *BlacklistRepo) List(query string, offset, limit int, deleted bool) ([]*model.Blacklist, int, error) {
	var where []string
	var args []any
	if deleted {
		where = append(where, "deleted_at IS NOT NULL")
	} else {
		where = append(where, "deleted_at IS NULL")
	}
	if q := strings.TrimSpace(query); q != "" {
		like := "%" + q + "%"
		where = append(where, "(email LIKE ? OR ban_reason_raw LIKE ?)")
		args = append(args, like, like)
	}
	cond := "WHERE " + strings.Join(where, " AND ")

	var total int
	if err := r.db.QueryRow("SELECT COUNT(*) FROM blacklist "+cond, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("统计失败: %w", err)
	}

	rows, err := r.db.Query(`
			SELECT id, email, ban_reason, ban_reason_raw, event_link, event_related_people,
			       COALESCE(banned_at,''), created_by, COALESCE(created_at,''), COALESCE(deleted_at,''),
			       COALESCE(submission_id,0)
			FROM blacklist `+cond+` ORDER BY id DESC LIMIT ? OFFSET ?`, append(args, limit, offset)...)
	if err != nil {
		return nil, 0, fmt.Errorf("查询失败: %w", err)
	}
	defer rows.Close()

	list := make([]*model.Blacklist, 0)
	for rows.Next() {
		b, err := scanBlacklist(rows)
		if err != nil {
			return nil, 0, err
		}
		list = append(list, b)
	}
	return list, total, rows.Err()
}

// SoftDelete 软删除记录。
func (r *BlacklistRepo) SoftDelete(id int64, deletedAt string) error {
	res, err := r.db.Exec("UPDATE blacklist SET deleted_at = ? WHERE id = ? AND deleted_at IS NULL", deletedAt, id)
	return checkAffected(res, err)
}

// Restore 从回收站恢复记录。
func (r *BlacklistRepo) Restore(id int64) error {
	res, err := r.db.Exec("UPDATE blacklist SET deleted_at = NULL WHERE id = ? AND deleted_at IS NOT NULL", id)
	return checkAffected(res, err)
}

// PermanentDelete 永久删除记录。
func (r *BlacklistRepo) PermanentDelete(id int64) error {
	res, err := r.db.Exec("DELETE FROM blacklist WHERE id = ? AND deleted_at IS NOT NULL", id)
	return checkAffected(res, err)
}

// ListImages 查询某举报申请（submission）关联的证据图片元数据，供黑名单记录展示。
func (r *BlacklistRepo) ListImages(submissionID int64) ([]*model.SubmissionImage, error) {
	list := make([]*model.SubmissionImage, 0)
	if submissionID <= 0 {
		return list, nil
	}
	rows, err := r.db.Query(`
		SELECT id, submission_id, file_hash, ext, size, sort_order
		FROM submission_images
		WHERE submission_id = ?
		ORDER BY sort_order ASC, id ASC`, submissionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var img model.SubmissionImage
		if err := rows.Scan(&img.ID, &img.SubmissionID, &img.FileHash, &img.Ext, &img.Size, &img.SortOrder); err != nil {
			return nil, err
		}
		list = append(list, &img)
	}
	return list, rows.Err()
}

type scanner interface {
	Scan(dest ...any) error
}

func scanBlacklist(s scanner) (*model.Blacklist, error) {
	var b model.Blacklist
	var deletedAt string
	if err := s.Scan(&b.ID, &b.Email, &b.BanReason, &b.BanReasonRaw, &b.EventLink,
		&b.EventRelatedPeople, &b.BannedAt, &b.CreatedBy, &b.CreatedAt, &deletedAt, &b.SubmissionID); err != nil {
		return nil, err
	}
	if deletedAt != "" {
		d := deletedAt
		b.DeletedAt = &d
	}
	return &b, nil
}
