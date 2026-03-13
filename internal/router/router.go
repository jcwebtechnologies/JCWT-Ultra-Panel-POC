package router

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/jcwt/ultra-panel/internal/auth"
	"github.com/jcwt/ultra-panel/internal/config"
	"github.com/jcwt/ultra-panel/internal/db"
	"github.com/jcwt/ultra-panel/internal/handlers"
)

// Setup creates and configures the HTTP router
func Setup(database *db.DB, cfg *config.Config, authMgr *auth.Manager, webFS http.FileSystem, version string) http.Handler {
	mux := http.NewServeMux()
	middleware := auth.NewMiddleware(authMgr)

	// --- Auth endpoints (no auth required) ---
	loginHandler := &authHandler{db: database, auth: authMgr}
	mux.Handle("/api/auth/login", middleware.RateLimit(loginHandler))

	// Bootstrap endpoint: create first admin using one-time setup token
	mux.Handle("/api/setup", middleware.RateLimit(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, `{"success":false,"error":"method not allowed"}`, http.StatusMethodNotAllowed)
			return
		}
		if !database.NeedsSetup() {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte(`{"success":false,"error":"setup already completed"}`))
			return
		}
		var req struct {
			SetupToken string `json:"setup_token"`
			Username   string `json:"username"`
			Password   string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"success":false,"error":"invalid request"}`))
			return
		}
		if len(req.Username) < 3 || len(req.Username) > 31 || len(req.Password) < 10 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"success":false,"error":"username must be 3-31 chars and password at least 10 chars"}`))
			return
		}
		ok, err := database.ValidateSetupToken(req.SetupToken)
		if err != nil || !ok {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte(`{"success":false,"error":"invalid or expired setup token"}`))
			return
		}
		hash, err := auth.HashPassword(req.Password)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"success":false,"error":"failed to hash password"}`))
			return
		}
		if err := database.CreateAdmin(req.Username, hash); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusConflict)
			w.Write([]byte(`{"success":false,"error":"failed to create admin"}`))
			return
		}
		database.ConsumeSetupToken()
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"success":true,"data":{"message":"Admin account created. You can now log in."}}`))
	})))

	// Setup status check (tells frontend whether setup is needed)
	mux.HandleFunc("/api/setup/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"data":    map[string]interface{}{"needs_setup": database.NeedsSetup()},
		})
	})

	// 2FA endpoints
	twofaHandler := &handlers.TwoFAHandler{DB: database, AuthMgr: authMgr}
	mux.Handle("/api/auth/2fa", middleware.RequireAuth(middleware.RequireCSRF(twofaHandler)))
	mux.Handle("/api/auth/2fa/verify", middleware.RateLimit(http.HandlerFunc(twofaHandler.Verify)))

	mux.Handle("/api/auth/logout", middleware.RequireAuth(middleware.RequireCSRF(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, `{"success":false,"error":"method not allowed"}`, http.StatusMethodNotAllowed)
			return
		}
		sessionID := auth.GetSessionID(r)
		if sessionID != "" {
			authMgr.DestroySession(sessionID)
		}
		auth.ClearSessionCookie(w)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"success":true}`))
	}))))
	mux.HandleFunc("/api/auth/check", func(w http.ResponseWriter, r *http.Request) {
		sessionID := auth.GetSessionID(r)
		sess, ok := authMgr.GetSession(sessionID)
		w.Header().Set("Content-Type", "application/json")
		if !ok {
			w.Write([]byte(`{"success":true,"data":{"authenticated":false}}`))
			return
		}
		w.Write([]byte(`{"success":true,"data":{"authenticated":true,"username":"` + sess.Username + `","role":"` + sess.Role + `","csrf_token":"` + sess.CSRFToken + `"}}`))
	})

	// --- Protected API endpoints ---
	// Dashboard
	mux.Handle("/api/dashboard", middleware.RequireAuth(&handlers.DashboardHandler{DB: database}))

	// Sites (admin + manager only)
	mux.Handle("/api/sites", middleware.RequireAuth(middleware.RequireCSRF(
		middleware.RequireRole("admin", "manager")(&handlers.SitesHandler{DB: database, Cfg: cfg}),
	)))

	// Databases (admin + manager only)
	mux.Handle("/api/databases", middleware.RequireAuth(middleware.RequireCSRF(
		middleware.RequireRole("admin", "manager")(&handlers.DatabasesHandler{DB: database}),
	)))

	// DB Users (admin + manager only)
	mux.Handle("/api/db-users", middleware.RequireAuth(middleware.RequireCSRF(
		middleware.RequireRole("admin", "manager")(&handlers.DBUsersHandler{DB: database}),
	)))

	// WordPress Tools & Updates (admin + manager only)
	mux.Handle("/api/wordpress", middleware.RequireAuth(middleware.RequireCSRF(
		middleware.RequireRole("admin", "manager")(&handlers.WordPressHandler{DB: database, Cfg: cfg}),
	)))

	// SSL (admin + manager only)
	mux.Handle("/api/ssl", middleware.RequireAuth(middleware.RequireCSRF(
		middleware.RequireRole("admin", "manager")(&handlers.SSLHandler{DB: database, Cfg: cfg}),
	)))

	// Cron (admin + manager only)
	mux.Handle("/api/cron", middleware.RequireAuth(middleware.RequireCSRF(
		middleware.RequireRole("admin", "manager")(&handlers.CronHandler{DB: database}),
	)))

	// Files (File Browser manager) — admin + manager only
	filesHandler := &handlers.FilesHandler{DB: database, Cfg: cfg}
	filesHandler.StartIdleReaper()
	mux.Handle("/api/files", middleware.RequireAuth(middleware.RequireCSRF(
		middleware.RequireRole("admin", "manager")(filesHandler),
	)))
	mux.Handle("/api/files/delete", middleware.RequireAuth(middleware.RequireCSRF(
		middleware.RequireRole("admin", "manager")(http.HandlerFunc(filesHandler.DeleteFile)),
	)))

	// File Browser reverse proxy — admin + manager only
	mux.Handle("/fb/", middleware.RequireAuth(
		middleware.RequireRole("admin", "manager")(http.HandlerFunc(filesHandler.ProxyHandler())),
	))

	// PHP Settings (admin + manager only)
	mux.Handle("/api/php-settings", middleware.RequireAuth(middleware.RequireCSRF(
		middleware.RequireRole("admin", "manager")(&handlers.PHPHandler{DB: database, Cfg: cfg}),
	)))

	// PHP Versions (all authenticated users can view)
	mux.Handle("/api/php-versions", middleware.RequireAuth(&handlers.PHPVersionsHandler{}))

	// Panel Settings (admin + manager only)
	mux.Handle("/api/settings", middleware.RequireAuth(middleware.RequireCSRF(
		middleware.RequireRole("admin", "manager")(&handlers.SettingsHandler{DB: database, Cfg: cfg, Version: version}),
	)))

	// Users Management (admin only)
	usersHandler := &handlers.UsersHandler{DB: database, Middleware: middleware}
	mux.Handle("/api/users", middleware.RequireAuth(middleware.RequireCSRF(
		middleware.RequireRole("admin")(usersHandler),
	)))

	// phpMyAdmin auto-login (admin + manager only)
	pmaHandler := &handlers.PhpMyAdminHandler{DB: database}
	pmaHandler.CleanupStaleTempUsers()
	mux.Handle("/api/pma", middleware.RequireAuth(middleware.RequireCSRF(
		middleware.RequireRole("admin", "manager")(pmaHandler),
	)))

	// Current user info
	mux.HandleFunc("/api/me", func(w http.ResponseWriter, r *http.Request) {
		sess := middleware.GetSessionFromRequest(r)
		if sess == nil {
			http.Error(w, `{"success":false,"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"success":true,"data":{"username":"` + sess.Username + `","role":"` + sess.Role + `","user_id":` + strconv.FormatInt(sess.UserID, 10) + `}}`))
	})

	// Public settings (branding + reCAPTCHA site key only, no auth required)
	mux.HandleFunc("/api/settings/public", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			http.Error(w, `{"success":false,"error":"method not allowed"}`, http.StatusMethodNotAllowed)
			return
		}
		settings, err := database.GetPanelSettings()
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"success":false,"error":"failed to load settings"}`))
			return
		}
		// Only expose safe public fields (never expose secret key)
		public := map[string]interface{}{
			"panel_name":         settings["panel_name"],
			"panel_tagline":      settings["panel_tagline"],
			"logo_url":           settings["logo_url"],
			"logo_url_dark":      settings["logo_url_dark"],
			"favicon_url":        settings["favicon_url"],
			"primary_color":      settings["primary_color"],
			"accent_color":       settings["accent_color"],
			"recaptcha_site_key": settings["recaptcha_site_key"],
			"version":            version,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "data": public})
	})

	// Services (admin only)
	mux.Handle("/api/services", middleware.RequireAuth(middleware.RequireCSRF(
		middleware.RequireRole("admin")(&handlers.ServicesHandler{DB: database}),
	)))

	// Vhost Editor (admin only)
	mux.Handle("/api/vhost", middleware.RequireAuth(middleware.RequireCSRF(
		middleware.RequireRole("admin")(&handlers.VhostHandler{DB: database, Cfg: cfg}),
	)))

	// Site Backups (admin + manager only)
	mux.Handle("/api/backups", middleware.RequireAuth(middleware.RequireCSRF(
		middleware.RequireRole("admin", "manager")(&handlers.BackupHandler{DB: database, Cfg: cfg}),
	)))

	// Backup Methods (panel-wide, admin only)
	mux.Handle("/api/backup-methods", middleware.RequireAuth(middleware.RequireCSRF(
		middleware.RequireRole("admin")(&handlers.BackupMethodsHandler{DB: database}),
	)))

	// Site Logs (admin + manager only)
	mux.Handle("/api/logs", middleware.RequireAuth(
		middleware.RequireRole("admin", "manager")(&handlers.LogsHandler{DB: database}),
	))

	// SSL Certificates (admin + manager only)
	mux.Handle("/api/ssl-certs", middleware.RequireAuth(middleware.RequireCSRF(
		middleware.RequireRole("admin", "manager")(&handlers.SSLCertsHandler{DB: database, Cfg: cfg}),
	)))

	// SMTP Settings (admin only)
	mux.Handle("/api/smtp", middleware.RequireAuth(middleware.RequireCSRF(
		middleware.RequireRole("admin")(&handlers.SMTPHandler{DB: database, Cfg: cfg}),
	)))

	// Firewall (admin only)
	mux.Handle("/api/firewall", middleware.RequireAuth(middleware.RequireCSRF(
		middleware.RequireRole("admin")(&handlers.FirewallHandler{DB: database, Cfg: cfg}),
	)))

	// Email Templates (admin only)
	mux.Handle("/api/email-templates", middleware.RequireAuth(middleware.RequireCSRF(
		middleware.RequireRole("admin")(&handlers.EmailTemplatesHandler{DB: database}),
	)))

	// Disk Usage (admin only)
	mux.Handle("/api/disk-usage", middleware.RequireAuth(middleware.RequireCSRF(
		middleware.RequireRole("admin")(&handlers.DiskUsageHandler{DB: database, Cfg: cfg}),
	)))

	// SSH Key Management (admin only)
	mux.Handle("/api/ssh", middleware.RequireAuth(middleware.RequireCSRF(
		middleware.RequireRole("admin")(&handlers.SSHHandler{DB: database}),
	)))

	// Serve uploaded files (require auth to prevent public access)
	uploadsDir := filepath.Join(cfg.DataDir, "uploads")
	os.MkdirAll(uploadsDir, 0750)
	mux.Handle("/api/uploads/", middleware.RequireAuth(
		http.StripPrefix("/api/uploads/", http.FileServer(http.Dir(uploadsDir))),
	))

	// Password change endpoint
	mux.Handle("/api/auth/change-password", middleware.RequireAuth(middleware.RequireCSRF(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, `{"success":false,"error":"method not allowed"}`, http.StatusMethodNotAllowed)
			return
		}

		var req struct {
			CurrentPassword string `json:"current_password"`
			NewPassword     string `json:"new_password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"success":false,"error":"invalid request"}`))
			return
		}

		if len(req.NewPassword) < 8 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"success":false,"error":"password must be at least 8 characters"}`))
			return
		}

		// Get current user from session
		sessionID := auth.GetSessionID(r)
		sess, ok := authMgr.GetSession(sessionID)
		if !ok {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"success":false,"error":"not authenticated"}`))
			return
		}

		// Verify current password
		_, hash, _, err := database.GetAdminByUsername(sess.Username)
		if err != nil || !auth.CheckPassword(hash, req.CurrentPassword) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"success":false,"error":"current password is incorrect"}`))
			return
		}

		// Update password
		newHash, err := auth.HashPassword(req.NewPassword)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"success":false,"error":"failed to hash password"}`))
			return
		}

		if err := database.UpdateAdminPassword(sess.UserID, newHash); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"success":false,"error":"failed to update password"}`))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"success":true,"data":{"updated":true}}`))
	}))))

	// --- Static frontend files ---
	mux.Handle("/", spaHandler{fs: webFS})

	// Wrap everything with security headers and body size limiter
	return auth.BodySizeLimit(auth.SecurityHeaders(mux))
}

