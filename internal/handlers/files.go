package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/jcwt/ultra-panel/internal/config"
	"github.com/jcwt/ultra-panel/internal/db"
)

// FilesHandler manages per-site File Browser instances.
// File Browser (https://filebrowser.org) is a full-featured web file manager
// supporting file editing, uploads, downloads, zip/unzip, and more.
type FilesHandler struct {
	DB        *db.DB
	Cfg       *config.Config
	mu        sync.Mutex
	instances map[int64]*fbInstance
}

type fbInstance struct {
	Port       int
	Process    *exec.Cmd
	Started    time.Time
	LastAccess time.Time
}

func (h *FilesHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case "GET":
		h.getOrStart(w, r)
	case "DELETE":
		h.stop(w, r)
	default:
		http.Error(w, `{"success":false,"error":"method not allowed"}`, http.StatusMethodNotAllowed)
	}
}

// getOrStart returns the File Browser URL for a site, starting one if needed.
func (h *FilesHandler) getOrStart(w http.ResponseWriter, r *http.Request) {
	siteIDStr := r.URL.Query().Get("site_id")
	siteID, err := strconv.ParseInt(siteIDStr, 10, 64)
	if err != nil {
		jsonError(w, "invalid site_id", http.StatusBadRequest)
		return
	}

	site, err := h.DB.GetSite(siteID)
	if err != nil {
		jsonError(w, "site not found", http.StatusNotFound)
		return
	}

	// Check if instance already running
	port, running := h.GetInstance(siteID)
	if running {
		jsonSuccess(w, map[string]interface{}{
			"port": port,
			"url":  fmt.Sprintf("/fb/%d/", siteID),
		})
		return
	}

	// Start a new instance
	sysUser, _ := site["system_user"].(string)
	if sysUser == "" {
		jsonError(w, "site missing system user", http.StatusInternalServerError)
		return
	}

	// Use home directory as root so users can access logs, backups, etc.
	homeDir := filepath.Join(h.Cfg.WebRootBase, sysUser)
	newPort, startErr := h.startInstance(siteID, homeDir, sysUser)
	if startErr != nil {
		log.Printf("Failed to start File Browser for site %d: %v", siteID, startErr)
		jsonError(w, fmt.Sprintf("failed to start file browser: %v", startErr), http.StatusInternalServerError)
		return
	}

	jsonSuccess(w, map[string]interface{}{
		"port": newPort,
		"url":  fmt.Sprintf("/fb/%d/", siteID),
	})
}

// stop terminates a File Browser instance for a site.
func (h *FilesHandler) stop(w http.ResponseWriter, r *http.Request) {
	siteIDStr := r.URL.Query().Get("site_id")
	siteID, err := strconv.ParseInt(siteIDStr, 10, 64)
	if err != nil {
		jsonError(w, "invalid site_id", http.StatusBadRequest)
		return
	}

	h.mu.Lock()
	inst, exists := h.instances[siteID]
	if exists {
		gracefulStop(inst.Process)
		delete(h.instances, siteID)
	}
	h.mu.Unlock()

	jsonSuccess(w, map[string]interface{}{"stopped": true})
}

// GetInstance returns the running instance port for reverse proxy routing.
func (h *FilesHandler) GetInstance(siteID int64) (int, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	inst, ok := h.instances[siteID]
	if !ok {
		return 0, false
	}
	// Verify process is still alive
	if inst.Process.ProcessState != nil {
		// Process has exited — clean up
		delete(h.instances, siteID)
		return 0, false
	}
	// Also verify the port is actually responding
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", inst.Port), 300*time.Millisecond)
	if err != nil {
		// Port not responding — process may be hanging, kill and clean up
		gracefulStop(inst.Process)
		delete(h.instances, siteID)
		return 0, false
	}
	conn.Close()
	inst.LastAccess = time.Now()
	return inst.Port, true
}

// StopAll cleans up all running instances (called on shutdown).
func (h *FilesHandler) StopAll() {
	h.mu.Lock()
	defer h.mu.Unlock()
	for id, inst := range h.instances {
		gracefulStop(inst.Process)
		delete(h.instances, id)
	}
}

// gracefulStop sends SIGTERM and waits up to 5 seconds for the process to exit.
// Falls back to SIGKILL if the process doesn't terminate in time.
func gracefulStop(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}
	// Try graceful termination first
	cmd.Process.Signal(syscall.SIGTERM)
	done := make(chan struct{})
	go func() {
		cmd.Wait()
		close(done)
	}()
	select {
	case <-done:
		// Exited cleanly
	case <-time.After(5 * time.Second):
		// Force kill
		cmd.Process.Kill()
	}
}

// ProxyToFileBrowser creates an HTTP handler that reverse-proxies /fb/{siteId}/ to the instance.
// If no instance is running, it auto-starts one.
func (h *FilesHandler) ProxyHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Parse /fb/{siteId}/...
		path := r.URL.Path
		if len(path) < 5 {
			http.NotFound(w, r)
			return
		}

		// Extract site ID from path: /fb/123/...
		parts := splitPath(path)
		if len(parts) < 2 {
			http.NotFound(w, r)
			return
		}

		siteID, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			http.NotFound(w, r)
			return
		}

		port, ok := h.GetInstance(siteID)
		if !ok {
			// Auto-start: look up site and start File Browser
			site, err := h.DB.GetSite(siteID)
			if err != nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusNotFound)
				w.Write([]byte(`{"success":false,"error":"site not found"}`))
				return
			}

			sysUser, _ := site["system_user"].(string)
			if sysUser == "" {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(`{"success":false,"error":"site missing system user"}`))
				return
			}

			// Use home directory as root so users can access logs, backups, etc.
			homeDir := filepath.Join(h.Cfg.WebRootBase, sysUser)
			newPort, startErr := h.startInstance(siteID, homeDir, sysUser)
			if startErr != nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusServiceUnavailable)
				w.Write([]byte(fmt.Sprintf(`{"success":false,"error":"failed to start file browser: %s"}`, startErr.Error())))
				return
			}
			port = newPort
		}

		// Reverse proxy to the File Browser instance
		proxy := &reverseProxy{target: fmt.Sprintf("http://127.0.0.1:%d", port)}
		proxy.ServeHTTP(w, r)
	}
}

