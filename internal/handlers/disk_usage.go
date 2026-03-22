package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/jcwt/ultra-panel/internal/config"
	"github.com/jcwt/ultra-panel/internal/db"
)

type DiskUsageHandler struct {
	DB  *db.DB
	Cfg *config.Config
}

func (h *DiskUsageHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	switch r.Method {
	case "GET":
		action := r.URL.Query().Get("action")
		if action == "site-tree" {
			h.siteTree(w, r)
		} else {
			h.allSites(w, r)
		}
	case "POST":
		action := r.URL.Query().Get("action")
		if action == "cleanup-tmp" {
			h.cleanupTmp(w, r)
		} else {
			jsonError(w, "invalid action", http.StatusBadRequest)
		}
	default:
		http.Error(w, `{"success":false,"error":"method not allowed"}`, http.StatusMethodNotAllowed)
	}
}

// DirEntry represents a directory in the tree
type DirEntry struct {
	Name     string      `json:"name"`
	Size     int64       `json:"size"`
	SizeStr  string      `json:"size_str"`
	Children []*DirEntry `json:"children,omitempty"`
}

// siteTree returns a 3-depth directory tree for a specific site
func (h *DiskUsageHandler) siteTree(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Query().Get("site_id")
	if idStr == "" {
		jsonError(w, "site_id required", http.StatusBadRequest)
		return
	}
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		jsonError(w, "invalid site_id", http.StatusBadRequest)
		return
	}
	site, err := h.DB.GetSite(id)
	if err != nil {
		jsonError(w, "site not found", http.StatusNotFound)
		return
	}

	sysUser, _ := site["system_user"].(string)
	homeDir := filepath.Join(h.Cfg.WebRootBase, sysUser)

	// Use du --max-depth=3 to get size of each directory up to 3 levels deep
	out, err := exec.Command("sudo", "du", "-b", "--max-depth=3", homeDir).CombinedOutput()
	if err != nil {
		// Fallback to simpler du
		out, err = exec.Command("sudo", "du", "-sb", homeDir).CombinedOutput()
		if err != nil {
			jsonSuccess(w, map[string]interface{}{"tree": nil, "total": "N/A"})
			return
		}
	}

	tree := parseDuOutput(string(out), homeDir)

	// Use du -sh for the human-readable total so it matches `du -sh` in the terminal
	// and the top strip (which also uses du -sh). The byte-count tree is for the
	// relative breakdown only; the total should reflect actual disk allocation.
	total := "N/A"
	if shOut, shErr := exec.Command("sudo", "du", "-sh", homeDir).CombinedOutput(); shErr == nil {
		if fields := strings.Fields(string(shOut)); len(fields) >= 1 {
			total = fields[0]
		}
	} else if tree != nil {
		total = tree.SizeStr // fallback to parsed bytes total
	}

	jsonSuccess(w, map[string]interface{}{"tree": tree, "total": total})
}

// allSites returns disk usage for all sites
func (h *DiskUsageHandler) allSites(w http.ResponseWriter, r *http.Request) {
	sites, err := h.DB.ListSites()
	if err != nil {
		jsonError(w, "failed to list sites", http.StatusInternalServerError)
		return
	}

	type SiteUsage struct {
		ID         int64  `json:"id"`
		Token      string `json:"token"`
		Domain     string `json:"domain"`
		SystemUser string `json:"system_user"`
		HomeDir    string `json:"home_dir"`
		Total      string `json:"total"`
		TotalBytes int64  `json:"total_bytes"`
		Htdocs     string `json:"htdocs"`
		Logs       string `json:"logs"`
		Tmp        string `json:"tmp"`
		Backups    string `json:"backups"`
	}

	var results []SiteUsage
	for _, s := range sites {
		id, _ := s["id"].(int64)
		token, _ := s["token"].(string)
		domain, _ := s["domain"].(string)
		sysUser, _ := s["system_user"].(string)
		homeDir := filepath.Join(h.Cfg.WebRootBase, sysUser)

		usage := SiteUsage{
			ID:         id,
			Token:      token,
			Domain:     domain,
			SystemUser: sysUser,
			HomeDir:    homeDir,
			Total:      "N/A",
			Htdocs:     "N/A",
			Logs:       "N/A",
			Tmp:        "N/A",
			Backups:    "N/A",
		}

		// Get total
		if out, err := exec.Command("sudo", "du", "-sb", homeDir).CombinedOutput(); err == nil {
			fields := strings.Fields(string(out))
			if len(fields) >= 1 {
				if bytes, err := strconv.ParseInt(fields[0], 10, 64); err == nil {
					usage.Total = formatBytes(bytes)
					usage.TotalBytes = bytes
				}
			}
		}

		// Get subdirectory sizes
		for _, sub := range []struct {
			name string
			ptr  *string
		}{
			{"htdocs", &usage.Htdocs},
			{"logs", &usage.Logs},
			{"tmp", &usage.Tmp},
			{"backups", &usage.Backups},
		} {
			subDir := filepath.Join(homeDir, sub.name)
			if out, err := exec.Command("sudo", "du", "-sb", subDir).CombinedOutput(); err == nil {
				fields := strings.Fields(string(out))
				if len(fields) >= 1 {
					if bytes, err := strconv.ParseInt(fields[0], 10, 64); err == nil {
						*sub.ptr = formatBytes(bytes)
					}
				}
			}
		}

		results = append(results, usage)
	}

	if results == nil {
		results = []SiteUsage{}
	}

	jsonSuccess(w, results)
}

