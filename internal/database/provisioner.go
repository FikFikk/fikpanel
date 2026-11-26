package database

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

var validIdentifier = regexp.MustCompile(`^[a-zA-Z0-9_\-]+$`)

// HostDatabaseInfo merepresentasikan metadata database aktual yang ditemukan di server host
type HostDatabaseInfo struct {
	Name      string
	Owner     string
	SizeBytes int64
	IsSystem  bool
}

// IsValidIdentifier memvalidasi nama database / username agar bebas dari injeksi karakter berbahaya
func IsValidIdentifier(name string) bool {
	return validIdentifier.MatchString(strings.TrimSpace(name))
}

// IsHardProtectedDatabase memeriksa database internal inti yang dilarang keras untuk dihapus
func IsHardProtectedDatabase(engine, name string) bool {
	n := strings.ToLower(strings.TrimSpace(name))
	switch strings.ToLower(engine) {
	case "mysql":
		return n == "mysql" || n == "information_schema" || n == "performance_schema" || n == "sys"
	case "postgresql", "postgres":
		return n == "postgres" || n == "template0" || n == "template1"
	case "mongodb":
		return n == "admin" || n == "local" || n == "config"
	default:
		return false
	}
}

// IsSystemDatabase menandai database bawaan sistem atau utilitas panel (termasuk phpmyadmin)
func IsSystemDatabase(engine, name string) bool {
	if IsHardProtectedDatabase(engine, name) {
		return true
	}
	n := strings.ToLower(strings.TrimSpace(name))
	return strings.ToLower(engine) == "mysql" && n == "phpmyadmin"
}

// mysqlClientBinary memilih biner CLI mariadb/mysql yang tersedia di sistem
func mysqlClientBinary() string {
	if _, err := exec.LookPath("mariadb"); err == nil {
		return "mariadb"
	}
	return "mysql"
}

// buildMysqlCommand membangun perintah CLI MySQL/MariaDB dengan kredensial debian.cnf atau root socket
func buildMysqlCommand(ctx context.Context, extraArgs ...string) *exec.Cmd {
	bin := mysqlClientBinary()
	args := []string{}
	if _, err := os.Stat("/etc/mysql/debian.cnf"); err == nil {
		args = append(args, "--defaults-file=/etc/mysql/debian.cnf")
	} else {
		args = append(args, "-u", "root")
	}
	args = append(args, extraArgs...)
	return exec.CommandContext(ctx, bin, args...)
}

// buildPostgresCommand membangun perintah psql yang berjalan sebagai pengguna OS postgres
func buildPostgresCommand(ctx context.Context, dbName string, extraArgs ...string) *exec.Cmd {
	psqlArgs := []string{"psql", "-v", "ON_ERROR_STOP=1"}
	if strings.TrimSpace(dbName) != "" {
		psqlArgs = append(psqlArgs, "-d", dbName)
	}
	psqlArgs = append(psqlArgs, extraArgs...)

	if _, err := exec.LookPath("runuser"); err == nil {
		args := append([]string{"-u", "postgres", "--"}, psqlArgs...)
		return exec.CommandContext(ctx, "runuser", args...)
	}
	if _, err := exec.LookPath("sudo"); err == nil {
		args := append([]string{"-u", "postgres"}, psqlArgs...)
		return exec.CommandContext(ctx, "sudo", args...)
	}
	// Fallback langsung dengan -U postgres
	directArgs := append([]string{"-U", "postgres"}, psqlArgs[1:]...)
	return exec.CommandContext(ctx, "psql", directArgs...)
}

// executeMysqlQuery mengeksekusi kueri SQL administratif ke instance MySQL lokal
func executeMysqlQuery(ctx context.Context, sqlQuery string) error {
	if runtime.GOOS != "linux" {
		// Pada lingkungan non-Linux / pengujian lokal, lewati eksekusi langsung ke daemon sistem
		return nil
	}

	cmd := buildMysqlCommand(ctx)
	// Alirkan kueri SQL melalui Stdin agar karakter khusus dalam password tidak rusak oleh argument parsing
	cmd.Stdin = strings.NewReader(sqlQuery)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("eksekusi mysql gagal: %w (output: %s)", err, strings.TrimSpace(string(output)))
	}
	return nil
}

