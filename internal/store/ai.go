package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// AIProvider menyimpan detail integrasi LLM pihak ketiga (OpenAI compat, Anthropic, Gemini)
type AIProvider struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Kind      string `json:"kind"` // 'openai_compat', 'anthropic', 'gemini'
	BaseURL   string `json:"base_url"`
	Model     string `json:"model"`
	APIKey    string `json:"api_key"`
	IsDefault bool   `json:"is_default"`
	CreatedAt int64  `json:"created_at"`
	UpdatedAt int64  `json:"updated_at"`
}

// MaskAPIKey menyamarkan kunci API agar tidak bocor ke klien (mis. sk-••••••1234)
func (p *AIProvider) MaskAPIKey() string {
	if len(p.APIKey) <= 8 {
		return "••••••••"
	}
	return p.APIKey[:3] + "••••••••" + p.APIKey[len(p.APIKey)-4:]
}

// AIConversation merepresentasikan satu sesi/utas percakapan pengguna dengan AI
type AIConversation struct {
	ID           int64  `json:"id"`
	UserID       int64  `json:"user_id"`
	Title        string `json:"title"`
	CreatedAt    int64  `json:"created_at"`
	UpdatedAt    int64  `json:"updated_at"`
	MessageCount int    `json:"message_count"`
}

// AIMessage adalah pesan individual dalam sesi percakapan
type AIMessage struct {
	ID             int64  `json:"id"`
	ConversationID int64  `json:"conversation_id"`
	Role           string `json:"role"` // 'user', 'assistant', 'system'
	Content        string `json:"content"`
	ToolName       string `json:"tool_name,omitempty"`
	TS             int64  `json:"ts"`
}

// ListAIProviders mengambil seluruh daftar provider AI yang tersimpan
func (s *Store) ListAIProviders(ctx context.Context) ([]AIProvider, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, kind, base_url, model, api_key, is_default, created_at, updated_at
		 FROM ai_providers ORDER BY is_default DESC, id ASC`)
	if err != nil {
		return nil, fmt.Errorf("gagal mengambil provider AI: %w", err)
	}
	defer rows.Close()

	var providers []AIProvider
	for rows.Next() {
		var p AIProvider
		var defInt int
		if err := rows.Scan(&p.ID, &p.Name, &p.Kind, &p.BaseURL, &p.Model, &p.APIKey, &defInt, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, fmt.Errorf("gagal membaca baris provider: %w", err)
		}
		p.IsDefault = defInt == 1
		providers = append(providers, p)
	}
	return providers, nil
}

// GetAIProvider mengambil provider berdasarkan ID
func (s *Store) GetAIProvider(ctx context.Context, id int64) (*AIProvider, error) {
	var p AIProvider
	var defInt int
	err := s.db.QueryRowContext(ctx,
		`SELECT id, name, kind, base_url, model, api_key, is_default, created_at, updated_at
		 FROM ai_providers WHERE id = ?`, id).
		Scan(&p.ID, &p.Name, &p.Kind, &p.BaseURL, &p.Model, &p.APIKey, &defInt, &p.CreatedAt, &p.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("gagal membaca provider ID %d: %w", id, err)
	}
	p.IsDefault = defInt == 1
	return &p, nil
}

// GetDefaultAIProvider mengambil provider yang berstatus default
func (s *Store) GetDefaultAIProvider(ctx context.Context) (*AIProvider, error) {
	var p AIProvider
	var defInt int
	err := s.db.QueryRowContext(ctx,
		`SELECT id, name, kind, base_url, model, api_key, is_default, created_at, updated_at
		 FROM ai_providers WHERE is_default = 1 LIMIT 1`).
		Scan(&p.ID, &p.Name, &p.Kind, &p.BaseURL, &p.Model, &p.APIKey, &defInt, &p.CreatedAt, &p.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("gagal membaca provider default: %w", err)
	}
	p.IsDefault = defInt == 1
	return &p, nil
}

// CreateAIProvider menambahkan provider baru ke database
func (s *Store) CreateAIProvider(ctx context.Context, p *AIProvider) error {
	now := time.Now().Unix()
	p.CreatedAt = now
	p.UpdatedAt = now

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if p.IsDefault {
		if _, err := tx.ExecContext(ctx, "UPDATE ai_providers SET is_default = 0"); err != nil {
			return fmt.Errorf("gagal mereset status default provider: %w", err)
		}
	} else {
		// Jika belum ada provider lain, jadikan provider pertama ini sebagai default otomatis
		var count int
		_ = tx.QueryRowContext(ctx, "SELECT COUNT(1) FROM ai_providers").Scan(&count)
		if count == 0 {
			p.IsDefault = true
		}
	}

	defInt := 0
	if p.IsDefault {
		defInt = 1
	}

	res, err := tx.ExecContext(ctx,
		`INSERT INTO ai_providers (name, kind, base_url, model, api_key, is_default, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		p.Name, p.Kind, p.BaseURL, p.Model, p.APIKey, defInt, p.CreatedAt, p.UpdatedAt)
	if err != nil {
		return fmt.Errorf("gagal menyimpan provider baru: %w", err)
	}

	id, err := res.LastInsertId()
	if err == nil {
		p.ID = id
	}
	return tx.Commit()
}

