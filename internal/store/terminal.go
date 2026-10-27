package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// TerminalServer merepresentasikan entitas server SSH jarak jauh yang tersimpan
type TerminalServer struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	Host       string `json:"host"`
	Port       int    `json:"port"`
	Username   string `json:"username"`
	AuthType   string `json:"auth_type"` // 'password' atau 'key'
	Password   string `json:"password,omitempty"`
	PrivateKey string `json:"private_key,omitempty"`
	Passphrase string `json:"passphrase,omitempty"`
	Remarks    string `json:"remarks"`
	CreatedAt  int64  `json:"created_at"`
	UpdatedAt  int64  `json:"updated_at"`
}

// TerminalCommand merepresentasikan pintasan perintah terminal yang dapat langsung dieksekusi atau ditempel
type TerminalCommand struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Content   string `json:"content"`
	CreatedAt int64  `json:"created_at"`
	UpdatedAt int64  `json:"updated_at"`
}

// ListTerminalServers mengambil daftar seluruh remote server yang tersimpan
func (s *Store) ListTerminalServers(ctx context.Context) ([]TerminalServer, error) {
	query := `SELECT id, name, host, port, username, auth_type, password, private_key, passphrase, coalesce(remarks, ''), created_at, updated_at 
	          FROM terminal_servers ORDER BY id DESC`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list terminal servers: %w", err)
	}
	defer rows.Close()

	var servers []TerminalServer
	for rows.Next() {
		var srv TerminalServer
		if err := rows.Scan(
			&srv.ID, &srv.Name, &srv.Host, &srv.Port, &srv.Username,
			&srv.AuthType, &srv.Password, &srv.PrivateKey, &srv.Passphrase,
			&srv.Remarks, &srv.CreatedAt, &srv.UpdatedAt,
		); err != nil {
			return nil, err
		}
		servers = append(servers, srv)
	}
	return servers, nil
}

// GetTerminalServer mengambil satu server berdasarkan ID
func (s *Store) GetTerminalServer(ctx context.Context, id int64) (*TerminalServer, error) {
	query := `SELECT id, name, host, port, username, auth_type, password, private_key, passphrase, coalesce(remarks, ''), created_at, updated_at 
	          FROM terminal_servers WHERE id = ?`
	row := s.db.QueryRowContext(ctx, query, id)

	var srv TerminalServer
	if err := row.Scan(
		&srv.ID, &srv.Name, &srv.Host, &srv.Port, &srv.Username,
		&srv.AuthType, &srv.Password, &srv.PrivateKey, &srv.Passphrase,
		&srv.Remarks, &srv.CreatedAt, &srv.UpdatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get terminal server: %w", err)
	}
	return &srv, nil
}

