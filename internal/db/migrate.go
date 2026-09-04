package db

import (
	"database/sql"
	"fmt"
)

// Migrate 创建所有表与索引（幂等），并运行增量迁移。
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
			deleted_at          DATETIME,
			submission_id       INTEGER NOT NULL DEFAULT 0
		)`,
		// 普通索引：按邮箱查询（含回收站记录）
		`CREATE INDEX IF NOT EXISTS idx_email ON blacklist(email)`,
		// 部分唯一索引：仅约束未删除记录，从而支持软删除后重新添加同名邮箱
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_email_active ON blacklist(email) WHERE deleted_at IS NULL`,

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

		`CREATE TABLE IF NOT EXISTS submissions (
			id                    INTEGER PRIMARY KEY AUTOINCREMENT,
			query_code            TEXT NOT NULL UNIQUE,
			type                  TEXT NOT NULL DEFAULT 'report',
			email                 TEXT NOT NULL,
			ban_reason            TEXT DEFAULT '',
			event_link            TEXT DEFAULT '',
			event_related_people  TEXT DEFAULT '',
			appeal_reason         TEXT DEFAULT '',
			appeal_evidence       TEXT DEFAULT '',
			status                TEXT NOT NULL DEFAULT 'pending',
			reject_reason         TEXT DEFAULT '',
			submitter_ip          TEXT DEFAULT '',
			submitter_ua          TEXT DEFAULT '',
			created_at            DATETIME,
			reviewed_at           DATETIME,
			reviewed_by           TEXT DEFAULT ''
		)`,
		`CREATE INDEX IF NOT EXISTS idx_submissions_status ON submissions(status)`,
		`CREATE INDEX IF NOT EXISTS idx_submissions_type ON submissions(type)`,
		`CREATE INDEX IF NOT EXISTS idx_submissions_email ON submissions(email)`,

		`CREATE TABLE IF NOT EXISTS submission_images (
			id            INTEGER PRIMARY KEY AUTOINCREMENT,
			submission_id INTEGER NOT NULL,
			file_hash     TEXT NOT NULL,
			ext           TEXT NOT NULL DEFAULT '',
			size          INTEGER NOT NULL DEFAULT 0,
			sort_order    INTEGER NOT NULL DEFAULT 0,
			FOREIGN KEY (submission_id) REFERENCES submissions(id) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_submission_images_submission ON submission_images(submission_id)`,
	}

	for _, s := range stmts {
		if _, err := database.Exec(s); err != nil {
			return fmt.Errorf("迁移失败: %w", err)
		}
	}

	// admin_user 表创建与增量迁移（旧版无 username/is_super/created_by 字段）
	if err := ensureAdminUserTable(database); err != nil {
		return fmt.Errorf("admin_user 表初始化失败: %w", err)
	}

	// blacklist 增量迁移：旧库补充 submission_id 列（关联产生该记录的举报申请，用于展示证据图片）
	if err := ensureBlacklistSubmissionID(database); err != nil {
		return fmt.Errorf("blacklist submission_id 迁移失败: %w", err)
	}

	return nil
}

// ensureBlacklistSubmissionID 给已存在的 blacklist 表补充 submission_id 列（幂等）。
func ensureBlacklistSubmissionID(db *sql.DB) error {
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('blacklist') WHERE name='submission_id'`).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	_, err := db.Exec(`ALTER TABLE blacklist ADD COLUMN submission_id INTEGER NOT NULL DEFAULT 0`)
	return err
}

// ensureAdminUserTable 确保 admin_user 表存在且为 v2 结构（含 username/is_super/created_by）。
// 若表不存在则直接创建；若表存在但为 v1 旧结构则升级。
func ensureAdminUserTable(db *sql.DB) error {
	// 检查表是否存在
	var tblCount int
	row := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='admin_user'`)
	if err := row.Scan(&tblCount); err != nil {
		return err
	}
	if tblCount == 0 {
		// 全新库，直接创建 v2 表
		_, err := db.Exec(`
			CREATE TABLE admin_user (
				id            INTEGER PRIMARY KEY AUTOINCREMENT,
				username      TEXT NOT NULL UNIQUE,
				password_hash TEXT NOT NULL,
				totp_secret   TEXT NOT NULL,
				is_super      INTEGER NOT NULL DEFAULT 0,
				created_at    DATETIME,
				created_by    TEXT DEFAULT ''
			)
		`)
		if err != nil {
			return err
		}
		_, err = db.Exec(`CREATE INDEX idx_admin_username ON admin_user(username)`)
		return err
	}

	// 表已存在，检查是否为 v2 结构
	var colCount int
	row = db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('admin_user') WHERE name IN ('username', 'is_super', 'created_by')`)
	if err := row.Scan(&colCount); err != nil {
		return err
	}
	if colCount >= 3 {
		// 已是 v2，确保索引存在
		_, err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_admin_username ON admin_user(username)`)
		return err
	}

	// v1 → v2 升级：重建表并迁移数据
	_, err := db.Exec(`
		CREATE TABLE admin_user_new (
			id            INTEGER PRIMARY KEY AUTOINCREMENT,
			username      TEXT NOT NULL UNIQUE,
			password_hash TEXT NOT NULL,
			totp_secret   TEXT NOT NULL,
			is_super      INTEGER NOT NULL DEFAULT 0,
			created_at    DATETIME,
			created_by    TEXT DEFAULT ''
		)
	`)
	if err != nil {
		return err
	}

	// 迁移旧数据（旧表只有 id=1 一条）
	_, err = db.Exec(`
		INSERT INTO admin_user_new (id, username, password_hash, totp_secret, is_super, created_at, created_by)
		SELECT id, 'admin', password_hash, totp_secret, 1, created_at, 'system'
		FROM admin_user
	`)
	if err != nil {
		return err
	}

	_, err = db.Exec(`DROP TABLE admin_user`)
	if err != nil {
		return err
	}
	_, err = db.Exec(`ALTER TABLE admin_user_new RENAME TO admin_user`)
	if err != nil {
		return err
	}
	_, err = db.Exec(`CREATE INDEX idx_admin_username ON admin_user(username)`)
	return err
}
