package repository

import (
	"database/sql"
	"errors"

	"blacklist-index/internal/model"
)

// AdminRepo 负责 admin_user 表的数据访问。
type AdminRepo struct {
	db *sql.DB
}

func NewAdminRepo(db *sql.DB) *AdminRepo { return &AdminRepo{db: db} }

// GetAdmin 获取唯一管理员记录。
func (r *AdminRepo) GetAdmin() (*model.AdminUser, error) {
	row := r.db.QueryRow(`SELECT id, password_hash, totp_secret, COALESCE(created_at,'') FROM admin_user WHERE id = 1`)
	var a model.AdminUser
	if err := row.Scan(&a.ID, &a.PasswordHash, &a.TOTPSecret, &a.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &a, nil
}

// CreateAdmin 创建管理员（id 固定为 1）。
func (r *AdminRepo) CreateAdmin(a *model.AdminUser) error {
	_, err := r.db.Exec(`INSERT INTO admin_user (id, password_hash, totp_secret, created_at) VALUES (1, ?, ?, ?)`,
		a.PasswordHash, a.TOTPSecret, a.CreatedAt)
	return err
}
