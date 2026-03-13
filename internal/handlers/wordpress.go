package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/jcwt/ultra-panel/internal/config"
	"github.com/jcwt/ultra-panel/internal/db"
	"github.com/jcwt/ultra-panel/internal/nginx"
)

const (
	wpMarkerStart = "/* JCWT Panel Optimization START */"
	wpMarkerEnd   = "/* JCWT Panel Optimization END */"
)

// WPToolsState persists per-site WordPress tool toggles.
// Stored as JSON at ~{sysUser}/.panel/wp-tools.json
type WPToolsState struct {
	AllowXMLRPC     bool `json:"allow_xmlrpc"`
	DisableWPCron   bool `json:"disable_wp_cron"`
	DisableFileEdit bool `json:"disable_file_edit"`
}

// WordPressHandler serves GET/POST /api/wordpress
type WordPressHandler struct {
	DB  *db.DB
	Cfg *config.Config
}

func (h *WordPressHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case "GET":
		h.status(w, r)
	case "POST":
		h.action(w, r)
	default:
		jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// GET /api/wordpress?action=status&site_id=X
func (h *WordPressHandler) status(w http.ResponseWriter, r *http.Request) {
	siteID, site, sysUser, _, err := h.resolveSite(r)
	_ = siteID
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	if site["site_type"] != "wordpress" {
		jsonError(w, "not a wordpress site", http.StatusBadRequest)
		return
	}

	state := loadWPToolsState(h.Cfg.DataDir, sysUser)
	jsonSuccess(w, map[string]interface{}{
		"allow_xmlrpc":      state.AllowXMLRPC,
		"disable_wp_cron":   state.DisableWPCron,
		"disable_file_edit": state.DisableFileEdit,
	})
}

// POST /api/wordpress  body: { "action": "...", "site_id": N }
func (h *WordPressHandler) action(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Action string `json:"action"`
		SiteID int64  `json:"site_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	r = r.WithContext(r.Context()) // ensure clean context

	// inject site_id as query param so resolveSite can pick it up via helper below
	siteID, site, sysUser, webRoot, err := h.resolveSiteByID(req.SiteID)
	_ = siteID
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	if site["site_type"] != "wordpress" {
		jsonError(w, "not a wordpress site", http.StatusBadRequest)
		return
	}

	phpVersion, _ := site["php_version"].(string)
	phpBin := fmt.Sprintf("php%s", phpVersion)
	wpCLI := "/usr/local/bin/wp"

	state := loadWPToolsState(h.Cfg.DataDir, sysUser)

	switch req.Action {
	case "toggle-xmlrpc":
		state.AllowXMLRPC = !state.AllowXMLRPC
		if err := saveWPToolsState(h.Cfg.DataDir, sysUser, state); err != nil {
			jsonError(w, "failed to save state", http.StatusInternalServerError)
			return
		}
		// Regenerate nginx vhost so the xmlrpc rule is applied / removed
		h.regenVHost(site, sysUser, webRoot, state)
		jsonSuccess(w, map[string]interface{}{"allow_xmlrpc": state.AllowXMLRPC})

	case "toggle-wp-cron":
		state.DisableWPCron = !state.DisableWPCron
		if err := saveWPToolsState(h.Cfg.DataDir, sysUser, state); err != nil {
			jsonError(w, "failed to save state", http.StatusInternalServerError)
			return
		}
		wpConfigPath := filepath.Join(webRoot, "wp-config.php")
		content, readErr := readFileAsRoot(wpConfigPath)
		if readErr != nil {
			jsonError(w, "failed to read wp-config.php", http.StatusInternalServerError)
			return
		}
		snippet := "define( 'DISABLE_WP_CRON', true );"
		updated := updateWPConfigSnippet(content, snippet, "DISABLE_WP_CRON", state.DisableWPCron)
		if writeErr := writeFileAsRoot(wpConfigPath, sysUser, updated); writeErr != nil {
			jsonError(w, "failed to write wp-config.php", http.StatusInternalServerError)
			return
		}
		jsonSuccess(w, map[string]interface{}{"disable_wp_cron": state.DisableWPCron})

	case "toggle-disable-file-edit":
		state.DisableFileEdit = !state.DisableFileEdit
		if err := saveWPToolsState(h.Cfg.DataDir, sysUser, state); err != nil {
			jsonError(w, "failed to save state", http.StatusInternalServerError)
			return
		}
		wpConfigPath := filepath.Join(webRoot, "wp-config.php")
		content, readErr := readFileAsRoot(wpConfigPath)
		if readErr != nil {
			jsonError(w, "failed to read wp-config.php", http.StatusInternalServerError)
			return
		}
		snippet := "define( 'DISALLOW_FILE_EDIT', true );"
		updated := updateWPConfigSnippet(content, snippet, "DISALLOW_FILE_EDIT", state.DisableFileEdit)
		if writeErr := writeFileAsRoot(wpConfigPath, sysUser, updated); writeErr != nil {
			jsonError(w, "failed to write wp-config.php", http.StatusInternalServerError)
			return
		}
		jsonSuccess(w, map[string]interface{}{"disable_file_edit": state.DisableFileEdit})

	case "check-updates":
		out, runErr := exec.Command("sudo", "-u", sysUser,
			phpBin, wpCLI, "--path="+webRoot,
			"core", "check-update", "--format=json").Output()
		coreHasUpdate := runErr == nil && strings.TrimSpace(string(out)) != "[]" && strings.TrimSpace(string(out)) != ""

		pluginOut, _ := exec.Command("sudo", "-u", sysUser,
			phpBin, wpCLI, "--path="+webRoot,
			"plugin", "list", "--update=available", "--format=json").Output()
		themeOut, _ := exec.Command("sudo", "-u", sysUser,
			phpBin, wpCLI, "--path="+webRoot,
			"theme", "list", "--update=available", "--format=json").Output()

		var plugins, themes []interface{}
		json.Unmarshal(pluginOut, &plugins)
		json.Unmarshal(themeOut, &themes)

		jsonSuccess(w, map[string]interface{}{
			"core_update_available": coreHasUpdate,
			"plugins_with_updates":  len(plugins),
			"themes_with_updates":   len(themes),
		})

	case "core-update":
		out, runErr := exec.Command("sudo", "-u", sysUser,
			phpBin, wpCLI, "--path="+webRoot, "core", "update").CombinedOutput()
		if runErr != nil {
			log.Printf("WordPress core-update failed: %s", strings.TrimSpace(string(out)))
			jsonError(w, "WordPress core update failed", http.StatusInternalServerError)
			return
		}
		jsonSuccess(w, map[string]interface{}{"output": strings.TrimSpace(string(out))})

	case "plugin-update":
		out, runErr := exec.Command("sudo", "-u", sysUser,
			phpBin, wpCLI, "--path="+webRoot, "plugin", "update", "--all").CombinedOutput()
		if runErr != nil {
			log.Printf("WordPress plugin-update failed: %s", strings.TrimSpace(string(out)))
			jsonError(w, "WordPress plugin update failed", http.StatusInternalServerError)
			return
		}
		jsonSuccess(w, map[string]interface{}{"output": strings.TrimSpace(string(out))})

	case "theme-update":
		out, runErr := exec.Command("sudo", "-u", sysUser,
			phpBin, wpCLI, "--path="+webRoot, "theme", "update", "--all").CombinedOutput()
		if runErr != nil {
			log.Printf("WordPress theme-update failed: %s", strings.TrimSpace(string(out)))
			jsonError(w, "WordPress theme update failed", http.StatusInternalServerError)
			return
		}
		jsonSuccess(w, map[string]interface{}{"output": strings.TrimSpace(string(out))})

	default:
		jsonError(w, "unknown action", http.StatusBadRequest)
	}
}

// --- helpers ---

func (h *WordPressHandler) resolveSite(r *http.Request) (int64, map[string]interface{}, string, string, error) {
	idStr := r.URL.Query().Get("site_id")
	return h.resolveSiteByIDStr(idStr)
}

func (h *WordPressHandler) resolveSiteByID(id int64) (int64, map[string]interface{}, string, string, error) {
	return h.resolveSiteByIDStr(fmt.Sprintf("%d", id))
}

func (h *WordPressHandler) resolveSiteByIDStr(idStr string) (int64, map[string]interface{}, string, string, error) {
	if idStr == "" {
		return 0, nil, "", "", fmt.Errorf("site_id required")
	}
	var id int64
	if _, err := fmt.Sscanf(idStr, "%d", &id); err != nil || id <= 0 {
		return 0, nil, "", "", fmt.Errorf("invalid site_id")
	}
	site, err := h.DB.GetSite(id)
	if err != nil {
		return 0, nil, "", "", fmt.Errorf("site not found")
	}
	sysUser, _ := site["system_user"].(string)
	webRoot, _ := site["web_root"].(string)
	return id, site, sysUser, webRoot, nil
}

// regenVHost regenerates the nginx vhost with the current WPToolsState applied.
func (h *WordPressHandler) regenVHost(site map[string]interface{}, sysUser, webRoot string, state WPToolsState) {
	domain, _ := site["domain"].(string)
	aliases, _ := site["aliases"].(string)
	siteType, _ := site["site_type"].(string)
	phpVersion, _ := site["php_version"].(string)
	proxyURL, _ := site["proxy_url"].(string)
	sslType, _ := site["ssl_type"].(string)
	sslCertPath, _ := site["ssl_cert_path"].(string)
	sslKeyPath, _ := site["ssl_key_path"].(string)
	accessLog, _ := site["access_log"].(int)
	errorLog, _ := site["error_log"].(int)

	wpSecurity := ""
	if siteType == "wordpress" {
		wpSecurity = nginx.BuildWPSecurityRules(state.AllowXMLRPC)
	}

	vhostData := nginx.VHostData{
		Domain:                 domain,
		Aliases:                aliases,
		User:                   sysUser,
		SiteType:               siteType,
		PHPVersion:             phpVersion,
		ProxyURL:               proxyURL,
		WebRoot:                webRoot,
		SSLType:                sslType,
		SSLCertPath:            sslCertPath,
		SSLKeyPath:             sslKeyPath,
		AccessLog:              accessLog == 1,
		ErrorLog:               errorLog == 1,
		WordPressSecurityRules: wpSecurity,
	}

	// Use template-based update if template exists, else full regen
	if tpl, err := loadVHostTemplate(h.Cfg.DataDir, domain); err == nil {
		expanded := nginx.ExpandVHostTemplate(tpl, vhostData)
		nginx.WriteConfigString(h.Cfg.NginxSitesAvailable, h.Cfg.NginxSitesEnabled, domain, expanded)
	} else {
		nginx.WriteVHost(h.Cfg.NginxSitesAvailable, h.Cfg.NginxSitesEnabled, domain, vhostData)
	}
	nginx.TestAndReload()
}

// loadWPToolsState reads per-site state from DataDir/wp-tools/{sysUser}.json
func loadWPToolsState(dataDir, sysUser string) WPToolsState {
	var state WPToolsState
	path := wpToolsPath(dataDir, sysUser)
	data, err := os.ReadFile(path)
	if err != nil {
		return state
	}
	json.Unmarshal(data, &state)
	return state
}

// saveWPToolsState persists per-site state to DataDir/wp-tools/{sysUser}.json
func saveWPToolsState(dataDir, sysUser string, state WPToolsState) error {
	path := wpToolsPath(dataDir, sysUser)
	if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
		return err
	}
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0640)
}

func wpToolsPath(dataDir, sysUser string) string {
	return filepath.Join(dataDir, "wp-tools", sysUser+".json")
}

// updateWPConfigSnippet inserts or removes a define() inside JCWT markers in wp-config.php.
// When enable=true the snippet is present inside the markers; when false it is removed.
// Any bare occurrence of the snippet outside the markers is also cleaned up.
func updateWPConfigSnippet(content, snippet, defineKey string, enable bool) string {
	start := wpMarkerStart
	end := wpMarkerEnd

	// Remove any standalone occurrence outside markers first
	bareRemove := "\n" + snippet
	content = strings.ReplaceAll(content, bareRemove, "")
	bare2 := snippet + "\n"
	content = strings.ReplaceAll(content, bare2, "")

	si := strings.Index(content, start)
	ei := strings.Index(content, end)

	if si < 0 || ei < 0 || si > ei {
		// Markers absent — inject them before the closing "That's all" comment or before <?php end
		insertPoint := strings.Index(content, "/* That's all, stop editing!")
		if insertPoint < 0 {
			insertPoint = strings.Index(content, "/* That's all")
		}
		if insertPoint < 0 {
			// Append before last ?>  or at end
			insertPoint = strings.LastIndex(content, "?>")
			if insertPoint < 0 {
				insertPoint = len(content)
			}
		}
		if enable {
			block := "\n" + start + "\n" + snippet + "\n" + end + "\n"
			content = content[:insertPoint] + block + content[insertPoint:]
		}
		return content
	}

	// Markers exist — rebuild block interior
	before := content[:si+len(start)]
	after := content[ei:]
	inner := content[si+len(start) : ei]

	// Remove the specific define from the inner block
	inner = strings.ReplaceAll(inner, "\n"+snippet, "")
	inner = strings.ReplaceAll(inner, snippet+"\n", "")
	inner = strings.ReplaceAll(inner, snippet, "")

	if enable {
		inner = inner + "\n" + snippet
	}

	return before + inner + after
}

// readFileAsRoot reads a file using sudo cat (needed for root-owned wp-config.php)
func readFileAsRoot(path string) (string, error) {
	out, err := exec.Command("sudo", "cat", path).Output()
	if err != nil {
		return "", fmt.Errorf("read %s: %w", path, err)
	}
	return string(out), nil
}

// writeFileAsRoot writes content to a file as the site system user using sudo tee
func writeFileAsRoot(path, sysUser, content string) error {
	cmd := exec.Command("sudo", "tee", path)
	cmd.Stdin = strings.NewReader(content)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("write %s: %s", path, strings.TrimSpace(string(out)))
	}
	// Restore ownership
	exec.Command("sudo", "chown", sysUser+":"+sysUser, path).Run()
	exec.Command("sudo", "chmod", "640", path).Run()
	return nil
}
