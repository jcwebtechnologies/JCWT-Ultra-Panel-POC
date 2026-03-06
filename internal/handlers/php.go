package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/jcwt/ultra-panel/internal/config"
	"github.com/jcwt/ultra-panel/internal/db"
	"github.com/jcwt/ultra-panel/internal/php"
)

type PHPHandler struct {
	DB  *db.DB
	Cfg *config.Config
}

func (h *PHPHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	switch r.Method {
	case "GET":
		h.get(w, r)
	case "PUT":
		h.update(w, r)
	default:
		http.Error(w, `{"success":false,"error":"method not allowed"}`, http.StatusMethodNotAllowed)
	}
}

func (h *PHPHandler) get(w http.ResponseWriter, r *http.Request) {
	siteIDStr := r.URL.Query().Get("site_id")
	siteID, err := strconv.ParseInt(siteIDStr, 10, 64)
	if err != nil {
		jsonError(w, "invalid site_id", http.StatusBadRequest)
		return
	}

	settings, err := h.DB.GetPHPSettings(siteID)
	if err != nil {
		// Auto-create defaults if settings don't exist
		h.DB.UpsertPHPSettings(siteID, "256M", 30, 60, 1000, "64M", "64M", "")
		settings, err = h.DB.GetPHPSettings(siteID)
		if err != nil {
			jsonError(w, "failed to create default settings", http.StatusInternalServerError)
			return
		}
	}

	jsonSuccess(w, settings)
}

// Allowed PHP setting values
var allowedMemoryLimit = map[string]bool{"128M": true, "256M": true, "512M": true, "768M": true, "1024M": true, "2048M": true}
var allowedExecTime = map[int]bool{30: true, 60: true, 120: true, 300: true, 600: true, 900: true}
var allowedInputTime = map[int]bool{60: true, 120: true, 300: true, 600: true, 900: true}
var allowedInputVars = map[int]bool{1000: true, 2000: true, 3000: true, 5000: true, 10000: true}
var allowedSizeLimit = map[string]bool{"2M": true, "4M": true, "8M": true, "16M": true, "32M": true, "64M": true, "128M": true, "256M": true, "512M": true, "1024M": true}

func (h *PHPHandler) update(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SiteID            int64  `json:"site_id"`
		MemoryLimit       string `json:"memory_limit"`
		MaxExecutionTime  int    `json:"max_execution_time"`
		MaxInputTime      int    `json:"max_input_time"`
		MaxInputVars      int    `json:"max_input_vars"`
		PostMaxSize       string `json:"post_max_size"`
		UploadMaxFilesize string `json:"upload_max_filesize"`
		CustomDirectives  string `json:"custom_directives"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Validate values against allowlists
	if !allowedMemoryLimit[req.MemoryLimit] {
		jsonError(w, "Invalid memory_limit value — settings may have been tampered with", http.StatusBadRequest)
		return
	}
	if !allowedExecTime[req.MaxExecutionTime] {
		jsonError(w, "Invalid max_execution_time value — settings may have been tampered with", http.StatusBadRequest)
		return
	}
	if !allowedInputTime[req.MaxInputTime] {
		jsonError(w, "Invalid max_input_time value — settings may have been tampered with", http.StatusBadRequest)
		return
	}
	if !allowedInputVars[req.MaxInputVars] {
		jsonError(w, "Invalid max_input_vars value — settings may have been tampered with", http.StatusBadRequest)
		return
	}
	if !allowedSizeLimit[req.PostMaxSize] {
		jsonError(w, "Invalid post_max_size value — settings may have been tampered with", http.StatusBadRequest)
		return
	}
	if !allowedSizeLimit[req.UploadMaxFilesize] {
		jsonError(w, "Invalid upload_max_filesize value — settings may have been tampered with", http.StatusBadRequest)
		return
	}

	// Save to DB
	if err := h.DB.UpsertPHPSettings(req.SiteID, req.MemoryLimit, req.MaxExecutionTime,
		req.MaxInputTime, req.MaxInputVars, req.PostMaxSize, req.UploadMaxFilesize,
		req.CustomDirectives); err != nil {
		jsonError(w, "failed to save settings", http.StatusInternalServerError)
		return
	}

	// Get site info to regenerate pool config
	site, err := h.DB.GetSite(req.SiteID)
	if err != nil {
		jsonError(w, "site not found", http.StatusNotFound)
		return
	}

	sysUser := site["system_user"].(string)
	phpVersion := site["php_version"].(string)
	webRoot := site["web_root"].(string)

	poolData := php.PoolData{
		User:              sysUser,
		PHPVersion:        phpVersion,
		WebRoot:           webRoot,
		MemoryLimit:       req.MemoryLimit,
		MaxExecutionTime:  req.MaxExecutionTime,
		MaxInputTime:      req.MaxInputTime,
		MaxInputVars:      req.MaxInputVars,
		PostMaxSize:       req.PostMaxSize,
		UploadMaxFilesize: req.UploadMaxFilesize,
		CustomDirectives:  req.CustomDirectives,
	}

	if err := php.WritePool(h.Cfg.PHPFPMBaseDir, phpVersion, sysUser, poolData); err != nil {
		jsonError(w, "failed to write pool config", http.StatusInternalServerError)
		return
	}

	if err := php.RestartFPM(phpVersion); err != nil {
		jsonError(w, "pool config saved but FPM restart failed", http.StatusInternalServerError)
		return
	}

	jsonSuccess(w, map[string]interface{}{"updated": true})
}

// PHPVersionsHandler returns available PHP versions
type PHPVersionsHandler struct{}

func (h *PHPVersionsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	versions := php.AvailableVersions()
	jsonSuccess(w, versions)
}
