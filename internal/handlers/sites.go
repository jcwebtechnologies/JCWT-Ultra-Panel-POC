package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/jcwt/ultra-panel/internal/config"
	"github.com/jcwt/ultra-panel/internal/db"
	"github.com/jcwt/ultra-panel/internal/email"
	"github.com/jcwt/ultra-panel/internal/nginx"
	"github.com/jcwt/ultra-panel/internal/php"
	"github.com/jcwt/ultra-panel/internal/system"
)

type SitesHandler struct {
	DB  *db.DB
	Cfg *config.Config
}

var domainRegex = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9-]*[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9-]*[a-zA-Z0-9])?)*$`)
var userRegex = regexp.MustCompile(`^[a-z][a-z0-9_]{1,30}$`)

func (h *SitesHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case "GET":
		if r.URL.Query().Get("action") == "disk-usage" {
			h.diskUsage(w, r)
		} else if r.URL.Query().Get("action") == "resource-usage" {
			h.resourceUsage(w, r)
		} else {
			h.list(w, r)
		}
	case "POST":
		h.create(w, r)
	case "PUT":
		if r.URL.Query().Get("action") == "update-security" {
			h.updateSecurity(w, r)
		} else if r.URL.Query().Get("action") == "update-logs" {
			h.updateLogs(w, r)
		} else {
			h.update(w, r)
		}
	case "DELETE":
		h.delete(w, r)
	default:
		http.Error(w, `{"success":false,"error":"method not allowed"}`, http.StatusMethodNotAllowed)
	}
}

func (h *SitesHandler) list(w http.ResponseWriter, r *http.Request) {
	// Support lookup by token
	token := r.URL.Query().Get("token")
	if token != "" {
		site, err := h.DB.GetSiteByToken(token)
		if err != nil {
			jsonError(w, "site not found", http.StatusNotFound)
			return
		}
		phpSettings, _ := h.DB.GetPHPSettings(site["id"].(int64))
		site["php_settings"] = phpSettings
		jsonSuccess(w, site)
		return
	}

	idStr := r.URL.Query().Get("id")
	if idStr != "" {
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			jsonError(w, "invalid id", http.StatusBadRequest)
			return
		}
		site, err := h.DB.GetSite(id)
		if err != nil {
			jsonError(w, "site not found", http.StatusNotFound)
			return
		}
		// Also get PHP settings
		phpSettings, _ := h.DB.GetPHPSettings(id)
		site["php_settings"] = phpSettings
		jsonSuccess(w, site)
		return
	}

	sites, err := h.DB.ListSites()
	if err != nil {
		jsonError(w, "failed to list sites", http.StatusInternalServerError)
		return
	}
	if sites == nil {
		sites = []map[string]interface{}{}
	}
	jsonSuccess(w, sites)
}

func (h *SitesHandler) diskUsage(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Query().Get("id")
	if idStr == "" {
		jsonError(w, "id required", http.StatusBadRequest)
		return
	}
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		jsonError(w, "invalid id", http.StatusBadRequest)
		return
	}
	site, err := h.DB.GetSite(id)
	if err != nil {
		jsonError(w, "site not found", http.StatusNotFound)
		return
	}
	webRoot, _ := site["web_root"].(string)
	if webRoot == "" {
		jsonSuccess(w, map[string]interface{}{"size": "N/A"})
		return
	}
	out, err := exec.Command("sudo", "du", "-sh", webRoot).CombinedOutput()
	if err != nil {
		// Try parent directory as fallback
		parent := filepath.Dir(webRoot)
		out, err = exec.Command("sudo", "du", "-sh", parent).CombinedOutput()
		if err != nil {
			jsonSuccess(w, map[string]interface{}{"size": "N/A"})
			return
		}
	}
	fields := strings.Fields(string(out))
	size := "N/A"
	if len(fields) >= 1 {
		size = fields[0]
	}
	jsonSuccess(w, map[string]interface{}{"size": size})
}

func (h *SitesHandler) resourceUsage(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Query().Get("id")
	if idStr == "" {
		jsonError(w, "id required", http.StatusBadRequest)
		return
	}
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		jsonError(w, "invalid id", http.StatusBadRequest)
		return
	}
	site, err := h.DB.GetSite(id)
	if err != nil {
		jsonError(w, "site not found", http.StatusNotFound)
		return
	}
	sysUser, _ := site["system_user"].(string)
	if !userRegex.MatchString(sysUser) {
		jsonError(w, "invalid system user", http.StatusInternalServerError)
		return
	}

	uid, err := lookupUID(sysUser)
	if err != nil {
		jsonError(w, "could not resolve site user", http.StatusInternalServerError)
		return
	}

	type ProcessInfo struct {
		PID    string  `json:"pid"`
		Name   string  `json:"name"`
		Cmd    string  `json:"cmd"`
		MemKB  int64   `json:"mem_kb"`
		MemMB  float64 `json:"mem_mb"`
		CPUPct float64 `json:"cpu_pct"`
	}

	// Single /proc scan — reads RSS, name, UID, and cmdline for every process.
	all := scanProcAll()

	// Collect processes owned by this site's user.
	var siteEntries []procInfo
	for i := range all {
		if all[i].UID == uid {
			siteEntries = append(siteEntries, all[i])
		}
	}

	// One ps call for all site PIDs to get CPU% (exec.Command, no shell — injection-safe).
	sitePIDs := make([]string, 0, len(siteEntries))
	for _, p := range siteEntries {
		sitePIDs = append(sitePIDs, p.PID)
	}
	cpuMap := fetchCPUPercents(sitePIDs)

	// Build per-site process list, sorted by memory descending.
	procs := make([]ProcessInfo, 0, len(siteEntries))
	var totalMemKB int64
	var totalCPU float64
	for _, p := range siteEntries {
		cpu := cpuMap[p.PID]
		procs = append(procs, ProcessInfo{
			PID:    p.PID,
			Name:   p.Name,
			Cmd:    p.Cmd,
			MemKB:  p.RssKB,
			MemMB:  float64(p.RssKB) / 1024.0,
			CPUPct: cpu,
		})
		totalMemKB += p.RssKB
		totalCPU += cpu
	}
	sort.Slice(procs, func(i, j int) bool { return procs[i].MemKB > procs[j].MemKB })

	jsonSuccess(w, map[string]interface{}{
		"processes":     procs,
		"process_count": len(procs),
		"total_mem_kb":  totalMemKB,
		"total_mem_mb":  float64(totalMemKB) / 1024.0,
		"total_cpu":     totalCPU,
	})
}

// ---- /proc parsing helpers ----

// procInfo holds information about a single running process read from /proc.
type procInfo struct {
	PID   string
	Name  string
	UID   int
	RssKB int64
	Cmd   string
}

// lookupUID returns the numeric UID for a Linux username by reading /etc/passwd.
// Avoids spawning an external command; immune to command injection.
func lookupUID(username string) (int, error) {
	data, err := os.ReadFile("/etc/passwd")
	if err != nil {
		return -1, err
	}
	for _, line := range strings.Split(string(data), "\n") {
		if line == "" || line[0] == '#' {
			continue
		}
		// Format: username:password:UID:GID:comment:home:shell
		fields := strings.SplitN(line, ":", 4)
		if len(fields) < 4 {
			continue
		}
		if fields[0] == username {
			uid, err := strconv.Atoi(fields[2])
			if err != nil {
				return -1, fmt.Errorf("invalid uid for %s", username)
			}
			return uid, nil
		}
	}
	return -1, fmt.Errorf("user not found: %s", username)
}

// readProcEntry reads /proc/[pid]/status and /proc/[pid]/cmdline for one process.
// Returns nil if the process vanishes mid-read (handles the TOCTOU race gracefully).
func readProcEntry(pid string) *procInfo {
	data, err := os.ReadFile("/proc/" + pid + "/status")
	if err != nil {
		return nil // process exited between ReadDir and now
	}
	p := &procInfo{PID: pid, UID: -1}
	for _, line := range strings.Split(string(data), "\n") {
		switch {
		case strings.HasPrefix(line, "Name:\t"):
			p.Name = strings.TrimPrefix(line, "Name:\t")
		case strings.HasPrefix(line, "Uid:\t"):
			// Uid:\treal\teffective\tsaved\tfs
			if f := strings.Fields(line); len(f) >= 2 {
				p.UID, _ = strconv.Atoi(f[1])
			}
		case strings.HasPrefix(line, "VmRSS:\t"):
			// VmRSS:\tNNN kB
			if f := strings.Fields(line); len(f) >= 2 {
				p.RssKB, _ = strconv.ParseInt(f[1], 10, 64)
			}
		}
	}
	if p.UID < 0 {
		return nil
	}
	// cmdline is NUL-separated argv; replace NULs with spaces and truncate for display.
	if raw, err := os.ReadFile("/proc/" + pid + "/cmdline"); err == nil && len(raw) > 0 {
		for i, b := range raw {
			if b == 0 {
				raw[i] = ' '
			}
		}
		cmd := strings.TrimSpace(string(raw))
		if len(cmd) > 200 {
			cmd = cmd[:200]
		}
		p.Cmd = cmd
	}
	return p
}

// scanProcAll walks /proc and returns all readable process entries.
// Processes that vanish mid-scan are silently skipped.
func scanProcAll() []procInfo {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil
	}
	result := make([]procInfo, 0, 64)
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if _, err := strconv.ParseUint(e.Name(), 10, 32); err != nil {
			continue // not a PID directory
		}
		if p := readProcEntry(e.Name()); p != nil {
			result = append(result, *p)
		}
	}
	return result
}

// fetchCPUPercents runs a single ps call for the given PID list and returns pid→cpu%.
// Uses exec.Command (no shell) — the PID list cannot cause injection.
// Deduplicates and caps at 512 PIDs to guard against pathological inputs.
func fetchCPUPercents(pids []string) map[string]float64 {
	result := make(map[string]float64, len(pids))
	if len(pids) == 0 {
		return result
	}
	seen := make(map[string]bool, len(pids))
	uniq := make([]string, 0, len(pids))
	for _, p := range pids {
		if !seen[p] {
			seen[p] = true
			uniq = append(uniq, p)
		}
	}
	if len(uniq) > 512 {
		uniq = uniq[:512]
	}
	out, err := exec.Command("ps", "-p", strings.Join(uniq, ","), "-o", "pid=,%cpu=").Output()
	if err != nil {
		return result
	}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		f := strings.Fields(line)
		if len(f) == 2 {
			var cpu float64
			fmt.Sscanf(f[1], "%f", &cpu)
			result[strings.TrimSpace(f[0])] = cpu
		}
	}
	return result
}

func (h *SitesHandler) create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Domain        string `json:"domain"`
		Aliases       string `json:"aliases"`
		SystemUser    string `json:"system_user"`
		SiteType      string `json:"site_type"`
		PHPVersion    string `json:"php_version"`
		ProxyURL      string `json:"proxy_url"`
		WPAdminUser   string `json:"wp_admin_user"`
		WPAdminEmail  string `json:"wp_admin_email"`
		WPAdminPass   string `json:"wp_admin_password"`
		WPSiteTitle   string `json:"wp_site_title"`
		WPTablePrefix string `json:"wp_table_prefix"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Validate
	if !domainRegex.MatchString(req.Domain) {
		jsonError(w, "invalid domain name", http.StatusBadRequest)
		return
	}
	if !userRegex.MatchString(req.SystemUser) {
		jsonError(w, "invalid system user (lowercase letters, numbers, underscore, 2-31 chars)", http.StatusBadRequest)
		return
	}
	if req.SiteType == "" {
		req.SiteType = "php" // default
	}

	validVersions := map[string]bool{"8.2": true, "8.3": true, "8.4": true, "8.5": true}
	if (req.SiteType == "php" || req.SiteType == "wordpress") && !validVersions[req.PHPVersion] {
		jsonError(w, "invalid PHP version", http.StatusBadRequest)
		return
	}

	if req.SiteType == "proxy" && req.ProxyURL == "" {
		jsonError(w, "proxy url is required for proxy sites", http.StatusBadRequest)
		return
	}

	// Validate site type
	validTypes := map[string]bool{"php": true, "html": true, "proxy": true, "wordpress": true}
	if !validTypes[req.SiteType] {
		jsonError(w, "invalid site type (must be php, html, proxy, or wordpress)", http.StatusBadRequest)
		return
	}

	// Validate WordPress fields
	if req.SiteType == "wordpress" {
		if req.WPAdminEmail == "" || req.WPAdminPass == "" {
			jsonError(w, "WordPress admin email and password are required", http.StatusBadRequest)
			return
		}
		if req.WPAdminUser == "" {
			req.WPAdminUser = "admin"
		}
		if req.WPSiteTitle == "" {
			req.WPSiteTitle = req.Domain
		}
		// Validate table prefix: alphanumeric and underscore, must end with _
		if req.WPTablePrefix == "" {
			req.WPTablePrefix = "wp_"
		}
		if !regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*_$`).MatchString(req.WPTablePrefix) {
			jsonError(w, "invalid table prefix: must contain only letters, numbers, underscore and end with _", http.StatusBadRequest)
			return
		}
		if len(req.WPTablePrefix) > 20 {
			jsonError(w, "table prefix too long (max 20 characters)", http.StatusBadRequest)
			return
		}
	}

	// Validate proxy URL format
	if req.SiteType == "proxy" && !strings.HasPrefix(req.ProxyURL, "http") {
		jsonError(w, "proxy URL must start with http:// or https://", http.StatusBadRequest)
		return
	}

	// Validate aliases (comma-separated domains)
	if req.Aliases != "" {
		for _, alias := range strings.Split(req.Aliases, ",") {
			alias = strings.TrimSpace(alias)
			if alias != "" && !domainRegex.MatchString(alias) {
				jsonError(w, fmt.Sprintf("invalid alias domain: %s", alias), http.StatusBadRequest)
				return
			}
		}
	}

	// Validate lengths
	if len(req.Domain) > 253 {
		jsonError(w, "domain too long (max 253 characters)", http.StatusBadRequest)
		return
	}
	if len(req.Aliases) > 1000 {
		jsonError(w, "aliases too long (max 1000 characters)", http.StatusBadRequest)
		return
	}

	webRoot := filepath.Join(h.Cfg.WebRootBase, req.SystemUser, "htdocs")

	// Create system user (handles all types, proxy won't use htdocs but good for isolation)
	if err := system.CreateSystemUser(req.SystemUser, h.Cfg.WebRootBase); err != nil {
		jsonError(w, fmt.Sprintf("failed to create system user: %v", err), http.StatusInternalServerError)
		return
	}

	// Create site in DB
	id, err := h.DB.CreateSite(req.Domain, req.Aliases, req.SystemUser, req.SiteType, req.PHPVersion, req.ProxyURL, webRoot)
	if err != nil {
		system.DeleteSystemUser(req.SystemUser)
		jsonError(w, fmt.Sprintf("failed to create site: %v", err), http.StatusInternalServerError)
		return
	}

	// cleanupSite rolls back all work done after CreateSite on any subsequent failure.
	cleanupSite := func() {
		h.DB.DeleteSite(id)
		system.DeleteSystemUser(req.SystemUser)
		nginx.RemoveVHost(h.Cfg.NginxSitesAvailable, h.Cfg.NginxSitesEnabled, req.Domain)
		if req.SiteType == "php" || req.SiteType == "wordpress" {
			php.RemovePool(h.Cfg.PHPFPMBaseDir, req.PHPVersion, req.SystemUser)
		}
	}

	// Write welcome page (skip for wordpress — WP files will replace it)
	if req.SiteType != "wordpress" {
		if err := system.WriteWelcomePage(webRoot, req.SiteType, req.Domain, req.SystemUser); err != nil {
			fmt.Printf("Failed to write welcome page: %v\n", err)
		}
	}

	// PHP specific configs (applies to both php and wordpress types)
	if req.SiteType == "php" || req.SiteType == "wordpress" {
		memoryLimit := "128M"
		postMaxSize := "16M"
		uploadMaxFilesize := "16M"
		if req.SiteType == "wordpress" {
			memoryLimit = "256M"
			postMaxSize = "64M"
			uploadMaxFilesize = "64M"
		}

		h.DB.UpsertPHPSettings(id, memoryLimit, 30, 30, 1000, postMaxSize, uploadMaxFilesize, "")

		poolData := php.PoolData{
			User: req.SystemUser, PHPVersion: req.PHPVersion, WebRoot: webRoot,
			HomeDir:     filepath.Dir(webRoot),
			MemoryLimit: memoryLimit, MaxExecutionTime: 30, MaxInputTime: 30,
			MaxInputVars: 1000, PostMaxSize: postMaxSize, UploadMaxFilesize: uploadMaxFilesize,
		}
		if err := php.WritePool(h.Cfg.PHPFPMBaseDir, req.PHPVersion, req.SystemUser, poolData); err != nil {
			cleanupSite()
			jsonError(w, fmt.Sprintf("failed to write PHP pool: %v", err), http.StatusInternalServerError)
			return
		}
		php.RestartFPM(req.PHPVersion)
	}

	// WordPress setup: download, extract, create DB/user, generate wp-config.php
	if req.SiteType == "wordpress" {
		if err := h.setupWordPress(id, req.Domain, req.SystemUser, webRoot, req.PHPVersion, req.WPAdminUser, req.WPAdminEmail, req.WPAdminPass, req.WPSiteTitle, req.WPTablePrefix); err != nil {
			cleanupSite()
			jsonError(w, fmt.Sprintf("WordPress setup failed: %v", err), http.StatusInternalServerError)
			return
		}
	}

	// For nginx template, wordpress uses "wordpress" siteType for optimized vhost
	vhostSiteType := req.SiteType

	// Generate Nginx vhost (initially without SSL)
	vhostData := nginx.VHostData{
		Domain: req.Domain, Aliases: req.Aliases, User: req.SystemUser,
		SiteType: vhostSiteType, PHPVersion: req.PHPVersion, ProxyURL: req.ProxyURL,
		WebRoot: webRoot, SSLType: "none",
		AccessLog: true, ErrorLog: true,
	}
	if err := nginx.WriteVHost(h.Cfg.NginxSitesAvailable, h.Cfg.NginxSitesEnabled, req.Domain, vhostData); err != nil {
		cleanupSite()
		jsonError(w, fmt.Sprintf("failed to write nginx config: %v", err), http.StatusInternalServerError)
		return
	}
	if err := nginx.TestAndReload(); err != nil {
		cleanupSite()
		jsonError(w, fmt.Sprintf("nginx config error: %v", err), http.StatusInternalServerError)
		return
	}

	// Auto-generate self-signed SSL certificate
	certPath, keyPath, sslErr := system.GenerateSelfSignedCert(h.Cfg.SSLBaseDir, req.Domain)

	// Set up log rotation (daily, 7 day retention)
	if err := system.WriteLogrotateConfig(h.Cfg.WebRootBase, req.SystemUser, req.Domain); err != nil {
		log.Printf("logrotate config for %s failed (non-fatal): %v", req.Domain, err)
	}
	if sslErr == nil {
		// Store in ssl_certificates table and activate
		certID, certErr := h.DB.CreateSSLCertificate(id, "self-signed", "Self-Signed (Auto)", certPath, keyPath, true)
		if certErr == nil {
			h.DB.ActivateSSLCertificate(id, certID)
		}

		h.DB.UpdateSite(id, req.Domain, req.Aliases, req.SiteType, req.PHPVersion, req.ProxyURL, "self-signed", certPath, keyPath)
		vhostData.SSLType = "self-signed"
		vhostData.SSLCertPath = certPath
		vhostData.SSLKeyPath = keyPath
		if err := nginx.WriteVHost(h.Cfg.NginxSitesAvailable, h.Cfg.NginxSitesEnabled, req.Domain, vhostData); err == nil {
			nginx.TestAndReload()
		}
	} else {
		log.Printf("Auto SSL for %s failed (site still created): %v", req.Domain, sslErr)
	}

	// Send WordPress creation email notification
	if req.SiteType == "wordpress" && req.WPAdminEmail != "" {
		go func() {
			sender := &email.Sender{DB: h.DB}
			vars := map[string]string{
				"Domain":      req.Domain,
				"SystemUser":  req.SystemUser,
				"PHPVersion":  req.PHPVersion,
				"WPAdminUser": req.WPAdminUser,
				"SiteTitle":   req.WPSiteTitle,
			}
			if err := sender.SendTemplatedEmail("wordpress_site_created", req.WPAdminEmail, vars); err != nil {
				log.Printf("WordPress creation email to %s failed (non-fatal): %v", req.WPAdminEmail, err)
			}
		}()
	}

	jsonSuccess(w, map[string]interface{}{"id": id, "domain": req.Domain, "web_root": webRoot})
}

// generatePassword creates a secure random password
func generatePassword(length int) string {
	b := make([]byte, length)
	rand.Read(b)
	return hex.EncodeToString(b)[:length]
}

// setupWordPress downloads, extracts, and configures WordPress
func (h *SitesHandler) setupWordPress(siteID int64, domain, sysUser, webRoot, phpVersion, wpAdminUser, wpAdminEmail, wpAdminPass, wpSiteTitle, wpTablePrefix string) error {
	homeDir := filepath.Dir(webRoot)
	tmpDir := filepath.Join(homeDir, "tmp")

	// Download latest WordPress
	wpArchive := filepath.Join(tmpDir, "wordpress.tar.gz")
	cmd := exec.Command("sudo", "wget", "-q", "https://wordpress.org/latest.tar.gz", "-O", wpArchive)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("download WordPress: %s", string(output))
	}

	// Extract WordPress to webroot (tar extracts to wordpress/ subfolder)
	cmd = exec.Command("sudo", "tar", "-xzf", wpArchive, "-C", tmpDir)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("extract WordPress: %s", string(output))
	}

	// Move WordPress files to webroot (overwrite welcome page)
	wpExtracted := filepath.Join(tmpDir, "wordpress")
	exec.Command("sudo", "rsync", "-a", "--delete", wpExtracted+"/", webRoot+"/").Run()
	exec.Command("sudo", "rm", "-rf", wpExtracted, wpArchive).Run()

	// Create WordPress database and user
	dbName := strings.ReplaceAll(sysUser, "-", "_") + "_wp"
	dbUser := dbName + "_u"
	dbPass := generatePassword(24)

	// Truncate dbUser to 32 chars (MariaDB max)
	if len(dbUser) > 32 {
		dbUser = dbUser[:32]
	}

	if err := system.MariaDBCreateDatabase(dbName); err != nil {
		return fmt.Errorf("create database: %v", err)
	}
	if err := system.MariaDBCreateUser(dbUser, dbPass); err != nil {
		return fmt.Errorf("create db user: %v", err)
	}
	if err := system.MariaDBGrantAccess(dbUser, dbName, "full"); err != nil {
		return fmt.Errorf("grant db access: %v", err)
	}

	// Record database and user in panel DB
	dbID, _ := h.DB.CreateDatabase(dbName, siteID)
	if dbID > 0 {
		h.DB.CreateDBUser(dbUser, dbID, "full")
	}

	// Install WP-CLI if not present
	wpCLI := "/usr/local/bin/wp"
	if _, err := exec.Command("test", "-f", wpCLI).CombinedOutput(); err != nil {
		exec.Command("sudo", "wget", "-q", "https://raw.githubusercontent.com/wp-cli/builds/gh-pages/phar/wp-cli.phar", "-O", wpCLI).Run()
		exec.Command("sudo", "chmod", "+x", wpCLI).Run()
	}

	// Generate wp-config.php using WP-CLI (uses wp-config-sample.php as base,
	// preserving the standard WordPress file structure with comments).
	phpBinPath := fmt.Sprintf("php%s", phpVersion)
	configCmd := exec.Command("sudo", "-u", sysUser,
		phpBinPath, wpCLI, "config", "create",
		"--path="+webRoot,
		"--dbname="+dbName,
		"--dbuser="+dbUser,
		"--dbpass="+dbPass,
		"--dbhost=localhost",
		"--dbcharset=utf8mb4",
		"--dbprefix="+wpTablePrefix,
		"--force",
	)
	if output, err := configCmd.CombinedOutput(); err != nil {
		log.Printf("WP-CLI config create failed for site %d: %v: %s", siteID, err, string(output))
		return fmt.Errorf("wp-config.php generation failed: %s", strings.TrimSpace(string(output)))
	}

	// Inject DISABLE_WP_CRON inside JCWT markers in wp-config.php
	wpConfigPath := filepath.Join(webRoot, "wp-config.php")
	if wpConfigContent, readErr := exec.Command("sudo", "cat", wpConfigPath).Output(); readErr == nil {
		updatedConfig := updateWPConfigSnippet(string(wpConfigContent),
			"define( 'DISABLE_WP_CRON', true );",
			"DISABLE_WP_CRON",
			true,
		)
		cmd := exec.Command("sudo", "tee", wpConfigPath)
		cmd.Stdin = strings.NewReader(updatedConfig)
		cmd.CombinedOutput()
		exec.Command("sudo", "chown", sysUser+":"+sysUser, wpConfigPath).Run()
		exec.Command("sudo", "chmod", "640", wpConfigPath).Run()
	}

	// Save initial WP tools state (wp-cron disabled by default)
	defaultState := WPToolsState{DisableWPCron: true}
	saveWPToolsState(h.Cfg.DataDir, sysUser, defaultState)

	// Run WordPress core install via WP-CLI
	installCmd := exec.Command("sudo", "-u", sysUser,
		phpBinPath, wpCLI, "core", "install",
		"--path="+webRoot,
		"--url=https://"+domain,
		"--title="+wpSiteTitle,
		"--admin_user="+wpAdminUser,
		"--admin_email="+wpAdminEmail,
		"--admin_password="+wpAdminPass,
		"--skip-email",
	)
	if output, err := installCmd.CombinedOutput(); err != nil {
		log.Printf("WP-CLI install output: %s", string(output))
		// Non-fatal — user can complete setup via browser
		log.Printf("WP-CLI install failed (user can complete via browser): %v", err)
	}

	// Set correct permissions
	exec.Command("sudo", "chown", "-R", sysUser+":"+sysUser, webRoot).Run()
	exec.Command("sudo", "find", webRoot, "-type", "d", "-exec", "chmod", "750", "{}", ";").Run()
	exec.Command("sudo", "find", webRoot, "-type", "f", "-exec", "chmod", "640", "{}", ";").Run()

	// Create system cron job for wp-cron.php (direct PHP call, every 30 minutes)
	cronLogDir := filepath.Join(homeDir, "logs", "custom-cron")
	exec.Command("sudo", "mkdir", "-p", cronLogDir).Run()
	exec.Command("sudo", "chown", "-R", sysUser+":"+sysUser, cronLogDir).Run()

	phpBin := fmt.Sprintf("/usr/bin/php%s", phpVersion)
	cronCommand := fmt.Sprintf("%s %s/wp-cron.php doing_wp_cron >> %s/wp-cron-exec.log 2>&1",
		phpBin, webRoot, cronLogDir)
	cronSchedule := "*/30 * * * *"
	cronID, cronErr := h.DB.CreateCronJob(siteID, cronSchedule, cronCommand)
	if cronErr == nil && cronID > 0 {
		// Sync crontab for the system user
		h.syncSiteCrontab(siteID, sysUser)
	}

	return nil
}

// syncSiteCrontab syncs the crontab for a site's system user
func (h *SitesHandler) syncSiteCrontab(siteID int64, sysUser string) {
	jobs, err := h.DB.ListCronJobs(siteID)
	if err != nil {
		return
	}
	var entries []system.CronEntry
	for _, j := range jobs {
		entries = append(entries, system.CronEntry{
			Schedule: j["schedule"].(string),
			Command:  j["command"].(string),
			Enabled:  j["enabled"].(bool),
		})
	}
	system.SyncCrontab(sysUser, entries)
}

func (h *SitesHandler) update(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID         int64  `json:"id"`
		Domain     string `json:"domain"`
		Aliases    string `json:"aliases"`
		PHPVersion string `json:"php_version"`
		ProxyURL   string `json:"proxy_url"`
		WebRoot    string `json:"web_root"`
		SSLType    string `json:"ssl_type"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Note: site_type is immutable after creation; it is read from the database.

	// Validate aliases
	if req.Aliases != "" {
		for _, alias := range strings.Split(req.Aliases, ",") {
			alias = strings.TrimSpace(alias)
			if alias != "" && !domainRegex.MatchString(alias) {
				jsonError(w, fmt.Sprintf("invalid alias domain: %s", alias), http.StatusBadRequest)
				return
			}
		}
	}

	site, err := h.DB.GetSite(req.ID)
	if err != nil {
		jsonError(w, "site not found", http.StatusNotFound)
		return
	}

	siteType := site["site_type"].(string)

	// Validate proxy URL for proxy sites
	if siteType == "proxy" && req.ProxyURL != "" && !strings.HasPrefix(req.ProxyURL, "http") {
		jsonError(w, "proxy URL must start with http:// or https://", http.StatusBadRequest)
		return
	}

	// Handle web root update for php/html/wordpress sites
	webRoot := site["web_root"].(string)
	if req.WebRoot != "" && siteType != "proxy" {
		// Validate: must be under /home/, no path traversal, no shell metacharacters
		if !isValidWebRoot(req.WebRoot) {
			jsonError(w, "invalid web root: must be a clean path under /home/ with no special characters", http.StatusBadRequest)
			return
		}
		// Ensure web root is under the correct user's home directory
		sysUser := site["system_user"].(string)
		expectedPrefix := filepath.Join("/home", sysUser) + "/"
		if !strings.HasPrefix(req.WebRoot, expectedPrefix) {
			jsonError(w, "web root must be under the site user's home directory", http.StatusBadRequest)
			return
		}
		webRoot = req.WebRoot
	}

	// Update database
	if err := h.DB.UpdateSite(req.ID, req.Domain, req.Aliases, siteType, req.PHPVersion, req.ProxyURL,
		req.SSLType, site["ssl_cert_path"].(string), site["ssl_key_path"].(string)); err != nil {
		jsonError(w, "failed to update site", http.StatusInternalServerError)
		return
	}

	// Update web root if changed
	if webRoot != site["web_root"].(string) {
		h.DB.Conn.Exec("UPDATE sites SET web_root=? WHERE id=?", webRoot, req.ID)
	}

	oldSiteType := site["site_type"].(string)
	sysUser := site["system_user"].(string)

	// If changing site type away from PHP/WordPress or updating PHP version, deal with FPM pool
	phpTypes := map[string]bool{"php": true, "wordpress": true}
	if phpTypes[oldSiteType] && (!phpTypes[siteType] || req.PHPVersion != site["php_version"].(string)) {
		oldVersion := site["php_version"].(string)
		php.RemovePool(h.Cfg.PHPFPMBaseDir, oldVersion, sysUser)
		php.RestartFPM(oldVersion)
	}

	// Create/Update FPM pool if it's currently a PHP/WordPress site
	if phpTypes[siteType] && (!phpTypes[oldSiteType] || req.PHPVersion != site["php_version"].(string)) {
		phpSettings, _ := h.DB.GetPHPSettings(req.ID)

		// If getting settings failed (e.g. was html before and didn't have settings), create defaults
		if phpSettings == nil {
			h.DB.UpsertPHPSettings(req.ID, "256M", 30, 60, 1000, "64M", "64M", "")
			phpSettings, _ = h.DB.GetPHPSettings(req.ID)
		}

		poolData := php.PoolData{
			User: sysUser, PHPVersion: req.PHPVersion, WebRoot: site["web_root"].(string),
			HomeDir:           filepath.Dir(site["web_root"].(string)),
			MemoryLimit:       phpSettings["memory_limit"].(string),
			MaxExecutionTime:  phpSettings["max_execution_time"].(int),
			MaxInputTime:      phpSettings["max_input_time"].(int),
			MaxInputVars:      phpSettings["max_input_vars"].(int),
			PostMaxSize:       phpSettings["post_max_size"].(string),
			UploadMaxFilesize: phpSettings["upload_max_filesize"].(string),
			CustomDirectives:  phpSettings["custom_directives"].(string),
		}
		php.WritePool(h.Cfg.PHPFPMBaseDir, req.PHPVersion, sysUser, poolData)
		php.RestartFPM(req.PHPVersion)
	}

	// Regenerate Nginx config
	accessLog, errorLog := siteLogFlags(site)
	wpState := loadWPToolsState(h.Cfg.DataDir, sysUser)

	wpSecurity := ""
	if siteType == "wordpress" {
		wpSecurity = nginx.BuildWPSecurityRules(wpState.AllowXMLRPC)
	}

	vhostData := nginx.VHostData{
		Domain: req.Domain, Aliases: req.Aliases, User: sysUser,
		SiteType: siteType, PHPVersion: req.PHPVersion, ProxyURL: req.ProxyURL,
		WebRoot: webRoot,
		SSLType: req.SSLType, SSLCertPath: site["ssl_cert_path"].(string),
		SSLKeyPath: site["ssl_key_path"].(string),
		AccessLog:  accessLog, ErrorLog: errorLog,
		WordPressSecurityRules: wpSecurity,
	}

	// Remove old config if domain changed
	oldDomain := site["domain"].(string)
	if oldDomain != req.Domain {
		nginx.RemoveVHost(h.Cfg.NginxSitesAvailable, h.Cfg.NginxSitesEnabled, oldDomain)
		// Rename vhost template too
		oldTplPath := vhostTemplatePath(h.Cfg.DataDir, oldDomain)
		newTplPath := vhostTemplatePath(h.Cfg.DataDir, req.Domain)
		os.Rename(oldTplPath, newTplPath)
	}

	// Use existing template only if SSL type AND aliases are unchanged; any structural
	// change forces a full regen so that {domain_aliases} is correctly inserted/removed.
	oldSSLType := site["ssl_type"].(string)
	oldAliases := site["aliases"].(string)
	tplPath := vhostTemplatePath(h.Cfg.DataDir, req.Domain)
	if tpl, tplErr := os.ReadFile(tplPath); tplErr == nil && oldSSLType == req.SSLType && oldAliases == req.Aliases {
		expanded := nginx.ExpandVHostTemplate(string(tpl), vhostData)
		if err := nginx.WriteConfigString(h.Cfg.NginxSitesAvailable, h.Cfg.NginxSitesEnabled, req.Domain, expanded); err != nil {
			jsonError(w, fmt.Sprintf("failed to write nginx config: %v", err), http.StatusInternalServerError)
			return
		}
	} else {
		// Full regen: generate template, save it, then apply
		newTpl, tplGenErr := nginx.GenerateVHostTemplate(vhostData)
		if tplGenErr == nil {
			saveVHostTemplate(h.Cfg.DataDir, req.Domain, newTpl)
		}
		if err := nginx.WriteVHost(h.Cfg.NginxSitesAvailable, h.Cfg.NginxSitesEnabled, req.Domain, vhostData); err != nil {
			jsonError(w, fmt.Sprintf("failed to write nginx config: %v", err), http.StatusInternalServerError)
			return
		}
	}
	nginx.TestAndReload()

	jsonSuccess(w, map[string]interface{}{"updated": true})
}

