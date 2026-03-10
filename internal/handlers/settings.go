package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/jcwt/ultra-panel/internal/config"
	"github.com/jcwt/ultra-panel/internal/db"
)

var hexColorRegex = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)
var allowedImageExts = map[string]bool{".png": true, ".jpg": true, ".jpeg": true, ".ico": true, ".svg": true, ".webp": true}

type SettingsHandler struct {
	DB      *db.DB
	Cfg     *config.Config
	Version string
}

func (h *SettingsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	switch r.Method {
	case "GET":
		h.get(w, r)
	case "PUT":
		h.update(w, r)
	case "POST":
		action := r.URL.Query().Get("action")
		if action == "upload-logo" {
			h.uploadLogo(w, r)
		} else if action == "upload-logo-dark" {
			h.uploadLogoDark(w, r)
		} else if action == "upload-favicon" {
			h.uploadFavicon(w, r)
		} else {
			jsonError(w, "invalid action", http.StatusBadRequest)
		}
	default:
		http.Error(w, `{"success":false,"error":"method not allowed"}`, http.StatusMethodNotAllowed)
	}
}

func (h *SettingsHandler) get(w http.ResponseWriter, r *http.Request) {
	settings, err := h.DB.GetPanelSettings()
	if err != nil {
		jsonError(w, "failed to load settings", http.StatusInternalServerError)
		return
	}
	settings["version"] = h.Version
	jsonSuccess(w, settings)
}

func (h *SettingsHandler) update(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PanelName          string `json:"panel_name"`
		PanelTagline       string `json:"panel_tagline"`
		LogoURL            string `json:"logo_url"`
		LogoURLDark        string `json:"logo_url_dark"`
		FaviconURL         string `json:"favicon_url"`
		PrimaryColor       string `json:"primary_color"`
		AccentColor        string `json:"accent_color"`
		FooterText         string `json:"footer_text"`
		SessionTimeout     int    `json:"session_timeout"`
		RecaptchaSiteKey   string `json:"recaptcha_site_key"`
		RecaptchaSecretKey string `json:"recaptcha_secret_key"`
		Timezone           string `json:"timezone"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Sanitize text inputs (strip HTML)
	req.PanelName = stripHTML(req.PanelName)
	req.PanelTagline = stripHTML(req.PanelTagline)
	req.FooterText = stripHTML(req.FooterText)

	// Validate lengths
	if len(req.PanelName) > 100 || len(req.PanelTagline) > 200 || len(req.FooterText) > 500 {
		jsonError(w, "text fields too long", http.StatusBadRequest)
		return
	}

	// Validate colors
	if req.PrimaryColor != "" && !hexColorRegex.MatchString(req.PrimaryColor) {
		jsonError(w, "invalid primary color format (use #RRGGBB)", http.StatusBadRequest)
		return
	}
	if req.AccentColor != "" && !hexColorRegex.MatchString(req.AccentColor) {
		jsonError(w, "invalid accent color format (use #RRGGBB)", http.StatusBadRequest)
		return
	}

	if req.SessionTimeout < 5 {
		req.SessionTimeout = 5
	}
	if req.SessionTimeout > 1440 {
		req.SessionTimeout = 1440
	}

	// Validate timezone
	timezoneRegex := regexp.MustCompile(`^[A-Za-z_]+/[A-Za-z_/]+$|^UTC$|^Etc/.*$`)
	if req.Timezone != "" && !timezoneRegex.MatchString(req.Timezone) {
		jsonError(w, "invalid timezone format", http.StatusBadRequest)
		return
	}
	if req.Timezone == "" {
		req.Timezone = "UTC"
	}

	if err := h.DB.UpdatePanelSettings(
		req.PanelName, req.PanelTagline, req.LogoURL, req.LogoURLDark, req.FaviconURL,
		req.PrimaryColor, req.AccentColor, req.FooterText,
		req.SessionTimeout,
		req.RecaptchaSiteKey, req.RecaptchaSecretKey,
		req.Timezone,
	); err != nil {
		jsonError(w, "failed to save settings", http.StatusInternalServerError)
		return
	}

	// Apply timezone to system
	if req.Timezone != "" {
		if out, err := exec.Command("sudo", "timedatectl", "set-timezone", req.Timezone).CombinedOutput(); err != nil {
			log.Printf("Failed to set timezone to %s: %s %v", req.Timezone, string(out), err)
		} else {
			log.Printf("Timezone set to %s", req.Timezone)
		}
	}

	jsonSuccess(w, map[string]interface{}{"updated": true})
}

func (h *SettingsHandler) uploadLogo(w http.ResponseWriter, r *http.Request) {
	url, err := h.handleFileUpload(r, "logo")
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	jsonSuccess(w, map[string]interface{}{"url": url})
}

func (h *SettingsHandler) uploadLogoDark(w http.ResponseWriter, r *http.Request) {
	url, err := h.handleFileUpload(r, "logo-dark")
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	jsonSuccess(w, map[string]interface{}{"url": url})
}

func (h *SettingsHandler) uploadFavicon(w http.ResponseWriter, r *http.Request) {
	url, err := h.handleFileUpload(r, "favicon")
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	jsonSuccess(w, map[string]interface{}{"url": url})
}

func (h *SettingsHandler) handleFileUpload(r *http.Request, fileType string) (string, error) {
	r.Body = http.MaxBytesReader(nil, r.Body, 5<<20) // 5MB max

	if err := r.ParseMultipartForm(5 << 20); err != nil {
		return "", err
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		return "", err
	}
	defer file.Close()

	// Validate file extension
	ext := strings.ToLower(filepath.Ext(header.Filename))
	if !allowedImageExts[ext] {
		return "", fmt.Errorf("invalid file type: only png, jpg, ico, svg, webp allowed")
	}

	// Create upload directory
	uploadDir := filepath.Join(h.Cfg.DataDir, "uploads")
	os.MkdirAll(uploadDir, 0755)

	destName := fileType + ext
	destPath := filepath.Join(uploadDir, destName)

	dst, err := os.Create(destPath)
	if err != nil {
		return "", err
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		return "", err
	}

	return "/api/uploads/" + destName, nil
}

// stripHTML removes HTML tags from a string
func stripHTML(s string) string {
	var result strings.Builder
	var inTag bool
	for _, r := range s {
		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
		case !inTag:
			result.WriteRune(r)
		}
	}
	return result.String()
}