// RunMysqlQueryOutput menjalankan query ke MySQL/MariaDB dan mengembalikan output mentah (TSV/batch)
func RunMysqlQueryOutput(ctx context.Context, dbName, sqlQuery string) (string, error) {
	if runtime.GOOS != "linux" {
		return "", fmt.Errorf("eksekusi mysql langsung hanya tersedia di host Linux")
	}
	args := []string{"--batch", "--raw", "-N"}
	if strings.TrimSpace(dbName) != "" {
		args = append(args, dbName)
	}
	cmd := buildMysqlCommand(ctx, args...)
	cmd.Stdin = strings.NewReader(sqlQuery)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s", strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

// RunMysqlQueryWithHeaders menjalankan query ke MySQL/MariaDB dengan baris header kolom (untuk SQL Studio)
func RunMysqlQueryWithHeaders(ctx context.Context, dbName, sqlQuery string) (string, error) {
	if runtime.GOOS != "linux" {
		return "", fmt.Errorf("eksekusi mysql langsung hanya tersedia di host Linux")
	}
	args := []string{"--batch", "--raw"}
	if strings.TrimSpace(dbName) != "" {
		args = append(args, dbName)
	}
	cmd := buildMysqlCommand(ctx, args...)
	cmd.Stdin = strings.NewReader(sqlQuery)
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("%s", msg)
	}
	return string(out), nil
}

// executePostgresQuery mengeksekusi perintah administratif ke instance PostgreSQL lokal
func executePostgresQuery(ctx context.Context, dbName, sqlQuery string) error {
	if runtime.GOOS != "linux" {
		return nil
	}
	cmd := buildPostgresCommand(ctx, dbName)
	cmd.Stdin = strings.NewReader(sqlQuery)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("eksekusi postgresql gagal: %w (output: %s)", err, strings.TrimSpace(string(output)))
	}
	return nil
}

// RunPostgresQueryOutput menjalankan query ke PostgreSQL dan mengembalikan output dipisahkan tab
func RunPostgresQueryOutput(ctx context.Context, dbName, sqlQuery string, includeHeader bool) (string, error) {
	if runtime.GOOS != "linux" {
		return "", fmt.Errorf("eksekusi postgresql langsung hanya tersedia di host Linux")
	}
	args := []string{"-A", "-F", "\t"}
	if !includeHeader {
		args = append(args, "-t")
	}
	cmd := buildPostgresCommand(ctx, dbName, args...)
	cmd.Stdin = strings.NewReader(sqlQuery)
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("%s", msg)
	}
	return string(out), nil
}