// UpdateAIProvider memperbarui konfigurasi provider yang ada
func (s *Store) UpdateAIProvider(ctx context.Context, p *AIProvider) error {
	p.UpdatedAt = time.Now().Unix()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if p.IsDefault {
		if _, err := tx.ExecContext(ctx, "UPDATE ai_providers SET is_default = 0 WHERE id != ?", p.ID); err != nil {
			return fmt.Errorf("gagal mengatur ulang default provider: %w", err)
		}
	}

	defInt := 0
	if p.IsDefault {
		defInt = 1
	}

	query := `UPDATE ai_providers SET name = ?, kind = ?, base_url = ?, model = ?, is_default = ?, updated_at = ?`
	args := []interface{}{p.Name, p.Kind, p.BaseURL, p.Model, defInt, p.UpdatedAt}

	// Hanya perbarui API key jika diisi (agar input kosong tidak menimpa kunci yang tersimpan)
	if p.APIKey != "" {
		query += `, api_key = ? WHERE id = ?`
		args = append(args, p.APIKey, p.ID)
	} else {
		query += ` WHERE id = ?`
		args = append(args, p.ID)
	}

	if _, err := tx.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("gagal memperbarui provider %d: %w", p.ID, err)
	}
	return tx.Commit()
}

// DeleteAIProvider menghapus provider AI dari database
func (s *Store) DeleteAIProvider(ctx context.Context, id int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var wasDefault int
	_ = tx.QueryRowContext(ctx, "SELECT is_default FROM ai_providers WHERE id = ?", id).Scan(&wasDefault)

	if _, err := tx.ExecContext(ctx, "DELETE FROM ai_providers WHERE id = ?", id); err != nil {
		return fmt.Errorf("gagal menghapus provider: %w", err)
	}

	// Jika yang dihapus adalah default, pilih salah satu provider tersisa sebagai default baru
	if wasDefault == 1 {
		_, _ = tx.ExecContext(ctx, "UPDATE ai_providers SET is_default = 1 WHERE id = (SELECT id FROM ai_providers LIMIT 1)")
	}

	return tx.Commit()
}

// SetDefaultAIProvider menjadikan provider tertentu sebagai default aktif
func (s *Store) SetDefaultAIProvider(ctx context.Context, id int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, "UPDATE ai_providers SET is_default = 0"); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "UPDATE ai_providers SET is_default = 1 WHERE id = ?", id); err != nil {
		return err
	}
	return tx.Commit()
}

// ListAIConversations mengambil seluruh daftar percakapan milik pengguna
func (s *Store) ListAIConversations(ctx context.Context, userID int64) ([]AIConversation, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT c.id, c.user_id, COALESCE(c.title, 'Percakapan Baru'), c.created_at, c.updated_at,
		        (SELECT COUNT(1) FROM ai_messages m WHERE m.conversation_id = c.id) as msg_count
		 FROM ai_conversations c
		 WHERE c.user_id = ?
		 ORDER BY c.updated_at DESC, c.id DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("gagal mengambil riwayat percakapan: %w", err)
	}
	defer rows.Close()

	var list []AIConversation
	for rows.Next() {
		var c AIConversation
		if err := rows.Scan(&c.ID, &c.UserID, &c.Title, &c.CreatedAt, &c.UpdatedAt, &c.MessageCount); err != nil {
			return nil, fmt.Errorf("gagal membaca baris percakapan: %w", err)
		}
		if c.UpdatedAt == 0 {
			c.UpdatedAt = c.CreatedAt
		}
		list = append(list, c)
	}
	return list, nil
}

