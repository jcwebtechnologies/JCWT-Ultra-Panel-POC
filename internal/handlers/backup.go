package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/jcwt/ultra-panel/internal/config"
	"github.com/jcwt/ultra-panel/internal/db"
)

type BackupHandler struct {
	DB  *db.DB
	Cfg *config.Config
}

func (h *BackupHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	switch r.Method {
	case "GET":
		h.list(w, r)
	case "POST":
		action := r.URL.Query().Get("action")
		switch action {
		case "restore":
			h.restore(w, r)
		case "create":
			h.create(w, r)
		case "schedule":
			h.updateSchedule(w, r)
		case "download-token":
			h.generateDownloadToken(w, r)
		default:
			h.create(w, r)
		}
	case "DELETE":
		h.delete(w, r)
	default:
		http.Error(w, `{"success":false,"error":"method not allowed"}`, http.StatusMethodNotAllowed)
	}
}

func (h *BackupHandler) list(w http.ResponseWriter, r *http.Request) {
	// Handle download action
	if r.URL.Query().Get("action") == "download" {
		h.download(w, r)
		return
	}

	siteIDStr := r.URL.Query().Get("site_id")
	if siteIDStr == "" {
		// List backup methods (panel-wide)
		methods, err := h.DB.ListBackupMethods()
		if err != nil {
			jsonError(w, "failed to list backup methods", http.StatusInternalServerError)
			return
		}
		if methods == nil {
			methods = []map[string]interface{}{}
		}
		jsonSuccess(w, map[string]interface{}{"methods": methods})
		return
	}

	siteID, err := strconv.ParseInt(siteIDStr, 10, 64)
	if err != nil {
		jsonError(w, "invalid site_id", http.StatusBadRequest)
		return
	}

	backups, err := h.DB.ListBackups(siteID)
	if err != nil {
		jsonError(w, "failed to list backups", http.StatusInternalServerError)
		return
	}
	if backups == nil {
		backups = []map[string]interface{}{}
	}

	schedule, _ := h.DB.GetBackupSchedule(siteID)

	jsonSuccess(w, map[string]interface{}{
		"backups":  backups,
		"schedule": schedule,
	})
}

func (h *BackupHandler) create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SiteID int64  `json:"site_id"`
		Type   string `json:"type"` // "full", "files", "database"
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Type == "" {
		req.Type = "full"
	}

	site, err := h.DB.GetSite(req.SiteID)
	if err != nil {
		jsonError(w, "site not found", http.StatusNotFound)
		return
	}

	domain := site["domain"].(string)
	webRoot := site["web_root"].(string)

	backupDir := filepath.Join(h.Cfg.DataDir, "backups", domain)
	os.MkdirAll(backupDir, 0750)

	timestamp := time.Now().Format("20060102-150405")
	backupName := fmt.Sprintf("%s-%s-%s.tar.gz", domain, req.Type, timestamp)
	backupPath := filepath.Join(backupDir, backupName)

	var tarArgs []string
	switch req.Type {
	case "files", "full":
		tarArgs = []string{"sudo", "tar", "-czf", backupPath, "-C", filepath.Dir(webRoot), filepath.Base(webRoot)}
	default:
		jsonError(w, "invalid backup type: use full, files, or database", http.StatusBadRequest)
		return
	}

	cmd := exec.Command(tarArgs[0], tarArgs[1:]...)
	if output, err := cmd.CombinedOutput(); err != nil {
		jsonError(w, fmt.Sprintf("backup failed: %s", string(output)), http.StatusInternalServerError)
		return
	}

	// If full backup, also dump databases for this site's user
	if req.Type == "full" {
		sysUser := site["system_user"].(string)
		dumpPath := filepath.Join(backupDir, fmt.Sprintf("%s-db-%s.sql.gz", domain, timestamp))
		dumpCmd := fmt.Sprintf("sudo mysqldump --all-databases --single-transaction -u root 2>/dev/null | gzip > %s", dumpPath)
		exec.Command("sudo", "bash", "-c", dumpCmd).Run()
		_ = sysUser
	}

	// Get backup file size
	var size string
	if info, err := os.Stat(backupPath); err == nil {
		size = strconv.FormatInt(info.Size(), 10)
	}

	id, err := h.DB.CreateBackup(req.SiteID, req.Type, "local", backupPath, size)
	if err != nil {
		jsonError(w, "backup created but failed to save record", http.StatusInternalServerError)
		return
	}

	// Clean old backups based on schedule retention
	schedule, _ := h.DB.GetBackupSchedule(req.SiteID)
	if retention, ok := schedule["retention"].(int); ok && retention > 0 {
		h.DB.CleanOldBackups(req.SiteID, retention)
	}

	jsonSuccess(w, map[string]interface{}{"id": id, "file": backupName, "size": size})
}

