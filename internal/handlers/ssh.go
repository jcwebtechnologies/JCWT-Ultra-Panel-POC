package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/jcwt/ultra-panel/internal/db"
	"github.com/jcwt/ultra-panel/internal/system"
)

type SSHHandler struct {
	DB *db.DB
}

func (h *SSHHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	action := r.URL.Query().Get("action")

	switch r.Method {
	case "GET":
		switch action {
		case "status":
			h.getStatus(w, r)
		case "view-key":
			h.viewKey(w, r)
		default:
			h.listKeys(w, r)
		}
	case "POST":
		switch action {
		case "toggle":
			h.toggleSSH(w, r)
		case "generate":
			h.generateKey(w, r)
		case "upload":
			h.uploadKey(w, r)
		case "authorize":
			h.authorizeKey(w, r)
		default:
			jsonError(w, "unknown action", http.StatusBadRequest)
		}
	case "DELETE":
		h.deleteKey(w, r)
	default:
		http.Error(w, `{"success":false,"error":"method not allowed"}`, http.StatusMethodNotAllowed)
	}
}

func (h *SSHHandler) getStatus(w http.ResponseWriter, r *http.Request) {
	siteID, sysUser, err := getSiteUserFromQuery(r, h.DB)
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	_ = siteID

	enabled := system.IsSSHEnabled(sysUser)
	jsonSuccess(w, map[string]interface{}{"ssh_enabled": enabled, "system_user": sysUser})
}

func (h *SSHHandler) toggleSSH(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SiteID  int64 `json:"site_id"`
		Enabled bool  `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request", http.StatusBadRequest)
		return
	}

	site, err := h.DB.GetSite(req.SiteID)
	if err != nil {
		jsonError(w, "site not found", http.StatusNotFound)
		return
	}
	sysUser := site["system_user"].(string)

	if req.Enabled {
		if err := system.EnableSSH(sysUser); err != nil {
			jsonError(w, fmt.Sprintf("failed to enable SSH: %v", err), http.StatusInternalServerError)
			return
		}
		log.Printf("SSH enabled for user %s (site %d)", sysUser, req.SiteID)
	} else {
		if err := system.DisableSSH(sysUser); err != nil {
			jsonError(w, fmt.Sprintf("failed to disable SSH: %v", err), http.StatusInternalServerError)
			return
		}
		log.Printf("SSH disabled for user %s (site %d)", sysUser, req.SiteID)
	}

	jsonSuccess(w, map[string]interface{}{"ssh_enabled": req.Enabled})
}

func (h *SSHHandler) listKeys(w http.ResponseWriter, r *http.Request) {
	siteIDStr := r.URL.Query().Get("site_id")
	siteID, err := strconv.ParseInt(siteIDStr, 10, 64)
	if err != nil {
		jsonError(w, "invalid site_id", http.StatusBadRequest)
		return
	}

	keys, err := h.DB.ListSSHKeys(siteID)
	if err != nil {
		jsonError(w, "failed to list keys", http.StatusInternalServerError)
		return
	}

	// Check which keys have private keys stored
	for _, k := range keys {
		full, err := h.DB.GetSSHKey(k["id"].(int64))
		if err == nil {
			k["has_private_key"] = full["private_key"].(string) != ""
		}
	}

	if keys == nil {
		keys = []map[string]interface{}{}
	}
	jsonSuccess(w, keys)
}

func (h *SSHHandler) viewKey(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Query().Get("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		jsonError(w, "invalid id", http.StatusBadRequest)
		return
	}

	keyType := r.URL.Query().Get("type") // "public" or "private"

	key, err := h.DB.GetSSHKey(id)
	if err != nil {
		jsonError(w, "key not found", http.StatusNotFound)
		return
	}

	if keyType == "private" {
		content := key["private_key"].(string)
		if content == "" {
			jsonError(w, "no private key stored for this key", http.StatusNotFound)
			return
		}
		jsonSuccess(w, map[string]interface{}{"content": content, "name": key["name"]})
	} else {
		jsonSuccess(w, map[string]interface{}{"content": key["public_key"], "name": key["name"]})
	}
}

func (h *SSHHandler) generateKey(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SiteID  int64  `json:"site_id"`
		Name    string `json:"name"`
		KeyType string `json:"key_type"`
		Bits    int    `json:"bits"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request", http.StatusBadRequest)
		return
	}

	if req.Name == "" {
		jsonError(w, "key name is required", http.StatusBadRequest)
		return
	}

	// Validate key type and bits
	switch req.KeyType {
	case "rsa":
		if req.Bits != 2048 && req.Bits != 4096 {
			jsonError(w, "RSA bits must be 2048 or 4096", http.StatusBadRequest)
			return
		}
	case "ecdsa":
		if req.Bits != 256 && req.Bits != 384 && req.Bits != 521 {
			jsonError(w, "ECDSA bits must be 256, 384, or 521", http.StatusBadRequest)
			return
		}
	default:
		jsonError(w, "key type must be rsa or ecdsa", http.StatusBadRequest)
		return
	}

	// Verify site exists
	if _, err := h.DB.GetSite(req.SiteID); err != nil {
		jsonError(w, "site not found", http.StatusNotFound)
		return
	}

	pubKey, privKey, fingerprint, err := system.GenerateSSHKeyPair(req.KeyType, req.Bits)
	if err != nil {
		jsonError(w, fmt.Sprintf("key generation failed: %v", err), http.StatusInternalServerError)
		return
	}

	id, err := h.DB.CreateSSHKey(req.SiteID, req.Name, req.KeyType, req.Bits, pubKey, privKey, fingerprint)
	if err != nil {
		jsonError(w, "failed to save key", http.StatusInternalServerError)
		return
	}

	log.Printf("SSH key generated: %s (%s %d) for site %d", req.Name, req.KeyType, req.Bits, req.SiteID)
	jsonSuccess(w, map[string]interface{}{
		"id": id, "name": req.Name, "key_type": req.KeyType, "bits": req.Bits,
		"public_key": pubKey, "fingerprint": fingerprint, "has_private_key": true,
	})
}

