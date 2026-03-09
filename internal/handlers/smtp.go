package handlers

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/smtp"
	"strings"
	"time"

	"github.com/jcwt/ultra-panel/internal/config"
	"github.com/jcwt/ultra-panel/internal/db"
)

type SMTPHandler struct {
	DB  *db.DB
	Cfg *config.Config
}

func (h *SMTPHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	switch r.Method {
	case "GET":
		h.get(w, r)
	case "PUT":
		h.update(w, r)
	case "POST":
		action := r.URL.Query().Get("action")
		if action == "test" {
			h.testEmail(w, r)
		} else {
			jsonError(w, "invalid action", http.StatusBadRequest)
		}
	default:
		http.Error(w, `{"success":false,"error":"method not allowed"}`, http.StatusMethodNotAllowed)
	}
}

func (h *SMTPHandler) get(w http.ResponseWriter, r *http.Request) {
	settings, err := h.DB.GetSMTPSettings()
	if err != nil {
		jsonError(w, "failed to load SMTP settings", http.StatusInternalServerError)
		return
	}
	// Never expose the password to the frontend
	if settings["password"] != "" {
		settings["password"] = "••••••••"
	}
	jsonSuccess(w, settings)
}

func (h *SMTPHandler) update(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Host        string `json:"host"`
		Port        int    `json:"port"`
		Encryption  string `json:"encryption"`
		AuthEnabled bool   `json:"auth_enabled"`
		Username    string `json:"username"`
		Password    string `json:"password"`
		FromEmail   string `json:"from_email"`
		FromName    string `json:"from_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Validate
	req.Host = strings.TrimSpace(req.Host)
	req.Username = strings.TrimSpace(req.Username)
	req.FromEmail = strings.TrimSpace(req.FromEmail)
	req.FromName = strings.TrimSpace(req.FromName)

	if len(req.Host) > 255 || len(req.Username) > 255 || len(req.FromEmail) > 255 || len(req.FromName) > 100 {
		jsonError(w, "field values too long", http.StatusBadRequest)
		return
	}

	if req.Port < 1 || req.Port > 65535 {
		req.Port = 587
	}

	switch req.Encryption {
	case "none", "tls", "ssl":
		// valid
	default:
		req.Encryption = "tls"
	}

	if req.FromEmail != "" && !emailRegex.MatchString(req.FromEmail) {
		jsonError(w, "invalid from email address", http.StatusBadRequest)
		return
	}

	// If password is masked placeholder, keep the existing password
	if req.Password == "••••••••" {
		existing, err := h.DB.GetSMTPSettings()
		if err == nil {
			if pw, ok := existing["password"].(string); ok {
				req.Password = pw
			}
		}
	}

	if err := h.DB.UpdateSMTPSettings(
		req.Host, req.Port, req.Encryption, req.AuthEnabled,
		req.Username, req.Password, req.FromEmail, req.FromName,
	); err != nil {
		jsonError(w, "failed to save SMTP settings", http.StatusInternalServerError)
		return
	}

	jsonSuccess(w, map[string]interface{}{"updated": true})
}

func (h *SMTPHandler) testEmail(w http.ResponseWriter, r *http.Request) {
	var req struct {
		To string `json:"to"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	req.To = strings.TrimSpace(req.To)
	if !emailRegex.MatchString(req.To) {
		jsonError(w, "invalid recipient email address", http.StatusBadRequest)
		return
	}

	settings, err := h.DB.GetSMTPSettings()
	if err != nil {
		jsonError(w, "failed to load SMTP settings", http.StatusInternalServerError)
		return
	}

	host, _ := settings["host"].(string)
	port, _ := settings["port"].(int)
	encryption, _ := settings["encryption"].(string)
	authEnabled, _ := settings["auth_enabled"].(bool)
	username, _ := settings["username"].(string)
	password, _ := settings["password"].(string)
	fromEmail, _ := settings["from_email"].(string)
	fromName, _ := settings["from_name"].(string)

	if host == "" {
		jsonError(w, "SMTP host is not configured", http.StatusBadRequest)
		return
	}
	if fromEmail == "" {
		jsonError(w, "from email is not configured", http.StatusBadRequest)
		return
	}

	// Build the email message
	from := fromEmail
	if fromName != "" {
		from = fmt.Sprintf("%s <%s>", fromName, fromEmail)
	}
	subject := "JCWT Ultra Panel — SMTP Test"
	body := "This is a test email from JCWT Ultra Panel to verify your SMTP configuration is working correctly.\r\n\r\nIf you received this, your SMTP settings are configured properly."

	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\nDate: %s\r\n\r\n%s",
		from, req.To, subject, time.Now().Format(time.RFC1123Z), body)

	addr := fmt.Sprintf("%s:%d", host, port)

	var auth_ smtp.Auth
	if authEnabled && username != "" && password != "" {
		auth_ = smtp.PlainAuth("", username, password, host)
	}

	if err := sendMail(addr, host, encryption, auth_, fromEmail, req.To, []byte(msg)); err != nil {
		jsonError(w, "SMTP test failed: "+err.Error(), http.StatusBadRequest)
		return
	}

	jsonSuccess(w, map[string]interface{}{"sent": true})
}

