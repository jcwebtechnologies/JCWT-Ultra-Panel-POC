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

	// Delete all associated DB users first
	dbUsers, _ := h.DB.ListDBUsersByDatabaseID(id)
	for _, u := range dbUsers {
		username := u["username"].(string)
		userID := u["id"].(int64)
		system.MariaDBDropUser(username)
		h.DB.DeleteDBUser(userID)
	}

	dbName, err := h.DB.DeleteDatabase(id)
	if err != nil {
		jsonError(w, "database not found", http.StatusNotFound)
		return
	}

	system.MariaDBDropDatabase(dbName)

	deletedUsers := make([]string, 0, len(dbUsers))
	for _, u := range dbUsers {
		deletedUsers = append(deletedUsers, u["username"].(string))
	}
	jsonSuccess(w, map[string]interface{}{"deleted": true, "deleted_users": deletedUsers})
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
		action := r.URL.Query().Get("action")
		switch action {
		case "privilege":
			h.updatePrivilege(w, r)
		default:
			h.changePassword(w, r)
		}
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
		Username       string `json:"username"`
		Password       string `json:"password"`
		DatabaseID     int64  `json:"database_id"`
		PrivilegeLevel string `json:"privilege_level"`
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

	// Validate privilege level
	validLevels := map[string]bool{"readonly": true, "readwrite": true, "full": true}
	if req.PrivilegeLevel == "" {
		req.PrivilegeLevel = "full"
	}
	if !validLevels[req.PrivilegeLevel] {
		jsonError(w, "invalid privilege_level: use readonly, readwrite, or full", http.StatusBadRequest)
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

	// Grant access with privilege level
	if err := system.MariaDBGrantAccess(req.Username, dbName, req.PrivilegeLevel); err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Store in panel DB
	id, err := h.DB.CreateDBUser(req.Username, req.DatabaseID, req.PrivilegeLevel)
	if err != nil {
		jsonError(w, "failed to save user record", http.StatusInternalServerError)
		return
	}

	jsonSuccess(w, map[string]interface{}{"id": id, "username": req.Username, "privilege_level": req.PrivilegeLevel})
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

func (h *DBUsersHandler) updatePrivilege(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID             int64  `json:"id"`
		PrivilegeLevel string `json:"privilege_level"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	validLevels := map[string]bool{"readonly": true, "readwrite": true, "full": true}
	if !validLevels[req.PrivilegeLevel] {
		jsonError(w, "invalid privilege_level: use readonly, readwrite, or full", http.StatusBadRequest)
		return
	}

	// Look up the user and their database
	var username string
	var dbID int64
	err := h.DB.Conn.QueryRow("SELECT username, database_id FROM db_users WHERE id = ?", req.ID).Scan(&username, &dbID)
	if err != nil {
		jsonError(w, "user not found", http.StatusNotFound)
		return
	}

	var dbName string
	h.DB.Conn.QueryRow("SELECT db_name FROM databases WHERE id = ?", dbID).Scan(&dbName)
	if dbName == "" {
		jsonError(w, "database not found", http.StatusNotFound)
		return
	}

	// Update MariaDB grants
	if err := system.MariaDBGrantAccess(username, dbName, req.PrivilegeLevel); err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Update panel DB
	if err := h.DB.UpdateDBUserPrivilege(req.ID, req.PrivilegeLevel); err != nil {
		jsonError(w, "failed to update privilege level", http.StatusInternalServerError)
		return
	}

	jsonSuccess(w, map[string]interface{}{"updated": true, "privilege_level": req.PrivilegeLevel})
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