// CreateTerminalServer menyimpan server baru ke database
func (s *Store) CreateTerminalServer(ctx context.Context, srv *TerminalServer) (int64, error) {
	now := time.Now().Unix()
	srv.CreatedAt = now
	srv.UpdatedAt = now
	if srv.Port <= 0 {
		srv.Port = 22
	}
	if srv.Username == "" {
		srv.Username = "root"
	}
	if srv.AuthType == "" {
		srv.AuthType = "password"
	}
	if srv.Name == "" {
		srv.Name = srv.Host
	}

	query := `INSERT INTO terminal_servers (name, host, port, username, auth_type, password, private_key, passphrase, remarks, created_at, updated_at)
	          VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	res, err := s.db.ExecContext(ctx, query,
		srv.Name, srv.Host, srv.Port, srv.Username, srv.AuthType,
		srv.Password, srv.PrivateKey, srv.Passphrase, srv.Remarks,
		srv.CreatedAt, srv.UpdatedAt,
	)
	if err != nil {
		return 0, fmt.Errorf("create terminal server: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	srv.ID = id
	return id, nil
}

// UpdateTerminalServer memperbarui data server SSH yang ada
func (s *Store) UpdateTerminalServer(ctx context.Context, srv *TerminalServer) error {
	srv.UpdatedAt = time.Now().Unix()
	if srv.Port <= 0 {
		srv.Port = 22
	}
	if srv.Username == "" {
		srv.Username = "root"
	}
	if srv.AuthType == "" {
		srv.AuthType = "password"
	}
	if srv.Name == "" {
		srv.Name = srv.Host
	}

	query := `UPDATE terminal_servers 
	          SET name = ?, host = ?, port = ?, username = ?, auth_type = ?, password = ?, private_key = ?, passphrase = ?, remarks = ?, updated_at = ?
	          WHERE id = ?`
	_, err := s.db.ExecContext(ctx, query,
		srv.Name, srv.Host, srv.Port, srv.Username, srv.AuthType,
		srv.Password, srv.PrivateKey, srv.Passphrase, srv.Remarks, srv.UpdatedAt,
		srv.ID,
	)
	if err != nil {
		return fmt.Errorf("update terminal server: %w", err)
	}
	return nil
}

// DeleteTerminalServer menghapus server berdasarkan ID
func (s *Store) DeleteTerminalServer(ctx context.Context, id int64) error {
	query := `DELETE FROM terminal_servers WHERE id = ?`
	_, err := s.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete terminal server: %w", err)
	}
	return nil
}

// ListTerminalCommands mengambil daftar seluruh pintasan perintah terminal
func (s *Store) ListTerminalCommands(ctx context.Context) ([]TerminalCommand, error) {
	query := `SELECT id, name, content, created_at, updated_at FROM terminal_commands ORDER BY id ASC`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list terminal commands: %w", err)
	}
	defer rows.Close()

	var commands []TerminalCommand
	for rows.Next() {
		var c TerminalCommand
		if err := rows.Scan(&c.ID, &c.Name, &c.Content, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		commands = append(commands, c)
	}
	return commands, nil
}

// GetTerminalCommand mengambil satu pintasan perintah berdasarkan ID
func (s *Store) GetTerminalCommand(ctx context.Context, id int64) (*TerminalCommand, error) {
	query := `SELECT id, name, content, created_at, updated_at FROM terminal_commands WHERE id = ?`
	row := s.db.QueryRowContext(ctx, query, id)

	var c TerminalCommand
	if err := row.Scan(&c.ID, &c.Name, &c.Content, &c.CreatedAt, &c.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get terminal command: %w", err)
	}
	return &c, nil
}

// CreateTerminalCommand membuat pintasan perintah baru
func (s *Store) CreateTerminalCommand(ctx context.Context, c *TerminalCommand) (int64, error) {
	now := time.Now().Unix()
	c.CreatedAt = now
	c.UpdatedAt = now

	query := `INSERT INTO terminal_commands (name, content, created_at, updated_at) VALUES (?, ?, ?, ?)`
	res, err := s.db.ExecContext(ctx, query, c.Name, c.Content, c.CreatedAt, c.UpdatedAt)
	if err != nil {
		return 0, fmt.Errorf("create terminal command: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	c.ID = id
	return id, nil
}

// UpdateTerminalCommand memperbarui pintasan perintah yang ada
func (s *Store) UpdateTerminalCommand(ctx context.Context, c *TerminalCommand) error {
	c.UpdatedAt = time.Now().Unix()
	query := `UPDATE terminal_commands SET name = ?, content = ?, updated_at = ? WHERE id = ?`
	_, err := s.db.ExecContext(ctx, query, c.Name, c.Content, c.UpdatedAt, c.ID)
	if err != nil {
		return fmt.Errorf("update terminal command: %w", err)
	}
	return nil
}

// DeleteTerminalCommand menghapus pintasan perintah berdasarkan ID
func (s *Store) DeleteTerminalCommand(ctx context.Context, id int64) error {
	query := `DELETE FROM terminal_commands WHERE id = ?`
	_, err := s.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete terminal command: %w", err)
	}
	return nil
}
