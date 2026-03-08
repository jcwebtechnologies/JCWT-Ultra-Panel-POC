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
	sysUser := site["system_user"].(string)

	backupDir := filepath.Join(h.Cfg.WebRootBase, sysUser, "backups")
	exec.Command("sudo", "mkdir", "-p", backupDir).Run()
	exec.Command("sudo", "chown", sysUser+":"+sysUser, backupDir).Run()
	exec.Command("sudo", "chmod", "0750", backupDir).Run()

	timestamp := time.Now().Format("20060102-150405")
	backupName := fmt.Sprintf("%s-%s-%s.tar.gz", domain, req.Type, timestamp)
	backupPath := filepath.Join(backupDir, backupName)

	// Create a staging directory for the backup contents
	stagingDir := filepath.Join(backupDir, fmt.Sprintf("staging-%s", timestamp))
	exec.Command("sudo", "mkdir", "-p", stagingDir).Run()
	defer exec.Command("sudo", "rm", "-rf", stagingDir).Run()

	switch req.Type {
	case "files":
		// Files only — tar the web root directly
		cmd := exec.Command("sudo", "tar", "-czf", backupPath, "-C", filepath.Dir(webRoot), filepath.Base(webRoot))
		if output, err := cmd.CombinedOutput(); err != nil {
			jsonError(w, fmt.Sprintf("backup failed: %s", string(output)), http.StatusInternalServerError)
			return
		}

	case "full":
		// Step 1: Copy web files into staging using tar pipe (avoids sudo cp which may not be in sudoers)
		htdocsStaging := filepath.Join(stagingDir, "htdocs")
		exec.Command("sudo", "mkdir", "-p", htdocsStaging).Run()
		pipeCmd := fmt.Sprintf("sudo tar cf - -C '%s' . | sudo tar xf - -C '%s'",
			webRoot, htdocsStaging)
		cmd := exec.Command("bash", "-c", pipeCmd)
		if output, err := cmd.CombinedOutput(); err != nil {
			jsonError(w, fmt.Sprintf("backup failed copying files: %s", string(output)), http.StatusInternalServerError)
			return
		}

		// Step 2: Dump each site database individually
		siteDbs, _ := h.DB.ListDatabasesBySite(req.SiteID)
		if len(siteDbs) > 0 {
			dbStaging := filepath.Join(stagingDir, "databases")
			exec.Command("sudo", "mkdir", "-p", dbStaging).Run()
			for _, db := range siteDbs {
				dbName := db["db_name"].(string)
				dumpFile := filepath.Join(dbStaging, dbName+".sql.gz")
				dumpCmd := fmt.Sprintf("sudo mysqldump --single-transaction %s 2>/dev/null | gzip > %s",
					dbName, dumpFile)
				exec.Command("bash", "-c", dumpCmd).Run()
			}
		}

		// Step 3: Export cron jobs
		cronJobs, _ := h.DB.ListCronJobs(req.SiteID)
		if len(cronJobs) > 0 {
			cronData, _ := json.MarshalIndent(cronJobs, "", "  ")
			cronFile := filepath.Join(stagingDir, "cron_jobs.json")
			writeCmd := exec.Command("sudo", "tee", cronFile)
			writeCmd.Stdin = strings.NewReader(string(cronData))
			writeCmd.Stdout = nil
			writeCmd.Run()
		}

		// Step 4: Tar the entire staging directory
		cmd = exec.Command("sudo", "tar", "-czf", backupPath, "-C", stagingDir, ".")
		if output, err := cmd.CombinedOutput(); err != nil {
			jsonError(w, fmt.Sprintf("backup failed: %s", string(output)), http.StatusInternalServerError)
			return
		}

	default:
		jsonError(w, "invalid backup type: use full or files", http.StatusBadRequest)
		return
	}

	// Set correct ownership on the backup file
	exec.Command("sudo", "chown", sysUser+":"+sysUser, backupPath).Run()
	exec.Command("sudo", "chmod", "0640", backupPath).Run()

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
		BackupID       int64    `json:"backup_id"`
		RestoreFiles   bool     `json:"restore_files"`
		RestoreDBs     bool     `json:"restore_databases"`
		RestoreCron    bool     `json:"restore_cron"`
		RestoreDBNames []string `json:"restore_db_names"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Default: if no components specified, restore everything
	if !req.RestoreFiles && !req.RestoreDBs && !req.RestoreCron {
		req.RestoreFiles = true
		req.RestoreDBs = true
		req.RestoreCron = true
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
	sysUser := site["system_user"].(string)
	filePath := backup["file_path"].(string)

	// Verify backup file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		jsonError(w, "backup file not found on disk", http.StatusNotFound)
		return
	}

	// Extract to a temp directory first to inspect contents
	tmpDir := filepath.Join(os.TempDir(), fmt.Sprintf("jcwt-restore-%d", time.Now().UnixNano()))
	os.MkdirAll(tmpDir, 0750)
	defer os.RemoveAll(tmpDir)

	cmd := exec.Command("sudo", "tar", "-xzf", filePath, "-C", tmpDir)
	if output, err := cmd.CombinedOutput(); err != nil {
		jsonError(w, fmt.Sprintf("restore failed to extract: %s", string(output)), http.StatusInternalServerError)
		return
	}

	var restored []string

	// Check if this is a new-style backup (with htdocs/, databases/, cron_jobs.json)
	// or an old-style backup (just the web root directory)
	htdocsPath := filepath.Join(tmpDir, "htdocs")
	isNewStyle := false
	if info, err := os.Stat(htdocsPath); err == nil && info.IsDir() {
		isNewStyle = true
	}

	// Restore files
	if req.RestoreFiles {
		if isNewStyle {
			// New-style: copy htdocs contents to web root
			cmd = exec.Command("sudo", "rsync", "-a", "--delete", htdocsPath+"/", webRoot+"/")
			if output, err := cmd.CombinedOutput(); err != nil {
				jsonError(w, fmt.Sprintf("restore files failed: %s", string(output)), http.StatusInternalServerError)
				return
			}
		} else {
			// Old-style: extract directly over web root parent
			cmd = exec.Command("sudo", "tar", "-xzf", filePath, "-C", filepath.Dir(webRoot))
			if output, err := cmd.CombinedOutput(); err != nil {
				jsonError(w, fmt.Sprintf("restore failed: %s", string(output)), http.StatusInternalServerError)
				return
			}
		}
		exec.Command("sudo", "chown", "-R", sysUser+":"+sysUser, webRoot).Run()
		restored = append(restored, "files")
	}

	// Restore databases
	if req.RestoreDBs && isNewStyle {
		dbDir := filepath.Join(tmpDir, "databases")
		if entries, err := os.ReadDir(dbDir); err == nil {
			// Build allowed set if specific databases requested
			allowedDBs := make(map[string]bool)
			if len(req.RestoreDBNames) > 0 {
				for _, name := range req.RestoreDBNames {
					allowedDBs[name] = true
				}
			}
			for _, entry := range entries {
				if strings.HasSuffix(entry.Name(), ".sql.gz") {
					dbName := strings.TrimSuffix(entry.Name(), ".sql.gz")
					// Skip if specific databases requested and this one isn't in the list
					if len(allowedDBs) > 0 && !allowedDBs[dbName] {
						continue
					}
					dumpFile := filepath.Join(dbDir, entry.Name())
					importCmd := fmt.Sprintf("gunzip -c %s | sudo mysql %s", dumpFile, dbName)
					exec.Command("bash", "-c", importCmd).Run()
					restored = append(restored, "database:"+dbName)
				}
			}
		}
	}

	// Restore cron jobs
	if req.RestoreCron && isNewStyle {
		cronFile := filepath.Join(tmpDir, "cron_jobs.json")
		if cronData, err := os.ReadFile(cronFile); err == nil {
			var cronJobs []map[string]interface{}
			if json.Unmarshal(cronData, &cronJobs) == nil {
				for _, job := range cronJobs {
					schedule, _ := job["schedule"].(string)
					command, _ := job["command"].(string)
					enabled, _ := job["enabled"].(bool)
					if schedule != "" && command != "" {
						id, err := h.DB.CreateCronJob(siteID, schedule, command)
						if err == nil && !enabled {
							h.DB.UpdateCronJob(id, schedule, command, false)
						}
					}
				}
				restored = append(restored, "cron")
			}
		}
	}

	jsonSuccess(w, map[string]interface{}{
		"message":  "backup restored successfully",
		"restored": restored,
	})
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

	// Remove file from disk (under user home, need sudo)
	if filePath != "" {
		exec.Command("sudo", "rm", "-f", filePath).Run()
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
