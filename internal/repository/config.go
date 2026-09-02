package repository

import (
	"database/sql"
	"errors"
)

// ConfigRepo 负责 app_config 表（键值缓存）的数据访问。
type ConfigRepo struct {
	db *sql.DB
}

func NewConfigRepo(db *sql.DB) *ConfigRepo { return &ConfigRepo{db: db} }

// Get 读取配置项，不存在时返回 ErrNotFound。
func (r *ConfigRepo) Get(key string) (string, error) {
	var v string
	if err := r.db.QueryRow("SELECT value FROM app_config WHERE key = ?", key).Scan(&v); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", ErrNotFound
		}
		return "", err
	}
	return v, nil
}

// Set 写入或更新配置项。
func (r *ConfigRepo) Set(key, value string) error {
	_, err := r.db.Exec(`
		INSERT INTO app_config (key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value`, key, value)
	return err
}
