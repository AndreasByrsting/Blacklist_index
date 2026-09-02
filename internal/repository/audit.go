package repository

import (
	"database/sql"
	"fmt"

	"blacklist-index/internal/model"
)

// AuditRepo 负责 audit_logs 表的数据访问。
type AuditRepo struct {
	db *sql.DB
}

func NewAuditRepo(db *sql.DB) *AuditRepo { return &AuditRepo{db: db} }

// Log 写入一条审计日志。
func (r *AuditRepo) Log(a *model.AuditLog) error {
	_, err := r.db.Exec(`
		INSERT INTO audit_logs (user, action, target, ip, user_agent, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		a.User, a.Action, a.Target, a.IP, a.UserAgent, a.CreatedAt)
	return err
}

// List 分页列出审计日志，可按操作类型筛选。
func (r *AuditRepo) List(action string, offset, limit int) ([]*model.AuditLog, int, error) {
	cond := ""
	var args []any
	if action != "" {
		cond = "WHERE action = ?"
		args = append(args, action)
	}

	var total int
	if err := r.db.QueryRow("SELECT COUNT(*) FROM audit_logs "+cond, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("统计失败: %w", err)
	}

	rows, err := r.db.Query(`
		SELECT id, user, action, COALESCE(target,''), ip, COALESCE(user_agent,''), COALESCE(created_at,'')
		FROM audit_logs `+cond+` ORDER BY id DESC LIMIT ? OFFSET ?`, append(args, limit, offset)...)
	if err != nil {
		return nil, 0, fmt.Errorf("查询失败: %w", err)
	}
	defer rows.Close()

	list := make([]*model.AuditLog, 0)
	for rows.Next() {
		var a model.AuditLog
		if err := rows.Scan(&a.ID, &a.User, &a.Action, &a.Target, &a.IP, &a.UserAgent, &a.CreatedAt); err != nil {
			return nil, 0, err
		}
		list = append(list, &a)
	}
	return list, total, rows.Err()
}

// Prune 保留最近 10000 条，删除更早的记录。
func (r *AuditRepo) Prune() error {
	_, err := r.db.Exec(`
		DELETE FROM audit_logs WHERE id NOT IN (
			SELECT id FROM audit_logs ORDER BY id DESC LIMIT 10000
		)`)
	return err
}
