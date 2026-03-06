package handlers

import (
	"encoding/json"
	"net/http"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/jcwt/ultra-panel/internal/db"
)

// Allowed services whitelist — prevents arbitrary service control
var allowedServices = map[string]string{
	"nginx":       "nginx",
	"mariadb":     "mariadb",
	"redis":       "redis-server",
	"php8.2":      "php8.2-fpm",
	"php8.3":      "php8.3-fpm",
	"php8.4":      "php8.4-fpm",
	"php8.5":      "php8.5-fpm",
	"jcwt-panel":  "jcwt-panel",
}

type ServicesHandler struct {
	DB          *db.DB
	restartMu   sync.Mutex
	restartLog  map[string][]time.Time
}

func (h *ServicesHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	switch r.Method {
	case "GET":
		h.list(w, r)
	case "POST":
		h.restart(w, r)
	default:
		http.Error(w, `{"success":false,"error":"method not allowed"}`, http.StatusMethodNotAllowed)
	}
}

// list returns the status of all services
func (h *ServicesHandler) list(w http.ResponseWriter, r *http.Request) {
	var services []map[string]interface{}

	for displayName, unitName := range allowedServices {
		status := getServiceStatus(unitName)
		services = append(services, map[string]interface{}{
			"name":    displayName,
			"unit":    unitName,
			"status":  status["status"],
			"active":  status["active"],
			"memory":  status["memory"],
			"uptime":  status["uptime"],
		})
	}

	jsonSuccess(w, services)
}

// restart restarts a whitelisted service with rate limiting
func (h *ServicesHandler) restart(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Service string `json:"service"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	unitName, ok := allowedServices[req.Service]
	if !ok {
		jsonError(w, "unknown service", http.StatusBadRequest)
		return
	}

	// Don't allow restarting the panel itself via API (that would kill the server)
	if req.Service == "jcwt-panel" {
		jsonError(w, "cannot restart panel via API — use systemctl directly", http.StatusForbidden)
		return
	}

	// Rate limit: max 3 restarts per service per 5 minutes
	h.restartMu.Lock()
	if h.restartLog == nil {
		h.restartLog = make(map[string][]time.Time)
	}
	now := time.Now()
	cutoff := now.Add(-5 * time.Minute)
	var recent []time.Time
	for _, t := range h.restartLog[req.Service] {
		if t.After(cutoff) {
			recent = append(recent, t)
		}
	}
	if len(recent) >= 3 {
		h.restartMu.Unlock()
		jsonError(w, "too many restart attempts — try again in a few minutes", http.StatusTooManyRequests)
		return
	}
	h.restartLog[req.Service] = append(recent, now)
	h.restartMu.Unlock()

	// Restart the service
	output, err := exec.Command("sudo", "systemctl", "restart", unitName).CombinedOutput()
	if err != nil {
		jsonError(w, "restart failed: "+strings.TrimSpace(string(output)), http.StatusInternalServerError)
		return
	}

	// Wait briefly and get new status
	time.Sleep(500 * time.Millisecond)
	status := getServiceStatus(unitName)

	jsonSuccess(w, map[string]interface{}{
		"restarted": true,
		"service":   req.Service,
		"status":    status["status"],
		"active":    status["active"],
	})
}

func getServiceStatus(unitName string) map[string]string {
	result := map[string]string{
		"status": "unknown",
		"active": "unknown",
		"memory": "",
		"uptime": "",
	}

	// Get active state
	out, err := exec.Command("systemctl", "is-active", unitName).Output()
	if err == nil {
		result["active"] = strings.TrimSpace(string(out))
		result["status"] = result["active"]
	} else {
		result["active"] = "inactive"
		result["status"] = "stopped"
	}

	// Get memory usage
	out, _ = exec.Command("systemctl", "show", unitName, "--property=MemoryCurrent", "--value").Output()
	mem := strings.TrimSpace(string(out))
	if mem != "" && mem != "[not set]" {
		result["memory"] = mem
	}

	// Get uptime (ActiveEnterTimestamp)
	out, _ = exec.Command("systemctl", "show", unitName, "--property=ActiveEnterTimestamp", "--value").Output()
	ts := strings.TrimSpace(string(out))
	if ts != "" {
		result["uptime"] = ts
	}

	return result
}
