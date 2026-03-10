package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/jcwt/ultra-panel/internal/db"
)

type EmailTemplatesHandler struct {
	DB *db.DB
}

func (h *EmailTemplatesHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	action := r.URL.Query().Get("action")
	switch r.Method {
	case "GET":
		if action == "layout" {
			h.getLayout(w, r)
		} else {
			h.list(w, r)
		}
	case "PUT":
		if action == "layout" {
			h.updateLayout(w, r)
		} else {
			h.update(w, r)
		}
	default:
		http.Error(w, `{"success":false,"error":"method not allowed"}`, http.StatusMethodNotAllowed)
	}
}

func (h *EmailTemplatesHandler) list(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Query().Get("id")
	if idStr != "" {
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			jsonError(w, "invalid id", http.StatusBadRequest)
			return
		}
		tmpl, err := h.DB.GetEmailTemplate(id)
		if err != nil {
			jsonError(w, "template not found", http.StatusNotFound)
			return
		}
		jsonSuccess(w, tmpl)
		return
	}

	templates, err := h.DB.ListEmailTemplates()
	if err != nil {
		jsonError(w, "failed to list email templates", http.StatusInternalServerError)
		return
	}
	if templates == nil {
		templates = []map[string]interface{}{}
	}
	jsonSuccess(w, templates)
}

func (h *EmailTemplatesHandler) update(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID          int64  `json:"id"`
		Subject     string `json:"subject"`
		BodyContent string `json:"body_content"`
		Enabled     bool   `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.ID == 0 {
		jsonError(w, "template id is required", http.StatusBadRequest)
		return
	}

	if len(req.Subject) > 500 {
		jsonError(w, "subject too long (max 500 characters)", http.StatusBadRequest)
		return
	}
	if len(req.BodyContent) > 50000 {
		jsonError(w, "body content too long (max 50000 characters)", http.StatusBadRequest)
		return
	}

	if err := h.DB.UpdateEmailTemplate(req.ID, req.Subject, req.BodyContent, req.Enabled); err != nil {
		jsonError(w, "failed to update template", http.StatusInternalServerError)
		return
	}

	jsonSuccess(w, map[string]interface{}{"updated": true})
}

func (h *EmailTemplatesHandler) getLayout(w http.ResponseWriter, r *http.Request) {
	layout, err := h.DB.GetEmailLayout()
	if err != nil {
		jsonError(w, "failed to load email layout", http.StatusInternalServerError)
		return
	}
	jsonSuccess(w, layout)
}

func (h *EmailTemplatesHandler) updateLayout(w http.ResponseWriter, r *http.Request) {
	var req struct {
		HeaderHTML string `json:"email_header_html"`
		FooterHTML string `json:"email_footer_html"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if len(req.HeaderHTML) > 50000 {
		jsonError(w, "header HTML too long (max 50000 characters)", http.StatusBadRequest)
		return
	}
	if len(req.FooterHTML) > 50000 {
		jsonError(w, "footer HTML too long (max 50000 characters)", http.StatusBadRequest)
		return
	}
	if err := h.DB.UpdateEmailLayout(req.HeaderHTML, req.FooterHTML); err != nil {
		jsonError(w, "failed to update email layout", http.StatusInternalServerError)
		return
	}
	jsonSuccess(w, map[string]interface{}{"updated": true})
}
