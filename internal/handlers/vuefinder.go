package handlers

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/jcwt/ultra-panel/internal/config"
	"github.com/jcwt/ultra-panel/internal/db"
)

// VueFinderHandler implements the native REST API driver protocol for VueFinder.
// It handles file operations (index, search, mkdir, mkfile, rename, delete, copy, move,
// upload, download, preview, save, archive, unarchive) securely bounded within site home directories.
type VueFinderHandler struct {
	DB  *db.DB
	Cfg *config.Config
}

// VueFile represents a file/directory entry in VueFinder format.
type VueFile struct {
	Name       string `json:"name"`
	Path       string `json:"path"`
	Size       int64  `json:"size"`
	Type       string `json:"type"` // "file" or "dir"
	Modified   int64  `json:"modified"`
	Mime       string `json:"mime"`
	Visibility string `json:"visibility"`
	Extension  string `json:"extension"`
}

type vueIndexResponse struct {
	Adapter  string    `json:"adapter"`
	Path     string    `json:"path"`
	Dirname  string    `json:"dirname"`
	Files    []VueFile `json:"files"`
	Storages []string  `json:"storages"`
}

func (h *VueFinderHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// site_id: ?site_id= query param, form value, or X-Site-Id header (4.x clean baseURL mode)
	siteIDStr := r.URL.Query().Get("site_id")
	if siteIDStr == "" {
		siteIDStr = r.FormValue("site_id")
	}
	if siteIDStr == "" {
		siteIDStr = r.Header.Get("X-Site-Id")
	}
	siteID, err := strconv.ParseInt(siteIDStr, 10, 64)
	if err != nil || siteID <= 0 {
		jsonError(w, "invalid site_id", http.StatusBadRequest)
		return
	}

	site, err := h.DB.GetSite(siteID)
	if err != nil {
		jsonError(w, "site not found", http.StatusNotFound)
		return
	}

	sysUser, _ := site["system_user"].(string)
	if sysUser == "" {
		jsonError(w, "site missing system user", http.StatusInternalServerError)
		return
	}

	homeDir := filepath.Join(h.Cfg.WebRootBase, sysUser)

	// Action resolution:
	//   VueFinder 4.x: POST /api/vuefinder/{action}  (path suffix after /api/vuefinder)
	//   VueFinder 2.x: GET/POST /api/vuefinder?q={action}
	action := ""
	if suffix := strings.TrimPrefix(r.URL.Path, "/api/vuefinder"); suffix != "" {
		action = strings.Trim(suffix, "/")
	}
	if action == "" {
		action = r.URL.Query().Get("q")
	}
	if action == "" {
		action = r.FormValue("q")
	}
	if action == "" && r.Method == "GET" {
		action = "index"
	}

	// Map VueFinder 4.x hyphenated action names to handler names
	switch action {
	case "create-file":
		action = "mkfile"
	case "create-folder":
		action = "mkdir"
	case "extract":
		action = "unarchive"
	}

	switch action {
	case "index":
		h.handleIndex(w, r, sysUser, homeDir)
	case "search":
		h.handleSearch(w, r, sysUser, homeDir)
	case "mkdir":
		h.handleMkdir(w, r, sysUser, homeDir)
	case "mkfile":
		h.handleMkfile(w, r, sysUser, homeDir)
	case "rename":
		h.handleRename(w, r, sysUser, homeDir)
	case "delete":
		h.handleDelete(w, r, sysUser, homeDir)
	case "copy":
		h.handleCopy(w, r, sysUser, homeDir)
	case "move":
		h.handleMove(w, r, sysUser, homeDir)
	case "upload":
		h.handleUpload(w, r, sysUser, homeDir)
	case "download":
		h.handleDownload(w, r, sysUser, homeDir)
	case "preview":
		h.handlePreview(w, r, sysUser, homeDir)
	case "save":
		h.handleSave(w, r, sysUser, homeDir)
	case "archive":
		h.handleArchive(w, r, sysUser, homeDir)
	case "unarchive":
		h.handleUnarchive(w, r, sysUser, homeDir)
	default:
		jsonError(w, fmt.Sprintf("unsupported action: %s", action), http.StatusBadRequest)
	}
}

