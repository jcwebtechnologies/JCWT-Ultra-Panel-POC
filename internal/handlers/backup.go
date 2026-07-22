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
	"github.com/jcwt/ultra-panel/internal/crypto"
	"github.com/jcwt/ultra-panel/internal/db"
	"github.com/jcwt/ultra-panel/internal/system"
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
					h.DB.UpdateBackupStatus(b["id"].(int64), status, filePath, parts[0], "")
				}
			}
		}
	}

	schedule, _ := h.DB.GetBackupSchedule(siteID)
	methods, _ := h.DB.ListBackupMethods()
	if methods == nil {
		methods = []map[string]interface{}{}
	}

	jsonSuccess(w, map[string]interface{}{
		"backups":  backups,
		"schedule": schedule,
		"methods":  methods,
	})
}

func (h *BackupHandler) create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SiteID   int64  `json:"site_id"`
		Type     string `json:"type"`      // "full", "files", "db"
		MethodID int64  `json:"method_id"` // 0 for local, > 0 for backup_methods record
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

	initialMethod := "local"
	if req.MethodID > 0 {
		if methodObj, err := h.DB.GetBackupMethod(req.MethodID); err == nil && methodObj != nil {
			if name, ok := methodObj["name"].(string); ok && name != "" {
				initialMethod = name
			}
		}
	}

	// Create a pending backup record
	id, err := h.DB.CreateBackupPending(req.SiteID, req.Type, initialMethod)
	if err != nil {
		jsonError(w, "failed to create backup record", http.StatusInternalServerError)
		return
	}

	// Return immediately with the backup ID for status polling
	jsonSuccess(w, map[string]interface{}{"id": id, "status": "in_progress"})

	// Run backup in background
	go h.runBackup(id, req.SiteID, req.Type, req.MethodID, site)
}

