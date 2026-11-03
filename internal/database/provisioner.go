package database

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"time"
)

var validIdentifier = regexp.MustCompile(`^[a-zA-Z0-9_\-]+$`)

// executeMysqlQuery mengeksekusi kueri SQL administratif ke instance MySQL lokal
func executeMysqlQuery(ctx context.Context, sqlQuery string) error {
	if runtime.GOOS != "linux" {
		// Pada lingkungan non-Linux / pengujian lokal, lewati eksekusi langsung ke daemon sistem
		return nil
	}

	// Cari argumen eksekusi biner mysql terbaik
	var cmd *exec.Cmd

	// Cek apakah berkas debian.cnf tersedia (standar Debian / Ubuntu)
	if _, err := os.Stat("/etc/mysql/debian.cnf"); err == nil {
		cmd = exec.CommandContext(ctx, "mysql", "--defaults-file=/etc/mysql/debian.cnf")
	} else {
		// Fallback ke koneksi root via UNIX socket auth_socket bawaan
		cmd = exec.CommandContext(ctx, "mysql", "-u", "root")
	}

	// Alirkan kueri SQL melalui Stdin agar karakter khusus dalam password tidak rusak oleh argument parsing
	cmd.Stdin = strings.NewReader(sqlQuery)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("eksekusi mysql gagal: %w (output: %s)", err, strings.TrimSpace(string(output)))
	}

	return nil
}

// ProvisionDatabase membuat database fisik, pengguna, dan hak akses di server database
func ProvisionDatabase(ctx context.Context, engine, dbName, username, password, accessHost string) error {
	dbName = strings.TrimSpace(dbName)
	username = strings.TrimSpace(username)

	if !validIdentifier.MatchString(dbName) {
		return fmt.Errorf("nama database hanya boleh memuat huruf, angka, garis bawah, atau strip")
	}
	if !validIdentifier.MatchString(username) {
		return fmt.Errorf("nama pengguna database hanya boleh memuat huruf, angka, garis bawah, atau strip")
	}

	switch strings.ToLower(engine) {
	case "mysql":
		// Escape karakter single quote pada password
		escapedPass := strings.ReplaceAll(password, "'", "''")

		// Siapkan kueri pembuatan skema dan hak akses lengkap
		queries := []string{
			fmt.Sprintf("CREATE DATABASE IF NOT EXISTS `%s` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;", dbName),
			fmt.Sprintf("CREATE USER IF NOT EXISTS '%s'@'localhost' IDENTIFIED BY '%s';", username, escapedPass),
			fmt.Sprintf("ALTER USER '%s'@'localhost' IDENTIFIED BY '%s';", username, escapedPass),
			fmt.Sprintf("GRANT ALL PRIVILEGES ON `%s`.* TO '%s'@'localhost';", dbName, username),
			fmt.Sprintf("CREATE USER IF NOT EXISTS '%s'@'127.0.0.1' IDENTIFIED BY '%s';", username, escapedPass),
			fmt.Sprintf("ALTER USER '%s'@'127.0.0.1' IDENTIFIED BY '%s';", username, escapedPass),
			fmt.Sprintf("GRANT ALL PRIVILEGES ON `%s`.* TO '%s'@'127.0.0.1';", dbName, username),
		}

		// Jika host akses bukan localhost / 127.0.0.1 (misal '%' untuk akses publik)
		cleanHost := strings.TrimSpace(accessHost)
		if cleanHost != "" && cleanHost != "localhost" && cleanHost != "127.0.0.1" {
			escapedHost := strings.ReplaceAll(cleanHost, "'", "''")
			queries = append(queries,
				fmt.Sprintf("CREATE USER IF NOT EXISTS '%s'@'%s' IDENTIFIED BY '%s';", username, escapedHost, escapedPass),
				fmt.Sprintf("ALTER USER '%s'@'%s' IDENTIFIED BY '%s';", username, escapedHost, escapedPass),
				fmt.Sprintf("GRANT ALL PRIVILEGES ON `%s`.* TO '%s'@'%s';", dbName, username, escapedHost),
			)
		}

		queries = append(queries, "FLUSH PRIVILEGES;")
		fullSQL := strings.Join(queries, " ")

		return executeMysqlQuery(ctx, fullSQL)

	default:
		// Engine lain saat ini belum memerlukan provisioning socket langsung
		return nil
	}
}

// DeprovisionDatabase menghapus database fisik dan pengguna terkait dari server
func DeprovisionDatabase(ctx context.Context, engine, dbName, username string) error {
	dbName = strings.TrimSpace(dbName)
	username = strings.TrimSpace(username)

	if !validIdentifier.MatchString(dbName) {
		return fmt.Errorf("nama database tidak valid")
	}

	switch strings.ToLower(engine) {
	case "mysql":
		queries := []string{
			fmt.Sprintf("DROP DATABASE IF EXISTS `%s`;", dbName),
		}
		if username != "" && validIdentifier.MatchString(username) {
			queries = append(queries,
				fmt.Sprintf("DROP USER IF EXISTS '%s'@'localhost';", username),
				fmt.Sprintf("DROP USER IF EXISTS '%s'@'127.0.0.1';", username),
				fmt.Sprintf("DROP USER IF EXISTS '%s'@'%%';", username),
			)
		}
		queries = append(queries, "FLUSH PRIVILEGES;")
		fullSQL := strings.Join(queries, " ")

		return executeMysqlQuery(ctx, fullSQL)

	default:
		return nil
	}
}