// spaHandler serves the SPA, falling back to index.html for client-side routing
type spaHandler struct {
	fs http.FileSystem
}

func (s spaHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	// Try to serve the file directly
	f, err := s.fs.Open(path)
	if err != nil {
		// Fall back to index.html for SPA routing
		f, err = s.fs.Open("/index.html")
		if err != nil {
			http.NotFound(w, r)
			return
		}
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		http.NotFound(w, r)
		return
	}

	if stat.IsDir() {
		// Try index.html in the directory
		indexPath := strings.TrimSuffix(path, "/") + "/index.html"
		f2, err := s.fs.Open(indexPath)
		if err != nil {
			f2, _ = s.fs.Open("/index.html")
		}
		if f2 != nil {
			defer f2.Close()
			stat2, _ := f2.Stat()
			if stat2 != nil {
				http.ServeContent(w, r, stat2.Name(), stat2.ModTime(), f2.(readSeeker))
				return
			}
		}
	}

	http.ServeContent(w, r, stat.Name(), stat.ModTime(), f.(readSeeker))
}

type readSeeker interface {
	Read(p []byte) (n int, err error)
	Seek(offset int64, whence int) (int64, error)
}

// authHandler handles login with optional reCAPTCHA verification
type authHandler struct {
	db   *db.DB
	auth *auth.Manager
}

