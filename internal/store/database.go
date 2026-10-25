package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// DatabaseRecord merepresentasikan satu baris database yang dikelola oleh FikPanel
type DatabaseRecord struct {
	ID          int64  `json:"id"`
	Engine      string `json:"engine"`       // 'mysql', 'postgresql', 'mongodb', 'redis'
	Name        string `json:"name"`
	Username    string `json:"username"`
	Password    string `json:"password"`
	SizeBytes   int64  `json:"size_bytes"`
	BackupCount int    `json:"backup_count"`
	Location    string `json:"location"`     // default: 'localhost'
	Note        string `json:"note"`
	CreatedAt   int64  `json:"created_at"`
}

// SystemTask merepresentasikan satu tugas antrean di latar belakang
type SystemTask struct {
	ID        string `json:"id"`
	Type      string `json:"type"`       // 'software_install', 'db_backup', dll.
	Target    string `json:"target"`     // misal: 'mongodb-7.0'
	Status    string `json:"status"`     // 'pending', 'running', 'success', 'failed', 'cancelled'
	Log       string `json:"log"`
	CreatedAt int64  `json:"created_at"`
	UpdatedAt int64  `json:"updated_at"`
}

// ListDatabases mengambil seluruh database terdaftar berdasarkan engine
func (s *Store) ListDatabases(ctx context.Context, engine string) ([]DatabaseRecord, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, engine, name, username, password, size_bytes, backup_count, location, note, created_at 
		 FROM databases WHERE engine = ? ORDER BY id DESC`, engine)
	if err != nil {
		return nil, fmt.Errorf("gagal query daftar database: %w", err)
	}
	defer rows.Close()

	var list []DatabaseRecord
	for rows.Next() {
		var d DatabaseRecord
		if err := rows.Scan(&d.ID, &d.Engine, &d.Name, &d.Username, &d.Password, &d.SizeBytes, &d.BackupCount, &d.Location, &d.Note, &d.CreatedAt); err != nil {
			return nil, fmt.Errorf("gagal memindai baris database: %w", err)
		}
		list = append(list, d)
	}
	if list == nil {
		list = []DatabaseRecord{}
	}
	return list, nil
}

// CreateDatabase menyimpan database baru ke dalam tabel
func (s *Store) CreateDatabase(ctx context.Context, d DatabaseRecord) (*DatabaseRecord, error) {
	now := time.Now().Unix()
	if d.Location == "" {
		d.Location = "localhost"
	}

	res, err := s.db.ExecContext(ctx,
		`INSERT INTO databases (engine, name, username, password, size_bytes, backup_count, location, note, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		d.Engine, d.Name, d.Username, d.Password, d.SizeBytes, d.BackupCount, d.Location, d.Note, now)
	if err != nil {
		return nil, fmt.Errorf("gagal menambahkan database baru: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	d.ID = id
	d.CreatedAt = now
	return &d, nil
}

// DeleteDatabase menghapus baris database dari database panel
func (s *Store) DeleteDatabase(ctx context.Context, engine, name string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM databases WHERE engine = ? AND name = ?`, engine, name)
	if err != nil {
		return fmt.Errorf("gagal menghapus database: %w", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// UpdateDatabasePassword memperbarui kata sandi pengguna database
func (s *Store) UpdateDatabasePassword(ctx context.Context, engine, name, newPassword string) error {
	res, err := s.db.ExecContext(ctx, `UPDATE databases SET password = ? WHERE engine = ? AND name = ?`, newPassword, engine, name)
	if err != nil {
		return fmt.Errorf("gagal memperbarui password database: %w", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// GetDatabaseCount menghitung jumlah database pada engine tertentu
func (s *Store) GetDatabaseCount(ctx context.Context, engine string) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM databases WHERE engine = ?`, engine).Scan(&count)
	return count, err
}

// CreateTask mendaftarkan pekerjaan baru ke dalam tabel system_tasks
func (s *Store) CreateTask(ctx context.Context, t SystemTask) error {
	now := time.Now().Unix()
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO system_tasks (id, type, target, status, log, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		t.ID, t.Type, t.Target, t.Status, t.Log, now, now)
	return err
}

// AppendTaskLog menambahkan teks log dan memperbarui status tugas
func (s *Store) AppendTaskLog(ctx context.Context, id, status, chunk string) error {
	now := time.Now().Unix()
	_, err := s.db.ExecContext(ctx,
		`UPDATE system_tasks 
		 SET status = ?, log = log || ?, updated_at = ? 
		 WHERE id = ?`,
		status, chunk, now, id)
	return err
}

// GetTask mengambil data status tugas berdasarkan ID
func (s *Store) GetTask(ctx context.Context, id string) (*SystemTask, error) {
	var t SystemTask
	err := s.db.QueryRowContext(ctx,
		`SELECT id, type, target, status, log, created_at, updated_at 
		 FROM system_tasks WHERE id = ?`, id).
		Scan(&t.ID, &t.Type, &t.Target, &t.Status, &t.Log, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// ListRecentTasks mengambil daftar pekerjaan terbaru untuk Messages Box
func (s *Store) ListRecentTasks(ctx context.Context, limit int) ([]SystemTask, error) {
	if limit <= 0 {
		limit = 10
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, type, target, status, log, created_at, updated_at 
		 FROM system_tasks ORDER BY created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []SystemTask
	for rows.Next() {
		var t SystemTask
		if err := rows.Scan(&t.ID, &t.Type, &t.Target, &t.Status, &t.Log, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		list = append(list, t)
	}
	if list == nil {
		list = []SystemTask{}
	}
	return list, nil
}
