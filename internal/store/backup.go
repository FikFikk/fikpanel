package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type BackupConfig struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	TargetType  string `json:"target_type"` // 'database', 'site', 'directory'
	TargetName  string `json:"target_name"`
	StorageType string `json:"storage_type"` // 'local', 's3'
	S3Endpoint  string `json:"s3_endpoint"`
	S3Bucket    string `json:"s3_bucket"`
	S3AccessKey string `json:"s3_access_key"`
	S3SecretKey string `json:"s3_secret_key"`
	S3Region    string `json:"s3_region"`
	KeepCopies  int    `json:"keep_copies"`
	AutoCron    bool   `json:"auto_cron"`
	CreatedAt   int64  `json:"created_at"`
	UpdatedAt   int64  `json:"updated_at"`
}

type BackupRecord struct {
	ID            int64  `json:"id"`
	ConfigID      int64  `json:"config_id"`
	FileName      string `json:"file_name"`
	FileSizeBytes int64  `json:"file_size_bytes"`
	StorageType   string `json:"storage_type"`
	StoragePath   string `json:"storage_path"`
	Status        string `json:"status"` // 'success', 'failed', 'running'
	ErrorMessage  string `json:"error_message"`
	CreatedAt     int64  `json:"created_at"`
}

func (s *Store) ListBackupConfigs(ctx context.Context) ([]BackupConfig, error) {
	query := `SELECT id, name, target_type, target_name, storage_type, 
	                 coalesce(s3_endpoint, ''), coalesce(s3_bucket, ''), coalesce(s3_access_key, ''), 
	                 coalesce(s3_secret_key, ''), coalesce(s3_region, 'us-east-1'), 
	                 keep_copies, auto_cron, created_at, updated_at 
	          FROM backup_configs ORDER BY id DESC`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list backup configs: %w", err)
	}
	defer rows.Close()

	var list []BackupConfig
	for rows.Next() {
		var c BackupConfig
		var autoCron int
		if err := rows.Scan(
			&c.ID, &c.Name, &c.TargetType, &c.TargetName, &c.StorageType,
			&c.S3Endpoint, &c.S3Bucket, &c.S3AccessKey, &c.S3SecretKey, &c.S3Region,
			&c.KeepCopies, &autoCron, &c.CreatedAt, &c.UpdatedAt,
		); err != nil {
			return nil, err
		}
		c.AutoCron = autoCron == 1
		list = append(list, c)
	}
	if list == nil {
		list = []BackupConfig{}
	}
	return list, nil
}

func (s *Store) GetBackupConfig(ctx context.Context, id int64) (*BackupConfig, error) {
	query := `SELECT id, name, target_type, target_name, storage_type, 
	                 coalesce(s3_endpoint, ''), coalesce(s3_bucket, ''), coalesce(s3_access_key, ''), 
	                 coalesce(s3_secret_key, ''), coalesce(s3_region, 'us-east-1'), 
	                 keep_copies, auto_cron, created_at, updated_at 
	          FROM backup_configs WHERE id = ?`
	row := s.db.QueryRowContext(ctx, query, id)
	var c BackupConfig
	var autoCron int
	if err := row.Scan(
		&c.ID, &c.Name, &c.TargetType, &c.TargetName, &c.StorageType,
		&c.S3Endpoint, &c.S3Bucket, &c.S3AccessKey, &c.S3SecretKey, &c.S3Region,
		&c.KeepCopies, &autoCron, &c.CreatedAt, &c.UpdatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	c.AutoCron = autoCron == 1
	return &c, nil
}

func (s *Store) CreateBackupConfig(ctx context.Context, c *BackupConfig) (int64, error) {
	now := time.Now().Unix()
	autoCron := 0
	if c.AutoCron {
		autoCron = 1
	}
	if c.KeepCopies <= 0 {
		c.KeepCopies = 5
	}
	query := `INSERT INTO backup_configs (name, target_type, target_name, storage_type, s3_endpoint, s3_bucket, s3_access_key, s3_secret_key, s3_region, keep_copies, auto_cron, created_at, updated_at) 
	          VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	res, err := s.db.ExecContext(ctx, query, c.Name, c.TargetType, c.TargetName, c.StorageType, c.S3Endpoint, c.S3Bucket, c.S3AccessKey, c.S3SecretKey, c.S3Region, c.KeepCopies, autoCron, now, now)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Store) DeleteBackupConfig(ctx context.Context, id int64) error {
	_, _ = s.db.ExecContext(ctx, `DELETE FROM backup_records WHERE config_id = ?`, id)
	_, err := s.db.ExecContext(ctx, `DELETE FROM backup_configs WHERE id = ?`, id)
	return err
}

func (s *Store) CreateBackupRecord(ctx context.Context, r *BackupRecord) (int64, error) {
	now := time.Now().Unix()
	query := `INSERT INTO backup_records (config_id, file_name, file_size_bytes, storage_type, storage_path, status, error_message, created_at) 
	          VALUES (?, ?, ?, ?, ?, ?, ?, ?)`
	res, err := s.db.ExecContext(ctx, query, r.ConfigID, r.FileName, r.FileSizeBytes, r.StorageType, r.StoragePath, r.Status, r.ErrorMessage, now)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Store) ListBackupRecords(ctx context.Context, configID int64, limit int) ([]BackupRecord, error) {
	if limit <= 0 {
		limit = 30
	}
	query := `SELECT id, config_id, file_name, file_size_bytes, storage_type, storage_path, status, coalesce(error_message, ''), created_at 
	          FROM backup_records WHERE config_id = ? ORDER BY created_at DESC LIMIT ?`
	rows, err := s.db.QueryContext(ctx, query, configID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []BackupRecord
	for rows.Next() {
		var r BackupRecord
		if err := rows.Scan(&r.ID, &r.ConfigID, &r.FileName, &r.FileSizeBytes, &r.StorageType, &r.StoragePath, &r.Status, &r.ErrorMessage, &r.CreatedAt); err != nil {
			return nil, err
		}
		records = append(records, r)
	}
	if records == nil {
		records = []BackupRecord{}
	}
	return records, nil
}
