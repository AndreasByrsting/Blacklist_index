package db

import (
	"database/sql"
	"fmt"
)

// Migrate 创建所有表与索引（幂等）。
func Migrate(database *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS blacklist (
			id                  INTEGER PRIMARY KEY AUTOINCREMENT,
			email               TEXT NOT NULL,
			ban_reason          TEXT DEFAULT '',
			ban_reason_raw      TEXT DEFAULT '',
			event_link          TEXT DEFAULT '',
			event_related_people TEXT DEFAULT '',
			banned_at           DATETIME,
			created_by          TEXT DEFAULT '',
			created_at          DATETIME,
			deleted_at          DATETIME
		)`,
		// 普通索引：按邮箱查询（含回收站记录）
		`CREATE INDEX IF NOT EXISTS idx_email ON blacklist(email)`,
		// 部分唯一索引：仅约束未删除记录，从而支持软删除后重新添加同名邮箱
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_email_active ON blacklist(email) WHERE deleted_at IS NULL`,

		`CREATE TABLE IF NOT EXISTS admin_user (
			id            INTEGER PRIMARY KEY,
			password_hash TEXT NOT NULL,
			totp_secret   TEXT NOT NULL,
			created_at    DATETIME
		)`,

		`CREATE TABLE IF NOT EXISTS announcement (
			id          INTEGER PRIMARY KEY,
			content     TEXT NOT NULL DEFAULT '',
			content_raw TEXT DEFAULT '',
			is_active   INTEGER DEFAULT 1,
			updated_at  DATETIME,
			updated_by  TEXT DEFAULT ''
		)`,

		`CREATE TABLE IF NOT EXISTS audit_logs (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			user       TEXT NOT NULL,
			action     TEXT NOT NULL,
			target     TEXT DEFAULT '',
			ip         TEXT NOT NULL,
			user_agent TEXT DEFAULT '',
			created_at DATETIME
		)`,
		`CREATE INDEX IF NOT EXISTS idx_audit_created ON audit_logs(created_at)`,

		`CREATE TABLE IF NOT EXISTS app_config (
			key   TEXT PRIMARY KEY,
			value TEXT NOT NULL
		)`,
	}

	for _, s := range stmts {
		if _, err := database.Exec(s); err != nil {
			return fmt.Errorf("迁移失败: %w", err)
		}
	}
	return nil
}