// startInstance starts a File Browser process for a site and returns the port.
func (h *FilesHandler) startInstance(siteID int64, webRoot, sysUser string) (int, error) {
	port, err := getFreePort()
	if err != nil {
		return 0, fmt.Errorf("allocate port: %w", err)
	}

	cmd := exec.Command("sudo", "-u", sysUser,
		"/usr/local/bin/filebrowser",
		"--noauth",
		"--root", webRoot,
		"--address", "127.0.0.1",
		"--port", strconv.Itoa(port),
		"--baseurl", fmt.Sprintf("/fb/%d", siteID),
		"--database", filepath.Join(filepath.Dir(webRoot), "tmp", fmt.Sprintf("filebrowser-%d.db", siteID)),
	)

	// Capture stderr for debugging
	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf

	if err := cmd.Start(); err != nil {
		return 0, fmt.Errorf("start process: %w", err)
	}

	// Wait for it to bind (up to 6 seconds)
	bound := false
	for i := 0; i < 12; i++ {
		time.Sleep(500 * time.Millisecond)

		// Check if process has already crashed
		if cmd.ProcessState != nil {
			stderrOut := stderrBuf.String()
			log.Printf("File Browser for site %d exited early. stderr: %s", siteID, stderrOut)
			return 0, fmt.Errorf("file browser exited immediately: %s", stderrOut)
		}

		conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 300*time.Millisecond)
		if err == nil {
			conn.Close()
			bound = true
			break
		}
	}

	if !bound {
		// Process started but never bound the port — kill it
		cmd.Process.Kill()
		stderrOut := stderrBuf.String()
		log.Printf("File Browser for site %d failed to bind port %d. stderr: %s", siteID, port, stderrOut)
		return 0, fmt.Errorf("file browser failed to start (port %d did not bind): %s", port, stderrOut)
	}

	h.mu.Lock()
	if h.instances == nil {
		h.instances = make(map[int64]*fbInstance)
	}
	h.instances[siteID] = &fbInstance{
		Port:       port,
		Process:    cmd,
		Started:    time.Now(),
		LastAccess: time.Now(),
	}
	h.mu.Unlock()

	// Cleanup when process exits
	go func() {
		cmd.Wait()
		h.mu.Lock()
		delete(h.instances, siteID)
		h.mu.Unlock()
		log.Printf("File Browser for site %d exited. stderr: %s", siteID, stderrBuf.String())
	}()

	log.Printf("File Browser started for site %d on port %d (user: %s, root: %s)", siteID, port, sysUser, webRoot)
	return port, nil
}

// StartIdleReaper launches a background goroutine that stops File Browser instances
// idle for more than 15 minutes. This prevents leaked processes when users navigate away.
func (h *FilesHandler) StartIdleReaper() {
	go func() {
		ticker := time.NewTicker(2 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			h.mu.Lock()
			now := time.Now()
			for id, inst := range h.instances {
				if now.Sub(inst.LastAccess) > 15*time.Minute {
					gracefulStop(inst.Process)
					delete(h.instances, id)
					log.Printf("Reaped idle File Browser instance for site %d (idle %s)", id, now.Sub(inst.LastAccess).Round(time.Minute))
				}
			}
			h.mu.Unlock()
		}
	}()
}

func splitPath(path string) []string {
	var parts []string
	for _, p := range split(path, '/') {
		if p != "" {
			parts = append(parts, p)
		}
	}
	return parts
}

func split(s string, sep byte) []string {
	var parts []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == sep {
			parts = append(parts, s[start:i])
			start = i + 1
		}
	}
	parts = append(parts, s[start:])
	return parts
}

func getFreePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()
	return port, nil
}

// reverseProxy is a minimal HTTP reverse proxy
type reverseProxy struct {
	target string
}

func (p *reverseProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	client := &http.Client{Timeout: 30 * time.Second}

	targetURL := p.target + r.URL.RequestURI()

	var resp *http.Response
	var lastErr error

	// Retry up to 5 times (file browser may still be starting)
	for attempt := 0; attempt < 5; attempt++ {
		proxyReq, err := http.NewRequest(r.Method, targetURL, r.Body)
		if err != nil {
			http.Error(w, "proxy error", http.StatusBadGateway)
			return
		}

		// Copy headers
		for key, values := range r.Header {
			for _, val := range values {
				proxyReq.Header.Add(key, val)
			}
		}

		resp, lastErr = client.Do(proxyReq)
		if lastErr == nil {
			break
		}
		// Wait before retry
		time.Sleep(1500 * time.Millisecond)
	}

	if lastErr != nil {
		http.Error(w, "file browser unavailable — please try refreshing", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	// Copy response headers
	for key, values := range resp.Header {
		for _, val := range values {
			w.Header().Add(key, val)
		}
	}
	w.WriteHeader(resp.StatusCode)

	// Stream response body
	buf := make([]byte, 32*1024)
	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			w.Write(buf[:n])
		}
		if err != nil {
			break
		}
	}
}

// MarshalJSON implements json.Marshaler for consistent API responses
func jsonMarshal(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}