// GetAIConversation mengambil percakapan spesifik milik pengguna
func (s *Store) GetAIConversation(ctx context.Context, id int64, userID int64) (*AIConversation, error) {
	var c AIConversation
	err := s.db.QueryRowContext(ctx,
		`SELECT c.id, c.user_id, COALESCE(c.title, 'Percakapan Baru'), c.created_at, c.updated_at,
		        (SELECT COUNT(1) FROM ai_messages m WHERE m.conversation_id = c.id) as msg_count
		 FROM ai_conversations c
		 WHERE c.id = ? AND c.user_id = ?`, id, userID).
		Scan(&c.ID, &c.UserID, &c.Title, &c.CreatedAt, &c.UpdatedAt, &c.MessageCount)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("gagal membaca percakapan %d: %w", id, err)
	}
	if c.UpdatedAt == 0 {
		c.UpdatedAt = c.CreatedAt
	}
	return &c, nil
}

// CreateAIConversation membuat utas percakapan baru
func (s *Store) CreateAIConversation(ctx context.Context, userID int64, title string) (*AIConversation, error) {
	now := time.Now().Unix()
	if title == "" {
		title = "Percakapan Baru"
	}

	res, err := s.db.ExecContext(ctx,
		"INSERT INTO ai_conversations (user_id, title, created_at, updated_at) VALUES (?, ?, ?, ?)",
		userID, title, now, now)
	if err != nil {
		return nil, fmt.Errorf("gagal membuat percakapan: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}

	return &AIConversation{
		ID:           id,
		UserID:       userID,
		Title:        title,
		CreatedAt:    now,
		UpdatedAt:    now,
		MessageCount: 0,
	}, nil
}

// UpdateAIConversationTitle mengubah judul percakapan
func (s *Store) UpdateAIConversationTitle(ctx context.Context, id int64, userID int64, title string) error {
	_, err := s.db.ExecContext(ctx,
		"UPDATE ai_conversations SET title = ?, updated_at = ? WHERE id = ? AND user_id = ?",
		title, time.Now().Unix(), id, userID)
	return err
}

// TouchAIConversation memperbarui waktu aktivitas percakapan
func (s *Store) TouchAIConversation(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx,
		"UPDATE ai_conversations SET updated_at = ? WHERE id = ?",
		time.Now().Unix(), id)
	return err
}

// DeleteAIConversation menghapus satu utas percakapan dan seluruh pesannya
func (s *Store) DeleteAIConversation(ctx context.Context, id int64, userID int64) error {
	_, err := s.db.ExecContext(ctx,
		"DELETE FROM ai_conversations WHERE id = ? AND user_id = ?",
		id, userID)
	return err
}

// ClearAllAIConversations menghapus seluruh riwayat percakapan pengguna
func (s *Store) ClearAllAIConversations(ctx context.Context, userID int64) error {
	_, err := s.db.ExecContext(ctx,
		"DELETE FROM ai_conversations WHERE user_id = ?",
		userID)
	return err
}

// ListAIMessages mengambil riwayat pesan dalam satu percakapan secara kronologis
func (s *Store) ListAIMessages(ctx context.Context, conversationID int64) ([]AIMessage, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, conversation_id, role, content, COALESCE(tool_name, ''), ts
		 FROM ai_messages
		 WHERE conversation_id = ?
		 ORDER BY ts ASC, id ASC`, conversationID)
	if err != nil {
		return nil, fmt.Errorf("gagal mengambil pesan riwayat: %w", err)
	}
	defer rows.Close()

	var msgs []AIMessage
	for rows.Next() {
		var m AIMessage
		if err := rows.Scan(&m.ID, &m.ConversationID, &m.Role, &m.Content, &m.ToolName, &m.TS); err != nil {
			return nil, fmt.Errorf("gagal membaca baris pesan: %w", err)
		}
		msgs = append(msgs, m)
	}
	return msgs, nil
}

// CreateAIMessage menyimpan pesan baru (baik dari user maupun asisten)
func (s *Store) CreateAIMessage(ctx context.Context, m *AIMessage) error {
	if m.TS == 0 {
		m.TS = time.Now().Unix()
	}

	res, err := s.db.ExecContext(ctx,
		`INSERT INTO ai_messages (conversation_id, role, content, tool_name, ts)
		 VALUES (?, ?, ?, ?, ?)`,
		m.ConversationID, m.Role, m.Content, m.ToolName, m.TS)
	if err != nil {
		return fmt.Errorf("gagal menyimpan pesan AI: %w", err)
	}

	id, err := res.LastInsertId()
	if err == nil {
		m.ID = id
	}

	// Sentuh percakapan agar urutan 'updated_at' terangkat ke paling atas
	_ = s.TouchAIConversation(ctx, m.ConversationID)
	return nil
}