// Security: resolves target path and ensures it stays strictly within the user's home directory
func resolveUserPath(homeDir, rawVuePath string) (string, string, error) {
	rel := parseVuePath(rawVuePath)
	full := filepath.Join(homeDir, rel)

	absHome, err := filepath.Abs(homeDir)
	if err != nil {
		return "", "", fmt.Errorf("invalid home dir")
	}

	absTarget, err := filepath.Abs(full)
	if err != nil {
		return "", "", fmt.Errorf("invalid target path")
	}

	if absTarget != absHome && !strings.HasPrefix(absTarget, absHome+string(os.PathSeparator)) {
		return "", "", fmt.Errorf("path traversal forbidden")
	}

	relResult, _ := filepath.Rel(absHome, absTarget)
	if relResult == "." {
		relResult = ""
	}
	return absTarget, relResult, nil
}

func parseVuePath(raw string) string {
	if idx := strings.Index(raw, "://"); idx != -1 {
		raw = raw[idx+3:]
	}
	raw = strings.TrimPrefix(raw, "/")
	return filepath.Clean(raw)
}

func toVuePath(relPath string) string {
	relPath = filepath.ToSlash(relPath)
	relPath = strings.TrimPrefix(relPath, "/")
	if relPath == "." || relPath == "" {
		return "storage://"
	}
	return "storage://" + relPath
}

func getVueDirname(vuePath string) string {
	if vuePath == "storage://" || vuePath == "storage:" || vuePath == "" {
		return "storage://"
	}
	p := strings.TrimPrefix(vuePath, "storage://")
	p = strings.TrimPrefix(p, "/")
	dir := filepath.Dir(p)
	if dir == "." || dir == "" {
		return "storage://"
	}
	return "storage://" + filepath.ToSlash(dir)
}

// Action: index (list directory contents)
func (h *VueFinderHandler) handleIndex(w http.ResponseWriter, r *http.Request, sysUser, homeDir string) {
	rawPath := r.URL.Query().Get("path")
	if rawPath == "" {
		rawPath = "storage://"
	}

	targetDir, relDir, err := resolveUserPath(homeDir, rawPath)
	if err != nil {
		jsonError(w, err.Error(), http.StatusForbidden)
		return
	}

	entries, err := os.ReadDir(targetDir)
	if err != nil {
		// Directory does not exist or not readable — return empty listing
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(vueIndexResponse{
			Adapter:  "local",
			Path:     toVuePath(relDir),
			Dirname:  getVueDirname(toVuePath(relDir)),
			Files:    []VueFile{},
			Storages: []string{"local"},
		})
		return
	}

	vueFiles := make([]VueFile, 0, len(entries))
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}

		name := entry.Name()
		if name == ".panel" {
			continue
		}

		relChild := filepath.Join(relDir, name)
		vueChildPath := toVuePath(relChild)

		isDir := entry.IsDir()
		fileType := "file"
		mimeType := mime.TypeByExtension(filepath.Ext(name))
		if mimeType == "" {
			mimeType = "application/octet-stream"
		}
		if isDir {
			fileType = "dir"
			mimeType = "directory"
		}

		ext := strings.TrimPrefix(filepath.Ext(name), ".")

		vueFiles = append(vueFiles, VueFile{
			Name:       name,
			Path:       vueChildPath,
			Size:       info.Size(),
			Type:       fileType,
			Modified:   info.ModTime().Unix(),
			Mime:       mimeType,
			Visibility: "public",
			Extension:  ext,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(vueIndexResponse{
		Adapter:  "local",
		Path:     toVuePath(relDir),
		Dirname:  getVueDirname(toVuePath(relDir)),
		Files:    vueFiles,
		Storages: []string{"local"},
	})
}

// Action: search
func (h *VueFinderHandler) handleSearch(w http.ResponseWriter, r *http.Request, sysUser, homeDir string) {
	rawPath := r.URL.Query().Get("path")
	filter := strings.ToLower(r.URL.Query().Get("filter"))

	targetDir, relDir, err := resolveUserPath(homeDir, rawPath)
	if err != nil {
		jsonError(w, err.Error(), http.StatusForbidden)
		return
	}

	vueFiles := []VueFile{}
	filepath.WalkDir(targetDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || path == targetDir {
			return nil
		}
		name := d.Name()
		if name == ".panel" {
			return filepath.SkipDir
		}
		if filter != "" && !strings.Contains(strings.ToLower(name), filter) {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return nil
		}

		relPath, _ := filepath.Rel(homeDir, path)
		isDir := d.IsDir()
		fileType := "file"
		mimeType := mime.TypeByExtension(filepath.Ext(name))
		if isDir {
			fileType = "dir"
			mimeType = "directory"
		}

		vueFiles = append(vueFiles, VueFile{
			Name:       name,
			Path:       toVuePath(relPath),
			Size:       info.Size(),
			Type:       fileType,
			Modified:   info.ModTime().Unix(),
			Mime:       mimeType,
			Visibility: "public",
			Extension:  strings.TrimPrefix(filepath.Ext(name), "."),
		})

		// Limit max search results to prevent memory spikes
		if len(vueFiles) >= 200 {
			return fmt.Errorf("limit reached")
		}
		return nil
	})

	_ = relDir
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"adapter": "local",
		"files":   vueFiles,
	})
}

