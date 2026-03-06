package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/jcwt/ultra-panel/internal/config"
	"github.com/jcwt/ultra-panel/internal/db"
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
		h.list(w, r)
	case "POST":
		h.create(w, r)
	case "PUT":
		if r.URL.Query().Get("action") == "update-security" {
			h.updateSecurity(w, r)
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

func (h *SitesHandler) create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Domain     string `json:"domain"`
		Aliases    string `json:"aliases"`
		SystemUser string `json:"system_user"`
		SiteType   string `json:"site_type"`
		PHPVersion string `json:"php_version"`
		ProxyURL   string `json:"proxy_url"`
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
	if req.SiteType == "php" && !validVersions[req.PHPVersion] {
		jsonError(w, "invalid PHP version", http.StatusBadRequest)
		return
	}

	if req.SiteType == "proxy" && req.ProxyURL == "" {
		jsonError(w, "proxy url is required for proxy sites", http.StatusBadRequest)
		return
	}

	// Validate site type
	validTypes := map[string]bool{"php": true, "html": true, "proxy": true}
	if !validTypes[req.SiteType] {
		jsonError(w, "invalid site type (must be php, html, or proxy)", http.StatusBadRequest)
		return
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

	// Write welcome page
	if err := system.WriteWelcomePage(webRoot, req.SiteType, req.Domain, req.SystemUser); err != nil {
		// Log but don't fail the whole creation
		fmt.Printf("Failed to write welcome page: %v\n", err)
	}

	// PHP specific configs
	if req.SiteType == "php" {
		// Create default PHP settings
		h.DB.UpsertPHPSettings(id, "256M", 30, 60, 1000, "64M", "64M", "")

		// Generate PHP-FPM pool
		poolData := php.PoolData{
			User: req.SystemUser, PHPVersion: req.PHPVersion, WebRoot: webRoot,
			MemoryLimit: "256M", MaxExecutionTime: 30, MaxInputTime: 60,
			MaxInputVars: 1000, PostMaxSize: "64M", UploadMaxFilesize: "64M",
		}
		if err := php.WritePool(h.Cfg.PHPFPMBaseDir, req.PHPVersion, req.SystemUser, poolData); err != nil {
			jsonError(w, fmt.Sprintf("failed to write PHP pool: %v", err), http.StatusInternalServerError)
			return
		}
		php.RestartFPM(req.PHPVersion)
	}

	// Generate Nginx vhost
	vhostData := nginx.VHostData{
		Domain: req.Domain, Aliases: req.Aliases, User: req.SystemUser,
		SiteType: req.SiteType, PHPVersion: req.PHPVersion, ProxyURL: req.ProxyURL, 
		WebRoot: webRoot, SSLType: "none",
	}
	if err := nginx.WriteVHost(h.Cfg.NginxSitesAvailable, h.Cfg.NginxSitesEnabled, req.Domain, vhostData); err != nil {
		jsonError(w, fmt.Sprintf("failed to write nginx config: %v", err), http.StatusInternalServerError)
		return
	}
	if err := nginx.TestAndReload(); err != nil {
		jsonError(w, fmt.Sprintf("nginx config error: %v", err), http.StatusInternalServerError)
		return
	}

	jsonSuccess(w, map[string]interface{}{"id": id, "domain": req.Domain, "web_root": webRoot})
}

func (h *SitesHandler) update(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID         int64  `json:"id"`
		Domain     string `json:"domain"`
		Aliases    string `json:"aliases"`
		SiteType   string `json:"site_type"`
		PHPVersion string `json:"php_version"`
		ProxyURL   string `json:"proxy_url"`
		WebRoot    string `json:"web_root"`
		SSLType    string `json:"ssl_type"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Validate site type
	validTypes := map[string]bool{"php": true, "html": true, "proxy": true}
	if req.SiteType != "" && !validTypes[req.SiteType] {
		jsonError(w, "invalid site type (must be php, html, or proxy)", http.StatusBadRequest)
		return
	}

	// Validate proxy URL
	if req.SiteType == "proxy" && req.ProxyURL != "" && !strings.HasPrefix(req.ProxyURL, "http") {
		jsonError(w, "proxy URL must start with http:// or https://", http.StatusBadRequest)
		return
	}

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

	// Handle web root update for php/html sites
	webRoot := site["web_root"].(string)
	if req.WebRoot != "" && (req.SiteType == "php" || req.SiteType == "html") {
		// Validate: must be under /home/ and not contain path traversal
		if strings.Contains(req.WebRoot, "..") || !strings.HasPrefix(req.WebRoot, "/home/") {
			jsonError(w, "invalid web root: must be under /home/ and cannot contain '..'", http.StatusBadRequest)
			return
		}
		webRoot = req.WebRoot
	}

	// Update database
	if err := h.DB.UpdateSite(req.ID, req.Domain, req.Aliases, req.SiteType, req.PHPVersion, req.ProxyURL,
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

	// If changing site type away from PHP or updating PHP version, deal with FPM pool
	if oldSiteType == "php" && (req.SiteType != "php" || req.PHPVersion != site["php_version"].(string)) {
		oldVersion := site["php_version"].(string)
		php.RemovePool(h.Cfg.PHPFPMBaseDir, oldVersion, sysUser)
		php.RestartFPM(oldVersion)
	}

	// Create/Update FPM pool if it's currently a PHP site
	if req.SiteType == "php" && (oldSiteType != "php" || req.PHPVersion != site["php_version"].(string)) {
		phpSettings, _ := h.DB.GetPHPSettings(req.ID)
		
		// If getting settings failed (e.g. was html before and didn't have settings), create defaults
		if phpSettings == nil {
			h.DB.UpsertPHPSettings(req.ID, "256M", 30, 60, 1000, "64M", "64M", "")
			phpSettings, _ = h.DB.GetPHPSettings(req.ID)
		}

		poolData := php.PoolData{
			User: sysUser, PHPVersion: req.PHPVersion, WebRoot: site["web_root"].(string),
			MemoryLimit: phpSettings["memory_limit"].(string),
			MaxExecutionTime: phpSettings["max_execution_time"].(int),
			MaxInputTime: phpSettings["max_input_time"].(int),
			MaxInputVars: phpSettings["max_input_vars"].(int),
			PostMaxSize: phpSettings["post_max_size"].(string),
			UploadMaxFilesize: phpSettings["upload_max_filesize"].(string),
			CustomDirectives: phpSettings["custom_directives"].(string),
		}
		php.WritePool(h.Cfg.PHPFPMBaseDir, req.PHPVersion, sysUser, poolData)
		php.RestartFPM(req.PHPVersion)
	}

	// Regenerate Nginx config
	vhostData := nginx.VHostData{
		Domain: req.Domain, Aliases: req.Aliases, User: sysUser,
		SiteType: req.SiteType, PHPVersion: req.PHPVersion, ProxyURL: req.ProxyURL, 
		WebRoot: webRoot,
		SSLType: req.SSLType, SSLCertPath: site["ssl_cert_path"].(string),
		SSLKeyPath: site["ssl_key_path"].(string),
	}

	// Remove old config if domain changed
	oldDomain := site["domain"].(string)
	if oldDomain != req.Domain {
		nginx.RemoveVHost(h.Cfg.NginxSitesAvailable, h.Cfg.NginxSitesEnabled, oldDomain)
	}

	if err := nginx.WriteVHost(h.Cfg.NginxSitesAvailable, h.Cfg.NginxSitesEnabled, req.Domain, vhostData); err != nil {
		jsonError(w, fmt.Sprintf("failed to write nginx config: %v", err), http.StatusInternalServerError)
		return
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

	domain := site["domain"].(string)
	sysUser := site["system_user"].(string)
	phpVersion := site["php_version"].(string)

	// Remove configs
	nginx.RemoveVHost(h.Cfg.NginxSitesAvailable, h.Cfg.NginxSitesEnabled, domain)
	nginx.Reload()
	php.RemovePool(h.Cfg.PHPFPMBaseDir, phpVersion, sysUser)
	php.RestartFPM(phpVersion)
	system.ClearCrontab(sysUser)

	// Remove from DB
	h.DB.DeleteSite(id)

	// Remove system user (and home directory)
	system.DeleteSystemUser(sysUser)

	// Remove SSL certificates
	system.RemoveCert(h.Cfg.SSLBaseDir, domain)

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

func (h *SitesHandler) updateSecurity(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SiteID           int64                    `json:"site_id"`
		BasicAuthEnabled bool                     `json:"basic_auth_enabled"`
		BasicAuthUsers   []map[string]interface{} `json:"basic_auth_users"`
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

	_, err = h.DB.Conn.Exec("UPDATE sites SET basic_auth_enabled = ?, basic_auth_users = ? WHERE id = ?", enabled, string(usersJSON), req.SiteID)
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
			// Ensure nginx can read the file
			exec.Command("sudo", "chmod", "644", htpasswdPath).Run()
		} else if !req.BasicAuthEnabled {
			// Remove htpasswd file when disabling
			htpasswdPath := fmt.Sprintf("/etc/nginx/htpasswd/%s.htpasswd", domain)
			exec.Command("sudo", "rm", "-f", htpasswdPath).Run()
		}

		nginx.WriteVHost(h.Cfg.NginxSitesAvailable, h.Cfg.NginxSitesEnabled, domain, nginx.VHostData{
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
		})
		nginx.Reload()
	}

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
