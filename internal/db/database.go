package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// Open 打开 SQLite 数据库，开启 WAL，并把手动设连接池为单连接。
func Open(dataDir string) (*sql.DB, error) {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, fmt.Errorf("创建数据目录失败: %w", err)
	}
	dbPath := filepath.ToSlash(filepath.Join(dataDir, "blacklist.db"))
	dsn := fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=synchronous(NORMAL)&_pragma=foreign_keys(ON)", dbPath)

	database, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("打开数据库失败: %w", err)
	}
	database.SetMaxOpenConns(1)
	database.SetMaxIdleConns(1)

	if err := database.Ping(); err != nil {
		return nil, fmt.Errorf("数据库连接失败: %w", err)
	}
	return database, nil
}

// IntegrityCheck 执行 PRAGMA integrity_check，失败则返回错误。
func IntegrityCheck(database *sql.DB) error {
	rows, err := database.Query("PRAGMA integrity_check")
	if err != nil {
		return fmt.Errorf("integrity_check 执行失败: %w", err)
	}
	defer rows.Close()

	var result string
	for rows.Next() {
		if err := rows.Scan(&result); err != nil {
			return fmt.Errorf("读取 integrity_check 结果失败: %w", err)
		}
		if result != "ok" {
			return fmt.Errorf("数据库完整性检查失败: %s", result)
		}
	}
	return rows.Err()
}
