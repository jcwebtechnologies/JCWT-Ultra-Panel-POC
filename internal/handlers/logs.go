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
	sysUser := site["system_user"].(string)
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
		logPath = fmt.Sprintf("/home/%s/logs/%s-access.log", sysUser, domain)
	case "error":
		logPath = fmt.Sprintf("/home/%s/logs/%s-error.log", sysUser, domain)
	case "php-fpm":
		phpVersion := site["php_version"].(string)
		logPath = fmt.Sprintf("/var/log/php%s-fpm.log", phpVersion)
	default:
		jsonError(w, "invalid log type: use access, error, or php-fpm", http.StatusBadRequest)
		return
	}

	var cmd *exec.Cmd
	if logType == "php-fpm" {
		// For PHP-FPM logs, filter out NOTICE lines to show only errors/warnings
		// Read more lines then filter and limit to requested count
		shellCmd := fmt.Sprintf("sudo tail -n %d %s | grep -v '] NOTICE:' | tail -n %d; exit 0", lines*10, logPath, lines)
		cmd = exec.Command("bash", "-c", shellCmd)
	} else {
		cmd = exec.Command("sudo", "tail", "-n", strconv.Itoa(lines), logPath)
	}
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Check if the file exists to give a meaningful message
		existsOut, _ := exec.Command("sudo", "test", "-f", logPath).CombinedOutput()
		_ = existsOut
		existsErr := exec.Command("sudo", "test", "-f", logPath).Run()
		hint := "Log file does not exist yet. Traffic may not have been logged or logging may be disabled."
		if existsErr == nil {
			hint = "Log file exists but could not be read."
		}
		jsonSuccess(w, map[string]interface{}{
			"content":  "",
			"log_path": logPath,
			"lines":    0,
			"type":     logType,
			"hint":     hint,
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