func (h *SitesHandler) delete(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Query().Get("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		jsonError(w, "invalid id", http.StatusBadRequest)
		return
	}

	site, err := h.DB.GetSite(id)
	if err != nil {
		jsonError(w, "site not found", http.StatusNotFound)
		return
	}

	// Check deletion protection
	if dp, ok := site["delete_protection"]; ok {
		if dpInt, ok2 := dp.(int); ok2 && dpInt == 1 {
			jsonError(w, "This site has deletion protection enabled. Disable it in the Security tab first.", http.StatusForbidden)
			return
		}
	}

	domain := site["domain"].(string)
	sysUser := site["system_user"].(string)
	phpVersion := site["php_version"].(string)

	// Remove configs
	nginx.RemoveVHost(h.Cfg.NginxSitesAvailable, h.Cfg.NginxSitesEnabled, domain)
	nginx.Reload()
	php.RemovePool(h.Cfg.PHPFPMBaseDir, phpVersion, sysUser)
	php.RestartFPM(phpVersion)
	system.ClearCrontab(sysUser)

	// Drop MariaDB users and databases for this site
	dbUsernames, _ := h.DB.ListDBUsersBySite(id)
	for _, u := range dbUsernames {
		system.MariaDBDropUser(u)
	}
	siteDbs, _ := h.DB.ListDatabasesBySite(id)
	for _, d := range siteDbs {
		if name, ok := d["db_name"].(string); ok {
			system.MariaDBDropDatabase(name)
		}
	}

	// Remove from DB (cascades to databases, db_users, cron_jobs, etc.)
	h.DB.DeleteSite(id)

	// Remove system user (and home directory)
	system.DeleteSystemUser(sysUser)

	// Remove SSL certificates
	system.RemoveCert(h.Cfg.SSLBaseDir, domain)

	// Remove logrotate config
	system.RemoveLogrotateConfig(domain)

	jsonSuccess(w, map[string]interface{}{"deleted": true})
}