// Action: mkdir
func (h *VueFinderHandler) handleMkdir(w http.ResponseWriter, r *http.Request, sysUser, homeDir string) {
	var body struct {
		Path string `json:"path"`
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonError(w, "invalid json body", http.StatusBadRequest)
		return
	}

	cleanName := filepath.Base(body.Name)
	if cleanName == "." || cleanName == ".." || cleanName == "" {
		jsonError(w, "invalid directory name", http.StatusBadRequest)
		return
	}

	targetParent, _, err := resolveUserPath(homeDir, body.Path)
	if err != nil {
		jsonError(w, err.Error(), http.StatusForbidden)
		return
	}

	// Relative path from home dir (panel-fsctl requires relative path under /home/USER/)
	newDirPath := filepath.Join(targetParent, cleanName)
	relNewDir, _ := filepath.Rel(homeDir, newDirPath)
	out, err := exec.Command("sudo", "/usr/local/sbin/panel-fsctl", "vf-mkdir", sysUser, relNewDir).CombinedOutput()
	if err != nil {
		log.Printf("mkdir failed for %s: %s", newDirPath, strings.TrimSpace(string(out)))
		jsonError(w, "failed to create directory", http.StatusInternalServerError)
		return
	}

	jsonSuccess(w, map[string]interface{}{"status": true, "message": "directory created"})
}

// Action: mkfile
func (h *VueFinderHandler) handleMkfile(w http.ResponseWriter, r *http.Request, sysUser, homeDir string) {
	var body struct {
		Path string `json:"path"`
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonError(w, "invalid json body", http.StatusBadRequest)
		return
	}

	cleanName := filepath.Base(body.Name)
	if cleanName == "." || cleanName == ".." || cleanName == "" {
		jsonError(w, "invalid file name", http.StatusBadRequest)
		return
	}

	targetParent, _, err := resolveUserPath(homeDir, body.Path)
	if err != nil {
		jsonError(w, err.Error(), http.StatusForbidden)
		return
	}

	newFilePath := filepath.Join(targetParent, cleanName)
	relNewFile, _ := filepath.Rel(homeDir, newFilePath)
	out, err := exec.Command("sudo", "/usr/local/sbin/panel-fsctl", "vf-mkfile", sysUser, relNewFile).CombinedOutput()
	if err != nil {
		log.Printf("mkfile failed for %s: %s", newFilePath, strings.TrimSpace(string(out)))
		jsonError(w, "failed to create file", http.StatusInternalServerError)
		return
	}

	jsonSuccess(w, map[string]interface{}{"status": true, "message": "file created"})
}

// Action: rename
func (h *VueFinderHandler) handleRename(w http.ResponseWriter, r *http.Request, sysUser, homeDir string) {
	var body struct {
		Item string `json:"item"`
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonError(w, "invalid json body", http.StatusBadRequest)
		return
	}

	cleanName := filepath.Base(body.Name)
	if cleanName == "." || cleanName == ".." || cleanName == "" {
		jsonError(w, "invalid target name", http.StatusBadRequest)
		return
	}

	oldPath, _, err := resolveUserPath(homeDir, body.Item)
	if err != nil {
		jsonError(w, err.Error(), http.StatusForbidden)
		return
	}

	newPath := filepath.Join(filepath.Dir(oldPath), cleanName)
	relOld, _ := filepath.Rel(homeDir, oldPath)
	relNew, _ := filepath.Rel(homeDir, newPath)
	out, err := exec.Command("sudo", "/usr/local/sbin/panel-fsctl", "vf-rename", sysUser, relOld, relNew).CombinedOutput()
	if err != nil {
		log.Printf("rename failed from %s to %s: %s", oldPath, newPath, strings.TrimSpace(string(out)))
		jsonError(w, "failed to rename item", http.StatusInternalServerError)
		return
	}

	jsonSuccess(w, map[string]interface{}{"status": true})
}

