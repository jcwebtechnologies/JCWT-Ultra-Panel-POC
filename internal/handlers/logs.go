package handlers

import (
	"fmt"
	"net/http"
	"os/exec"
	"strconv"
	"strings"

	"github.com/jcwt/ultra-panel/internal/db"
)

type LogsHandler struct {
	DB *db.DB
}

func (h *LogsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != "GET" {
		http.Error(w, `{"success":false,"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

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
	logType := r.URL.Query().Get("type")
	if logType == "" {
		logType = "access"
	}

	linesStr := r.URL.Query().Get("lines")
	lines := 100
	if linesStr != "" {
		if n, err := strconv.Atoi(linesStr); err == nil && n > 0 && n <= 5000 {
			lines = n
		}
	}

	var logPath string
	switch logType {
	case "access":
		logPath = fmt.Sprintf("/var/log/nginx/%s-access.log", domain)
	case "error":
		logPath = fmt.Sprintf("/var/log/nginx/%s-error.log", domain)
	case "php-fpm":
		phpVersion := site["php_version"].(string)
		logPath = fmt.Sprintf("/var/log/php%s-fpm.log", phpVersion)
	default:
		jsonError(w, "invalid log type: use access, error, or php-fpm", http.StatusBadRequest)
		return
	}

	cmd := exec.Command("sudo", "tail", "-n", strconv.Itoa(lines), logPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Log file may not exist yet
		jsonSuccess(w, map[string]interface{}{
			"content":  "",
			"log_path": logPath,
			"lines":    0,
			"type":     logType,
		})
		return
	}

	content := strings.TrimSpace(string(output))
	lineCount := 0
	if content != "" {
		lineCount = strings.Count(content, "\n") + 1
	}

	jsonSuccess(w, map[string]interface{}{
		"content":  content,
		"log_path": logPath,
		"lines":    lineCount,
		"type":     logType,
	})
}
