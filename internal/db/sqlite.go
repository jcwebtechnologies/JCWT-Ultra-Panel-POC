package db

import (
	cryptoRand "crypto/rand"
	"database/sql"
	_ "embed"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
)

//go:embed schema.sql
var schemaSQL string

// DB wraps the SQLite connection
type DB struct {
	Conn *sql.DB
}

// Open opens or creates the SQLite database
func Open(dataDir string) (*DB, error) {
	if err := os.MkdirAll(dataDir, 0750); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}

	dbPath := filepath.Join(dataDir, "panel.db")
	conn, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_foreign_keys=on")
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	conn.SetMaxOpenConns(1) // SQLite single-writer

	if _, err := conn.Exec(schemaSQL); err != nil {
		return nil, fmt.Errorf("run schema: %w", err)
	}

	// Simple migrations for existing databases
	conn.Exec("ALTER TABLE sites ADD COLUMN site_type TEXT NOT NULL DEFAULT 'php'")
	conn.Exec("ALTER TABLE sites ADD COLUMN proxy_url TEXT DEFAULT ''")
	conn.Exec("ALTER TABLE sites ADD COLUMN basic_auth_enabled INTEGER DEFAULT 0")
	conn.Exec("ALTER TABLE sites ADD COLUMN basic_auth_users TEXT DEFAULT ''")
	conn.Exec("ALTER TABLE panel_settings ADD COLUMN recaptcha_site_key TEXT DEFAULT ''")
	conn.Exec("ALTER TABLE panel_settings ADD COLUMN recaptcha_secret_key TEXT DEFAULT ''")
	conn.Exec("ALTER TABLE panel_settings ADD COLUMN timezone TEXT DEFAULT 'UTC'")
	conn.Exec("ALTER TABLE admin_users ADD COLUMN role TEXT DEFAULT 'admin'")
	conn.Exec("ALTER TABLE admin_users ADD COLUMN email TEXT DEFAULT ''")
	conn.Exec("ALTER TABLE sites ADD COLUMN delete_protection INTEGER DEFAULT 0")
	conn.Exec("ALTER TABLE admin_users ADD COLUMN totp_secret TEXT DEFAULT ''")

	// Add token column to sites
	conn.Exec("ALTER TABLE sites ADD COLUMN token TEXT DEFAULT ''")

	// Add download_token column to backups
	conn.Exec("ALTER TABLE backups ADD COLUMN download_token TEXT DEFAULT ''")
	conn.Exec("ALTER TABLE backups ADD COLUMN download_token_expires DATETIME DEFAULT NULL")

	// Add privilege_level column to db_users
	conn.Exec("ALTER TABLE db_users ADD COLUMN privilege_level TEXT DEFAULT 'full'")
	// Migrate any existing 'administrator' users to 'full'
	conn.Exec("UPDATE db_users SET privilege_level = 'full' WHERE privilege_level = 'administrator'")

	// Add access_log and error_log columns to sites
	conn.Exec("ALTER TABLE sites ADD COLUMN access_log INTEGER DEFAULT 1")
	conn.Exec("ALTER TABLE sites ADD COLUMN error_log INTEGER DEFAULT 1")

	// Generate tokens for any existing sites that don't have one
	rows, _ := conn.Query("SELECT id FROM sites WHERE token = '' OR token IS NULL")
	if rows != nil {
		var ids []int64
		for rows.Next() {
			var id int64
			rows.Scan(&id)
			ids = append(ids, id)
		}
		rows.Close()
		for _, id := range ids {
			b := make([]byte, 16)
			cryptoRand.Read(b)
			token := hex.EncodeToString(b)
			conn.Exec("UPDATE sites SET token = ? WHERE id = ?", token, id)
		}
	}

	return &DB{Conn: conn}, nil
}

// Close closes the database connection
func (d *DB) Close() error {
	return d.Conn.Close()
}

// --- Admin User queries ---

