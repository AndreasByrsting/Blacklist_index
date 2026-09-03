package repository

import (
	"database/sql"
	"errors"
	"strings"

	"blacklist-index/internal/model"
)

// ErrUsernameExists 表示用户名已存在。
var ErrUsernameExists = errors.New("用户名已存在")

// AdminRepo 负责 admin_user 表的数据访问。
type AdminRepo struct {
	db *sql.DB
}

func NewAdminRepo(db *sql.DB) *AdminRepo { return &AdminRepo{db: db} }

// GetByUsername 按用户名查询管理员。
func (r *AdminRepo) GetByUsername(username string) (*model.AdminUser, error) {
	row := r.db.QueryRow(`
		SELECT id, username, password_hash, totp_secret, is_super, COALESCE(created_at,''), COALESCE(created_by,'')
		FROM admin_user WHERE username = ?`, username)
	return scanAdmin(row)
}

// GetByID 按 ID 查询管理员。
func (r *AdminRepo) GetByID(id int64) (*model.AdminUser, error) {
	row := r.db.QueryRow(`
		SELECT id, username, password_hash, totp_secret, is_super, COALESCE(created_at,''), COALESCE(created_by,'')
		FROM admin_user WHERE id = ?`, id)
	return scanAdmin(row)
}

// List 列出所有管理员（按 ID 升序）。
func (r *AdminRepo) List() ([]*model.AdminUser, error) {
	rows, err := r.db.Query(`
		SELECT id, username, password_hash, totp_secret, is_super, COALESCE(created_at,''), COALESCE(created_by,'')
		FROM admin_user ORDER BY id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := make([]*model.AdminUser, 0)
	for rows.Next() {
		a, err := scanAdmin(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, a)
	}
	return list, rows.Err()
}

// Count 返回管理员总数。
func (r *AdminRepo) Count() (int, error) {
	var n int
	err := r.db.QueryRow(`SELECT COUNT(*) FROM admin_user`).Scan(&n)
	return n, err
}

// Create 创建新管理员。用户名唯一冲突时返回 ErrUsernameExists。
func (r *AdminRepo) Create(a *model.AdminUser) error {
	res, err := r.db.Exec(`
		INSERT INTO admin_user (username, password_hash, totp_secret, is_super, created_at, created_by)
		VALUES (?, ?, ?, ?, ?, ?)`,
		a.Username, a.PasswordHash, a.TOTPSecret, boolToInt(a.IsSuper), a.CreatedAt, a.CreatedBy)
	if err != nil {
		if isUniqueConstraint(err) {
			return ErrUsernameExists
		}
		return err
	}
	id, err := res.LastInsertId()
	if err == nil {
		a.ID = id
	}
	return nil
}

// UpdatePassword 更新管理员密码哈希。
func (r *AdminRepo) UpdatePassword(id int64, passwordHash string) error {
	res, err := r.db.Exec(`UPDATE admin_user SET password_hash = ? WHERE id = ?`, passwordHash, id)
	return checkAffected(res, err)
}

// UpdateTOTPSecret 更新管理员 TOTP 密钥。
func (r *AdminRepo) UpdateTOTPSecret(id int64, secret string) error {
	res, err := r.db.Exec(`UPDATE admin_user SET totp_secret = ? WHERE id = ?`, secret, id)
	return checkAffected(res, err)
}

// Delete 删除管理员。不能删除自己（由上层判断），且至少保留一名超级管理员。
func (r *AdminRepo) Delete(id int64) error {
	res, err := r.db.Exec(`DELETE FROM admin_user WHERE id = ?`, id)
	return checkAffected(res, err)
}

type adminScanner interface {
	Scan(dest ...any) error
}

func scanAdmin(s adminScanner) (*model.AdminUser, error) {
	var a model.AdminUser
	var isSuper int
	if err := s.Scan(&a.ID, &a.Username, &a.PasswordHash, &a.TOTPSecret, &isSuper, &a.CreatedAt, &a.CreatedBy); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	a.IsSuper = isSuper != 0
	return &a, nil
}

func boolToInt(b bool) int {
	if b { return 1 }
	return 0
}

// isUniqueConstraint 粗略判断是否为 SQLite 唯一约束冲突错误。
func isUniqueConstraint(err error) bool {
	return err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed")
}