// ProvisionDatabase membuat database fisik, pengguna, dan hak akses di server database (MySQL & PostgreSQL)
func ProvisionDatabase(ctx context.Context, engine, dbName, username, password, accessHost string) error {
	dbName = strings.TrimSpace(dbName)
	username = strings.TrimSpace(username)

	if !validIdentifier.MatchString(dbName) {
		return fmt.Errorf("nama database hanya boleh memuat huruf, angka, garis bawah, atau strip")
	}
	if !validIdentifier.MatchString(username) {
		return fmt.Errorf("nama pengguna database hanya boleh memuat huruf, angka, garis bawah, atau strip")
	}

	escapedPass := strings.ReplaceAll(password, "'", "''")

	switch strings.ToLower(engine) {
	case "mysql":
		queries := []string{
			fmt.Sprintf("CREATE DATABASE IF NOT EXISTS `%s` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;", dbName),
			fmt.Sprintf("CREATE USER IF NOT EXISTS '%s'@'localhost' IDENTIFIED BY '%s';", username, escapedPass),
			fmt.Sprintf("ALTER USER '%s'@'localhost' IDENTIFIED BY '%s';", username, escapedPass),
			fmt.Sprintf("GRANT ALL PRIVILEGES ON `%s`.* TO '%s'@'localhost';", dbName, username),
			fmt.Sprintf("CREATE USER IF NOT EXISTS '%s'@'127.0.0.1' IDENTIFIED BY '%s';", username, escapedPass),
			fmt.Sprintf("ALTER USER '%s'@'127.0.0.1' IDENTIFIED BY '%s';", username, escapedPass),
			fmt.Sprintf("GRANT ALL PRIVILEGES ON `%s`.* TO '%s'@'127.0.0.1';", dbName, username),
		}

		// Jika host akses bukan localhost / 127.0.0.1 (misal '%' untuk akses jarak jauh)
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
		return executeMysqlQuery(ctx, strings.Join(queries, " "))

	case "postgresql", "postgres":
		// 1. Buat atau perbarui role pengguna secara idempoten
		roleSQL := fmt.Sprintf(`DO $$
BEGIN
   IF NOT EXISTS (SELECT FROM pg_catalog.pg_roles WHERE rolname = '%s') THEN
      CREATE ROLE "%s" WITH LOGIN PASSWORD '%s';
   ELSE
      ALTER ROLE "%s" WITH LOGIN PASSWORD '%s';
   END IF;
END
$$;`, username, username, escapedPass, username, escapedPass)
		if err := executePostgresQuery(ctx, "postgres", roleSQL); err != nil {
			return err
		}

		// 2. Periksa apakah database sudah ada sebelum mengeksekusi CREATE DATABASE
		existsOut, _ := RunPostgresQueryOutput(ctx, "postgres", fmt.Sprintf("SELECT 1 FROM pg_database WHERE datname = '%s';", dbName), false)
		if strings.TrimSpace(existsOut) != "1" {
			createDBSQL := fmt.Sprintf(`CREATE DATABASE "%s" OWNER "%s" ENCODING 'UTF8';`, dbName, username)
			if err := executePostgresQuery(ctx, "postgres", createDBSQL); err != nil {
				return err
			}
		}

		// 3. Berikan seluruh hak akses database dan skema public
		grantSQL := fmt.Sprintf(`GRANT ALL PRIVILEGES ON DATABASE "%s" TO "%s";`, dbName, username)
		_ = executePostgresQuery(ctx, "postgres", grantSQL)
		_ = executePostgresQuery(ctx, dbName, fmt.Sprintf(`GRANT ALL ON SCHEMA public TO "%s";`, username))
		return nil

	default:
		return nil
	}
}

// DeprovisionDatabase menghapus database fisik dan pengguna terkait dari server (MySQL & PostgreSQL)
func DeprovisionDatabase(ctx context.Context, engine, dbName, username string) error {
	dbName = strings.TrimSpace(dbName)
	username = strings.TrimSpace(username)

	if !validIdentifier.MatchString(dbName) {
		return fmt.Errorf("nama database tidak valid")
	}
	if IsHardProtectedDatabase(engine, dbName) {
		return fmt.Errorf("database sistem inti '%s' tidak boleh dihapus", dbName)
	}

	switch strings.ToLower(engine) {
	case "mysql":
		queries := []string{
			fmt.Sprintf("DROP DATABASE IF EXISTS `%s`;", dbName),
		}
		if username != "" && validIdentifier.MatchString(username) && strings.ToLower(username) != "root" {
			queries = append(queries,
				fmt.Sprintf("DROP USER IF EXISTS '%s'@'localhost';", username),
				fmt.Sprintf("DROP USER IF EXISTS '%s'@'127.0.0.1';", username),
				fmt.Sprintf("DROP USER IF EXISTS '%s'@'%%';", username),
			)
		}
		queries = append(queries, "FLUSH PRIVILEGES;")
		return executeMysqlQuery(ctx, strings.Join(queries, " "))

	case "postgresql", "postgres":
		// Putus seluruh koneksi aktif ke database target terlebih dahulu agar DROP DATABASE berhasil
		termSQL := fmt.Sprintf(`SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = '%s' AND pid <> pg_backend_pid();`, dbName)
		_ = executePostgresQuery(ctx, "postgres", termSQL)

		if err := executePostgresQuery(ctx, "postgres", fmt.Sprintf(`DROP DATABASE IF EXISTS "%s";`, dbName)); err != nil {
			return err
		}
		if username != "" && validIdentifier.MatchString(username) && strings.ToLower(username) != "postgres" {
			_ = executePostgresQuery(ctx, "postgres", fmt.Sprintf(`DROP ROLE IF EXISTS "%s";`, username))
		}
		return nil

	default:
		return nil
	}
}

