package handlers

import (
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/jcwt/ultra-panel/internal/config"
	"github.com/jcwt/ultra-panel/internal/db"
	"github.com/jcwt/ultra-panel/internal/nginx"
	"github.com/jcwt/ultra-panel/internal/system"
)

type SSLHandler struct {
	DB  *db.DB
	Cfg *config.Config
}

func (h *SSLHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	switch r.Method {
	case "POST":
		h.manage(w, r)
	default:
		http.Error(w, `{"success":false,"error":"method not allowed"}`, http.StatusMethodNotAllowed)
	}
}

func (h *SSLHandler) manage(w http.ResponseWriter, r *http.Request) {
	action := r.URL.Query().Get("action")
	idStr := r.URL.Query().Get("site_id")
	siteID, err := strconv.ParseInt(idStr, 10, 64)
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

	switch action {
	case "self-signed":
		certPath, keyPath, err := system.GenerateSelfSignedCert(h.Cfg.SSLBaseDir, domain)
		if err != nil {
			jsonError(w, fmt.Sprintf("failed to generate cert: %v", err), http.StatusInternalServerError)
			return
		}

		h.DB.UpdateSite(siteID, domain, site["aliases"].(string), siteType, phpVersion, proxyURL, "self-signed", certPath, keyPath)

		vhostData := nginx.VHostData{
			Domain: domain, Aliases: site["aliases"].(string), User: sysUser,
			SiteType: siteType, PHPVersion: phpVersion, ProxyURL: proxyURL, WebRoot: webRoot,
			SSLType: "self-signed", SSLCertPath: certPath, SSLKeyPath: keyPath,
			AccessLog: func() bool { a, _ := siteLogFlags(site); return a }(),
			ErrorLog:  func() bool { _, e := siteLogFlags(site); return e }(),
		}
		nginx.WriteVHost(h.Cfg.NginxSitesAvailable, h.Cfg.NginxSitesEnabled, domain, vhostData)
		nginx.TestAndReload()

		jsonSuccess(w, map[string]interface{}{"ssl_type": "self-signed", "cert_path": certPath})

	case "custom":
		// Multipart form with cert and key files
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

		certPath, keyPath, err := system.SaveCustomCert(h.Cfg.SSLBaseDir, domain, certData, keyData)
		if err != nil {
			jsonError(w, fmt.Sprintf("failed to save cert: %v", err), http.StatusInternalServerError)
			return
		}

		h.DB.UpdateSite(siteID, domain, site["aliases"].(string), siteType, phpVersion, proxyURL, "custom", certPath, keyPath)

		vhostData := nginx.VHostData{
			Domain: domain, Aliases: site["aliases"].(string), User: sysUser,
			SiteType: siteType, PHPVersion: phpVersion, ProxyURL: proxyURL, WebRoot: webRoot,
			SSLType: "custom", SSLCertPath: certPath, SSLKeyPath: keyPath,
			AccessLog: func() bool { a, _ := siteLogFlags(site); return a }(),
			ErrorLog:  func() bool { _, e := siteLogFlags(site); return e }(),
		}
		nginx.WriteVHost(h.Cfg.NginxSitesAvailable, h.Cfg.NginxSitesEnabled, domain, vhostData)
		nginx.TestAndReload()

		if siteType == "wordpress" {
			wpUpdateURLScheme(sysUser, phpVersion, webRoot, "https", domain)
		}

		jsonSuccess(w, map[string]interface{}{"ssl_type": "custom", "cert_path": certPath})

	case "disable":
		jsonError(w, "SSL cannot be disabled — at least one certificate must always be active", http.StatusBadRequest)

	default:
		jsonError(w, "invalid action: use self-signed, custom, or disable", http.StatusBadRequest)
	}
}
