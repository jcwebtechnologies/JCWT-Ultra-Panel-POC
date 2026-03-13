package handlers

import (
	"encoding/json"
	"fmt"
	"log"
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
		case "status":
			h.status(w, r)
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

	// Fill in missing sizes for completed backups
	for _, b := range backups {
		status, _ := b["status"].(string)
		size, _ := b["size"].(string)
		filePath, _ := b["file_path"].(string)
		if status == "completed" && size == "" && filePath != "" {
			sizeCmd := exec.Command("sudo", "du", "-b", filePath)
			if sizeOut, err := sizeCmd.Output(); err == nil {
				parts := strings.Fields(strings.TrimSpace(string(sizeOut)))
				if len(parts) > 0 {
					b["size"] = parts[0]
					h.DB.UpdateBackupStatus(b["id"].(int64), status, filePath, parts[0])
				}
			}
		}
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

	// Create a pending backup record
	id, err := h.DB.CreateBackupPending(req.SiteID, req.Type, "local")
	if err != nil {
		jsonError(w, "failed to create backup record", http.StatusInternalServerError)
		return
	}

	// Return immediately with the backup ID for status polling
	jsonSuccess(w, map[string]interface{}{"id": id, "status": "in_progress"})

	// Run backup in background
	go h.runBackup(id, req.SiteID, req.Type, site)
}

func (h *BackupHandler) runBackup(backupID, siteID int64, backupType string, site map[string]interface{}) {
	domain := site["domain"].(string)
	webRoot := site["web_root"].(string)
	sysUser := site["system_user"].(string)

	backupDir := filepath.Join(h.Cfg.WebRootBase, sysUser, "backups")
	exec.Command("sudo", "mkdir", "-p", backupDir).Run()
	exec.Command("sudo", "chown", sysUser+":"+sysUser, backupDir).Run()
	exec.Command("sudo", "chmod", "0750", backupDir).Run()

	timestamp := time.Now().Format("20060102-150405")
	backupName := fmt.Sprintf("%s-%s-%s.tar.gz", domain, backupType, timestamp)
	backupPath := filepath.Join(backupDir, backupName)

	// Create a staging directory for the backup contents
	stagingDir := filepath.Join(backupDir, fmt.Sprintf("staging-%s", timestamp))
	exec.Command("sudo", "mkdir", "-p", stagingDir).Run()
	defer exec.Command("sudo", "rm", "-rf", stagingDir).Run()

	var backupErr error

	switch backupType {
	case "files":
		cmd := exec.Command("sudo", "tar", "-czf", backupPath, "-C", filepath.Dir(webRoot), filepath.Base(webRoot))
		if output, err := cmd.CombinedOutput(); err != nil {
			backupErr = fmt.Errorf("backup failed: %s", string(output))
		}

	case "full":
		htdocsStaging := filepath.Join(stagingDir, "htdocs")
		exec.Command("sudo", "mkdir", "-p", htdocsStaging).Run()
		cmd := exec.Command("sudo", "rsync", "-a", "--delete", webRoot+"/", htdocsStaging+"/")
		if output, err := cmd.CombinedOutput(); err != nil {
			backupErr = fmt.Errorf("backup failed copying files: %s", string(output))
		}

		if backupErr == nil {
			siteDbs, _ := h.DB.ListDatabasesBySite(siteID)
			if len(siteDbs) > 0 {
				dbStaging := filepath.Join(stagingDir, "databases")
				exec.Command("sudo", "mkdir", "-p", dbStaging).Run()
				for _, db := range siteDbs {
					dbName := db["db_name"].(string)
					if !isValidDBName(dbName) {
						log.Printf("Skipping invalid database name: %s", dbName)
						continue
					}
					dumpFile := filepath.Join(dbStaging, dbName+".sql.gz")
					dumpCmd := exec.Command("sudo", "mysqldump", "--single-transaction", dbName)
					gzipCmd := exec.Command("gzip")
					outFile, err := os.Create(dumpFile)
					if err != nil {
						log.Printf("Failed to create dump file %s: %v", dumpFile, err)
						continue
					}
					gzipCmd.Stdout = outFile
					pipe, err := dumpCmd.StdoutPipe()
					if err != nil {
						outFile.Close()
						continue
					}
					gzipCmd.Stdin = pipe
					dumpCmd.Start()
					gzipCmd.Start()
					dumpCmd.Wait()
					gzipCmd.Wait()
					outFile.Close()
				}
			}
		}

		if backupErr == nil {
			cronJobs, _ := h.DB.ListCronJobs(siteID)
			if len(cronJobs) > 0 {
				cronData, _ := json.MarshalIndent(cronJobs, "", "  ")
				cronFile := filepath.Join(stagingDir, "cron_jobs.json")
				writeCmd := exec.Command("sudo", "tee", cronFile)
				writeCmd.Stdin = strings.NewReader(string(cronData))
				writeCmd.Stdout = nil
				writeCmd.Run()
			}
		}

		if backupErr == nil {
			cmd = exec.Command("sudo", "tar", "-czf", backupPath, "-C", stagingDir, ".")
			if output, err := cmd.CombinedOutput(); err != nil {
				backupErr = fmt.Errorf("backup failed: %s", string(output))
			}
		}

	default:
		backupErr = fmt.Errorf("invalid backup type: %s", backupType)
	}

	if backupErr != nil {
		log.Printf("Backup %d failed: %v", backupID, backupErr)
		h.DB.UpdateBackupStatus(backupID, "failed", "", "")
		return
	}

	// Set correct ownership on the backup file
	exec.Command("sudo", "chown", sysUser+":"+sysUser, backupPath).Run()
	exec.Command("sudo", "chmod", "0640", backupPath).Run()

	// Get backup file size
	var size string
	sizeCmd := exec.Command("sudo", "du", "-b", backupPath)
	if sizeOut, err := sizeCmd.Output(); err == nil {
		parts := strings.Fields(strings.TrimSpace(string(sizeOut)))
		if len(parts) > 0 {
			size = parts[0]
		}
	}

	h.DB.UpdateBackupStatus(backupID, "completed", backupPath, size)
	log.Printf("Backup %d completed: %s (%s bytes)", backupID, backupPath, size)

	// Clean old backups based on schedule retention
	schedule, _ := h.DB.GetBackupSchedule(siteID)
	if retention, ok := schedule["retention"].(int); ok && retention > 0 {
		h.DB.CleanOldBackups(siteID, retention)
	}
}

func (h *BackupHandler) status(w http.ResponseWriter, r *http.Request) {
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

	jsonSuccess(w, map[string]interface{}{
		"id":     backup["id"],
		"status": backup["status"],
		"size":   backup["size"],
	})
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
	backupPath := backup["file_path"].(string)

	// Verify backup file exists
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		jsonError(w, "backup file not found on disk", http.StatusNotFound)
		return
	}

	// List archive contents to detect backup style (no extraction needed)
	listCmd := exec.Command("sudo", "tar", "-tzf", backupPath)
	listOut, err := listCmd.Output()
	if err != nil {
		jsonError(w, "failed to read backup archive", http.StatusInternalServerError)
		return
	}
	archiveFiles := strings.Split(strings.TrimSpace(string(listOut)), "\n")

	isNewStyle := false
	var dbFiles []string
	hasCron := false
	for _, f := range archiveFiles {
		if strings.HasPrefix(f, "./htdocs/") || f == "./htdocs" || strings.HasPrefix(f, "htdocs/") || f == "htdocs" {
			isNewStyle = true
		}
		if (strings.HasPrefix(f, "./databases/") || strings.HasPrefix(f, "databases/")) && strings.HasSuffix(f, ".sql.gz") {
			dbFiles = append(dbFiles, f)
		}
		if f == "./cron_jobs.json" || f == "cron_jobs.json" {
			hasCron = true
		}
	}

	var restored []string

	// Restore files
	if req.RestoreFiles {
		if isNewStyle {
			// Extract htdocs/ from archive directly into webroot using a temp dir under the user's home
			homeDir := filepath.Dir(webRoot)
			restoreDir := filepath.Join(homeDir, "tmp", "restore-stage")
			exec.Command("sudo", "mkdir", "-p", restoreDir).Run()
			exec.Command("sudo", "chown", sysUser+":"+sysUser, restoreDir).Run()
			// Clean up staging dir when done
			defer exec.Command("sudo", "rm", "-rf", restoreDir).Run()

			cmd := exec.Command("sudo", "tar", "-xzf", backupPath, "-C", restoreDir, "./htdocs")
			if output, err := cmd.CombinedOutput(); err != nil {
				// Try without ./ prefix (legacy backups)
				cmd = exec.Command("sudo", "tar", "-xzf", backupPath, "-C", restoreDir, "htdocs")
				if output2, err2 := cmd.CombinedOutput(); err2 != nil {
					log.Printf("restore files failed: %s / %s", string(output), string(output2))
					jsonError(w, "restore files failed", http.StatusInternalServerError)
					return
				}
			}

			htdocsStage := filepath.Join(restoreDir, "htdocs")
			cmd = exec.Command("sudo", "rsync", "-a", "--delete", htdocsStage+"/", webRoot+"/")
			if output, err := cmd.CombinedOutput(); err != nil {
				log.Printf("restore files (rsync) failed: %s", string(output))
				jsonError(w, "restore files failed", http.StatusInternalServerError)
				return
			}
		} else {
			// Old-style: extract directly over web root parent
			cmd := exec.Command("sudo", "tar", "-xzf", backupPath, "-C", filepath.Dir(webRoot))
			if output, err := cmd.CombinedOutput(); err != nil {
				log.Printf("restore failed: %s", string(output))
				jsonError(w, "restore failed", http.StatusInternalServerError)
				return
			}
		}
		exec.Command("sudo", "chown", "-R", sysUser+":"+sysUser, webRoot).Run()
		restored = append(restored, "files")
	}

	// Restore databases — pipe directly from archive, no temp files
	if req.RestoreDBs && len(dbFiles) > 0 {
		allowedDBs := make(map[string]bool)
		if len(req.RestoreDBNames) > 0 {
			for _, name := range req.RestoreDBNames {
				allowedDBs[name] = true
			}
		}
		for _, dbFile := range dbFiles {
			baseName := filepath.Base(dbFile)
			dbName := strings.TrimSuffix(baseName, ".sql.gz")
			if len(allowedDBs) > 0 && !allowedDBs[dbName] {
				continue
			}
			if !isValidDBName(dbName) {
				log.Printf("Skipping invalid database name during restore: %s", dbName)
				continue
			}

			// Ensure the database exists before importing
			createSQL := fmt.Sprintf("CREATE DATABASE IF NOT EXISTS `%s` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;", dbName)
			exec.Command("sudo", "mysql", "-e", createSQL).Run()

			// Extract .sql.gz to stdout and pipe through gunzip into mysql
			tarCmd := exec.Command("sudo", "tar", "-xzf", backupPath, "--to-stdout", dbFile)
			gunzipCmd := exec.Command("gunzip")
			mysqlCmd := exec.Command("sudo", "mysql", dbName)
			pipe1, err := tarCmd.StdoutPipe()
			if err != nil {
				log.Printf("Failed pipe for %s: %v", dbName, err)
				continue
			}
			gunzipCmd.Stdin = pipe1
			pipe2, err := gunzipCmd.StdoutPipe()
			if err != nil {
				log.Printf("Failed pipe for %s: %v", dbName, err)
				continue
			}
			mysqlCmd.Stdin = pipe2
			tarCmd.Start()
			gunzipCmd.Start()
			if output, err := mysqlCmd.CombinedOutput(); err != nil {
				log.Printf("Failed to restore database %s: %s: %s", dbName, err, string(output))
			} else {
				restored = append(restored, "database:"+dbName)
			}
			tarCmd.Wait()
			gunzipCmd.Wait()

			// Re-create panel DB record if missing
			existingDBs, _ := h.DB.ListDatabasesBySite(siteID)
			found := false
			for _, d := range existingDBs {
				if d["db_name"].(string) == dbName {
					found = true
					break
				}
			}
			if !found {
				h.DB.CreateDatabase(dbName, siteID)
			}
		}
	}

	// Restore cron jobs — extract to stdout, no temp file
	if req.RestoreCron && isNewStyle && hasCron {
		cronCmd := exec.Command("sudo", "tar", "-xzf", backupPath, "--to-stdout", "cron_jobs.json")
		cronData, err := cronCmd.Output()
		if err != nil {
			// Try with ./ prefix
			cronCmd = exec.Command("sudo", "tar", "-xzf", backupPath, "--to-stdout", "./cron_jobs.json")
			cronData, err = cronCmd.Output()
		}
		if err == nil {
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

	// Check file exists via sudo (panel user may not have direct access)
	checkCmd := exec.Command("sudo", "test", "-f", filePath)
	if err := checkCmd.Run(); err != nil {
		jsonError(w, "backup file not found on disk", http.StatusNotFound)
		return
	}

	// Get file size for Content-Length header
	sizeCmd := exec.Command("sudo", "du", "-b", filePath)
	if sizeOut, err := sizeCmd.Output(); err == nil {
		parts := strings.Fields(strings.TrimSpace(string(sizeOut)))
		if len(parts) > 0 {
			w.Header().Set("Content-Length", parts[0])
		}
	}

	filename := filepath.Base(filePath)
	w.Header().Set("Content-Type", "application/gzip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	w.Header().Set("Cache-Control", "no-store")

	// Stream file content via sudo cat (panel user may not have direct read access)
	catCmd := exec.Command("sudo", "cat", filePath)
	catCmd.Stdout = w
	catCmd.Run()
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