// UpdateUserPassword mengubah kata sandi pengguna di engine database fisik (MySQL & PostgreSQL)
func UpdateUserPassword(ctx context.Context, engine, username, newPassword string) error {
	username = strings.TrimSpace(username)
	if !validIdentifier.MatchString(username) {
		return fmt.Errorf("nama pengguna database tidak valid")
	}
	escapedPass := strings.ReplaceAll(newPassword, "'", "''")

	switch strings.ToLower(engine) {
	case "mysql":
		queries := []string{
			fmt.Sprintf("CREATE USER IF NOT EXISTS '%s'@'localhost' IDENTIFIED BY '%s';", username, escapedPass),
			fmt.Sprintf("ALTER USER '%s'@'localhost' IDENTIFIED BY '%s';", username, escapedPass),
			fmt.Sprintf("CREATE USER IF NOT EXISTS '%s'@'127.0.0.1' IDENTIFIED BY '%s';", username, escapedPass),
			fmt.Sprintf("ALTER USER '%s'@'127.0.0.1' IDENTIFIED BY '%s';", username, escapedPass),
			fmt.Sprintf("ALTER USER IF EXISTS '%s'@'%%' IDENTIFIED BY '%s';", username, escapedPass),
			"FLUSH PRIVILEGES;",
		}
		return executeMysqlQuery(ctx, strings.Join(queries, " "))

	case "postgresql", "postgres":
		sql := fmt.Sprintf(`ALTER ROLE "%s" WITH PASSWORD '%s';`, username, escapedPass)
		return executePostgresQuery(ctx, "postgres", sql)

	default:
		return nil
	}
}

// UpdateRootPassword memperbarui kata sandi akun administrator utama (root / postgres / redis requirepass)
func UpdateRootPassword(ctx context.Context, engine, newPassword string) error {
	newPassword = strings.TrimSpace(newPassword)
	if len(newPassword) < 6 {
		return fmt.Errorf("kata sandi root minimal 6 karakter")
	}
	if runtime.GOOS != "linux" {
		return nil
	}

	escapedPass := strings.ReplaceAll(newPassword, "'", "''")
	switch strings.ToLower(engine) {
	case "mysql":
		queries := []string{
			fmt.Sprintf("ALTER USER 'root'@'localhost' IDENTIFIED BY '%s';", escapedPass),
			fmt.Sprintf("ALTER USER IF EXISTS 'root'@'127.0.0.1' IDENTIFIED BY '%s';", escapedPass),
			"FLUSH PRIVILEGES;",
		}
		return executeMysqlQuery(ctx, strings.Join(queries, " "))

	case "postgresql", "postgres":
		return executePostgresQuery(ctx, "postgres", fmt.Sprintf("ALTER USER postgres WITH PASSWORD '%s';", escapedPass))

	case "redis":
		// Terapkan requirepass secara langsung di runtime Redis dan persist ke redis.conf
		cmd := exec.CommandContext(ctx, "redis-cli", "CONFIG", "SET", "requirepass", newPassword)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("gagal menerapkan requirepass Redis: %w (%s)", err, strings.TrimSpace(string(out)))
		}
		cfgPath := ResolveConfigPath("redis")
		if raw, err := os.ReadFile(cfgPath); err == nil {
			lines := strings.Split(string(raw), "\n")
			updated := false
			for i, line := range lines {
				trimmed := strings.TrimSpace(line)
				if strings.HasPrefix(trimmed, "requirepass ") || strings.HasPrefix(trimmed, "# requirepass ") {
					lines[i] = fmt.Sprintf("requirepass %s", newPassword)
					updated = true
					break
				}
			}
			if !updated {
				lines = append(lines, fmt.Sprintf("requirepass %s", newPassword))
			}
			_ = os.WriteFile(cfgPath, []byte(strings.Join(lines, "\n")), 0640)
		}
		return nil

	default:
		return fmt.Errorf("pembaruan kata sandi root belum didukung untuk engine %s", engine)
	}
}

