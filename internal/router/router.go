package router

import (
	"encoding/json"
	"fmt"
	"io"
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

	// 2FA endpoints
	twofaHandler := &handlers.TwoFAHandler{DB: database, AuthMgr: authMgr}
	mux.Handle("/api/auth/2fa", middleware.RequireAuth(middleware.RequireCSRF(twofaHandler)))
	mux.Handle("/api/auth/2fa/verify", middleware.RateLimit(http.HandlerFunc(twofaHandler.Verify)))

	mux.HandleFunc("/api/auth/logout", func(w http.ResponseWriter, r *http.Request) {
		sessionID := auth.GetSessionID(r)
		if sessionID != "" {
			authMgr.DestroySession(sessionID)
		}
		auth.ClearSessionCookie(w)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"success":true}`))
	})
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

	// Sites
	mux.Handle("/api/sites", middleware.RequireAuth(middleware.RequireCSRF(&handlers.SitesHandler{DB: database, Cfg: cfg})))

	// Databases
	mux.Handle("/api/databases", middleware.RequireAuth(middleware.RequireCSRF(&handlers.DatabasesHandler{DB: database})))

	// DB Users
	mux.Handle("/api/db-users", middleware.RequireAuth(middleware.RequireCSRF(&handlers.DBUsersHandler{DB: database})))

	// SSL
	mux.Handle("/api/ssl", middleware.RequireAuth(middleware.RequireCSRF(&handlers.SSLHandler{DB: database, Cfg: cfg})))

	// Cron
	mux.Handle("/api/cron", middleware.RequireAuth(middleware.RequireCSRF(&handlers.CronHandler{DB: database})))

	// Files (File Browser manager)
	filesHandler := &handlers.FilesHandler{DB: database, Cfg: cfg}
	filesHandler.StartIdleReaper()
	mux.Handle("/api/files", middleware.RequireAuth(middleware.RequireCSRF(filesHandler)))

	// File Browser reverse proxy — requires auth but not CSRF (File Browser handles its own requests)
	mux.Handle("/fb/", middleware.RequireAuth(http.HandlerFunc(filesHandler.ProxyHandler())))

	// PHP Settings
	mux.Handle("/api/php-settings", middleware.RequireAuth(middleware.RequireCSRF(&handlers.PHPHandler{DB: database, Cfg: cfg})))

	// PHP Versions
	mux.Handle("/api/php-versions", middleware.RequireAuth(&handlers.PHPVersionsHandler{}))

	// Panel Settings (admin + manager only)
	mux.Handle("/api/settings", middleware.RequireAuth(middleware.RequireCSRF(&handlers.SettingsHandler{DB: database, Cfg: cfg, Version: version})))

	// Users Management (admin only)
	usersHandler := &handlers.UsersHandler{DB: database, Middleware: middleware}
	mux.Handle("/api/users", middleware.RequireAuth(middleware.RequireCSRF(
		middleware.RequireRole("admin")(usersHandler),
	)))

	// phpMyAdmin auto-login (POST /api/pma returns a URL the frontend opens directly)
	pmaHandler := &handlers.PhpMyAdminHandler{DB: database}
	pmaHandler.CleanupStaleTempUsers()
	mux.Handle("/api/pma", middleware.RequireAuth(middleware.RequireCSRF(pmaHandler)))

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
			"favicon_url":        settings["favicon_url"],
			"primary_color":      settings["primary_color"],
			"accent_color":       settings["accent_color"],
			"recaptcha_site_key": settings["recaptcha_site_key"],
			"version":            version,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "data": public})
	})

	// Services
	mux.Handle("/api/services", middleware.RequireAuth(middleware.RequireCSRF(&handlers.ServicesHandler{DB: database})))

	// Vhost Editor
	mux.Handle("/api/vhost", middleware.RequireAuth(middleware.RequireCSRF(&handlers.VhostHandler{DB: database, Cfg: cfg})))

	// Site Backups
	mux.Handle("/api/backups", middleware.RequireAuth(middleware.RequireCSRF(&handlers.BackupHandler{DB: database, Cfg: cfg})))

	// Backup Methods (panel-wide, admin only)
	mux.Handle("/api/backup-methods", middleware.RequireAuth(middleware.RequireCSRF(
		middleware.RequireRole("admin")(&handlers.BackupMethodsHandler{DB: database}),
	)))

	// Site Logs
	mux.Handle("/api/logs", middleware.RequireAuth(&handlers.LogsHandler{DB: database}))

	// SSL Certificates (multi-cert)
	mux.Handle("/api/ssl-certs", middleware.RequireAuth(middleware.RequireCSRF(&handlers.SSLCertsHandler{DB: database, Cfg: cfg})))

	// Firewall (admin only)
	mux.Handle("/api/firewall", middleware.RequireAuth(middleware.RequireCSRF(
		middleware.RequireRole("admin")(&handlers.FirewallHandler{DB: database, Cfg: cfg}),
	)))

	// Serve uploaded files
	uploadsDir := filepath.Join(cfg.DataDir, "uploads")
	os.MkdirAll(uploadsDir, 0755)
	mux.Handle("/api/uploads/", http.StripPrefix("/api/uploads/", http.FileServer(http.Dir(uploadsDir))))

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
			if !verifyCaptcha(secretKey, req.CaptchaToken) {
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

// verifyCaptcha verifies a Google reCAPTCHA v2 token
func verifyCaptcha(secretKey, token string) bool {
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
		Success bool `json:"success"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return false
	}
	return result.Success
}

// Unused import suppression
var _ = fmt.Sprintf
