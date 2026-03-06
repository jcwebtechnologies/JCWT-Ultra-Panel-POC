package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/jcwt/ultra-panel/internal/db"
	"github.com/jcwt/ultra-panel/internal/system"
)

type CronHandler struct {
	DB *db.DB
}

func (h *CronHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	switch r.Method {
	case "GET":
		h.list(w, r)
	case "POST":
		h.create(w, r)
	case "PUT":
		h.update(w, r)
	case "DELETE":
		h.delete(w, r)
	default:
		http.Error(w, `{"success":false,"error":"method not allowed"}`, http.StatusMethodNotAllowed)
	}
}

func (h *CronHandler) list(w http.ResponseWriter, r *http.Request) {
	siteIDStr := r.URL.Query().Get("site_id")
	siteID, err := strconv.ParseInt(siteIDStr, 10, 64)
	if err != nil {
		jsonError(w, "invalid site_id", http.StatusBadRequest)
		return
	}

	jobs, err := h.DB.ListCronJobs(siteID)
	if err != nil {
		jsonError(w, "failed to list cron jobs", http.StatusInternalServerError)
		return
	}
	if jobs == nil {
		jobs = []map[string]interface{}{}
	}
	jsonSuccess(w, jobs)
}

func (h *CronHandler) create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SiteID   int64  `json:"site_id"`
		Schedule string `json:"schedule"`
		Command  string `json:"command"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Schedule == "" || req.Command == "" {
		jsonError(w, "schedule and command are required", http.StatusBadRequest)
		return
	}

	id, err := h.DB.CreateCronJob(req.SiteID, req.Schedule, req.Command)
	if err != nil {
		jsonError(w, "failed to create cron job", http.StatusInternalServerError)
		return
	}

	// Sync crontab for the site user
	h.syncSiteCrontab(req.SiteID)

	jsonSuccess(w, map[string]interface{}{"id": id})
}

func (h *CronHandler) update(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID       int64  `json:"id"`
		SiteID   int64  `json:"site_id"`
		Schedule string `json:"schedule"`
		Command  string `json:"command"`
		Enabled  bool   `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.DB.UpdateCronJob(req.ID, req.Schedule, req.Command, req.Enabled); err != nil {
		jsonError(w, "failed to update cron job", http.StatusInternalServerError)
		return
	}

	h.syncSiteCrontab(req.SiteID)
	jsonSuccess(w, map[string]interface{}{"updated": true})
}

func (h *CronHandler) delete(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Query().Get("id")
	siteIDStr := r.URL.Query().Get("site_id")
	id, _ := strconv.ParseInt(idStr, 10, 64)
	siteID, _ := strconv.ParseInt(siteIDStr, 10, 64)

	if err := h.DB.DeleteCronJob(id); err != nil {
		jsonError(w, "failed to delete cron job", http.StatusInternalServerError)
		return
	}

	h.syncSiteCrontab(siteID)
	jsonSuccess(w, map[string]interface{}{"deleted": true})
}

func (h *CronHandler) syncSiteCrontab(siteID int64) {
	site, err := h.DB.GetSite(siteID)
	if err != nil {
		return
	}
	sysUser := site["system_user"].(string)

	jobs, err := h.DB.ListCronJobs(siteID)
	if err != nil {
		return
	}

	var entries []system.CronEntry
	for _, j := range jobs {
		entries = append(entries, system.CronEntry{
			Schedule: j["schedule"].(string),
			Command:  j["command"].(string),
			Enabled:  j["enabled"].(bool),
		})
	}

	system.SyncCrontab(sysUser, entries)
}
