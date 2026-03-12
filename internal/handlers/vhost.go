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

	"github.com/jcwt/ultra-panel/internal/config"
	"github.com/jcwt/ultra-panel/internal/db"
	"github.com/jcwt/ultra-panel/internal/nginx"
)

type VhostHandler struct {
	DB  *db.DB
	Cfg *config.Config
}

// --- vhost template file helpers (used across vhost.go, sites.go, wordpress.go) ---

func vhostTemplatePath(dataDir, domain string) string {
	return filepath.Join(dataDir, "vhost-templates", domain+".tpl")
}

func loadVHostTemplate(dataDir, domain string) (string, error) {
	data, err := os.ReadFile(vhostTemplatePath(dataDir, domain))
	return string(data), err
}

func saveVHostTemplate(dataDir, domain, tpl string) error {
	dir := filepath.Join(dataDir, "vhost-templates")
	if err := os.MkdirAll(dir, 0750); err != nil {
		return err
	}
	return os.WriteFile(vhostTemplatePath(dataDir, domain), []byte(tpl), 0640)
}

func (h *VhostHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	switch r.Method {
	case "GET":
		h.read(w, r)
	case "PUT":
		h.update(w, r)
	case "POST":
		h.reset(w, r)
	default:
		http.Error(w, `{"success":false,"error":"method not allowed"}`, http.StatusMethodNotAllowed)
	}
}

func (h *VhostHandler) read(w http.ResponseWriter, r *http.Request) {
	siteID, err := strconv.ParseInt(r.URL.Query().Get("site_id"), 10, 64)
	if err != nil {
		jsonError(w, "invalid site_id", http.StatusBadRequest)
		return
	}

	site, err := h.DB.GetSite(siteID)
	if err != nil {
		jsonError(w, "site not found", http.StatusNotFound)
		return
	}

	domain := site["domain"].(string)
	sysUser := site["system_user"].(string)

	// Prefer the template file; if absent, generate from current nginx config
	tpl, tplErr := loadVHostTemplate(h.Cfg.DataDir, domain)
	if tplErr != nil {
		// Fall back: tokenise the live nginx config
		confPath := filepath.Join(h.Cfg.NginxSitesAvailable, domain+".conf")
		raw, readErr := os.ReadFile(confPath)
		if readErr != nil {
			jsonError(w, "could not read vhost config", http.StatusInternalServerError)
			return
		}
		tpl = tokenizeVHost(string(raw), site, sysUser)
		// Persist for future use
		saveVHostTemplate(h.Cfg.DataDir, domain, tpl)
	}

	jsonSuccess(w, map[string]interface{}{
		"config": tpl,
		"path":   vhostTemplatePath(h.Cfg.DataDir, domain),
	})
}

func (h *VhostHandler) update(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SiteID int64  `json:"site_id"`
		Config string `json:"config"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	site, err := h.DB.GetSite(req.SiteID)
	if err != nil {
		jsonError(w, "site not found", http.StatusNotFound)
		return
	}

	domain := site["domain"].(string)
	sysUser := site["system_user"].(string)
	siteType := site["site_type"].(string)

	// Save the template (user-edited, with {tokens})
	if err := saveVHostTemplate(h.Cfg.DataDir, domain, req.Config); err != nil {
		jsonError(w, fmt.Sprintf("failed to save vhost template: %v", err), http.StatusInternalServerError)
		return
	}

	// Build current WP security rules
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
		WordPressSecurityRules: wpSecurity,
	}

	// Expand tokens → actual config
	expanded := nginx.ExpandVHostTemplate(req.Config, vhostData)

	confPath := filepath.Join(h.Cfg.NginxSitesAvailable, domain+".conf")
	cmd := exec.Command("sudo", "tee", confPath)
	cmd.Stdin = strings.NewReader(expanded)
	cmd.Stdout = nil
	if output, err := cmd.CombinedOutput(); err != nil {
		jsonError(w, fmt.Sprintf("failed to write vhost: %s", string(output)), http.StatusInternalServerError)
		return
	}

	if err := nginx.TestConfig(); err != nil {
		jsonError(w, fmt.Sprintf("nginx config invalid: %v — config saved but not reloaded", err), http.StatusBadRequest)
		return
	}

	nginx.Reload()
	jsonSuccess(w, map[string]interface{}{"message": "vhost updated and nginx reloaded"})
}

func (h *VhostHandler) reset(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SiteID int64 `json:"site_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	site, err := h.DB.GetSite(req.SiteID)
	if err != nil {
		jsonError(w, "site not found", http.StatusNotFound)
		return
	}

	domain := site["domain"].(string)
	sysUser := site["system_user"].(string)
	siteType := site["site_type"].(string)

	accessLog, errorLog := siteLogFlags(site)

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
		AccessLog:              accessLog,
		ErrorLog:               errorLog,
		WordPressSecurityRules: wpSecurity,
	}

	// Generate fresh template and save
	newTpl, tplErr := nginx.GenerateVHostTemplate(vhostData)
	if tplErr != nil {
		jsonError(w, fmt.Sprintf("failed to generate vhost template: %v", tplErr), http.StatusInternalServerError)
		return
	}
	saveVHostTemplate(h.Cfg.DataDir, domain, newTpl)

	if err := nginx.WriteVHost(h.Cfg.NginxSitesAvailable, h.Cfg.NginxSitesEnabled, domain, vhostData); err != nil {
		jsonError(w, fmt.Sprintf("failed to regenerate vhost: %v", err), http.StatusInternalServerError)
		return
	}

	nginx.TestAndReload()

	jsonSuccess(w, map[string]interface{}{
		"config":  newTpl,
		"message": "vhost reset to default and nginx reloaded",
	})
}

// tokenizeVHost converts an actual nginx config to a template with {tokens}.
func tokenizeVHost(config string, site map[string]interface{}, sysUser string) string {
	domain := site["domain"].(string)
	webRoot := site["web_root"].(string)
	phpVer, _ := site["php_version"].(string)
	aliases, _ := site["aliases"].(string)
	proxyURL, _ := site["proxy_url"].(string)
	sslCert, _ := site["ssl_cert_path"].(string)
	sslKey, _ := site["ssl_key_path"].(string)
	logsDir := "/home/" + sysUser + "/logs"

	if aliases != "" {
		config = strings.ReplaceAll(config, " "+aliases, " {domain_aliases}")
	}
	if proxyURL != "" {
		config = strings.ReplaceAll(config, proxyURL, "{proxy_url}")
	}
	if sslCert != "" {
		config = strings.ReplaceAll(config, sslCert, "{ssl_cert}")
	}
	if sslKey != "" {
		config = strings.ReplaceAll(config, sslKey, "{ssl_key}")
	}
	config = strings.ReplaceAll(config, logsDir, "{logs_dir}")
	config = strings.ReplaceAll(config, webRoot, "{site_root}")
	if phpVer != "" {
		config = strings.ReplaceAll(config, "php"+phpVer+"-fpm", "php{php_version}-fpm")
	}
	// Try to tokenize WordPress security block (try both allowXMLRPC variants)
	for _, allowXMLRPC := range []bool{false, true} {
		wpRules := nginx.BuildWPSecurityRules(allowXMLRPC)
		if strings.Contains(config, wpRules) {
			config = strings.ReplaceAll(config, wpRules, "{wordpress_security}")
			break
		}
	}
	// Replace domain last to avoid partial matches when domain appears inside phpver-fpm socket paths etc.
	config = strings.ReplaceAll(config, domain, "{domain}")
	config = strings.ReplaceAll(config, sysUser, "{user}")
	return config
}
