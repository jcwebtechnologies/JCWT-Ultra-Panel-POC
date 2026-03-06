package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/jcwt/ultra-panel/internal/auth"
	"github.com/jcwt/ultra-panel/internal/db"
	"github.com/jcwt/ultra-panel/internal/totp"
)

// TwoFAHandler manages 2FA setup and management (protected endpoints)
type TwoFAHandler struct {
	DB      *db.DB
	AuthMgr *auth.Manager
}

func (h *TwoFAHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	action := r.URL.Query().Get("action")
	switch {
	case r.Method == "GET" && action == "status":
		h.status(w, r)
	case r.Method == "POST" && action == "setup":
		h.setup(w, r)
	case r.Method == "POST" && action == "enable":
		h.enable(w, r)
	case r.Method == "POST" && action == "disable":
		h.disable(w, r)
	default:
		jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *TwoFAHandler) getSession(r *http.Request) (*auth.Session, bool) {
	sessionID := auth.GetSessionID(r)
	return h.AuthMgr.GetSession(sessionID)
}

func (h *TwoFAHandler) status(w http.ResponseWriter, r *http.Request) {
	sess, ok := h.getSession(r)
	if !ok {
		jsonError(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	secret, _ := h.DB.GetTOTPSecret(sess.UserID)
	jsonSuccess(w, map[string]interface{}{
		"enabled": secret != "",
	})
}

func (h *TwoFAHandler) setup(w http.ResponseWriter, r *http.Request) {
	sess, ok := h.getSession(r)
	if !ok {
		jsonError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Check if already enabled
	existing, _ := h.DB.GetTOTPSecret(sess.UserID)
	if existing != "" {
		jsonError(w, "2FA is already enabled — disable it first to reconfigure", http.StatusConflict)
		return
	}

	secret, err := totp.GenerateSecret()
	if err != nil {
		jsonError(w, "failed to generate secret", http.StatusInternalServerError)
		return
	}

	// Get panel name for issuer
	settings, _ := h.DB.GetPanelSettings()
	issuer := "JCWT Ultra Panel"
	if settings != nil {
		if name, ok := settings["panel_name"].(string); ok && name != "" {
			issuer = name
		}
	}

	uri := totp.BuildOTPAuthURI(secret, sess.Username, issuer)
	jsonSuccess(w, map[string]interface{}{
		"secret": secret,
		"uri":    uri,
		"issuer": issuer,
	})
}

func (h *TwoFAHandler) enable(w http.ResponseWriter, r *http.Request) {
	sess, ok := h.getSession(r)
	if !ok {
		jsonError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req struct {
		Secret string `json:"secret"`
		Code   string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request", http.StatusBadRequest)
		return
	}

	if req.Secret == "" || req.Code == "" {
		jsonError(w, "secret and code are required", http.StatusBadRequest)
		return
	}

	if !totp.ValidateCode(req.Secret, req.Code) {
		jsonError(w, "Invalid verification code — make sure your authenticator app is synced", http.StatusBadRequest)
		return
	}

	if err := h.DB.SetTOTPSecret(sess.UserID, req.Secret); err != nil {
		jsonError(w, "failed to enable 2FA", http.StatusInternalServerError)
		return
	}

	jsonSuccess(w, map[string]interface{}{"enabled": true})
}

func (h *TwoFAHandler) disable(w http.ResponseWriter, r *http.Request) {
	sess, ok := h.getSession(r)
	if !ok {
		jsonError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request", http.StatusBadRequest)
		return
	}

	// Verify password
	_, hash, _, err := h.DB.GetAdminByUsername(sess.Username)
	if err != nil || !auth.CheckPassword(hash, req.Password) {
		jsonError(w, "incorrect password", http.StatusUnauthorized)
		return
	}

	if err := h.DB.SetTOTPSecret(sess.UserID, ""); err != nil {
		jsonError(w, "failed to disable 2FA", http.StatusInternalServerError)
		return
	}

	jsonSuccess(w, map[string]interface{}{"disabled": true})
}

// Verify handles unprotected 2FA verification during login
func (h *TwoFAHandler) Verify(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != "POST" {
		jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Token string `json:"token"`
		Code  string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request", http.StatusBadRequest)
		return
	}

	pending, ok := h.AuthMgr.Get2FAPending(req.Token)
	if !ok {
		jsonError(w, "Invalid or expired token — please log in again", http.StatusUnauthorized)
		return
	}

	secret, err := h.DB.GetTOTPSecret(pending.UserID)
	if err != nil || secret == "" {
		jsonError(w, "2FA not configured for this account", http.StatusBadRequest)
		return
	}

	if !totp.ValidateCode(secret, req.Code) {
		jsonError(w, "Invalid verification code", http.StatusUnauthorized)
		return
	}

	// Consume the pending token
	h.AuthMgr.Consume2FAPending(req.Token)

	// Create actual session
	sessionID, csrfToken := h.AuthMgr.CreateSession(pending.UserID, pending.Username, pending.Role)
	auth.SetSessionCookie(w, sessionID)

	jsonSuccess(w, map[string]interface{}{
		"csrf_token": csrfToken,
		"username":   pending.Username,
		"role":       pending.Role,
	})
}
