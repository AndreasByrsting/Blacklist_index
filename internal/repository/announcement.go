package repository

import (
	"database/sql"
	"errors"

	"blacklist-index/internal/model"
)

// AnnouncementRepo 负责 announcement 表（单行）的数据访问。
type AnnouncementRepo struct {
	db *sql.DB
}

func NewAnnouncementRepo(db *sql.DB) *AnnouncementRepo { return &AnnouncementRepo{db: db} }

// GetActive 获取当前生效的公告。
func (r *AnnouncementRepo) GetActive() (*model.Announcement, error) {
	row := r.db.QueryRow(`
		SELECT id, content, COALESCE(content_raw,''), is_active, COALESCE(updated_at,''), COALESCE(updated_by,'')
		FROM announcement WHERE id = 1 AND is_active = 1`)
	var a model.Announcement
	if err := row.Scan(&a.ID, &a.Content, &a.ContentRaw, &a.IsActive, &a.UpdatedAt, &a.UpdatedBy); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &a, nil
}

// Upsert 保存或更新公告（固定 id=1）。
func (r *AnnouncementRepo) Upsert(a *model.Announcement) error {
	_, err := r.db.Exec(`
		INSERT INTO announcement (id, content, content_raw, is_active, updated_at, updated_by)
		VALUES (1, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET content=excluded.content, content_raw=excluded.content_raw,
			is_active=excluded.is_active, updated_at=excluded.updated_at, updated_by=excluded.updated_by`,
		a.Content, a.ContentRaw, a.IsActive, a.UpdatedAt, a.UpdatedBy)
	return err
}
