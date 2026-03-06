package handlers

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strconv"

	"github.com/jcwt/ultra-panel/internal/db"
	"github.com/jcwt/ultra-panel/internal/system"
)

var dbNameRegex = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_]{0,63}$`)
var dbUserRegex = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_]{0,31}$`)

type DatabasesHandler struct {
	DB *db.DB
}

func (h *DatabasesHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	switch r.Method {
	case "GET":
		h.list(w, r)
	case "POST":
		h.create(w, r)
	case "DELETE":
		h.delete(w, r)
	default:
		http.Error(w, `{"success":false,"error":"method not allowed"}`, http.StatusMethodNotAllowed)
	}
}

func (h *DatabasesHandler) list(w http.ResponseWriter, r *http.Request) {
	dbs, err := h.DB.ListDatabases()
	if err != nil {
		jsonError(w, "failed to list databases", http.StatusInternalServerError)
		return
	}
	if dbs == nil {
		dbs = []map[string]interface{}{}
	}
	jsonSuccess(w, dbs)
}

func (h *DatabasesHandler) create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		DBName string `json:"db_name"`
		SiteID int64  `json:"site_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.DBName == "" {
		jsonError(w, "db_name is required", http.StatusBadRequest)
		return
	}
	if !dbNameRegex.MatchString(req.DBName) {
		jsonError(w, "invalid database name: use only letters, numbers, underscore (max 64 chars, must start with letter)", http.StatusBadRequest)
		return
	}
	if req.SiteID <= 0 {
		jsonError(w, "site_id is required — databases must be linked to a site", http.StatusBadRequest)
		return
	}

	// Check if database name already exists in panel DB
	var existCount int
	h.DB.Conn.QueryRow("SELECT COUNT(*) FROM databases WHERE db_name = ?", req.DBName).Scan(&existCount)
	if existCount > 0 {
		jsonError(w, "a database with this name already exists", http.StatusConflict)
		return
	}

	// Create in MariaDB
	if err := system.MariaDBCreateDatabase(req.DBName); err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Store in panel DB
	id, err := h.DB.CreateDatabase(req.DBName, req.SiteID)
	if err != nil {
		jsonError(w, "failed to save database record", http.StatusInternalServerError)
		return
	}

	jsonSuccess(w, map[string]interface{}{"id": id, "db_name": req.DBName})
}

func (h *DatabasesHandler) delete(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Query().Get("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		jsonError(w, "invalid id", http.StatusBadRequest)
		return
	}

	dbName, err := h.DB.DeleteDatabase(id)
	if err != nil {
		jsonError(w, "database not found", http.StatusNotFound)
		return
	}

	system.MariaDBDropDatabase(dbName)
	jsonSuccess(w, map[string]interface{}{"deleted": true})
}

// DBUsersHandler manages MariaDB users
type DBUsersHandler struct {
	DB *db.DB
}

func (h *DBUsersHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	switch r.Method {
	case "GET":
		h.list(w, r)
	case "POST":
		h.create(w, r)
	case "PUT":
		h.changePassword(w, r)
	case "DELETE":
		h.delete(w, r)
	default:
		http.Error(w, `{"success":false,"error":"method not allowed"}`, http.StatusMethodNotAllowed)
	}
}

func (h *DBUsersHandler) list(w http.ResponseWriter, r *http.Request) {
	users, err := h.DB.ListDBUsers()
	if err != nil {
		jsonError(w, "failed to list users", http.StatusInternalServerError)
		return
	}
	if users == nil {
		users = []map[string]interface{}{}
	}
	jsonSuccess(w, users)
}

func (h *DBUsersHandler) create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username   string `json:"username"`
		Password   string `json:"password"`
		DatabaseID int64  `json:"database_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Username == "" || req.Password == "" {
		jsonError(w, "username and password are required", http.StatusBadRequest)
		return
	}
	if !dbUserRegex.MatchString(req.Username) {
		jsonError(w, "invalid username: use only letters, numbers, underscore (max 32 chars, must start with letter)", http.StatusBadRequest)
		return
	}
	if len(req.Password) < 8 {
		jsonError(w, "password must be at least 8 characters", http.StatusBadRequest)
		return
	}

	// Get db name for grant
	var dbName string
	h.DB.Conn.QueryRow("SELECT db_name FROM databases WHERE id = ?", req.DatabaseID).Scan(&dbName)
	if dbName == "" {
		jsonError(w, "database not found", http.StatusNotFound)
		return
	}

	// Check if username already exists in panel DB
	var existCount int
	h.DB.Conn.QueryRow("SELECT COUNT(*) FROM db_users WHERE username = ?", req.Username).Scan(&existCount)
	if existCount > 0 {
		jsonError(w, "a database user with this username already exists", http.StatusConflict)
		return
	}

	// Create in MariaDB
	if err := system.MariaDBCreateUser(req.Username, req.Password); err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Grant access
	if err := system.MariaDBGrantAccess(req.Username, dbName); err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Store in panel DB
	id, err := h.DB.CreateDBUser(req.Username, req.DatabaseID)
	if err != nil {
		jsonError(w, "failed to save user record", http.StatusInternalServerError)
		return
	}

	jsonSuccess(w, map[string]interface{}{"id": id, "username": req.Username})
}

func (h *DBUsersHandler) changePassword(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username    string `json:"username"`
		NewPassword string `json:"new_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Username == "" || req.NewPassword == "" {
		jsonError(w, "username and new_password are required", http.StatusBadRequest)
		return
	}
	if len(req.NewPassword) < 8 {
		jsonError(w, "password must be at least 8 characters", http.StatusBadRequest)
		return
	}

	if err := system.MariaDBChangePassword(req.Username, req.NewPassword); err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	jsonSuccess(w, map[string]interface{}{"updated": true})
}

func (h *DBUsersHandler) delete(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Query().Get("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		jsonError(w, "invalid id", http.StatusBadRequest)
		return
	}

	username, err := h.DB.DeleteDBUser(id)
	if err != nil {
		jsonError(w, "user not found", http.StatusNotFound)
		return
	}

	system.MariaDBDropUser(username)
	jsonSuccess(w, map[string]interface{}{"deleted": true})
}