func (h *SSHHandler) uploadKey(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SiteID     int64  `json:"site_id"`
		Name       string `json:"name"`
		KeyType    string `json:"key_type"`
		Bits       int    `json:"bits"`
		PublicKey  string `json:"public_key"`
		PrivateKey string `json:"private_key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request", http.StatusBadRequest)
		return
	}

	if req.Name == "" {
		jsonError(w, "key name is required", http.StatusBadRequest)
		return
	}
	if req.PublicKey == "" {
		jsonError(w, "public key is required", http.StatusBadRequest)
		return
	}

	req.PublicKey = strings.TrimSpace(req.PublicKey)
	req.PrivateKey = strings.TrimSpace(req.PrivateKey)

	if req.KeyType == "" {
		req.KeyType = "rsa"
	}

	// Verify site exists
	if _, err := h.DB.GetSite(req.SiteID); err != nil {
		jsonError(w, "site not found", http.StatusNotFound)
		return
	}

	// Get fingerprint from public key
	fingerprint, err := system.GetSSHFingerprint(req.PublicKey)
	if err != nil {
		jsonError(w, fmt.Sprintf("invalid public key: %v", err), http.StatusBadRequest)
		return
	}

	id, err := h.DB.CreateSSHKey(req.SiteID, req.Name, req.KeyType, req.Bits, req.PublicKey, req.PrivateKey, fingerprint)
	if err != nil {
		jsonError(w, "failed to save key", http.StatusInternalServerError)
		return
	}

	log.Printf("SSH key uploaded: %s for site %d", req.Name, req.SiteID)
	jsonSuccess(w, map[string]interface{}{
		"id": id, "name": req.Name, "fingerprint": fingerprint,
		"has_private_key": req.PrivateKey != "",
	})
}

func (h *SSHHandler) authorizeKey(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID         int64 `json:"id"`
		SiteID     int64 `json:"site_id"`
		Authorized bool  `json:"authorized"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request", http.StatusBadRequest)
		return
	}

	site, err := h.DB.GetSite(req.SiteID)
	if err != nil {
		jsonError(w, "site not found", http.StatusNotFound)
		return
	}

	// Update DB
	if err := h.DB.UpdateSSHKeyAuthorized(req.ID, req.Authorized); err != nil {
		jsonError(w, "failed to update key", http.StatusInternalServerError)
		return
	}

	// Sync authorized_keys file
	sysUser := site["system_user"].(string)
	homeDir := site["web_root"].(string)
	// web_root is /home/user/htdocs, we need /home/user
	homeDir = strings.TrimSuffix(homeDir, "/htdocs")

	authorizedKeys, _ := h.DB.ListAuthorizedSSHKeys(req.SiteID)
	var pubKeys []string
	for _, k := range authorizedKeys {
		pubKeys = append(pubKeys, k["public_key"].(string))
	}

	if err := system.SyncAuthorizedKeys(sysUser, homeDir, pubKeys); err != nil {
		log.Printf("Failed to sync authorized_keys for %s: %v", sysUser, err)
		jsonError(w, fmt.Sprintf("key updated in DB but failed to sync authorized_keys: %v", err), http.StatusInternalServerError)
		return
	}

	action := "authorized"
	if !req.Authorized {
		action = "deauthorized"
	}
	log.Printf("SSH key %d %s for site %d", req.ID, action, req.SiteID)
	jsonSuccess(w, map[string]interface{}{"updated": true, "authorized": req.Authorized})
}

func (h *SSHHandler) deleteKey(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Query().Get("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		jsonError(w, "invalid id", http.StatusBadRequest)
		return
	}

	siteIDStr := r.URL.Query().Get("site_id")
	siteID, err := strconv.ParseInt(siteIDStr, 10, 64)
	if err != nil {
		jsonError(w, "invalid site_id", http.StatusBadRequest)
		return
	}

	// Check if key was authorized — if so, sync after delete
	key, err := h.DB.GetSSHKey(id)
	if err != nil {
		jsonError(w, "key not found", http.StatusNotFound)
		return
	}
	wasAuthorized := key["authorized"].(bool)

	if err := h.DB.DeleteSSHKey(id); err != nil {
		jsonError(w, "failed to delete key", http.StatusInternalServerError)
		return
	}

	// Re-sync authorized_keys if the deleted key was authorized
	if wasAuthorized {
		site, err := h.DB.GetSite(siteID)
		if err == nil {
			sysUser := site["system_user"].(string)
			homeDir := strings.TrimSuffix(site["web_root"].(string), "/htdocs")
			authorizedKeys, _ := h.DB.ListAuthorizedSSHKeys(siteID)
			var pubKeys []string
			for _, k := range authorizedKeys {
				pubKeys = append(pubKeys, k["public_key"].(string))
			}
			system.SyncAuthorizedKeys(sysUser, homeDir, pubKeys)
		}
	}

	log.Printf("SSH key %d deleted for site %d", id, siteID)
	jsonSuccess(w, map[string]interface{}{"deleted": true})
}
