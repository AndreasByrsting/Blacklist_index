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
		       COALESCE(banned_at,''), created_by, COALESCE(created_at,''), COALESCE(deleted_at,'')
		FROM blacklist WHERE email = ? AND deleted_at IS NULL`, email)
	b, err := scanBlacklist(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return b, err
}

// GetByID 按 ID 查询（含已软删除记录），用于回收站操作。
func (r *BlacklistRepo) GetByID(id int64) (*model.Blacklist, error) {
	row := r.db.QueryRow(`
		SELECT id, email, ban_reason, ban_reason_raw, event_link, event_related_people,
		       COALESCE(banned_at,''), created_by, COALESCE(created_at,''), COALESCE(deleted_at,'')
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
		INSERT INTO blacklist (email, ban_reason, ban_reason_raw, event_link, event_related_people, banned_at, created_by, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		b.Email, b.BanReason, b.BanReasonRaw, b.EventLink, b.EventRelatedPeople, b.BannedAt, b.CreatedBy, b.CreatedAt)
	return err
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
		       COALESCE(banned_at,''), created_by, COALESCE(created_at,''), COALESCE(deleted_at,'')
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

type scanner interface {
	Scan(dest ...any) error
}

func scanBlacklist(s scanner) (*model.Blacklist, error) {
	var b model.Blacklist
	var deletedAt string
	if err := s.Scan(&b.ID, &b.Email, &b.BanReason, &b.BanReasonRaw, &b.EventLink,
		&b.EventRelatedPeople, &b.BannedAt, &b.CreatedBy, &b.CreatedAt, &deletedAt); err != nil {
		return nil, err
	}
	if deletedAt != "" {
		d := deletedAt
		b.DeletedAt = &d
	}
	return &b, nil
}

func checkAffected(res sql.Result, err error) error {
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