func (h *authHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, `{"success":false,"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Username     string `json:"username"`
		Password     string `json:"password"`
		CaptchaToken string `json:"captcha_token"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"success":false,"error":"invalid request"}`))
		return
	}

	// Validate input lengths
	if len(req.Username) == 0 || len(req.Username) > 100 || len(req.Password) == 0 || len(req.Password) > 200 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"success":false,"error":"invalid credentials format"}`))
		return
	}

	// Verify reCAPTCHA if configured
	settings, _ := h.db.GetPanelSettings()
	if settings != nil {
		secretKey, _ := settings["recaptcha_secret_key"].(string)
		if secretKey != "" {
			if req.CaptchaToken == "" {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte(`{"success":false,"error":"captcha verification required"}`))
				return
			}
			if !verifyCaptcha(secretKey, req.CaptchaToken, r.Host) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				w.Write([]byte(`{"success":false,"error":"captcha verification failed"}`))
				return
			}
		}
	}

	id, hash, role, err := h.db.GetAdminByUsername(req.Username)
	if err != nil || !auth.CheckPassword(hash, req.Password) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"success":false,"error":"invalid credentials"}`))
		return
	}

	// Check if 2FA is enabled for this user
	totpSecret, _ := h.db.GetTOTPSecret(id)
	if totpSecret != "" {
		// Don't create session — create pending 2FA token
		pendingToken := h.auth.Create2FAPendingToken(id, req.Username, role)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"success":true,"data":{"requires_2fa":true,"twofa_token":"` + pendingToken + `"}}`))
		return
	}

	sessionID, csrfToken := h.auth.CreateSession(id, req.Username, role)
	auth.SetSessionCookie(w, sessionID)

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"success":true,"data":{"csrf_token":"` + csrfToken + `","username":"` + req.Username + `","role":"` + role + `"}}`))
}

// verifyCaptcha verifies a Google reCAPTCHA v2 token with hostname validation
func verifyCaptcha(secretKey, token, expectedHostname string) bool {
	resp, err := http.PostForm("https://www.google.com/recaptcha/api/siteverify", url.Values{
		"secret":   {secretKey},
		"response": {token},
	})
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false
	}

	var result struct {
		Success    bool     `json:"success"`
		Hostname   string   `json:"hostname"`
		ErrorCodes []string `json:"error-codes"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return false
	}
	if !result.Success {
		return false
	}
	// Validate hostname to prevent token reuse from other origins
	if expectedHostname != "" && result.Hostname != expectedHostname {
		log.Printf("reCAPTCHA hostname mismatch: expected %s, got %s", expectedHostname, result.Hostname)
		return false
	}
	return true
}

// Unused import suppression
var _ = fmt.Sprintf