func sendMail(addr, host, encryption string, auth smtp.Auth, from, to string, msg []byte) error {
	tlsConf := &tls.Config{ServerName: host}

	switch encryption {
	case "ssl":
		// Implicit TLS (port 465)
		conn, err := tls.DialWithDialer(&net.Dialer{Timeout: 10 * time.Second}, "tcp", addr, tlsConf)
		if err != nil {
			return fmt.Errorf("TLS connection failed: %w", err)
		}
		defer conn.Close()

		c, err := smtp.NewClient(conn, host)
		if err != nil {
			return fmt.Errorf("SMTP client failed: %w", err)
		}
		defer c.Close()

		if auth != nil {
			if err := c.Auth(auth); err != nil {
				return fmt.Errorf("authentication failed: %w", err)
			}
		}
		if err := c.Mail(from); err != nil {
			return fmt.Errorf("MAIL FROM failed: %w", err)
		}
		if err := c.Rcpt(to); err != nil {
			return fmt.Errorf("RCPT TO failed: %w", err)
		}
		w, err := c.Data()
		if err != nil {
			return fmt.Errorf("DATA failed: %w", err)
		}
		if _, err := w.Write(msg); err != nil {
			return fmt.Errorf("write failed: %w", err)
		}
		if err := w.Close(); err != nil {
			return fmt.Errorf("close data failed: %w", err)
		}
		return c.Quit()

	case "tls":
		// STARTTLS (port 587)
		conn, err := net.DialTimeout("tcp", addr, 10*time.Second)
		if err != nil {
			return fmt.Errorf("connection failed: %w", err)
		}
		defer conn.Close()

		c, err := smtp.NewClient(conn, host)
		if err != nil {
			return fmt.Errorf("SMTP client failed: %w", err)
		}
		defer c.Close()

		if err := c.StartTLS(tlsConf); err != nil {
			return fmt.Errorf("STARTTLS failed: %w", err)
		}
		if auth != nil {
			if err := c.Auth(auth); err != nil {
				return fmt.Errorf("authentication failed: %w", err)
			}
		}
		if err := c.Mail(from); err != nil {
			return fmt.Errorf("MAIL FROM failed: %w", err)
		}
		if err := c.Rcpt(to); err != nil {
			return fmt.Errorf("RCPT TO failed: %w", err)
		}
		w, err := c.Data()
		if err != nil {
			return fmt.Errorf("DATA failed: %w", err)
		}
		if _, err := w.Write(msg); err != nil {
			return fmt.Errorf("write failed: %w", err)
		}
		if err := w.Close(); err != nil {
			return fmt.Errorf("close data failed: %w", err)
		}
		return c.Quit()

	default:
		// No encryption
		return smtp.SendMail(addr, auth, from, []string{to}, msg)
	}
}