func (h *BackupHandler) restore(w http.ResponseWriter, r *http.Request) {
	var req struct {
		BackupID int64 `json:"backup_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	backup, err := h.DB.GetBackup(req.BackupID)
	if err != nil {
		jsonError(w, "backup not found", http.StatusNotFound)
		return
	}

	siteID := backup["site_id"].(int64)
	site, err := h.DB.GetSite(siteID)
	if err != nil {
		jsonError(w, "site not found", http.StatusNotFound)
		return
	}

	webRoot := site["web_root"].(string)
	filePath := backup["file_path"].(string)

	// Verify backup file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		jsonError(w, "backup file not found on disk", http.StatusNotFound)
		return
	}

	// Extract backup to web root
	cmd := exec.Command("sudo", "tar", "-xzf", filePath, "-C", filepath.Dir(webRoot))
	if output, err := cmd.CombinedOutput(); err != nil {
		jsonError(w, fmt.Sprintf("restore failed: %s", string(output)), http.StatusInternalServerError)
		return
	}

	// Fix ownership
	sysUser := site["system_user"].(string)
	exec.Command("sudo", "chown", "-R", sysUser+":"+sysUser, webRoot).Run()

	jsonSuccess(w, map[string]interface{}{"message": "backup restored successfully"})
}

func (h *BackupHandler) delete(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Query().Get("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		jsonError(w, "invalid id", http.StatusBadRequest)
		return
	}

	filePath, err := h.DB.DeleteBackup(id)
	if err != nil {
		jsonError(w, "backup not found", http.StatusNotFound)
		return
	}

	// Remove file from disk
	if filePath != "" {
		os.Remove(filePath)
	}

	jsonSuccess(w, map[string]interface{}{"message": "backup deleted"})
}

func (h *BackupHandler) download(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		jsonError(w, "download token required", http.StatusBadRequest)
		return
	}

	backup, err := h.DB.ValidateBackupDownloadToken(token)
	if err != nil {
		jsonError(w, "invalid or expired download token", http.StatusForbidden)
		return
	}

	filePath := backup["file_path"].(string)
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		jsonError(w, "backup file not found on disk", http.StatusNotFound)
		return
	}

	filename := filepath.Base(filePath)
	w.Header().Set("Content-Type", "application/gzip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	w.Header().Set("Cache-Control", "no-store")
	http.ServeFile(w, r, filePath)
}

// generateDownloadToken creates a one-time download token for a backup
func (h *BackupHandler) generateDownloadToken(w http.ResponseWriter, r *http.Request) {
	var req struct {
		BackupID int64 `json:"backup_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Verify backup exists
	if _, err := h.DB.GetBackup(req.BackupID); err != nil {
		jsonError(w, "backup not found", http.StatusNotFound)
		return
	}

	token, err := h.DB.GenerateBackupDownloadToken(req.BackupID)
	if err != nil {
		jsonError(w, "failed to generate download token", http.StatusInternalServerError)
		return
	}

	jsonSuccess(w, map[string]interface{}{"token": token})
}

func (h *BackupHandler) updateSchedule(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SiteID    int64  `json:"site_id"`
		Frequency string `json:"frequency"` // "disabled", "daily", "weekly", "monthly"
		Retention int    `json:"retention"`
		Method    string `json:"method"` // "local"
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	validFreqs := map[string]bool{"disabled": true, "daily": true, "weekly": true, "monthly": true}
	if !validFreqs[req.Frequency] {
		jsonError(w, "invalid frequency", http.StatusBadRequest)
		return
	}
	if req.Retention < 1 {
		req.Retention = 7
	}
	if req.Method == "" {
		req.Method = "local"
	}

	if err := h.DB.UpsertBackupSchedule(req.SiteID, req.Frequency, req.Retention, req.Method); err != nil {
		jsonError(w, "failed to update schedule", http.StatusInternalServerError)
		return
	}

	jsonSuccess(w, map[string]interface{}{"message": "schedule updated"})
}

// BackupMethodsHandler manages panel-wide backup methods
type BackupMethodsHandler struct {
	DB *db.DB
}

func (h *BackupMethodsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	switch r.Method {
	case "GET":
		methods, err := h.DB.ListBackupMethods()
		if err != nil {
			jsonError(w, "failed to list backup methods", http.StatusInternalServerError)
			return
		}
		if methods == nil {
			methods = []map[string]interface{}{}
		}
		jsonSuccess(w, methods)
	case "POST":
		var req struct {
			Name   string `json:"name"`
			Type   string `json:"type"`
			Config string `json:"config"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			jsonError(w, "invalid request body", http.StatusBadRequest)
			return
		}
		validTypes := map[string]bool{"local": true, "s3": true, "sftp": true, "gdrive": true, "dropbox": true}
		if !validTypes[req.Type] {
			jsonError(w, "invalid type", http.StatusBadRequest)
			return
		}
		if strings.TrimSpace(req.Name) == "" {
			jsonError(w, "name is required", http.StatusBadRequest)
			return
		}
		id, err := h.DB.CreateBackupMethod(req.Name, req.Type, req.Config)
		if err != nil {
			jsonError(w, "failed to create backup method", http.StatusInternalServerError)
			return
		}
		jsonSuccess(w, map[string]interface{}{"id": id})
	case "PUT":
		var req struct {
			ID      int64  `json:"id"`
			Name    string `json:"name"`
			Type    string `json:"type"`
			Config  string `json:"config"`
			Enabled bool   `json:"enabled"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			jsonError(w, "invalid request body", http.StatusBadRequest)
			return
		}
		if err := h.DB.UpdateBackupMethod(req.ID, req.Name, req.Type, req.Config, req.Enabled); err != nil {
			jsonError(w, "failed to update backup method", http.StatusInternalServerError)
			return
		}
		jsonSuccess(w, map[string]interface{}{"message": "updated"})
	case "DELETE":
		idStr := r.URL.Query().Get("id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			jsonError(w, "invalid id", http.StatusBadRequest)
			return
		}
		if err := h.DB.DeleteBackupMethod(id); err != nil {
			jsonError(w, "failed to delete", http.StatusInternalServerError)
			return
		}
		jsonSuccess(w, map[string]interface{}{"message": "deleted"})
	default:
		http.Error(w, `{"success":false,"error":"method not allowed"}`, http.StatusMethodNotAllowed)
	}
}
