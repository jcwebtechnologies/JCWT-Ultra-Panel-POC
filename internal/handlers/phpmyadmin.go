package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jcwt/ultra-panel/internal/db"
)

// PhpMyAdminHandler provides per-database phpMyAdmin access with auto-login.
// It generates one-time tokens that a signon PHP script uses to authenticate.
type PhpMyAdminHandler struct {
	DB     *db.DB
	mu     sync.Mutex
	tokens map[string]*pmaToken
}

type pmaToken struct {
	DBName    string
	DBUser    string
	DBPass    string
	CreatedAt time.Time
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
// The frontend calls this, gets a token, and redirects to /pma/signon?token=xxx
func (h *PhpMyAdminHandler) createToken(w http.ResponseWriter, r *http.Request) {
	var req struct {
		DatabaseID int64 `json:"database_id"`
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

	// Create a temporary MariaDB user for phpMyAdmin access
	// Generate a random password for this session
	passBytes := make([]byte, 16)
	rand.Read(passBytes)
	tempPass := hex.EncodeToString(passBytes)

	// Create or update a temporary PMA user for this database
	tempUser := fmt.Sprintf("pma_%s", dbName)
	if len(tempUser) > 32 {
		tempUser = tempUser[:32] // MariaDB username limit
	}

	// Create the temp user with access only to this specific database
	cmds := []string{
		fmt.Sprintf("DROP USER IF EXISTS '%s'@'localhost';", tempUser),
		fmt.Sprintf("CREATE USER '%s'@'localhost' IDENTIFIED BY '%s';", tempUser, tempPass),
		fmt.Sprintf("GRANT ALL PRIVILEGES ON `%s`.* TO '%s'@'localhost';", dbName, tempUser),
		"FLUSH PRIVILEGES;",
	}

	for _, sql := range cmds {
		if out, err := exec.Command("sudo", "mysql", "-e", sql).CombinedOutput(); err != nil {
			log.Printf("PMA temp user SQL error: %s %v (sql: %s)", string(out), err, sql)
			jsonError(w, "failed to create temporary database access", http.StatusInternalServerError)
			return
		}
	}

	// Generate a cryptographic token
	tokenBytes := make([]byte, 32)
	rand.Read(tokenBytes)
	token := hex.EncodeToString(tokenBytes)

	h.mu.Lock()
	if h.tokens == nil {
		h.tokens = make(map[string]*pmaToken)
	}
	h.tokens[token] = &pmaToken{
		DBName:    dbName,
		DBUser:    tempUser,
		DBPass:    tempPass,
		CreatedAt: time.Now(),
	}
	h.mu.Unlock()

	// Clean up old tokens (older than 5 minutes) and temp users
	go h.cleanupOldTokens()

	// Schedule cleanup of the temp user after 30 minutes
	go func() {
		time.Sleep(30 * time.Minute)
		exec.Command("sudo", "mysql", "-e",
			fmt.Sprintf("DROP USER IF EXISTS '%s'@'localhost'; FLUSH PRIVILEGES;", tempUser)).Run()
		log.Printf("Cleaned up PMA temp user: %s", tempUser)
	}()

	jsonSuccess(w, map[string]interface{}{
		"token":   token,
		"url":     fmt.Sprintf("/pma/signon?token=%s", token),
		"db_name": dbName,
	})
}

// ValidateToken checks a token and returns the credentials (used by the signon handler)
func (h *PhpMyAdminHandler) ValidateToken(token string) (dbName, dbUser, dbPass string, ok bool) {
	h.mu.Lock()
	defer h.mu.Unlock()

	t, exists := h.tokens[token]
	if !exists {
		return "", "", "", false
	}

	// Tokens are one-time use — delete after validation
	// Also check if token is not too old (5 minutes max)
	if time.Since(t.CreatedAt) > 5*time.Minute {
		delete(h.tokens, token)
		return "", "", "", false
	}

	delete(h.tokens, token)
	return t.DBName, t.DBUser, t.DBPass, true
}

func (h *PhpMyAdminHandler) cleanupOldTokens() {
	h.mu.Lock()
	defer h.mu.Unlock()

	for token, t := range h.tokens {
		if time.Since(t.CreatedAt) > 5*time.Minute {
			delete(h.tokens, token)
		}
	}
}

// SignonHandler serves the auto-login page that sets phpMyAdmin session and redirects.
// Flow: Go validates token → writes a unique signon PHP file → redirects browser to
// nginx (port 80) which serves the PHP via PHP-FPM → PHP sets session → redirects to phpMyAdmin.
func (h *PhpMyAdminHandler) SignonHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := r.URL.Query().Get("token")
		if token == "" {
			http.Error(w, "Missing token", http.StatusBadRequest)
			return
		}

		dbName, dbUser, dbPass, ok := h.ValidateToken(token)
		if !ok {
			http.Error(w, "Invalid or expired token. Please try again from the panel.", http.StatusUnauthorized)
			return
		}

		// Use a unique filename per token to avoid race conditions
		signonFile := fmt.Sprintf("signon_%s.php", token[:16])
		pmaSignonPath := "/usr/share/phpmyadmin/" + signonFile

		// Generate a PHP page that sets the phpMyAdmin signon session and redirects
		signContent := fmt.Sprintf(`<?php
session_name('SignonSession');
session_start();
$_SESSION['PMA_single_signon_user'] = '%s';
$_SESSION['PMA_single_signon_password'] = '%s';
$_SESSION['PMA_single_signon_host'] = 'localhost';
header('Location: /pma/index.php?db=%s');
exit;
?>`, dbUser, dbPass, dbName)

		writeCmd := exec.Command("sudo", "tee", pmaSignonPath)
		writeCmd.Stdin = strings.NewReader(signContent)
		writeCmd.Stdout = nil // suppress tee's stdout echo
		if out, err := writeCmd.CombinedOutput(); err != nil {
			log.Printf("Failed to write PMA signon: %s %v", string(out), err)
			http.Error(w, "Internal error", http.StatusInternalServerError)
			return
		}

		// Build redirect URL pointing to nginx (port 80) instead of the panel port
		// so PHP-FPM can process the signon PHP file
		host := r.Host
		if h, _, err := net.SplitHostPort(host); err == nil {
			host = h
		}
		// Re-add brackets for raw IPv6 addresses
		if net.ParseIP(host) != nil && strings.Contains(host, ":") {
			host = "[" + host + "]"
		}
		redirectURL := "http://" + host + "/pma/" + signonFile

		// Redirect to nginx-served signon PHP which sets session and redirects to phpMyAdmin
		http.Redirect(w, r, redirectURL, http.StatusFound)

		// Clean up the signon file after a short delay
		go func() {
			time.Sleep(10 * time.Second)
			exec.Command("sudo", "rm", "-f", pmaSignonPath).Run()
		}()
	}
}

// PhpMyAdminInstalled verifies phpMyAdmin is installed
func PhpMyAdminInstalled() bool {
	_, err := exec.Command("ls", "/usr/share/phpmyadmin/index.php").Output()
	return err == nil
}

// GetDBInfo returns the database name for a given database ID
func (h *PhpMyAdminHandler) GetDBInfo(dbID int64) (name string, err error) {
	err = h.DB.Conn.QueryRow("SELECT db_name FROM databases WHERE id = ?", dbID).Scan(&name)
	return
}

// TokenCount returns the number of active tokens (for debugging)
func (h *PhpMyAdminHandler) TokenCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.tokens)
}

func intParam(r *http.Request, key string) (int64, error) {
	return strconv.ParseInt(r.URL.Query().Get(key), 10, 64)
}