// getSiteUserFromQuery extracts site_id, looks up site and returns system_user
func getSiteUserFromQuery(r *http.Request, database *db.DB) (int64, string, error) {
	idStr := r.URL.Query().Get("site_id")
	siteID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return 0, "", fmt.Errorf("invalid site_id")
	}
	site, err := database.GetSite(siteID)
	if err != nil {
		return 0, "", fmt.Errorf("site not found")
	}
	return siteID, site["system_user"].(string), nil
}

func jsonSuccess(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "data": data})
}

func jsonError(w http.ResponseWriter, msg string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": msg})
}

// siteLogFlags extracts access_log and error_log booleans from a site map.
// Defaults to true (enabled) if not present.
func siteLogFlags(site map[string]interface{}) (accessLog, errorLog bool) {
	accessLog = true
	errorLog = true
	if v, ok := site["access_log"]; ok {
		switch val := v.(type) {
		case int64:
			accessLog = val != 0
		case int:
			accessLog = val != 0
		case bool:
			accessLog = val
		}
	}
	if v, ok := site["error_log"]; ok {
		switch val := v.(type) {
		case int64:
			errorLog = val != 0
		case int:
			errorLog = val != 0
		case bool:
			errorLog = val
		}
	}
	return
}

func (h *SitesHandler) updateSecurity(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SiteID           int64                    `json:"site_id"`
		BasicAuthEnabled bool                     `json:"basic_auth_enabled"`
		BasicAuthUsers   []map[string]interface{} `json:"basic_auth_users"`
		DeleteProtection *bool                    `json:"delete_protection,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.SiteID <= 0 {
		jsonError(w, "site_id is required", http.StatusBadRequest)
		return
	}

	// Validate users
	for _, u := range req.BasicAuthUsers {
		username, _ := u["username"].(string)
		password, _ := u["password"].(string)
		if username == "" || password == "" {
			jsonError(w, "each user must have a non-empty username and password", http.StatusBadRequest)
			return
		}
		if len(username) > 64 || len(password) > 128 {
			jsonError(w, "username or password too long", http.StatusBadRequest)
			return
		}
	}

	usersJSON, err := json.Marshal(req.BasicAuthUsers)
	if err != nil {
		jsonError(w, "failed to encode users", http.StatusInternalServerError)
		return
	}

	enabled := 0
	if req.BasicAuthEnabled {
		enabled = 1
	}

	dp := 0
	if req.DeleteProtection != nil && *req.DeleteProtection {
		dp = 1
	}

	_, err = h.DB.Conn.Exec("UPDATE sites SET basic_auth_enabled = ?, basic_auth_users = ?, delete_protection = ? WHERE id = ?", enabled, string(usersJSON), dp, req.SiteID)
	if err != nil {
		jsonError(w, "failed to save security settings", http.StatusInternalServerError)
		return
	}

	// Regenerate nginx config with/without auth
	site, err := h.DB.GetSite(req.SiteID)
	if err == nil {
		domain, _ := site["domain"].(string)
		siteType, _ := site["site_type"].(string)
		phpVer, _ := site["php_version"].(string)
		webRoot, _ := site["web_root"].(string)
		proxyURL, _ := site["proxy_url"].(string)
		aliases, _ := site["aliases"].(string)
		sysUser, _ := site["system_user"].(string)
		sslType, _ := site["ssl_type"].(string)
		sslCert, _ := site["ssl_cert_path"].(string)
		sslKey, _ := site["ssl_key_path"].(string)

		// Generate htpasswd file if basic auth is enabled
		if req.BasicAuthEnabled && len(req.BasicAuthUsers) > 0 {
			if out, err := exec.Command("sudo", "mkdir", "-p", "/etc/nginx/htpasswd").CombinedOutput(); err != nil {
				log.Printf("Failed to create htpasswd dir: %s %v", string(out), err)
			}
			htpasswdPath := fmt.Sprintf("/etc/nginx/htpasswd/%s.htpasswd", domain)
			for i, u := range req.BasicAuthUsers {
				username, _ := u["username"].(string)
				password, _ := u["password"].(string)
				if username == "" || password == "" {
					continue
				}
				var cmd *exec.Cmd
				if i == 0 {
					// Create new file with first user (-c creates file, -B uses bcrypt, -b takes password from args)
					cmd = exec.Command("sudo", "htpasswd", "-c", "-B", "-b", htpasswdPath, username, password)
				} else {
					// Append to existing file
					cmd = exec.Command("sudo", "htpasswd", "-B", "-b", htpasswdPath, username, password)
				}
				if out, err := cmd.CombinedOutput(); err != nil {
					log.Printf("htpasswd error for user %s: %s %v", username, string(out), err)
				}
			}
			// Ensure nginx can read the file (owner root, group www-data)
			exec.Command("sudo", "chown", "root:www-data", htpasswdPath).Run()
			exec.Command("sudo", "chmod", "640", htpasswdPath).Run()
		} else if !req.BasicAuthEnabled {
			// Remove htpasswd file when disabling
			htpasswdPath := fmt.Sprintf("/etc/nginx/htpasswd/%s.htpasswd", domain)
			exec.Command("sudo", "rm", "-f", htpasswdPath).Run()
		}

		secAccessLog, secErrorLog := siteLogFlags(site)
		err = nginx.WriteVHost(h.Cfg.NginxSitesAvailable, h.Cfg.NginxSitesEnabled, domain, nginx.VHostData{
			Domain:           domain,
			Aliases:          aliases,
			User:             sysUser,
			WebRoot:          webRoot,
			SiteType:         siteType,
			PHPVersion:       phpVer,
			ProxyURL:         proxyURL,
			SSLType:          sslType,
			SSLCertPath:      sslCert,
			SSLKeyPath:       sslKey,
			BasicAuthEnabled: req.BasicAuthEnabled,
			AccessLog:        secAccessLog,
			ErrorLog:         secErrorLog,
		})
		if err != nil {
			log.Printf("Failed to write vhost for %s: %v", domain, err)
			jsonError(w, "failed to update nginx config", http.StatusInternalServerError)
			return
		}
		if err := nginx.TestAndReload(); err != nil {
			log.Printf("Nginx test/reload failed for %s: %v", domain, err)
			jsonError(w, "nginx configuration error — check server logs", http.StatusInternalServerError)
			return
		}
	}

	jsonSuccess(w, map[string]interface{}{"updated": true})
}

func (h *SitesHandler) updateLogs(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SiteID    int64 `json:"site_id"`
		AccessLog bool  `json:"access_log"`
		ErrorLog  bool  `json:"error_log"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.SiteID <= 0 {
		jsonError(w, "site_id is required", http.StatusBadRequest)
		return
	}

	site, err := h.DB.GetSite(req.SiteID)
	if err != nil {
		jsonError(w, "site not found", http.StatusNotFound)
		return
	}

	al := 0
	if req.AccessLog {
		al = 1
	}
	el := 0
	if req.ErrorLog {
		el = 1
	}
	_, err = h.DB.Conn.Exec("UPDATE sites SET access_log=?, error_log=? WHERE id=?", al, el, req.SiteID)
	if err != nil {
		jsonError(w, "failed to update log settings", http.StatusInternalServerError)
		return
	}

	// Regenerate vhost with new log settings
	site["access_log"] = al
	site["error_log"] = el
	domain := site["domain"].(string)
	sysUser := site["system_user"].(string)
	siteType := site["site_type"].(string)

	wpSecurity := ""
	if siteType == "wordpress" {
		wpState := loadWPToolsState(h.Cfg.DataDir, sysUser)
		wpSecurity = nginx.BuildWPSecurityRules(wpState.AllowXMLRPC)
	}

	vhostData := nginx.VHostData{
		Domain:                 domain,
		Aliases:                site["aliases"].(string),
		User:                   sysUser,
		SiteType:               siteType,
		PHPVersion:             site["php_version"].(string),
		ProxyURL:               site["proxy_url"].(string),
		WebRoot:                site["web_root"].(string),
		SSLType:                site["ssl_type"].(string),
		SSLCertPath:            site["ssl_cert_path"].(string),
		SSLKeyPath:             site["ssl_key_path"].(string),
		AccessLog:              req.AccessLog,
		ErrorLog:               req.ErrorLog,
		WordPressSecurityRules: wpSecurity,
	}

	// Log changes always force a full regen (structural change: on/off)
	os.Remove(vhostTemplatePath(h.Cfg.DataDir, domain))
	newTpl, tplGenErr := nginx.GenerateVHostTemplate(vhostData)
	if tplGenErr == nil {
		saveVHostTemplate(h.Cfg.DataDir, domain, newTpl)
	}
	if err := nginx.WriteVHost(h.Cfg.NginxSitesAvailable, h.Cfg.NginxSitesEnabled, domain, vhostData); err != nil {
		jsonError(w, fmt.Sprintf("failed to update nginx config: %v", err), http.StatusInternalServerError)
		return
	}
	nginx.TestAndReload()

	jsonSuccess(w, map[string]interface{}{"updated": true})
}

func sanitizeDomain(s string) string {
	return strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '.' || r == '-' {
			return r
		}
		return -1
	}, s)
}