// Action: delete
func (h *VueFinderHandler) handleDelete(w http.ResponseWriter, r *http.Request, sysUser, homeDir string) {
	var body struct {
		Items []string `json:"items"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonError(w, "invalid json body", http.StatusBadRequest)
		return
	}

	absHome, _ := filepath.Abs(homeDir)

	for _, item := range body.Items {
		targetPath, _, err := resolveUserPath(homeDir, item)
		if err != nil {
			jsonError(w, err.Error(), http.StatusForbidden)
			return
		}

		// Never delete home dir itself
		if targetPath == absHome {
			jsonError(w, "cannot delete home directory", http.StatusForbidden)
			return
		}

		relTarget, _ := filepath.Rel(homeDir, targetPath)
		out, err := exec.Command("sudo", "/usr/local/sbin/panel-fsctl", "vf-delete", sysUser, relTarget).CombinedOutput()
		if err != nil {
			log.Printf("delete failed for %s: %s", targetPath, strings.TrimSpace(string(out)))
			jsonError(w, "failed to delete item", http.StatusInternalServerError)
			return
		}
	}

	jsonSuccess(w, map[string]interface{}{"status": true})
}

// Action: copy
func (h *VueFinderHandler) handleCopy(w http.ResponseWriter, r *http.Request, sysUser, homeDir string) {
	var body struct {
		Items       []string `json:"items"`
		Destination string   `json:"destination"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonError(w, "invalid json body", http.StatusBadRequest)
		return
	}

	destDir, _, err := resolveUserPath(homeDir, body.Destination)
	if err != nil {
		jsonError(w, err.Error(), http.StatusForbidden)
		return
	}

	for _, item := range body.Items {
		srcPath, _, err := resolveUserPath(homeDir, item)
		if err != nil {
			jsonError(w, err.Error(), http.StatusForbidden)
			return
		}

		relSrc, _ := filepath.Rel(homeDir, srcPath)
		// dest for copy is the directory; panel-fsctl vf-copy takes src and dest
		destFile := filepath.Join(destDir, filepath.Base(srcPath))
		relDest, _ := filepath.Rel(homeDir, destFile)
		out, err := exec.Command("sudo", "/usr/local/sbin/panel-fsctl", "vf-copy", sysUser, relSrc, relDest).CombinedOutput()
		if err != nil {
			log.Printf("copy failed from %s to %s: %s", srcPath, destDir, strings.TrimSpace(string(out)))
			jsonError(w, "failed to copy item", http.StatusInternalServerError)
			return
		}
	}

	jsonSuccess(w, map[string]interface{}{"status": true})
}