// ListHostDatabases memindai seluruh database fisik yang benar-benar ada di server host
func ListHostDatabases(ctx context.Context, engine string) ([]HostDatabaseInfo, error) {
	if runtime.GOOS != "linux" {
		return []HostDatabaseInfo{}, nil
	}

	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	switch strings.ToLower(engine) {
	case "mysql":
		q := `SELECT s.SCHEMA_NAME, COALESCE(SUM(t.DATA_LENGTH + t.INDEX_LENGTH), 0)
FROM information_schema.SCHEMATA s
LEFT JOIN information_schema.TABLES t ON s.SCHEMA_NAME = t.TABLE_SCHEMA
WHERE s.SCHEMA_NAME NOT IN ('information_schema', 'performance_schema', 'mysql', 'sys')
GROUP BY s.SCHEMA_NAME
ORDER BY s.SCHEMA_NAME ASC;`
		out, err := RunMysqlQueryOutput(dbCtx, "", q)
		if err != nil {
			return nil, fmt.Errorf("gagal memindai daftar database MySQL: %w", err)
		}
		var list []HostDatabaseInfo
		scanner := bufio.NewScanner(strings.NewReader(out))
		for scanner.Scan() {
			parts := strings.Split(scanner.Text(), "\t")
			if len(parts) < 2 {
				continue
			}
			name := strings.TrimSpace(parts[0])
			if name == "" {
				continue
			}
			sz, _ := strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 64)
			list = append(list, HostDatabaseInfo{
				Name:      name,
				Owner:     name,
				SizeBytes: sz,
				IsSystem:  IsSystemDatabase("mysql", name),
			})
		}
		return list, nil

	case "postgresql", "postgres":
		q := `SELECT d.datname, pg_get_userbyid(d.datdba), pg_database_size(d.datname)
FROM pg_database d
WHERE d.datistemplate = false AND d.datname <> 'postgres'
ORDER BY d.datname ASC;`
		out, err := RunPostgresQueryOutput(dbCtx, "postgres", q, false)
		if err != nil {
			return nil, fmt.Errorf("gagal memindai daftar database PostgreSQL: %w", err)
		}
		var list []HostDatabaseInfo
		scanner := bufio.NewScanner(strings.NewReader(out))
		for scanner.Scan() {
			parts := strings.Split(scanner.Text(), "\t")
			if len(parts) < 3 {
				continue
			}
			name := strings.TrimSpace(parts[0])
			owner := strings.TrimSpace(parts[1])
			if name == "" {
				continue
			}
			sz, _ := strconv.ParseInt(strings.TrimSpace(parts[2]), 10, 64)
			list = append(list, HostDatabaseInfo{
				Name:      name,
				Owner:     owner,
				SizeBytes: sz,
				IsSystem:  IsSystemDatabase("postgresql", name),
			})
		}
		return list, nil

	default:
		return []HostDatabaseInfo{}, nil
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

		bin := mysqlClientBinary()
		query := "SELECT table_schema, COALESCE(SUM(data_length + index_length), 0) FROM information_schema.TABLES GROUP BY table_schema;"
		cmd := buildMysqlCommand(dbCtx, "-N", "-e", query)

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

		out, err := RunPostgresQueryOutput(dbCtx, "postgres", "SELECT datname, pg_database_size(datname) FROM pg_database;", false)
		if err == nil && strings.TrimSpace(out) != "" {
			scanner := bufio.NewScanner(strings.NewReader(out))
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
