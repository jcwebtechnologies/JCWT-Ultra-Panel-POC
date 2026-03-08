package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"github.com/jcwt/ultra-panel/internal/db"
)

type FirewallHandler struct {
	DB *db.DB
}

func (h *FirewallHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	switch r.Method {
	case "GET":
		h.list(w, r)
	case "POST":
		action := r.URL.Query().Get("action")
		switch action {
		case "toggle":
			h.toggle(w, r)
		default:
			h.create(w, r)
		}
	case "PUT":
		h.update(w, r)
	case "DELETE":
		h.delete(w, r)
	default:
		http.Error(w, `{"success":false,"error":"method not allowed"}`, http.StatusMethodNotAllowed)
	}
}

func (h *FirewallHandler) list(w http.ResponseWriter, r *http.Request) {
	rules, err := h.DB.ListFirewallRules()
	if err != nil {
		jsonError(w, "failed to list firewall rules", http.StatusInternalServerError)
		return
	}
	if rules == nil {
		rules = []map[string]interface{}{}
	}

	// Prepend default rules (always present, not stored in DB)
	defaults := []map[string]interface{}{
		{"id": int64(-1), "direction": "in", "action": "allow", "protocol": "tcp", "port": "22", "source": "", "comment": "SSH (default)", "enabled": true, "is_default": true},
		{"id": int64(-2), "direction": "in", "action": "allow", "protocol": "tcp", "port": "443", "source": "", "comment": "HTTPS (default)", "enabled": true, "is_default": true},
	}
	rules = append(defaults, rules...)

	// Get ufw status
	status := "unknown"
	cmd := exec.Command("sudo", "ufw", "status")
	output, err := cmd.CombinedOutput()
	if err == nil {
		out := string(output)
		if strings.Contains(out, "Status: active") {
			status = "active"
		} else if strings.Contains(out, "Status: inactive") {
			status = "inactive"
		}
	}

	jsonSuccess(w, map[string]interface{}{
		"rules":  rules,
		"status": status,
	})
}

func (h *FirewallHandler) create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Direction string `json:"direction"`
		Action    string `json:"action"`
		Protocol  string `json:"protocol"`
		Port      string `json:"port"`
		Source    string `json:"source"`
		Comment   string `json:"comment"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Validate
	if req.Direction != "in" && req.Direction != "out" {
		jsonError(w, "direction must be 'in' or 'out'", http.StatusBadRequest)
		return
	}
	if req.Action != "allow" && req.Action != "deny" && req.Action != "reject" {
		jsonError(w, "action must be 'allow', 'deny', or 'reject'", http.StatusBadRequest)
		return
	}
	if req.Protocol != "tcp" && req.Protocol != "udp" && req.Protocol != "any" {
		jsonError(w, "protocol must be 'tcp', 'udp', or 'any'", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.Port) == "" {
		jsonError(w, "port is required", http.StatusBadRequest)
		return
	}
	// Validate port format: number or range (e.g. 8080 or 3000:3100)
	portRe := regexp.MustCompile(`^\d+([:\-]\d+)?$`)
	if !portRe.MatchString(strings.TrimSpace(req.Port)) {
		jsonError(w, "port must be a number or range (e.g. 8080 or 3000:3100)", http.StatusBadRequest)
		return
	}

	// Apply the rule via ufw
	if err := applyUFWRule(req.Action, req.Direction, req.Protocol, req.Port, req.Source); err != nil {
		jsonError(w, fmt.Sprintf("failed to apply firewall rule: %v", err), http.StatusInternalServerError)
		return
	}

	id, err := h.DB.CreateFirewallRule(req.Direction, req.Action, req.Protocol, req.Port, req.Source, req.Comment)
	if err != nil {
		jsonError(w, "rule applied but failed to save record", http.StatusInternalServerError)
		return
	}

	jsonSuccess(w, map[string]interface{}{"id": id, "message": "rule created and applied"})
}

func (h *FirewallHandler) update(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID        int64  `json:"id"`
		Direction string `json:"direction"`
		Action    string `json:"action"`
		Protocol  string `json:"protocol"`
		Port      string `json:"port"`
		Source    string `json:"source"`
		Comment   string `json:"comment"`
		Enabled   bool   `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.DB.UpdateFirewallRule(req.ID, req.Direction, req.Action, req.Protocol, req.Port, req.Source, req.Comment, req.Enabled); err != nil {
		jsonError(w, "failed to update rule", http.StatusInternalServerError)
		return
	}

	// Re-sync all rules to ufw
	h.syncRulesToUFW()

	jsonSuccess(w, map[string]interface{}{"message": "rule updated"})
}

func (h *FirewallHandler) delete(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Query().Get("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		jsonError(w, "invalid id", http.StatusBadRequest)
		return
	}

	if err := h.DB.DeleteFirewallRule(id); err != nil {
		jsonError(w, "failed to delete rule", http.StatusInternalServerError)
		return
	}

	// Re-sync
	h.syncRulesToUFW()

	jsonSuccess(w, map[string]interface{}{"message": "rule deleted"})
}

func (h *FirewallHandler) toggle(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Enable bool `json:"enable"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	var cmd *exec.Cmd
	if req.Enable {
		cmd = exec.Command("sudo", "ufw", "--force", "enable")
	} else {
		cmd = exec.Command("sudo", "ufw", "disable")
	}

	if output, err := cmd.CombinedOutput(); err != nil {
		jsonError(w, fmt.Sprintf("failed to toggle firewall: %s", string(output)), http.StatusInternalServerError)
		return
	}

	status := "inactive"
	if req.Enable {
		status = "active"
	}

	jsonSuccess(w, map[string]interface{}{"status": status})
}

// syncRulesToUFW resets ufw and re-applies all enabled rules from the database
func (h *FirewallHandler) syncRulesToUFW() {
	rules, err := h.DB.ListFirewallRules()
	if err != nil {
		return
	}

	// Reset ufw (keeps defaults)
	exec.Command("sudo", "ufw", "--force", "reset").Run()
	exec.Command("sudo", "ufw", "default", "deny", "incoming").Run()
	exec.Command("sudo", "ufw", "default", "allow", "outgoing").Run()

	// Always allow SSH and HTTPS
	exec.Command("sudo", "ufw", "allow", "22/tcp").Run()
	exec.Command("sudo", "ufw", "allow", "443/tcp").Run()

	for _, rule := range rules {
		enabled, _ := rule["enabled"].(bool)
		if !enabled {
			continue
		}
		action, _ := rule["action"].(string)
		direction, _ := rule["direction"].(string)
		protocol, _ := rule["protocol"].(string)
		port, _ := rule["port"].(string)
		source, _ := rule["source"].(string)
		applyUFWRule(action, direction, protocol, port, source)
	}

	exec.Command("sudo", "ufw", "--force", "enable").Run()
}

func applyUFWRule(action, direction, protocol, port, source string) error {
	// Build ufw command: sudo ufw [allow|deny|reject] [in|out] [from <source>] [proto <protocol>] [to any port <port>]
	args := []string{"sudo", "ufw"}

	args = append(args, action)

	if direction == "in" {
		args = append(args, "in")
	} else {
		args = append(args, "out")
	}

	if source != "" && source != "any" {
		args = append(args, "from", source)
	}

	if protocol != "any" {
		args = append(args, "proto", protocol)
	}

	args = append(args, "to", "any", "port", port)

	cmd := exec.Command(args[0], args[1:]...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %s", err, string(output))
	}
	return nil
}