// UpdateUserPassword mengubah kata sandi pengguna di engine database fisik
func UpdateUserPassword(ctx context.Context, engine, username, newPassword string) error {
	username = strings.TrimSpace(username)
	if !validIdentifier.MatchString(username) {
		return fmt.Errorf("nama pengguna database tidak valid")
	}

	switch strings.ToLower(engine) {
	case "mysql":
		escapedPass := strings.ReplaceAll(newPassword, "'", "''")
		queries := []string{
			fmt.Sprintf("CREATE USER IF NOT EXISTS '%s'@'localhost' IDENTIFIED BY '%s';", username, escapedPass),
			fmt.Sprintf("ALTER USER '%s'@'localhost' IDENTIFIED BY '%s';", username, escapedPass),
			fmt.Sprintf("CREATE USER IF NOT EXISTS '%s'@'127.0.0.1' IDENTIFIED BY '%s';", username, escapedPass),
			fmt.Sprintf("ALTER USER '%s'@'127.0.0.1' IDENTIFIED BY '%s';", username, escapedPass),
			fmt.Sprintf("ALTER USER IF EXISTS '%s'@'%%' IDENTIFIED BY '%s';", username, escapedPass),
			"FLUSH PRIVILEGES;",
		}
		fullSQL := strings.Join(queries, " ")

		return executeMysqlQuery(ctx, fullSQL)

	default:
		return nil
	}
}

// GetDatabaseSizes menghitung ukuran riil masing-masing basis data dalam satuan byte
func GetDatabaseSizes(ctx context.Context, engine string) (map[string]int64, error) {
	sizes := make(map[string]int64)
	if runtime.GOOS != "linux" {
		return sizes, nil
	}

	switch strings.ToLower(engine) {
	case "mysql":
		dbCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()

		bin := "mariadb"
		if _, err := exec.LookPath("mariadb"); err != nil {
			bin = "mysql"
		}

		query := "SELECT table_schema, COALESCE(SUM(data_length + index_length), 0) FROM information_schema.TABLES GROUP BY table_schema;"
		var cmd *exec.Cmd
		if _, err := os.Stat("/etc/mysql/debian.cnf"); err == nil {
			cmd = exec.CommandContext(dbCtx, bin, "--defaults-file=/etc/mysql/debian.cnf", "-N", "-e", query)
		} else {
			cmd = exec.CommandContext(dbCtx, bin, "-u", "root", "-N", "-e", query)
		}

		out, err := cmd.Output()
		if err != nil || len(out) == 0 {
			// Percobaan cadangan via UNIX socket lokal
			for _, sock := range []string{"/var/run/mysqld/mysqld.sock", "/run/mysqld/mysqld.sock", "/tmp/mysql.sock"} {
				if _, errStat := os.Stat(sock); errStat == nil {
					retryCmd := exec.CommandContext(dbCtx, bin, "-u", "root", fmt.Sprintf("--socket=%s", sock), "-N", "-e", query)
					if retryOut, errRetry := retryCmd.Output(); errRetry == nil && len(retryOut) > 0 {
						out = retryOut
						err = nil
						break
					}
				}
			}
		}

		if err != nil || len(out) == 0 {
			return sizes, nil
		}

		scanner := bufio.NewScanner(strings.NewReader(string(out)))
		for scanner.Scan() {
			fields := strings.Fields(scanner.Text())
			if len(fields) >= 2 {
				dbName := fields[0]
				var sz int64
				if _, parseErr := fmt.Sscanf(fields[1], "%d", &sz); parseErr == nil {
					sizes[dbName] = sz
				}
			}
		}
		return sizes, nil

	case "postgresql", "postgres":
		dbCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()

		cmd := exec.CommandContext(dbCtx, "psql", "-U", "postgres", "-t", "-A", "-F", "\t", "-c", "SELECT datname, pg_database_size(datname) FROM pg_database;")
		out, err := cmd.Output()
		if err == nil && len(out) > 0 {
			scanner := bufio.NewScanner(strings.NewReader(string(out)))
			for scanner.Scan() {
				fields := strings.Split(scanner.Text(), "\t")
				if len(fields) >= 2 {
					dbName := strings.TrimSpace(fields[0])
					var sz int64
					if _, parseErr := fmt.Sscanf(strings.TrimSpace(fields[1]), "%d", &sz); parseErr == nil {
						sizes[dbName] = sz
					}
				}
			}
		}
		return sizes, nil

	default:
		return sizes, nil
	}
}