func (d *DB) GetAdminByUsername(username string) (int64, string, string, error) {
	var id int64
	var hash, role string
	err := d.Conn.QueryRow("SELECT id, password_hash, COALESCE(role,'admin') FROM admin_users WHERE username = ?", username).Scan(&id, &hash, &role)
	return id, hash, role, err
}

func (d *DB) CreateAdmin(username, passwordHash string) error {
	_, err := d.Conn.Exec("INSERT INTO admin_users (username, password_hash, role) VALUES (?, ?, 'admin')", username, passwordHash)
	return err
}

func (d *DB) CreateAdminWithRole(username, passwordHash, role, email string) error {
	_, err := d.Conn.Exec("INSERT INTO admin_users (username, password_hash, role, email) VALUES (?, ?, ?, ?)", username, passwordHash, role, email)
	return err
}

func (d *DB) ListAdminUsers() ([]map[string]interface{}, error) {
	rows, err := d.Conn.Query("SELECT id, username, COALESCE(role,'admin'), COALESCE(email,''), created_at FROM admin_users ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var users []map[string]interface{}
	for rows.Next() {
		var id int64
		var username, role, email, created string
		if err := rows.Scan(&id, &username, &role, &email, &created); err != nil {
			return nil, err
		}
		users = append(users, map[string]interface{}{
			"id": id, "username": username, "role": role, "email": email, "created_at": created,
		})
	}
	return users, nil
}

func (d *DB) UpdateAdminUser(id int64, role, email string) error {
	_, err := d.Conn.Exec("UPDATE admin_users SET role = ?, email = ? WHERE id = ?", role, email, id)
	return err
}

func (d *DB) UpdateAdminPassword(id int64, passwordHash string) error {
	_, err := d.Conn.Exec("UPDATE admin_users SET password_hash = ? WHERE id = ?", passwordHash, id)
	return err
}

func (d *DB) DeleteAdminUser(id int64) error {
	_, err := d.Conn.Exec("DELETE FROM admin_users WHERE id = ?", id)
	return err
}

func (d *DB) AdminCount() (int, error) {
	var count int
	err := d.Conn.QueryRow("SELECT COUNT(*) FROM admin_users").Scan(&count)
	return count, err
}

// --- TOTP queries ---

func (d *DB) GetTOTPSecret(userID int64) (string, error) {
	var secret string
	err := d.Conn.QueryRow("SELECT COALESCE(totp_secret, '') FROM admin_users WHERE id = ?", userID).Scan(&secret)
	return secret, err
}

func (d *DB) SetTOTPSecret(userID int64, secret string) error {
	_, err := d.Conn.Exec("UPDATE admin_users SET totp_secret = ? WHERE id = ?", secret, userID)
	return err
}

// --- Site queries ---

func (d *DB) ListSites() ([]map[string]interface{}, error) {
	rows, err := d.Conn.Query("SELECT id, COALESCE(token,''), domain, aliases, system_user, site_type, php_version, proxy_url, web_root, ssl_type, created_at FROM sites ORDER BY id DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sites []map[string]interface{}
	for rows.Next() {
		var id int64
		var token, domain, aliases, sysUser, siteType, phpVer, proxyUrl, webRoot, sslType, createdAt string
		if err := rows.Scan(&id, &token, &domain, &aliases, &sysUser, &siteType, &phpVer, &proxyUrl, &webRoot, &sslType, &createdAt); err != nil {
			return nil, err
		}
		sites = append(sites, map[string]interface{}{
			"id":          id,
			"token":       token,
			"domain":      domain,
			"aliases":     aliases,
			"system_user": sysUser,
			"site_type":   siteType,
			"php_version": phpVer,
			"proxy_url":   proxyUrl,
			"web_root":    webRoot,
			"ssl_type":    sslType,
			"created_at":  createdAt,
		})
	}
	return sites, nil
}

func (d *DB) GetSite(id int64) (map[string]interface{}, error) {
	var token, domain, aliases, sysUser, siteType, phpVer, proxyUrl, webRoot, sslType, certPath, keyPath, createdAt string
	var basicAuthEnabled, deleteProtection int
	var basicAuthUsers string
	var accessLog, errorLog int
	err := d.Conn.QueryRow("SELECT COALESCE(token,''), domain, aliases, system_user, site_type, php_version, proxy_url, web_root, ssl_type, ssl_cert_path, ssl_key_path, created_at, COALESCE(basic_auth_enabled,0), COALESCE(basic_auth_users,''), COALESCE(delete_protection,0), COALESCE(access_log,1), COALESCE(error_log,1) FROM sites WHERE id = ?", id).
		Scan(&token, &domain, &aliases, &sysUser, &siteType, &phpVer, &proxyUrl, &webRoot, &sslType, &certPath, &keyPath, &createdAt, &basicAuthEnabled, &basicAuthUsers, &deleteProtection, &accessLog, &errorLog)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"id": id, "token": token, "domain": domain, "aliases": aliases, "system_user": sysUser,
		"site_type": siteType, "php_version": phpVer, "proxy_url": proxyUrl, "web_root": webRoot,
		"ssl_type": sslType, "ssl_cert_path": certPath, "ssl_key_path": keyPath, "created_at": createdAt,
		"basic_auth_enabled": basicAuthEnabled, "basic_auth_users": basicAuthUsers,
		"delete_protection": deleteProtection,
		"access_log": accessLog, "error_log": errorLog,
	}, nil
}

func (d *DB) GetSiteByToken(token string) (map[string]interface{}, error) {
	var id int64
	err := d.Conn.QueryRow("SELECT id FROM sites WHERE token = ?", token).Scan(&id)
	if err != nil {
		return nil, err
	}
	return d.GetSite(id)
}

func (d *DB) CreateSite(domain, aliases, sysUser, siteType, phpVersion, proxyUrl, webRoot string) (int64, error) {
	// Generate random token
	b := make([]byte, 16)
	cryptoRand.Read(b)
	token := hex.EncodeToString(b)

	res, err := d.Conn.Exec(
		"INSERT INTO sites (token, domain, aliases, system_user, site_type, php_version, proxy_url, web_root) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
		token, domain, aliases, sysUser, siteType, phpVersion, proxyUrl, webRoot,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (d *DB) UpdateSite(id int64, domain, aliases, siteType, phpVersion, proxyUrl, sslType, certPath, keyPath string) error {
	_, err := d.Conn.Exec(
		"UPDATE sites SET domain=?, aliases=?, site_type=?, php_version=?, proxy_url=?, ssl_type=?, ssl_cert_path=?, ssl_key_path=? WHERE id=?",
		domain, aliases, siteType, phpVersion, proxyUrl, sslType, certPath, keyPath, id,
	)
	return err
}

func (d *DB) DeleteSite(id int64) error {
	_, err := d.Conn.Exec("DELETE FROM sites WHERE id = ?", id)
	return err
}

// --- PHP Settings queries ---

func (d *DB) GetPHPSettings(siteID int64) (map[string]interface{}, error) {
	var id int64
	var memLimit, postMax, uploadMax, custom string
	var execTime, inputTime, inputVars int
	err := d.Conn.QueryRow(
		"SELECT id, memory_limit, max_execution_time, max_input_time, max_input_vars, post_max_size, upload_max_filesize, custom_directives FROM php_settings WHERE site_id = ?",
		siteID,
	).Scan(&id, &memLimit, &execTime, &inputTime, &inputVars, &postMax, &uploadMax, &custom)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"id": id, "site_id": siteID, "memory_limit": memLimit,
		"max_execution_time": execTime, "max_input_time": inputTime,
		"max_input_vars": inputVars, "post_max_size": postMax,
		"upload_max_filesize": uploadMax, "custom_directives": custom,
	}, nil
}

func (d *DB) UpsertPHPSettings(siteID int64, memLimit string, execTime, inputTime, inputVars int, postMax, uploadMax, custom string) error {
	// Try update first
	res, err := d.Conn.Exec(`UPDATE php_settings SET
		memory_limit=?, max_execution_time=?, max_input_time=?, max_input_vars=?,
		post_max_size=?, upload_max_filesize=?, custom_directives=? WHERE site_id=?`,
		memLimit, execTime, inputTime, inputVars, postMax, uploadMax, custom, siteID,
	)
	if err != nil {
		return err
	}
	rows, _ := res.RowsAffected()
	if rows > 0 {
		return nil
	}
	// Insert if no existing row
	_, err = d.Conn.Exec(`INSERT INTO php_settings (site_id, memory_limit, max_execution_time, max_input_time, max_input_vars, post_max_size, upload_max_filesize, custom_directives)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		siteID, memLimit, execTime, inputTime, inputVars, postMax, uploadMax, custom,
	)
	return err
}

// --- Database queries ---

func (d *DB) ListDatabases() ([]map[string]interface{}, error) {
	rows, err := d.Conn.Query("SELECT d.id, d.db_name, d.site_id, d.created_at, COALESCE(s.domain,'') FROM databases d LEFT JOIN sites s ON d.site_id = s.id ORDER BY d.id DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var dbs []map[string]interface{}
	for rows.Next() {
		var id int64
		var siteID sql.NullInt64
		var dbName, createdAt, domain string
		if err := rows.Scan(&id, &dbName, &siteID, &createdAt, &domain); err != nil {
			return nil, err
		}
		m := map[string]interface{}{"id": id, "db_name": dbName, "created_at": createdAt, "site_domain": domain}
		if siteID.Valid {
			m["site_id"] = siteID.Int64
		}
		dbs = append(dbs, m)
	}
	return dbs, nil
}

func (d *DB) CreateDatabase(dbName string, siteID int64) (int64, error) {
	res, err := d.Conn.Exec("INSERT INTO databases (db_name, site_id) VALUES (?, ?)", dbName, siteID)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (d *DB) ListDatabasesBySite(siteID int64) ([]map[string]interface{}, error) {
	rows, err := d.Conn.Query(`SELECT d.id, d.db_name, d.site_id, COALESCE(s.domain,'') as site_domain, d.created_at
		FROM databases d LEFT JOIN sites s ON d.site_id = s.id WHERE d.site_id = ? ORDER BY d.db_name`, siteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []map[string]interface{}
	for rows.Next() {
		var id, siteIdVal int64
		var name, domain, created string
		if err := rows.Scan(&id, &name, &siteIdVal, &domain, &created); err != nil {
			return nil, err
		}
		result = append(result, map[string]interface{}{
			"id": id, "db_name": name, "site_id": siteIdVal,
			"site_domain": domain, "created_at": created,
		})
	}
	return result, nil
}

func (d *DB) DeleteDatabase(id int64) (string, error) {
	var dbName string
	err := d.Conn.QueryRow("SELECT db_name FROM databases WHERE id = ?", id).Scan(&dbName)
	if err != nil {
		return "", err
	}
	_, err = d.Conn.Exec("DELETE FROM databases WHERE id = ?", id)
	return dbName, err
}

// --- DB User queries ---

func (d *DB) ListDBUsers() ([]map[string]interface{}, error) {
	rows, err := d.Conn.Query(`
		SELECT u.id, u.username, u.database_id, d.db_name, COALESCE(u.privilege_level,'full'), u.created_at
		FROM db_users u LEFT JOIN databases d ON u.database_id = d.id ORDER BY u.id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []map[string]interface{}
	for rows.Next() {
		var id, dbID int64
		var username, dbName, privilegeLevel, createdAt string
		if err := rows.Scan(&id, &username, &dbID, &dbName, &privilegeLevel, &createdAt); err != nil {
			return nil, err
		}
		users = append(users, map[string]interface{}{
			"id": id, "username": username, "database_id": dbID, "db_name": dbName,
			"privilege_level": privilegeLevel, "created_at": createdAt,
		})
	}
	return users, nil
}

func (d *DB) CreateDBUser(username string, databaseID int64, privilegeLevel string) (int64, error) {
	if privilegeLevel == "" {
		privilegeLevel = "full"
	}
	res, err := d.Conn.Exec("INSERT INTO db_users (username, database_id, privilege_level) VALUES (?, ?, ?)", username, databaseID, privilegeLevel)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (d *DB) UpdateDBUserPrivilege(id int64, privilegeLevel string) error {
	_, err := d.Conn.Exec("UPDATE db_users SET privilege_level = ? WHERE id = ?", privilegeLevel, id)
	return err
}

func (d *DB) DeleteDBUser(id int64) (string, error) {
	var username string
	err := d.Conn.QueryRow("SELECT username FROM db_users WHERE id = ?", id).Scan(&username)
	if err != nil {
		return "", err
	}
	_, err = d.Conn.Exec("DELETE FROM db_users WHERE id = ?", id)
	return username, err
}

// --- Cron Job queries ---

func (d *DB) ListCronJobs(siteID int64) ([]map[string]interface{}, error) {
	rows, err := d.Conn.Query("SELECT id, schedule, command, enabled, created_at FROM cron_jobs WHERE site_id = ? ORDER BY id", siteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []map[string]interface{}
	for rows.Next() {
		var id int64
		var enabled int
		var schedule, command, createdAt string
		if err := rows.Scan(&id, &schedule, &command, &enabled, &createdAt); err != nil {
			return nil, err
		}
		jobs = append(jobs, map[string]interface{}{
			"id": id, "site_id": siteID, "schedule": schedule, "command": command, "enabled": enabled == 1, "created_at": createdAt,
		})
	}
	return jobs, nil
}

func (d *DB) CreateCronJob(siteID int64, schedule, command string) (int64, error) {
	res, err := d.Conn.Exec("INSERT INTO cron_jobs (site_id, schedule, command) VALUES (?, ?, ?)", siteID, schedule, command)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (d *DB) UpdateCronJob(id int64, schedule, command string, enabled bool) error {
	en := 0
	if enabled {
		en = 1
	}
	_, err := d.Conn.Exec("UPDATE cron_jobs SET schedule=?, command=?, enabled=? WHERE id=?", schedule, command, en, id)
	return err
}

func (d *DB) DeleteCronJob(id int64) error {
	_, err := d.Conn.Exec("DELETE FROM cron_jobs WHERE id = ?", id)
	return err
}

// --- Panel Settings queries ---

func (d *DB) GetPanelSettings() (map[string]interface{}, error) {
	var name, tagline, logoURL, faviconURL, primaryColor, accentColor, footerText string
	var recaptchaSiteKey, recaptchaSecretKey, timezone string
	var sessionTimeout int
	err := d.Conn.QueryRow(
		"SELECT panel_name, panel_tagline, logo_url, favicon_url, primary_color, accent_color, footer_text, session_timeout, COALESCE(recaptcha_site_key,''), COALESCE(recaptcha_secret_key,''), COALESCE(timezone,'UTC') FROM panel_settings WHERE id = 1",
	).Scan(&name, &tagline, &logoURL, &faviconURL, &primaryColor, &accentColor, &footerText, &sessionTimeout, &recaptchaSiteKey, &recaptchaSecretKey, &timezone)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"panel_name": name, "panel_tagline": tagline, "logo_url": logoURL,
		"favicon_url": faviconURL, "primary_color": primaryColor, "accent_color": accentColor,
		"footer_text": footerText, "session_timeout": sessionTimeout,
		"recaptcha_site_key": recaptchaSiteKey, "recaptcha_secret_key": recaptchaSecretKey,
		"timezone": timezone,
	}, nil
}

func (d *DB) UpdatePanelSettings(name, tagline, logoURL, faviconURL, primaryColor, accentColor, footerText string, sessionTimeout int, recaptchaSiteKey, recaptchaSecretKey, timezone string) error {
	_, err := d.Conn.Exec(`UPDATE panel_settings SET
		panel_name=?, panel_tagline=?, logo_url=?, favicon_url=?,
		primary_color=?, accent_color=?, footer_text=?,
		session_timeout=?, recaptcha_site_key=?, recaptcha_secret_key=?, timezone=? WHERE id = 1`,
		name, tagline, logoURL, faviconURL, primaryColor, accentColor, footerText, sessionTimeout, recaptchaSiteKey, recaptchaSecretKey, timezone,
	)
	return err
}

// --- Backup Methods queries ---

func (d *DB) ListBackupMethods() ([]map[string]interface{}, error) {
	rows, err := d.Conn.Query("SELECT id, name, type, config, enabled, created_at FROM backup_methods ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var methods []map[string]interface{}
	for rows.Next() {
		var id int64
		var enabled int
		var name, mtype, config, created string
		if err := rows.Scan(&id, &name, &mtype, &config, &enabled, &created); err != nil {
			return nil, err
		}
		methods = append(methods, map[string]interface{}{
			"id": id, "name": name, "type": mtype, "config": config, "enabled": enabled == 1, "created_at": created,
		})
	}
	return methods, nil
}

func (d *DB) CreateBackupMethod(name, mtype, config string) (int64, error) {
	res, err := d.Conn.Exec("INSERT INTO backup_methods (name, type, config) VALUES (?, ?, ?)", name, mtype, config)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (d *DB) UpdateBackupMethod(id int64, name, mtype, config string, enabled bool) error {
	en := 0
	if enabled {
		en = 1
	}
	_, err := d.Conn.Exec("UPDATE backup_methods SET name=?, type=?, config=?, enabled=? WHERE id=?", name, mtype, config, en, id)
	return err
}

func (d *DB) DeleteBackupMethod(id int64) error {
	_, err := d.Conn.Exec("DELETE FROM backup_methods WHERE id = ?", id)
	return err
}

// --- Backup queries ---

func (d *DB) ListBackups(siteID int64) ([]map[string]interface{}, error) {
	rows, err := d.Conn.Query("SELECT id, type, method, file_path, size, status, created_at FROM backups WHERE site_id = ? ORDER BY id DESC", siteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var backups []map[string]interface{}
	for rows.Next() {
		var id int64
		var btype, method, filePath, size, status, created string
		if err := rows.Scan(&id, &btype, &method, &filePath, &size, &status, &created); err != nil {
			return nil, err
		}
		backups = append(backups, map[string]interface{}{
			"id": id, "site_id": siteID, "type": btype, "method": method,
			"file_path": filePath, "size": size, "status": status, "created_at": created,
		})
	}
	return backups, nil
}

func (d *DB) CreateBackup(siteID int64, btype, method, filePath, size string) (int64, error) {
	res, err := d.Conn.Exec("INSERT INTO backups (site_id, type, method, file_path, size) VALUES (?, ?, ?, ?, ?)",
		siteID, btype, method, filePath, size)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (d *DB) GetBackup(id int64) (map[string]interface{}, error) {
	var siteID int64
	var btype, method, filePath, size, status, created string
	err := d.Conn.QueryRow("SELECT site_id, type, method, file_path, size, status, created_at FROM backups WHERE id = ?", id).
		Scan(&siteID, &btype, &method, &filePath, &size, &status, &created)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"id": id, "site_id": siteID, "type": btype, "method": method,
		"file_path": filePath, "size": size, "status": status, "created_at": created,
	}, nil
}

func (d *DB) DeleteBackup(id int64) (string, error) {
	var filePath string
	err := d.Conn.QueryRow("SELECT file_path FROM backups WHERE id = ?", id).Scan(&filePath)
	if err != nil {
		return "", err
	}
	_, err = d.Conn.Exec("DELETE FROM backups WHERE id = ?", id)
	return filePath, err
}

func (d *DB) CleanOldBackups(siteID int64, keep int) error {
	_, err := d.Conn.Exec(`DELETE FROM backups WHERE site_id = ? AND id NOT IN (
		SELECT id FROM backups WHERE site_id = ? ORDER BY id DESC LIMIT ?
	)`, siteID, siteID, keep)
	return err
}

// GenerateBackupDownloadToken creates a one-time download token for a backup (valid 5 minutes)
func (d *DB) GenerateBackupDownloadToken(backupID int64) (string, error) {
	b := make([]byte, 32)
	cryptoRand.Read(b)
	token := hex.EncodeToString(b)
	_, err := d.Conn.Exec(
		"UPDATE backups SET download_token = ?, download_token_expires = datetime('now', '+5 minutes') WHERE id = ?",
		token, backupID,
	)
	if err != nil {
		return "", err
	}
	return token, nil
}

// ValidateBackupDownloadToken validates and consumes a one-time download token
func (d *DB) ValidateBackupDownloadToken(token string) (map[string]interface{}, error) {
	var id, siteID int64
	var filePath string
	err := d.Conn.QueryRow(
		"SELECT id, site_id, file_path FROM backups WHERE download_token = ? AND download_token_expires > datetime('now')",
		token,
	).Scan(&id, &siteID, &filePath)
	if err != nil {
		return nil, err
	}
	// Invalidate the token after use (single-use)
	d.Conn.Exec("UPDATE backups SET download_token = '', download_token_expires = NULL WHERE id = ?", id)
	return map[string]interface{}{
		"id": id, "site_id": siteID, "file_path": filePath,
	}, nil
}

// --- Backup Schedule queries ---

func (d *DB) GetBackupSchedule(siteID int64) (map[string]interface{}, error) {
	var id int64
	var frequency, method string
	var retention int
	err := d.Conn.QueryRow("SELECT id, frequency, retention, method FROM backup_schedules WHERE site_id = ?", siteID).
		Scan(&id, &frequency, &retention, &method)
	if err != nil {
		return map[string]interface{}{"frequency": "disabled", "retention": 7, "method": "local"}, nil
	}
	return map[string]interface{}{
		"id": id, "site_id": siteID, "frequency": frequency, "retention": retention, "method": method,
	}, nil
}

func (d *DB) UpsertBackupSchedule(siteID int64, frequency string, retention int, method string) error {
	res, err := d.Conn.Exec("UPDATE backup_schedules SET frequency=?, retention=?, method=? WHERE site_id=?",
		frequency, retention, method, siteID)
	if err != nil {
		return err
	}
	rows, _ := res.RowsAffected()
	if rows > 0 {
		return nil
	}
	_, err = d.Conn.Exec("INSERT INTO backup_schedules (site_id, frequency, retention, method) VALUES (?, ?, ?, ?)",
		siteID, frequency, retention, method)
	return err
}

// --- SSL Certificates queries ---

func (d *DB) ListSSLCertificates(siteID int64) ([]map[string]interface{}, error) {
	rows, err := d.Conn.Query("SELECT id, type, label, cert_path, key_path, active, created_at FROM ssl_certificates WHERE site_id = ? ORDER BY id DESC", siteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var certs []map[string]interface{}
	for rows.Next() {
		var id int64
		var active int
		var ctype, label, certPath, keyPath, created string
		if err := rows.Scan(&id, &ctype, &label, &certPath, &keyPath, &active, &created); err != nil {
			return nil, err
		}
		certs = append(certs, map[string]interface{}{
			"id": id, "site_id": siteID, "type": ctype, "label": label,
			"cert_path": certPath, "key_path": keyPath, "active": active == 1, "created_at": created,
		})
	}
	return certs, nil
}

func (d *DB) CreateSSLCertificate(siteID int64, ctype, label, certPath, keyPath string, active bool) (int64, error) {
	act := 0
	if active {
		act = 1
	}
	res, err := d.Conn.Exec("INSERT INTO ssl_certificates (site_id, type, label, cert_path, key_path, active) VALUES (?, ?, ?, ?, ?, ?)",
		siteID, ctype, label, certPath, keyPath, act)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (d *DB) ActivateSSLCertificate(siteID, certID int64) error {
	// Deactivate all certs for this site
	d.Conn.Exec("UPDATE ssl_certificates SET active = 0 WHERE site_id = ?", siteID)
	// Activate the specified cert
	_, err := d.Conn.Exec("UPDATE ssl_certificates SET active = 1 WHERE id = ? AND site_id = ?", certID, siteID)
	return err
}

func (d *DB) GetSSLCertificate(id int64) (map[string]interface{}, error) {
	var siteID int64
	var active int
	var ctype, label, certPath, keyPath, created string
	err := d.Conn.QueryRow("SELECT site_id, type, label, cert_path, key_path, active, created_at FROM ssl_certificates WHERE id = ?", id).
		Scan(&siteID, &ctype, &label, &certPath, &keyPath, &active, &created)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"id": id, "site_id": siteID, "type": ctype, "label": label,
		"cert_path": certPath, "key_path": keyPath, "active": active == 1, "created_at": created,
	}, nil
}

func (d *DB) DeleteSSLCertificate(id int64) error {
	_, err := d.Conn.Exec("DELETE FROM ssl_certificates WHERE id = ?", id)
	return err
}

func (d *DB) CountSSLByType(siteID int64, ctype string) (int, error) {
	var count int
	err := d.Conn.QueryRow("SELECT COUNT(*) FROM ssl_certificates WHERE site_id = ? AND type = ?", siteID, ctype).Scan(&count)
	return count, err
}

// --- Firewall Rules queries ---

func (d *DB) ListFirewallRules() ([]map[string]interface{}, error) {
	rows, err := d.Conn.Query("SELECT id, direction, action, protocol, port, source, comment, enabled, created_at FROM firewall_rules ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var rules []map[string]interface{}
	for rows.Next() {
		var id int64
		var enabled int
		var direction, action, protocol, port, source, comment, created string
		if err := rows.Scan(&id, &direction, &action, &protocol, &port, &source, &comment, &enabled, &created); err != nil {
			return nil, err
		}
		rules = append(rules, map[string]interface{}{
			"id": id, "direction": direction, "action": action, "protocol": protocol,
			"port": port, "source": source, "comment": comment, "enabled": enabled == 1, "created_at": created,
		})
	}
	return rules, nil
}

func (d *DB) CreateFirewallRule(direction, action, protocol, port, source, comment string) (int64, error) {
	res, err := d.Conn.Exec("INSERT INTO firewall_rules (direction, action, protocol, port, source, comment) VALUES (?, ?, ?, ?, ?, ?)",
		direction, action, protocol, port, source, comment)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (d *DB) UpdateFirewallRule(id int64, direction, action, protocol, port, source, comment string, enabled bool) error {
	en := 0
	if enabled {
		en = 1
	}
	_, err := d.Conn.Exec("UPDATE firewall_rules SET direction=?, action=?, protocol=?, port=?, source=?, comment=?, enabled=? WHERE id=?",
		direction, action, protocol, port, source, comment, en, id)
	return err
}

func (d *DB) DeleteFirewallRule(id int64) error {
	_, err := d.Conn.Exec("DELETE FROM firewall_rules WHERE id = ?", id)
	return err
}
