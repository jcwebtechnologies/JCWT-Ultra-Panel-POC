package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/jcwt/ultra-panel/internal/config"
	"github.com/jcwt/ultra-panel/internal/db"
	"github.com/jcwt/ultra-panel/internal/nginx"
	"github.com/jcwt/ultra-panel/internal/system"
)

type SSLCertsHandler struct {
	DB  *db.DB
	Cfg *config.Config
}

func (h *SSLCertsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	switch r.Method {
	case "GET":
		h.list(w, r)
	case "POST":
		action := r.URL.Query().Get("action")
		switch action {
		case "activate":
			h.activate(w, r)
		default:
			h.create(w, r)
		}
	case "DELETE":
		h.delete(w, r)
	default:
		http.Error(w, `{"success":false,"error":"method not allowed"}`, http.StatusMethodNotAllowed)
	}
}

func (h *SSLCertsHandler) list(w http.ResponseWriter, r *http.Request) {
	siteID, err := strconv.ParseInt(r.URL.Query().Get("site_id"), 10, 64)
	if err != nil {
		jsonError(w, "invalid site_id", http.StatusBadRequest)
		return
	}

	certs, err := h.DB.ListSSLCertificates(siteID)
	if err != nil {
		jsonError(w, "failed to list certificates", http.StatusInternalServerError)
		return
	}
	if certs == nil {
		certs = []map[string]interface{}{}
	}

	jsonSuccess(w, map[string]interface{}{"certificates": certs})
}

func (h *SSLCertsHandler) create(w http.ResponseWriter, r *http.Request) {
	siteIDStr := r.URL.Query().Get("site_id")
	certType := r.URL.Query().Get("type")

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

	domain := site["domain"].(string)
	sysUser := site["system_user"].(string)
	siteType := site["site_type"].(string)
	phpVersion := site["php_version"].(string)
	proxyURL := site["proxy_url"].(string)
	webRoot := site["web_root"].(string)

	var certPath, keyPath, label string

	switch certType {
	case "self-signed":
		certPath, keyPath, err = system.GenerateSelfSignedCert(h.Cfg.SSLBaseDir, domain)
		if err != nil {
			jsonError(w, fmt.Sprintf("failed to generate cert: %v", err), http.StatusInternalServerError)
			return
		}
		label = "Self-Signed"

	case "custom":
		if err := r.ParseMultipartForm(10 << 20); err != nil {
			jsonError(w, "invalid form data", http.StatusBadRequest)
			return
		}
		certFile, _, err := r.FormFile("certificate")
		if err != nil {
			jsonError(w, "certificate file required", http.StatusBadRequest)
			return
		}
		defer certFile.Close()

		keyFile, _, err := r.FormFile("private_key")
		if err != nil {
			jsonError(w, "private key file required", http.StatusBadRequest)
			return
		}
		defer keyFile.Close()

		certData, _ := io.ReadAll(certFile)
		keyData, _ := io.ReadAll(keyFile)

		certPath, keyPath, err = system.SaveCustomCert(h.Cfg.SSLBaseDir, domain, certData, keyData)
		if err != nil {
			jsonError(w, fmt.Sprintf("failed to save cert: %v", err), http.StatusInternalServerError)
			return
		}

		labelVal := r.FormValue("label")
		if labelVal != "" {
			label = labelVal
		} else {
			label = "Custom Certificate"
		}

	default:
		jsonError(w, "invalid type: use self-signed or custom", http.StatusBadRequest)
		return
	}

	// Save to DB and activate
	id, err := h.DB.CreateSSLCertificate(siteID, certType, label, certPath, keyPath, true)
	if err != nil {
		jsonError(w, "failed to save certificate record", http.StatusInternalServerError)
		return
	}

	// Activate this cert (deactivates others)
	h.DB.ActivateSSLCertificate(siteID, id)

	// Update site's SSL and regenerate vhost
	h.DB.UpdateSite(siteID, domain, site["aliases"].(string), siteType, phpVersion, proxyURL, certType, certPath, keyPath)

	vhostData := nginx.VHostData{
		Domain: domain, Aliases: site["aliases"].(string), User: sysUser,
		SiteType: siteType, PHPVersion: phpVersion, ProxyURL: proxyURL, WebRoot: webRoot,
		SSLType: certType, SSLCertPath: certPath, SSLKeyPath: keyPath,
		AccessLog: func() bool { a, _ := siteLogFlags(site); return a }(),
		ErrorLog:  func() bool { _, e := siteLogFlags(site); return e }(),
	}
	nginx.WriteVHost(h.Cfg.NginxSitesAvailable, h.Cfg.NginxSitesEnabled, domain, vhostData)
	nginx.TestAndReload()

	jsonSuccess(w, map[string]interface{}{
		"id": id, "type": certType, "label": label, "cert_path": certPath,
	})
}

func (h *SSLCertsHandler) activate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		CertID int64 `json:"cert_id"`
		SiteID int64 `json:"site_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	cert, err := h.DB.GetSSLCertificate(req.CertID)
	if err != nil {
		jsonError(w, "certificate not found", http.StatusNotFound)
		return
	}

	site, err := h.DB.GetSite(req.SiteID)
	if err != nil {
		jsonError(w, "site not found", http.StatusNotFound)
		return
	}

	// Activate this cert
	if err := h.DB.ActivateSSLCertificate(req.SiteID, req.CertID); err != nil {
		jsonError(w, "failed to activate certificate", http.StatusInternalServerError)
		return
	}

	domain := site["domain"].(string)
	certPath := cert["cert_path"].(string)
	keyPath := cert["key_path"].(string)
	certType := cert["type"].(string)
	sysUser := site["system_user"].(string)
	siteType := site["site_type"].(string)
	phpVersion := site["php_version"].(string)
	proxyURL := site["proxy_url"].(string)
	webRoot := site["web_root"].(string)

	// Update site record
	h.DB.UpdateSite(req.SiteID, domain, site["aliases"].(string), siteType, phpVersion, proxyURL, certType, certPath, keyPath)

	// Regenerate vhost
	vhostData := nginx.VHostData{
		Domain: domain, Aliases: site["aliases"].(string), User: sysUser,
		SiteType: siteType, PHPVersion: phpVersion, ProxyURL: proxyURL, WebRoot: webRoot,
		SSLType: certType, SSLCertPath: certPath, SSLKeyPath: keyPath,
		AccessLog: func() bool { a, _ := siteLogFlags(site); return a }(),
		ErrorLog:  func() bool { _, e := siteLogFlags(site); return e }(),
	}
	nginx.WriteVHost(h.Cfg.NginxSitesAvailable, h.Cfg.NginxSitesEnabled, domain, vhostData)
	nginx.TestAndReload()

	jsonSuccess(w, map[string]interface{}{"message": "certificate activated"})
}

func (h *SSLCertsHandler) delete(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Query().Get("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		jsonError(w, "invalid id", http.StatusBadRequest)
		return
	}

	cert, err := h.DB.GetSSLCertificate(id)
	if err != nil {
		jsonError(w, "certificate not found", http.StatusNotFound)
		return
	}

	// Don't allow deleting the active cert
	if active, ok := cert["active"].(bool); ok && active {
		jsonError(w, "cannot delete the active certificate — activate another first", http.StatusBadRequest)
		return
	}

	if err := h.DB.DeleteSSLCertificate(id); err != nil {
		jsonError(w, "failed to delete certificate", http.StatusInternalServerError)
		return
	}

	jsonSuccess(w, map[string]interface{}{"message": "certificate deleted"})
}
