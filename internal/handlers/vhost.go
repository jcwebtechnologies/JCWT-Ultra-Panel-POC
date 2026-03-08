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
	confPath := filepath.Join(h.Cfg.NginxSitesAvailable, domain+".conf")

	data, err := os.ReadFile(confPath)
	if err != nil {
		jsonError(w, "could not read vhost config", http.StatusInternalServerError)
		return
	}

	// Replace actual values with template variables for the editor
	config := string(data)
	config = strings.ReplaceAll(config, site["web_root"].(string), "{site_root}")
	config = strings.ReplaceAll(config, "/home/"+sysUser+"/logs", "{logs_dir}")
	config = strings.ReplaceAll(config, domain, "{domain}")
	config = strings.ReplaceAll(config, sysUser, "{user}")
	if phpVer := site["php_version"].(string); phpVer != "" {
		config = strings.ReplaceAll(config, phpVer, "{php_version}")
	}

	jsonSuccess(w, map[string]interface{}{
		"config": config,
		"path":   confPath,
	})
}

func (h *VhostHandler) update(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SiteID  int64  `json:"site_id"`
		Config  string `json:"config"`
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
	confPath := filepath.Join(h.Cfg.NginxSitesAvailable, domain+".conf")

	// Replace template variables with actual values
	config := req.Config
	config = strings.ReplaceAll(config, "{domain}", domain)
	config = strings.ReplaceAll(config, "{user}", sysUser)
	config = strings.ReplaceAll(config, "{site_root}", site["web_root"].(string))
	config = strings.ReplaceAll(config, "{php_version}", site["php_version"].(string))
	config = strings.ReplaceAll(config, "{logs_dir}", "/home/"+sysUser+"/logs")

	// Write config via tee
	cmd := exec.Command("sudo", "tee", confPath)
	cmd.Stdin = strings.NewReader(config)
	cmd.Stdout = nil
	if output, err := cmd.CombinedOutput(); err != nil {
		jsonError(w, fmt.Sprintf("failed to write vhost: %s", string(output)), http.StatusInternalServerError)
		return
	}

	// Validate nginx config
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

	accessLog, errorLog := siteLogFlags(site)
	vhostData := nginx.VHostData{
		Domain:      domain,
		Aliases:     site["aliases"].(string),
		User:        sysUser,
		SiteType:    site["site_type"].(string),
		PHPVersion:  site["php_version"].(string),
		ProxyURL:    site["proxy_url"].(string),
		WebRoot:     site["web_root"].(string),
		SSLType:     site["ssl_type"].(string),
		SSLCertPath: site["ssl_cert_path"].(string),
		SSLKeyPath:  site["ssl_key_path"].(string),
		AccessLog:   accessLog,
		ErrorLog:    errorLog,
	}

	if err := nginx.WriteVHost(h.Cfg.NginxSitesAvailable, h.Cfg.NginxSitesEnabled, domain, vhostData); err != nil {
		jsonError(w, fmt.Sprintf("failed to regenerate vhost: %v", err), http.StatusInternalServerError)
		return
	}

	nginx.TestAndReload()

	// Read back the newly generated config and replace with template variables
	newConf, err := os.ReadFile(filepath.Join(h.Cfg.NginxSitesAvailable, domain+".conf"))
	if err != nil {
		jsonSuccess(w, map[string]interface{}{"message": "vhost reset to default and nginx reloaded"})
		return
	}
	config := string(newConf)
	config = strings.ReplaceAll(config, site["web_root"].(string), "{site_root}")
	config = strings.ReplaceAll(config, "/home/"+sysUser+"/logs", "{logs_dir}")
	config = strings.ReplaceAll(config, domain, "{domain}")
	config = strings.ReplaceAll(config, sysUser, "{user}")
	if phpVer := site["php_version"].(string); phpVer != "" {
		config = strings.ReplaceAll(config, phpVer, "{php_version}")
	}
	jsonSuccess(w, map[string]interface{}{"message": "vhost reset to default and nginx reloaded", "config": config})
}