// Action: move
func (h *VueFinderHandler) handleMove(w http.ResponseWriter, r *http.Request, sysUser, homeDir string) {
	var body struct {
		Items       []string `json:"items"`
		Destination string   `json:"destination"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonError(w, "invalid json body", http.StatusBadRequest)
		return
	}

	destDir, _, err := resolveUserPath(homeDir, body.Destination)
	if err != nil {
		jsonError(w, err.Error(), http.StatusForbidden)
		return
	}

	for _, item := range body.Items {
		srcPath, _, err := resolveUserPath(homeDir, item)
		if err != nil {
			jsonError(w, err.Error(), http.StatusForbidden)
			return
		}

		relSrc, _ := filepath.Rel(homeDir, srcPath)
		destFile := filepath.Join(destDir, filepath.Base(srcPath))
		relDest, _ := filepath.Rel(homeDir, destFile)
		out, err := exec.Command("sudo", "/usr/local/sbin/panel-fsctl", "vf-move", sysUser, relSrc, relDest).CombinedOutput()
		if err != nil {
			log.Printf("move failed from %s to %s: %s", srcPath, destDir, strings.TrimSpace(string(out)))
			jsonError(w, "failed to move item", http.StatusInternalServerError)
			return
		}
	}

	jsonSuccess(w, map[string]interface{}{"status": true})
}

// Action: upload
func (h *VueFinderHandler) handleUpload(w http.ResponseWriter, r *http.Request, sysUser, homeDir string) {
	mr, err := r.MultipartReader()
	if err != nil {
		jsonError(w, "invalid multipart form", http.StatusBadRequest)
		return
	}

	var targetPath string
	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			jsonError(w, "error reading multipart form", http.StatusBadRequest)
			return
		}

		formName := part.FormName()
		if formName == "path" {
			pathBuf, _ := io.ReadAll(part)
			targetPath, _, err = resolveUserPath(homeDir, string(pathBuf))
			if err != nil {
				jsonError(w, err.Error(), http.StatusForbidden)
				return
			}
			continue
		}

		if part.FileName() != "" {
			if targetPath == "" {
				targetPath = homeDir
			}
			cleanFileName := filepath.Base(part.FileName())
			destFilePath := filepath.Join(targetPath, cleanFileName)

			// Stream upload via panel-fsctl vf-write (reads from stdin)
			relDest, _ := filepath.Rel(homeDir, destFilePath)
			cmd := exec.Command("sudo", "/usr/local/sbin/panel-fsctl", "vf-write", sysUser, relDest)
			cmd.Stdin = part
			if out, err := cmd.CombinedOutput(); err != nil {
				log.Printf("upload failed for %s: %s", destFilePath, strings.TrimSpace(string(out)))
				jsonError(w, "failed to save uploaded file", http.StatusInternalServerError)
				return
			}
		}
	}

	jsonSuccess(w, map[string]interface{}{"status": true, "message": "files uploaded successfully"})
}

// Action: download
func (h *VueFinderHandler) handleDownload(w http.ResponseWriter, r *http.Request, sysUser, homeDir string) {
	rawPath := r.URL.Query().Get("path")
	targetPath, _, err := resolveUserPath(homeDir, rawPath)
	if err != nil {
		jsonError(w, err.Error(), http.StatusForbidden)
		return
	}

	info, err := os.Stat(targetPath)
	if err != nil {
		jsonError(w, "file not found", http.StatusNotFound)
		return
	}

	if info.IsDir() {
		// Compress directory on the fly for download
		w.Header().Set("Content-Type", "application/zip")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s.zip\"", filepath.Base(targetPath)))
		cmd := exec.Command("sudo", "/usr/bin/zip", "-r", "-", filepath.Base(targetPath))
		cmd.Dir = filepath.Dir(targetPath)
		cmd.Stdout = w
		cmd.Run()
		return
	}

	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filepath.Base(targetPath)))
	w.Header().Set("Content-Length", strconv.FormatInt(info.Size(), 10))

	// Read file via panel-fsctl vf-read (validates path stays under home dir)
	relPath, _ := filepath.Rel(homeDir, targetPath)
	cmd := exec.Command("sudo", "/usr/local/sbin/panel-fsctl", "vf-read", sysUser, relPath)
	cmd.Stdout = w
	cmd.Run()
}

// Action: preview (read file for image or text preview)
func (h *VueFinderHandler) handlePreview(w http.ResponseWriter, r *http.Request, sysUser, homeDir string) {
	rawPath := r.URL.Query().Get("path")
	targetPath, _, err := resolveUserPath(homeDir, rawPath)
	if err != nil {
		jsonError(w, err.Error(), http.StatusForbidden)
		return
	}

	mimeType := mime.TypeByExtension(filepath.Ext(targetPath))
	if mimeType == "" {
		mimeType = "text/plain; charset=utf-8"
	}
	w.Header().Set("Content-Type", mimeType)

	relPath, _ := filepath.Rel(homeDir, targetPath)
	cmd := exec.Command("sudo", "/usr/local/sbin/panel-fsctl", "vf-read", sysUser, relPath)
	cmd.Stdout = w
	cmd.Run()
}

// Action: save (save edited file content)
func (h *VueFinderHandler) handleSave(w http.ResponseWriter, r *http.Request, sysUser, homeDir string) {
	var body struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonError(w, "invalid json body", http.StatusBadRequest)
		return
	}

	targetPath, _, err := resolveUserPath(homeDir, body.Path)
	if err != nil {
		jsonError(w, err.Error(), http.StatusForbidden)
		return
	}

	relPath, _ := filepath.Rel(homeDir, targetPath)
	cmd := exec.Command("sudo", "/usr/local/sbin/panel-fsctl", "vf-write", sysUser, relPath)
	cmd.Stdin = strings.NewReader(body.Content)
	if out, err := cmd.CombinedOutput(); err != nil {
		log.Printf("save file failed for %s: %s", targetPath, strings.TrimSpace(string(out)))
		jsonError(w, "failed to save file content", http.StatusInternalServerError)
		return
	}

	jsonSuccess(w, map[string]interface{}{"status": true, "message": "file saved"})
}

// Action: archive (create zip/tar archive)
func (h *VueFinderHandler) handleArchive(w http.ResponseWriter, r *http.Request, sysUser, homeDir string) {
	var body struct {
		Items []string `json:"items"`
		Name  string   `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonError(w, "invalid json body", http.StatusBadRequest)
		return
	}

	if len(body.Items) == 0 {
		jsonError(w, "no items specified", http.StatusBadRequest)
		return
	}

	archiveName := filepath.Base(body.Name)
	if archiveName == "" || archiveName == "." {
		archiveName = "archive.zip"
	}
	if !strings.HasSuffix(archiveName, ".zip") && !strings.HasSuffix(archiveName, ".tar.gz") {
		archiveName += ".zip"
	}

	firstItemPath, _, err := resolveUserPath(homeDir, body.Items[0])
	if err != nil {
		jsonError(w, err.Error(), http.StatusForbidden)
		return
	}

	targetDir := filepath.Dir(firstItemPath)
	fullOutput := filepath.Join(targetDir, archiveName)

	var itemBases []string
	for _, item := range body.Items {
		itemPath, _, err := resolveUserPath(homeDir, item)
		if err == nil {
			itemBases = append(itemBases, filepath.Base(itemPath))
		}
	}

	var cmd *exec.Cmd
	if strings.HasSuffix(archiveName, ".tar.gz") {
		cmdArgs := append([]string{"sudo", "/usr/bin/tar", "-czf", fullOutput, "-C", targetDir}, itemBases...)
		cmd = exec.Command(cmdArgs[0], cmdArgs[1:]...)
	} else {
		cmdArgs := append([]string{"sudo", "/usr/bin/zip", "-r", fullOutput}, itemBases...)
		cmd = exec.Command(cmdArgs[0], cmdArgs[1:]...)
		cmd.Dir = targetDir
	}

	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("archive failed: %s", strings.TrimSpace(string(out)))
		jsonError(w, "compression failed", http.StatusInternalServerError)
		return
	}

	// Fix ownership after archive creation
	exec.Command("sudo", "/usr/bin/chown", sysUser+":"+sysUser, fullOutput).Run()
	jsonSuccess(w, map[string]interface{}{"status": true, "archive": archiveName})
}

