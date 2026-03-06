package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/jcwt/ultra-panel/internal/db"
)

type DashboardHandler struct {
	DB *db.DB
}

func (h *DashboardHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, `{"success":false,"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	stats := map[string]interface{}{
		"total_sites":     0,
		"total_databases": 0,
		"total_cron_jobs": 0,
		"disk_used_pct":   0.0,
		"memory_used_pct": 0.0,
		"uptime":          "",
		"php_versions":    []string{},
		"hostname":        "",
		"os_info":         "",
		"arch":            runtime.GOARCH,
	}

	// Count sites
	var siteCount int
	h.DB.Conn.QueryRow("SELECT COUNT(*) FROM sites").Scan(&siteCount)
	stats["total_sites"] = siteCount

	// Count databases
	var dbCount int
	h.DB.Conn.QueryRow("SELECT COUNT(*) FROM databases").Scan(&dbCount)
	stats["total_databases"] = dbCount

	// Count cron jobs
	var cronCount int
	h.DB.Conn.QueryRow("SELECT COUNT(*) FROM cron_jobs").Scan(&cronCount)
	stats["total_cron_jobs"] = cronCount

	// System info
	hostname, _ := os.Hostname()
	stats["hostname"] = hostname

	// Uptime
	if uptime, err := exec.Command("uptime", "-p").Output(); err == nil {
		stats["uptime"] = strings.TrimSpace(string(uptime))
	}

	// Memory
	if meminfo, err := os.ReadFile("/proc/meminfo"); err == nil {
		lines := strings.Split(string(meminfo), "\n")
		var total, available int64
		for _, line := range lines {
			if strings.HasPrefix(line, "MemTotal:") {
				fmt.Sscanf(line, "MemTotal: %d kB", &total)
			}
			if strings.HasPrefix(line, "MemAvailable:") {
				fmt.Sscanf(line, "MemAvailable: %d kB", &available)
			}
		}
		if total > 0 {
			stats["memory_used_pct"] = float64(total-available) / float64(total) * 100
			stats["memory_total_mb"] = total / 1024
			stats["memory_used_mb"] = (total - available) / 1024
		}
	}

	// Disk usage (size, used, percentage)
	if df, err := exec.Command("df", "--output=size,used,pcent", "/").Output(); err == nil {
		lines := strings.Split(strings.TrimSpace(string(df)), "\n")
		if len(lines) >= 2 {
			fields := strings.Fields(lines[1])
			if len(fields) >= 3 {
				totalKB, _ := strconv.ParseFloat(fields[0], 64)
				usedKB, _ := strconv.ParseFloat(fields[1], 64)
				pct := strings.TrimSuffix(strings.TrimSpace(fields[2]), "%")
				stats["disk_used_pct"] = pct
				stats["disk_total_gb"] = fmt.Sprintf("%.1f", totalKB/1024/1024)
				stats["disk_used_gb"] = fmt.Sprintf("%.1f", usedKB/1024/1024)
			}
		}
	}

	// PHP versions
	versions := []string{"8.2", "8.3", "8.4", "8.5"}
	var availableVersions []string
	for _, v := range versions {
		if _, err := os.Stat("/usr/bin/php" + v); err == nil {
			availableVersions = append(availableVersions, v)
		}
	}
	if len(availableVersions) == 0 {
		availableVersions = versions
	}
	stats["php_versions"] = availableVersions
	stats["server_time"] = time.Now().Format(time.RFC3339)

	// Public IP addresses (via external services, 2-second timeout)
	var ipv4s, ipv6s []string

	// Get public IPv4
	if out, err := exec.Command("curl", "-4s", "--max-time", "2", "https://icanhazip.com").Output(); err == nil {
		ip := strings.TrimSpace(string(out))
		if ip != "" {
			ipv4s = append(ipv4s, ip)
		}
	}

	// Get public IPv6
	if out, err := exec.Command("curl", "-6s", "--max-time", "2", "https://icanhazip.com").Output(); err == nil {
		ip := strings.TrimSpace(string(out))
		if ip != "" {
			ipv6s = append(ipv6s, ip)
		}
	}

	stats["ipv4_addresses"] = ipv4s
	stats["ipv6_addresses"] = ipv6s

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "data": stats})
}