// cleanupTmp removes tmp directory contents for a site
func (h *DiskUsageHandler) cleanupTmp(w http.ResponseWriter, r *http.Request) {
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

	sysUser, _ := site["system_user"].(string)
	tmpDir := filepath.Join(h.Cfg.WebRootBase, sysUser, "tmp")

	// Validate the tmp directory is inside WebRootBase before touching it
	absTmp, _ := filepath.Abs(tmpDir)
	absBase, _ := filepath.Abs(h.Cfg.WebRootBase)
	if !strings.HasPrefix(absTmp, absBase+string(filepath.Separator)) {
		jsonError(w, "invalid tmp path", http.StatusBadRequest)
		return
	}

	// Remove and recreate the tmp directory to clear its contents.
	// This uses only commands already allowed in sudoers (rm -rf, mkdir, chown, chmod).
	if out, err := exec.Command("sudo", "rm", "-rf", tmpDir).CombinedOutput(); err != nil {
		log.Printf("cleanup rm failed: %s", strings.TrimSpace(string(out)))
		jsonError(w, "cleanup failed", http.StatusInternalServerError)
		return
	}
	if out, err := exec.Command("sudo", "mkdir", "-p", tmpDir).CombinedOutput(); err != nil {
		log.Printf("cleanup mkdir failed: %s", strings.TrimSpace(string(out)))
		jsonError(w, "cleanup failed", http.StatusInternalServerError)
		return
	}
	exec.Command("sudo", "chown", "-R", sysUser+":"+sysUser, tmpDir).Run()
	exec.Command("sudo", "chmod", "750", tmpDir).Run()

	jsonSuccess(w, map[string]interface{}{"cleaned": true})
}

// parseDuOutput parses `du -b --max-depth=3` output into a tree
func parseDuOutput(output, rootPath string) *DirEntry {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) == 0 {
		return nil
	}

	// Parse all entries
	type entry struct {
		path  string
		bytes int64
	}
	var entries []entry
	for _, line := range lines {
		fields := strings.SplitN(strings.TrimSpace(line), "\t", 2)
		if len(fields) < 2 {
			continue
		}
		bytes, err := strconv.ParseInt(fields[0], 10, 64)
		if err != nil {
			continue
		}
		path := fields[1]
		entries = append(entries, entry{path: path, bytes: bytes})
	}

	if len(entries) == 0 {
		return nil
	}

	// Build a map of path -> size
	sizeMap := make(map[string]int64)
	for _, e := range entries {
		sizeMap[e.path] = e.bytes
	}

	// Root entry
	root := &DirEntry{
		Name:    filepath.Base(rootPath),
		Size:    sizeMap[rootPath],
		SizeStr: formatBytes(sizeMap[rootPath]),
	}

	// Build child entries (depth 1, 2, 3)
	childMap := make(map[string]*DirEntry)
	childMap[rootPath] = root

	// Sort entries by path for consistent ordering
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].path < entries[j].path
	})

	for _, e := range entries {
		if e.path == rootPath {
			continue
		}
		rel, err := filepath.Rel(rootPath, e.path)
		if err != nil {
			continue
		}
		depth := strings.Count(rel, string(filepath.Separator)) + 1
		if depth > 3 {
			continue
		}

		node := &DirEntry{
			Name:    filepath.Base(e.path),
			Size:    e.bytes,
			SizeStr: formatBytes(e.bytes),
		}
		childMap[e.path] = node

		// Find parent
		parentPath := filepath.Dir(e.path)
		if parent, ok := childMap[parentPath]; ok {
			parent.Children = append(parent.Children, node)
		}
	}

	return root
}

func formatBytes(b int64) string {
	if b == 0 {
		return "0 B"
	}
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	sizes := []string{"KB", "MB", "GB", "TB"}
	return fmt.Sprintf("%.1f %s", float64(b)/float64(div), sizes[exp])
}