// Action: unarchive (extract zip/tar archive with Zip-Slip protection)
func (h *VueFinderHandler) handleUnarchive(w http.ResponseWriter, r *http.Request, sysUser, homeDir string) {
	var body struct {
		Item        string `json:"item"`
		Destination string `json:"destination"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonError(w, "invalid json body", http.StatusBadRequest)
		return
	}

	archivePath, _, err := resolveUserPath(homeDir, body.Item)
	if err != nil {
		jsonError(w, err.Error(), http.StatusForbidden)
		return
	}

	destDir := filepath.Dir(archivePath)
	if body.Destination != "" {
		destDir, _, err = resolveUserPath(homeDir, body.Destination)
		if err != nil {
			jsonError(w, err.Error(), http.StatusForbidden)
			return
		}
	}

	// Zip-Slip Protection Check for .zip files before executing extraction
	if strings.HasSuffix(strings.ToLower(archivePath), ".zip") {
		rZip, err := zip.OpenReader(archivePath)
		if err == nil {
			absHome, _ := filepath.Abs(homeDir)
			for _, f := range rZip.File {
				targetFile := filepath.Join(destDir, f.Name)
				absTarget, _ := filepath.Abs(targetFile)
				if !strings.HasPrefix(absTarget, absHome+string(os.PathSeparator)) && absTarget != absHome {
					rZip.Close()
					jsonError(w, "zip slip detected: archive entry escapes home directory", http.StatusForbidden)
					return
				}
			}
			rZip.Close()
		}
	}

	var cmd *exec.Cmd
	lower := strings.ToLower(archivePath)
	if strings.HasSuffix(lower, ".zip") {
		cmd = exec.Command("sudo", "/usr/bin/unzip", "-o", archivePath, "-d", destDir)
	} else if strings.HasSuffix(lower, ".tar.gz") || strings.HasSuffix(lower, ".tgz") || strings.HasSuffix(lower, ".tar") {
		cmd = exec.Command("sudo", "/usr/bin/tar", "-xzf", archivePath, "-C", destDir)
	} else {
		jsonError(w, "unsupported archive format", http.StatusBadRequest)
		return
	}

	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("unarchive failed for %s: %s", archivePath, strings.TrimSpace(string(out)))
		jsonError(w, "extraction failed", http.StatusInternalServerError)
		return
	}

	exec.Command("sudo", "/usr/bin/chown", "-R", sysUser+":"+sysUser, destDir).Run()
	jsonSuccess(w, map[string]interface{}{"status": true, "message": "unarchived successfully"})
}

// Silence unused variable warnings
var _ = bytes.Buffer{}
var _ = time.Now
