package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/jcwt/ultra-panel/internal/db"
)

// PhpMyAdminHandler provides per-database phpMyAdmin access with auto-login.
// It generates one-time tokens, writes a signon PHP file, and returns a URL
// that the frontend opens in a new tab. Nginx + PHP-FPM process the signon
// PHP on the same domain, so it works through reverse proxies.
type PhpMyAdminHandler struct {
	DB *db.DB
}

func (h *PhpMyAdminHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case "POST":
		h.createToken(w, r)
	default:
		http.Error(w, `{"success":false,"error":"method not allowed"}`, http.StatusMethodNotAllowed)
	}
}

// createToken generates a one-time auto-login token for phpMyAdmin access.
// Flow: API creates temp MariaDB user → writes signon PHP to /usr/share/phpmyadmin/ →
// returns /pma/signon_<id>.php URL → frontend opens that URL → nginx serves it via PHP-FPM →
// PHP sets session → redirects to phpMyAdmin index.
func (h *PhpMyAdminHandler) createToken(w http.ResponseWriter, r *http.Request) {
	var req struct {
		DatabaseID int64 `json:"database_id"`
		DBUserID   int64 `json:"db_user_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.DatabaseID <= 0 {
		jsonError(w, "database_id is required", http.StatusBadRequest)
		return
	}

	// Fetch database name from database record
	var dbName string
	err := h.DB.Conn.QueryRow("SELECT db_name FROM databases WHERE id = ?", req.DatabaseID).Scan(&dbName)
	if err != nil {
		jsonError(w, "database not found", http.StatusNotFound)
		return
	}

	// Determine privilege level: if a specific DB user is provided, match their privileges
	privilegeLevel := "full"
	if req.DBUserID > 0 {
		var pLevel string
		err := h.DB.Conn.QueryRow(
			"SELECT COALESCE(privilege_level, 'full') FROM db_users WHERE id = ? AND database_id = ?",
			req.DBUserID, req.DatabaseID,
		).Scan(&pLevel)
		if err != nil {
			jsonError(w, "database user not found", http.StatusNotFound)
			return
		}
		privilegeLevel = pLevel
	}

	// Build grant SQL based on privilege level
	var grantPrivileges string
	switch privilegeLevel {
	case "readonly":
		grantPrivileges = "SELECT"
	case "readwrite":
		grantPrivileges = "SELECT, INSERT, UPDATE, DELETE"
	default:
		grantPrivileges = "ALL PRIVILEGES"
	}

	// Create a temporary MariaDB user for phpMyAdmin access
	passBytes := make([]byte, 16)
	rand.Read(passBytes)
	tempPass := hex.EncodeToString(passBytes)

	tempUser := fmt.Sprintf("pma_%s", dbName)
	if len(tempUser) > 32 {
		tempUser = tempUser[:32]
	}

	cmds := []string{
		fmt.Sprintf("DROP USER IF EXISTS '%s'@'localhost';", tempUser),
		fmt.Sprintf("CREATE USER '%s'@'localhost' IDENTIFIED BY '%s';", tempUser, tempPass),
		fmt.Sprintf("GRANT %s ON `%s`.* TO '%s'@'localhost';", grantPrivileges, dbName, tempUser),
		"FLUSH PRIVILEGES;",
	}

	for _, sql := range cmds {
		if out, err := exec.Command("sudo", "mysql", "-e", sql).CombinedOutput(); err != nil {
			log.Printf("PMA temp user SQL error: %s %v (sql: %s)", string(out), err, sql)
			jsonError(w, "failed to create temporary database access", http.StatusInternalServerError)
			return
		}
	}

	// Generate a unique ID for the signon file
	idBytes := make([]byte, 16)
	rand.Read(idBytes)
	signonID := hex.EncodeToString(idBytes)

	// Write the signon PHP directly into phpMyAdmin's directory.
	// Nginx serves /pma/ via alias to /usr/share/phpmyadmin/, so
	// /pma/signon_<id>.php will be processed by PHP-FPM on the same domain.
	signonFile := fmt.Sprintf("signon_%s.php", signonID)
	pmaSignonPath := "/usr/share/phpmyadmin/" + signonFile

	signContent := fmt.Sprintf(`<?php
session_name('SignonSession');
session_start();
$_SESSION['PMA_single_signon_user'] = '%s';
$_SESSION['PMA_single_signon_password'] = '%s';
$_SESSION['PMA_single_signon_host'] = 'localhost';
header('Location: /pma/index.php?db=%s');
exit;
?>`, tempUser, tempPass, dbName)

	writeCmd := exec.Command("sudo", "tee", pmaSignonPath)
	writeCmd.Stdin = strings.NewReader(signContent)
	writeCmd.Stdout = nil // discard tee's stdout echo — prevents credentials leaking into logs
	var signonStderr strings.Builder
	writeCmd.Stderr = &signonStderr
	if err := writeCmd.Run(); err != nil {
		log.Printf("PMA signon write failed for db %q: %v; detail: %s",
			dbName, err, strings.TrimSpace(signonStderr.String()))
		jsonError(w, "failed to prepare phpMyAdmin access", http.StatusInternalServerError)
		return
	}

	// Schedule cleanup of the signon file and temp user
	go func() {
		time.Sleep(30 * time.Second)
		exec.Command("sudo", "rm", "-f", pmaSignonPath).Run()
	}()
	go func() {
		time.Sleep(30 * time.Minute)
		exec.Command("sudo", "mysql", "-e",
			fmt.Sprintf("DROP USER IF EXISTS '%s'@'localhost'; FLUSH PRIVILEGES;", tempUser)).Run()
		log.Printf("Cleaned up PMA temp user: %s", tempUser)
	}()

	// Return the URL that the frontend opens in a new tab.
	// This is a relative path so it works through any reverse proxy.
	jsonSuccess(w, map[string]interface{}{
		"url":     fmt.Sprintf("/pma/%s", signonFile),
		"db_name": dbName,
	})
}

// PhpMyAdminInstalled verifies phpMyAdmin is installed
func PhpMyAdminInstalled() bool {
	_, err := os.Stat("/usr/share/phpmyadmin/index.php")
	return err == nil
}

// CleanupStaleTempUsers drops any leftover pma_* MariaDB users and signon PHP files
// from a previous panel run. Called once at startup.
func (h *PhpMyAdminHandler) CleanupStaleTempUsers() {
	// Drop pma_* MariaDB users
	out, err := exec.Command("sudo", "mysql", "-N", "-e",
		"SELECT CONCAT('''', user, '''@''', host, '''') FROM mysql.user WHERE user LIKE 'pma\\_%'").Output()
	if err != nil {
		return
	}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		exec.Command("sudo", "mysql", "-e", fmt.Sprintf("DROP USER IF EXISTS %s;", line)).Run()
	}
	if len(strings.TrimSpace(string(out))) > 0 {
		exec.Command("sudo", "mysql", "-e", "FLUSH PRIVILEGES;").Run()
		log.Printf("Cleaned up stale PMA temp users at startup")
	}

	// Remove leftover signon PHP files
	exec.Command("sudo", "bash", "-c", "rm -f /usr/share/phpmyadmin/signon_*.php").Run()
}

// GetDBInfo returns the database name for a given database ID
func (h *PhpMyAdminHandler) GetDBInfo(dbID int64) (name string, err error) {
	err = h.DB.Conn.QueryRow("SELECT db_name FROM databases WHERE id = ?", dbID).Scan(&name)
	return
}

func intParam(r *http.Request, key string) (int64, error) {
	return strconv.ParseInt(r.URL.Query().Get(key), 10, 64)
}
