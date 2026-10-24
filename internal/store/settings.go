package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// GetSetting mengambil nilai setting berdasarkan key, string kosong jika belum ada
func (s *Store) GetSetting(ctx context.Context, key string) (string, error) {
	var value string
	err := s.db.QueryRowContext(ctx, "SELECT value FROM settings WHERE key = ?", key).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("gagal membaca setting %s: %w", key, err)
	}
	return value, nil
}

// SetSetting menyimpan atau memperbarui nilai setting (upsert)
func (s *Store) SetSetting(ctx context.Context, key, value string) error {
	_, err := s.db.ExecContext(ctx,
		"INSERT INTO settings (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value",
		key, value)
	if err != nil {
		return fmt.Errorf("gagal menyimpan setting %s: %w", key, err)
	}
	return nil
}