func (h *BackupHandler) runBackup(backupID, siteID int64, backupType string, methodID int64, site map[string]interface{}) {
	domain := site["domain"].(string)
	webRoot := site["web_root"].(string)
	sysUser := site["system_user"].(string)

	backupDir := filepath.Join(h.Cfg.WebRootBase, sysUser, "backups")
	exec.Command("sudo", "mkdir", "-p", backupDir).Run()
	exec.Command("sudo", "chown", sysUser+":"+sysUser, backupDir).Run()
	exec.Command("sudo", "chmod", "0755", backupDir).Run()

	timestamp := time.Now().Format("20060102-150405")
	backupName := fmt.Sprintf("%s-%s-%s.tar.gz", domain, backupType, timestamp)
	backupPath := filepath.Join(backupDir, backupName)

	// Create a staging directory for the backup contents
	stagingDir := filepath.Join(backupDir, fmt.Sprintf("staging-%s", timestamp))
	exec.Command("sudo", "mkdir", "-p", stagingDir).Run()
	stagingRel, _ := filepath.Rel("/home/"+sysUser, stagingDir)
	defer exec.Command("sudo", "/usr/local/sbin/panel-fsctl", "delete-staging", sysUser, stagingRel).Run()

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
		cleanWebRoot := strings.TrimSuffix(webRoot, "/")
		cmd := exec.Command("sudo", "rsync", "-a", "--delete", cleanWebRoot+"/", htdocsStaging+"/")
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
					teeCmd := exec.Command("sudo", "tee", dumpFile)

					pipe1, err := dumpCmd.StdoutPipe()
					if err != nil {
						log.Printf("Failed pipe1 for %s: %v", dbName, err)
						continue
					}
					gzipCmd.Stdin = pipe1

					pipe2, err := gzipCmd.StdoutPipe()
					if err != nil {
						log.Printf("Failed pipe2 for %s: %v", dbName, err)
						continue
					}
					teeCmd.Stdin = pipe2
					teeCmd.Stdout = nil

					if err := dumpCmd.Start(); err != nil {
						log.Printf("Failed to start mysqldump for %s: %v", dbName, err)
						continue
					}
					if err := gzipCmd.Start(); err != nil {
						log.Printf("Failed to start gzip for %s: %v", dbName, err)
						continue
					}
					if err := teeCmd.Start(); err != nil {
						log.Printf("Failed to start tee for %s: %v", dbName, err)
						continue
					}

					dumpCmd.Wait()
					gzipCmd.Wait()
					if err := teeCmd.Wait(); err != nil {
						log.Printf("Failed to write dump file %s: %v", dumpFile, err)
					} else {
						log.Printf("Successfully created database dump for %s at %s", dbName, dumpFile)
					}
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
		h.DB.UpdateBackupStatus(backupID, "failed", "", "", "")
		return
	}

	// Set correct ownership on the backup file (0644 so jcwt-panel daemon can read for SFTP upload)
	exec.Command("sudo", "chown", sysUser+":"+sysUser, backupPath).Run()
	exec.Command("sudo", "chmod", "0644", backupPath).Run()

	// Get backup file size
	var size string
	sizeCmd := exec.Command("sudo", "du", "-b", backupPath)
	if sizeOut, err := sizeCmd.Output(); err == nil {
		parts := strings.Fields(strings.TrimSpace(string(sizeOut)))
		if len(parts) > 0 {
			size = parts[0]
		}
	}

	methodLabel := "Local"

	// If a remote backup method was selected (e.g. SFTP), perform remote upload!
	if methodID > 0 {
		if methodObj, err := h.DB.GetBackupMethod(methodID); err == nil && methodObj != nil {
			mType, _ := methodObj["type"].(string)
			mName, _ := methodObj["name"].(string)

			if strings.ToLower(mType) == "sftp" {
				var cfg map[string]interface{}
				if cfgStr, ok := methodObj["config"].(string); ok {
					_ = json.Unmarshal([]byte(cfgStr), &cfg)
				}
				if cfg != nil {
					host, _ := cfg["host"].(string)
					portStr, _ := cfg["port"].(string)
					port, _ := strconv.Atoi(portStr)
					if port == 0 {
						if pNum, ok := cfg["port"].(float64); ok {
							port = int(pNum)
						} else {
							port = 22
						}
					}
					user, _ := cfg["username"].(string)
					authType, _ := cfg["auth_type"].(string)
					password, _ := cfg["password"].(string)
					privateKey, _ := cfg["private_key"].(string)
					passphrase, _ := cfg["key_passphrase"].(string)
					remotePath, _ := cfg["remote_path"].(string)

					if crypto.IsEncrypted(password) {
						password, _ = crypto.Decrypt(password)
					}
					if crypto.IsEncrypted(privateKey) {
						privateKey, _ = crypto.Decrypt(privateKey)
					}

					log.Printf("Uploading backup %d (%s) to remote SFTP %s:%d...", backupID, backupPath, host, port)
					if err := system.UploadViaSFTP(host, port, user, authType, password, privateKey, passphrase, remotePath, backupPath); err != nil {
						log.Printf("Backup %d remote SFTP upload failed: %v", backupID, err)
						h.DB.UpdateBackupStatus(backupID, "failed", backupPath, size, "SFTP (Failed)")
						return
					}
					methodLabel = fmt.Sprintf("SFTP (%s)", host)
					if mName != "" {
						methodLabel = mName
					}
				}
			}
		}
	}

	h.DB.UpdateBackupStatus(backupID, "completed", backupPath, size, methodLabel)
	log.Printf("Backup %d completed: %s (%s bytes, method: %s)", backupID, backupPath, size, methodLabel)

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
	backupPath, _ := backup["file_path"].(string)
	methodStr, _ := backup["method"].(string)
	fileName := filepath.Base(backupPath)

	var archivePathToUse string
	var cleanupTmpFile string

	// 1. Prioritize restoring from Remote Destination (SFTP) if backup was configured with SFTP/remote method
	if methodStr != "" && strings.ToLower(methodStr) != "local" {
		methods, _ := h.DB.ListBackupMethods()
		for _, m := range methods {
			mType, _ := m["type"].(string)
			mName, _ := m["name"].(string)
			if strings.EqualFold(mType, "sftp") || strings.EqualFold(mName, methodStr) || strings.HasPrefix(strings.ToLower(methodStr), "sftp") {
				if cfgStr, ok := m["config"].(string); ok {
					var cfg map[string]interface{}
					_ = json.Unmarshal([]byte(cfgStr), &cfg)
					if cfg != nil {
						host, _ := cfg["host"].(string)
						portStr, _ := cfg["port"].(string)
						port, _ := strconv.Atoi(portStr)
						if port == 0 {
							if pNum, ok := cfg["port"].(float64); ok {
								port = int(pNum)
							} else {
								port = 22
							}
						}
						user, _ := cfg["username"].(string)
						authType, _ := cfg["auth_type"].(string)
						password, _ := cfg["password"].(string)
						privateKey, _ := cfg["private_key"].(string)
						passphrase, _ := cfg["key_passphrase"].(string)
						remotePath, _ := cfg["remote_path"].(string)

						if crypto.IsEncrypted(password) {
							password, _ = crypto.Decrypt(password)
						}
						if crypto.IsEncrypted(privateKey) {
							privateKey, _ = crypto.Decrypt(privateKey)
						}

						tmpTarget := fmt.Sprintf("/tmp/jcwt-restore-%d-%s", req.BackupID, fileName)
						log.Printf("Fetching remote SFTP backup %s from %s for restore...", fileName, host)
						if err := system.DownloadViaSFTP(host, port, user, authType, password, privateKey, passphrase, remotePath, fileName, tmpTarget); err == nil {
							archivePathToUse = tmpTarget
							cleanupTmpFile = tmpTarget
							log.Printf("Successfully fetched remote backup %s to %s", fileName, tmpTarget)
						} else {
							log.Printf("Failed to fetch remote backup over SFTP: %v (falling back to local archive)", err)
						}
					}
				}
				break
			}
		}
	}

	// 2. Fallback to Local Archive on Disk if remote fetch wasn't performed or failed
	if archivePathToUse == "" {
		if exec.Command("sudo", "test", "-f", backupPath).Run() == nil {
			archivePathToUse = backupPath
			log.Printf("Restoring backup %d from local disk archive: %s", req.BackupID, backupPath)
		}
	}

	if cleanupTmpFile != "" {
		defer os.Remove(cleanupTmpFile)
	}

	if archivePathToUse == "" {
		jsonError(w, "backup archive file not found on remote storage or local disk", http.StatusNotFound)
		return
	}

	// List archive contents to detect backup style (no extraction needed)
	listCmd := exec.Command("sudo", "tar", "-tzf", archivePathToUse)
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
			homeDir := filepath.Dir(webRoot)
			restoreDir := filepath.Join(homeDir, "tmp", "restore-stage")
			exec.Command("sudo", "mkdir", "-p", restoreDir).Run()
			exec.Command("sudo", "chown", sysUser+":"+sysUser, restoreDir).Run()
			restoreRel, _ := filepath.Rel(homeDir, restoreDir)
			defer exec.Command("sudo", "/usr/local/sbin/panel-fsctl", "delete-staging", sysUser, restoreRel).Run()

			cmd := exec.Command("sudo", "tar", "-xzf", archivePathToUse, "-C", restoreDir, "./htdocs")
			if output, err := cmd.CombinedOutput(); err != nil {
				cmd = exec.Command("sudo", "tar", "-xzf", archivePathToUse, "-C", restoreDir, "htdocs")
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
			cmd := exec.Command("sudo", "tar", "-xzf", archivePathToUse, "-C", filepath.Dir(webRoot))
			if output, err := cmd.CombinedOutput(); err != nil {
				log.Printf("restore failed: %s", string(output))
				jsonError(w, "restore failed", http.StatusInternalServerError)
				return
			}
		}
		exec.Command("sudo", "chown", "-R", sysUser+":"+sysUser, webRoot).Run()
		restored = append(restored, "files")
	}

	// Restore databases — pipe directly from archive
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

			// Ensure database exists before importing
			createSQL := fmt.Sprintf("CREATE DATABASE IF NOT EXISTS `%s` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;", dbName)
			exec.Command("sudo", "mysql", "-e", createSQL).Run()

			tarCmd := exec.Command("sudo", "tar", "-xzf", archivePathToUse, "--to-stdout", dbFile)
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

	// Restore cron jobs
	if req.RestoreCron && isNewStyle && hasCron {
		cronCmd := exec.Command("sudo", "tar", "-xzf", archivePathToUse, "--to-stdout", "cron_jobs.json")
		cronData, err := cronCmd.Output()
		if err != nil {
			cronCmd = exec.Command("sudo", "tar", "-xzf", archivePathToUse, "--to-stdout", "./cron_jobs.json")
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

	backup, err := h.DB.GetBackup(id)
	if err != nil {
		jsonError(w, "backup not found", http.StatusNotFound)
		return
	}

	filePath, _ := backup["file_path"].(string)
	methodStr, _ := backup["method"].(string)

	// 1. Delete remote backup file if stored on remote destination (e.g. SFTP)
	if methodStr != "" && strings.ToLower(methodStr) != "local" {
		methods, _ := h.DB.ListBackupMethods()
		for _, m := range methods {
			mType, _ := m["type"].(string)
			mName, _ := m["name"].(string)
			if strings.EqualFold(mType, "sftp") || strings.EqualFold(mName, methodStr) || strings.HasPrefix(strings.ToLower(methodStr), "sftp") {
				if cfgStr, ok := m["config"].(string); ok {
					var cfg map[string]interface{}
					_ = json.Unmarshal([]byte(cfgStr), &cfg)
					if cfg != nil {
						host, _ := cfg["host"].(string)
						portStr, _ := cfg["port"].(string)
						port, _ := strconv.Atoi(portStr)
						if port == 0 {
							if pNum, ok := cfg["port"].(float64); ok {
								port = int(pNum)
							} else {
								port = 22
							}
						}
						user, _ := cfg["username"].(string)
						authType, _ := cfg["auth_type"].(string)
						password, _ := cfg["password"].(string)
						privateKey, _ := cfg["private_key"].(string)
						passphrase, _ := cfg["key_passphrase"].(string)
						remotePath, _ := cfg["remote_path"].(string)

						if crypto.IsEncrypted(password) {
							password, _ = crypto.Decrypt(password)
						}
						if crypto.IsEncrypted(privateKey) {
							privateKey, _ = crypto.Decrypt(privateKey)
						}

						rFileName := filepath.Base(filePath)
						log.Printf("Deleting remote SFTP backup file %s from %s...", rFileName, host)
						if err := system.DeleteViaSFTP(host, port, user, authType, password, privateKey, passphrase, remotePath, rFileName); err != nil {
							log.Printf("Failed to delete remote SFTP backup file %s: %v", rFileName, err)
						}
					}
				}
				break
			}
		}
	}

	// 2. Remove local file from disk via panel-fsctl helper
	if filePath != "" {
		const homeBase = "/home/"
		if strings.HasPrefix(filePath, homeBase) {
			parts := strings.SplitN(filePath[len(homeBase):], "/", 3)
			if len(parts) == 3 && parts[1] == "backups" {
				exec.Command("sudo", "/usr/local/sbin/panel-fsctl", "delete-backup",
					parts[0], filepath.Base(filePath)).Run()
			}
		}
	}

	// 3. Delete database record
	_, err = h.DB.DeleteBackup(id)
	if err != nil {
		jsonError(w, "failed to delete backup record", http.StatusInternalServerError)
		return
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

	filePath, _ := backup["file_path"].(string)
	methodStr, _ := backup["method"].(string)
	fileName := filepath.Base(filePath)

	var downloadPathToUse string
	var cleanupDownloadTmp string

	// 1. Check local file on disk
	if exec.Command("sudo", "test", "-f", filePath).Run() == nil {
		downloadPathToUse = filePath
	} else if methodStr != "" && strings.ToLower(methodStr) != "local" {
		// 2. If local file was deleted but backup was saved on remote SFTP, fetch remote file for download
		methods, _ := h.DB.ListBackupMethods()
		for _, m := range methods {
			mType, _ := m["type"].(string)
			mName, _ := m["name"].(string)
			if strings.EqualFold(mType, "sftp") || strings.EqualFold(mName, methodStr) || strings.HasPrefix(strings.ToLower(methodStr), "sftp") {
				if cfgStr, ok := m["config"].(string); ok {
					var cfg map[string]interface{}
					_ = json.Unmarshal([]byte(cfgStr), &cfg)
					if cfg != nil {
						host, _ := cfg["host"].(string)
						portStr, _ := cfg["port"].(string)
						port, _ := strconv.Atoi(portStr)
						if port == 0 {
							if pNum, ok := cfg["port"].(float64); ok {
								port = int(pNum)
							} else {
								port = 22
							}
						}
						user, _ := cfg["username"].(string)
						authType, _ := cfg["auth_type"].(string)
						password, _ := cfg["password"].(string)
						privateKey, _ := cfg["private_key"].(string)
						passphrase, _ := cfg["key_passphrase"].(string)
						remotePath, _ := cfg["remote_path"].(string)

						if crypto.IsEncrypted(password) {
							password, _ = crypto.Decrypt(password)
						}
						if crypto.IsEncrypted(privateKey) {
							privateKey, _ = crypto.Decrypt(privateKey)
						}

						tmpTarget := fmt.Sprintf("/tmp/jcwt-dl-%s", fileName)
						log.Printf("Fetching remote SFTP backup %s for download...", fileName)
						if err := system.DownloadViaSFTP(host, port, user, authType, password, privateKey, passphrase, remotePath, fileName, tmpTarget); err == nil {
							downloadPathToUse = tmpTarget
							cleanupDownloadTmp = tmpTarget
						}
					}
				}
				break
			}
		}
	}

	if cleanupDownloadTmp != "" {
		defer os.Remove(cleanupDownloadTmp)
	}

	if downloadPathToUse == "" {
		jsonError(w, "backup file not found on disk or remote storage", http.StatusNotFound)
		return
	}

	// Get file size for Content-Length header
	sizeCmd := exec.Command("sudo", "du", "-b", downloadPathToUse)
	if sizeOut, err := sizeCmd.Output(); err == nil {
		parts := strings.Fields(strings.TrimSpace(string(sizeOut)))
		if len(parts) > 0 {
			w.Header().Set("Content-Length", parts[0])
		}
	}

	w.Header().Set("Content-Type", "application/gzip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", fileName))
	w.Header().Set("Cache-Control", "no-store")

	// Stream file content via sudo cat
	catCmd := exec.Command("sudo", "cat", downloadPathToUse)
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

	action := r.URL.Query().Get("action")
	if action == "test-connection" && r.Method == "POST" {
		h.testConnection(w, r)
		return
	}

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
		for _, m := range methods {
			if cfg, ok := m["config"].(string); ok {
				m["config"] = sanitizeConfigForUI(cfg)
			}
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
		configToSave := processConfigForSave(req.Type, req.Config, "")
		id, err := h.DB.CreateBackupMethod(req.Name, req.Type, configToSave)
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
		// Retrieve existing record to retain encrypted password/key if unmodified
		existingConfig := ""
		if existing, err := h.DB.GetBackupMethod(req.ID); err == nil {
			if cfg, ok := existing["config"].(string); ok {
				existingConfig = cfg
			}
		}
		configToSave := processConfigForSave(req.Type, req.Config, existingConfig)
		if err := h.DB.UpdateBackupMethod(req.ID, req.Name, req.Type, configToSave, req.Enabled); err != nil {
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

func (h *BackupMethodsHandler) testConnection(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID     int64                  `json:"id"`
		Type   string                 `json:"type"`
		Config map[string]interface{} `json:"config"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Type == "sftp" {
		host, _ := req.Config["host"].(string)
		portStr, _ := req.Config["port"].(string)
		port, _ := strconv.Atoi(portStr)
		if port == 0 {
			if pNum, ok := req.Config["port"].(float64); ok {
				port = int(pNum)
			} else {
				port = 22
			}
		}
		user, _ := req.Config["username"].(string)
		authType, _ := req.Config["auth_type"].(string)
		password, _ := req.Config["password"].(string)
		privateKey, _ := req.Config["private_key"].(string)
		passphrase, _ := req.Config["key_passphrase"].(string)
		remotePath, _ := req.Config["remote_path"].(string)

		// If editing existing record and password/key is placeholder, load from DB
		if (password == "********" || privateKey == "[SSH Private Key Stored]" || password == "" && privateKey == "") && req.ID > 0 {
			if existing, err := h.DB.GetBackupMethod(req.ID); err == nil {
				if cfgStr, ok := existing["config"].(string); ok {
					var exMap map[string]interface{}
					if json.Unmarshal([]byte(cfgStr), &exMap) == nil {
						if password == "********" || password == "" {
							if exPass, ok := exMap["password"].(string); ok {
								password = exPass
							}
						}
						if privateKey == "[SSH Private Key Stored]" || privateKey == "" {
							if exKey, ok := exMap["private_key"].(string); ok {
								privateKey = exKey
							}
						}
					}
				}
			}
		}

		if crypto.IsEncrypted(password) {
			password, _ = crypto.Decrypt(password)
		}
		if crypto.IsEncrypted(privateKey) {
			privateKey, _ = crypto.Decrypt(privateKey)
		}

		if err := system.TestSFTPConnection(host, port, user, authType, password, privateKey, passphrase, remotePath); err != nil {
			jsonError(w, fmt.Sprintf("SFTP connection failed: %v", err), http.StatusBadRequest)
			return
		}

		jsonSuccess(w, map[string]interface{}{"message": "SFTP connection successful! Remote host reached and write permissions verified."})
		return
	}

	jsonSuccess(w, map[string]interface{}{"message": "Backup destination settings validated."})
}

func processConfigForSave(mType, cfgJSON, existingConfigJSON string) string {
	if mType != "sftp" {
		return cfgJSON
	}
	var newMap map[string]interface{}
	if err := json.Unmarshal([]byte(cfgJSON), &newMap); err != nil {
		return cfgJSON
	}
	var exMap map[string]interface{}
	if existingConfigJSON != "" {
		_ = json.Unmarshal([]byte(existingConfigJSON), &exMap)
	}

	// Handle password
	if pass, ok := newMap["password"].(string); ok {
		if pass == "********" && exMap != nil {
			newMap["password"] = exMap["password"]
		} else if pass != "" && !crypto.IsEncrypted(pass) {
			if enc, err := crypto.Encrypt(pass); err == nil {
				newMap["password"] = enc
			}
		}
	}

	// Handle private key
	if key, ok := newMap["private_key"].(string); ok {
		if key == "[SSH Private Key Stored]" && exMap != nil {
			newMap["private_key"] = exMap["private_key"]
		} else if key != "" && !crypto.IsEncrypted(key) {
			if enc, err := crypto.Encrypt(key); err == nil {
				newMap["private_key"] = enc
			}
		}
	}

	out, err := json.Marshal(newMap)
	if err != nil {
		return cfgJSON
	}
	return string(out)
}

func sanitizeConfigForUI(cfgJSON string) string {
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(cfgJSON), &m); err != nil {
		return cfgJSON
	}
	if pass, ok := m["password"].(string); ok && pass != "" {
		m["has_password"] = true
		m["password"] = "********"
	}
	if key, ok := m["private_key"].(string); ok && key != "" {
		m["has_private_key"] = true
		m["private_key"] = "[SSH Private Key Stored]"
	}
	out, _ := json.Marshal(m)
	return string(out)
}
