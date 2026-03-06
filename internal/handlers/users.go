package handlers

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strconv"

	"github.com/jcwt/ultra-panel/internal/auth"
	"github.com/jcwt/ultra-panel/internal/db"
)

type UsersHandler struct {
	DB         *db.DB
	Middleware *auth.Middleware
}

var usernameRegex = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_]{2,30}$`)
var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)

func (h *UsersHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case "GET":
		h.list(w, r)
	case "POST":
		h.create(w, r)
	case "PUT":
		h.update(w, r)
	case "DELETE":
		h.delete(w, r)
	default:
		http.Error(w, `{"success":false,"error":"method not allowed"}`, http.StatusMethodNotAllowed)
	}
}

func (h *UsersHandler) list(w http.ResponseWriter, r *http.Request) {
	users, err := h.DB.ListAdminUsers()
	if err != nil {
		jsonError(w, "failed to list users", http.StatusInternalServerError)
		return
	}
	if users == nil {
		users = []map[string]interface{}{}
	}
	jsonSuccess(w, users)
}

func (h *UsersHandler) create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Role     string `json:"role"`
		Email    string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Validate
	if !usernameRegex.MatchString(req.Username) {
		jsonError(w, "username must be 3-31 chars, start with letter, only letters/numbers/underscore", http.StatusBadRequest)
		return
	}
	if len(req.Password) < 8 {
		jsonError(w, "password must be at least 8 characters", http.StatusBadRequest)
		return
	}
	if len(req.Password) > 128 {
		jsonError(w, "password too long", http.StatusBadRequest)
		return
	}

	validRoles := map[string]bool{"admin": true, "manager": true, "viewer": true}
	if !validRoles[req.Role] {
		jsonError(w, "role must be admin, manager, or viewer", http.StatusBadRequest)
		return
	}
	if req.Email != "" && !emailRegex.MatchString(req.Email) {
		jsonError(w, "invalid email format", http.StatusBadRequest)
		return
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		jsonError(w, "failed to hash password", http.StatusInternalServerError)
		return
	}

	if err := h.DB.CreateAdminWithRole(req.Username, hash, req.Role, req.Email); err != nil {
		jsonError(w, "failed to create user (username may already exist)", http.StatusConflict)
		return
	}

	jsonSuccess(w, map[string]interface{}{"created": true})
}

func (h *UsersHandler) update(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID       int64  `json:"id"`
		Role     string `json:"role"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.ID <= 0 {
		jsonError(w, "user id is required", http.StatusBadRequest)
		return
	}

	// Prevent self-demotion from admin
	sess := h.Middleware.GetSessionFromRequest(r)
	if sess != nil && sess.UserID == req.ID && sess.Role == "admin" && req.Role != "admin" {
		jsonError(w, "cannot change your own admin role", http.StatusForbidden)
		return
	}

	validRoles := map[string]bool{"admin": true, "manager": true, "viewer": true}
	if req.Role != "" && !validRoles[req.Role] {
		jsonError(w, "role must be admin, manager, or viewer", http.StatusBadRequest)
		return
	}
	if req.Email != "" && !emailRegex.MatchString(req.Email) {
		jsonError(w, "invalid email format", http.StatusBadRequest)
		return
	}

	// Update role/email
	if req.Role != "" {
		if err := h.DB.UpdateAdminUser(req.ID, req.Role, req.Email); err != nil {
			jsonError(w, "failed to update user", http.StatusInternalServerError)
			return
		}
	}

	// Update password if provided
	if req.Password != "" {
		if len(req.Password) < 8 {
			jsonError(w, "password must be at least 8 characters", http.StatusBadRequest)
			return
		}
		hash, err := auth.HashPassword(req.Password)
		if err != nil {
			jsonError(w, "failed to hash password", http.StatusInternalServerError)
			return
		}
		if err := h.DB.UpdateAdminPassword(req.ID, hash); err != nil {
			jsonError(w, "failed to update password", http.StatusInternalServerError)
			return
		}
	}

	jsonSuccess(w, map[string]interface{}{"updated": true})
}

func (h *UsersHandler) delete(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Query().Get("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		jsonError(w, "invalid user id", http.StatusBadRequest)
		return
	}

	// Prevent self-deletion
	sess := h.Middleware.GetSessionFromRequest(r)
	if sess != nil && sess.UserID == id {
		jsonError(w, "cannot delete your own account", http.StatusForbidden)
		return
	}

	// Ensure at least one admin remains
	users, _ := h.DB.ListAdminUsers()
	adminCount := 0
	var targetRole string
	for _, u := range users {
		uid, _ := u["id"].(int64)
		role, _ := u["role"].(string)
		if uid == id {
			targetRole = role
		}
		if role == "admin" {
			adminCount++
		}
	}
	if targetRole == "admin" && adminCount <= 1 {
		jsonError(w, "cannot delete the last admin user", http.StatusForbidden)
		return
	}

	if err := h.DB.DeleteAdminUser(id); err != nil {
		jsonError(w, "failed to delete user", http.StatusInternalServerError)
		return
	}

	jsonSuccess(w, map[string]interface{}{"deleted": true})
}
