package store

import (
	"context"
	"fmt"
	"time"
)

// AuditLog merepresentasikan catatan kejadian audit keamanan di sistem
type AuditLog struct {
	ID     int64  `json:"id"`
	TS     int64  `json:"ts"`
	UserID *int64 `json:"user_id,omitempty"`
	Actor  string `json:"actor"`
	Action string `json:"action"`
	Target string `json:"target"`
	OK     int    `json:"ok"`
	Detail string `json:"detail"`
}

// RecordAudit menyimpan catatan audit log baru ke database
func (s *Store) RecordAudit(ctx context.Context, actor, action, target string, ok bool, detail string, userID *int64) error {
	now := time.Now().Unix()
	okInt := 0
	if ok {
		okInt = 1
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO audit_log (ts, user_id, actor, action, target, ok, detail)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, now, userID, actor, action, target, okInt, detail)
	if err != nil {
		return fmt.Errorf("gagal merekam log audit: %w", err)
	}
	return nil
}

// ListAuditLogs mengambil riwayat audit log terbaru dengan limit tertentu
func (s *Store) ListAuditLogs(ctx context.Context, limit int) ([]AuditLog, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT id, ts, user_id, actor, action, COALESCE(target, ''), ok, COALESCE(detail, '')
		FROM audit_log
		ORDER BY ts DESC, id DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("gagal query daftar audit log: %w", err)
	}
	defer rows.Close()

	var list []AuditLog
	for rows.Next() {
		var item AuditLog
		err := rows.Scan(
			&item.ID, &item.TS, &item.UserID, &item.Actor, &item.Action,
			&item.Target, &item.OK, &item.Detail,
		)
		if err != nil {
			return nil, fmt.Errorf("gagal memindai baris audit: %w", err)
		}
		list = append(list, item)
	}

	return list, nil
}