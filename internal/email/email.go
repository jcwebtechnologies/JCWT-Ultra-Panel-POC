package email

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"net/smtp"
	"strings"
	"text/template"
	"time"

	"github.com/jcwt/ultra-panel/internal/db"
)

// Sender handles sending templated emails via SMTP
type Sender struct {
	DB *db.DB
}

// SendTemplatedEmail sends an email using a template slug and variable data
func (s *Sender) SendTemplatedEmail(slug string, to string, vars map[string]string) error {
	// Get the template
	tmpl, err := s.DB.GetEmailTemplateBySlug(slug)
	if err != nil {
		return fmt.Errorf("template %q not found: %w", slug, err)
	}

	enabled, _ := tmpl["enabled"].(bool)
	if !enabled {
		log.Printf("[email] Template %q is disabled, skipping send to %s", slug, to)
		return nil
	}

	// Get SMTP settings
	settings, err := s.DB.GetSMTPSettings()
	if err != nil {
		return fmt.Errorf("SMTP settings not configured: %w", err)
	}

	host, _ := settings["host"].(string)
	if host == "" {
		return fmt.Errorf("SMTP host not configured")
	}
	fromEmail, _ := settings["from_email"].(string)
	if fromEmail == "" {
		return fmt.Errorf("SMTP from email not configured")
	}

	port, _ := settings["port"].(int)
	encryption, _ := settings["encryption"].(string)
	authEnabled, _ := settings["auth_enabled"].(bool)
	username, _ := settings["username"].(string)
	password, _ := settings["password"].(string)
	fromName, _ := settings["from_name"].(string)

	// Render subject
	subjectTmpl, _ := tmpl["subject"].(string)
	subject := renderTemplate(subjectTmpl, vars)

	// Render body content
	bodyContent, _ := tmpl["body_content"].(string)
	renderedBody := renderTemplate(bodyContent, vars)

	// Wrap in common header/footer
	panelName := fromName
	if panelName == "" {
		panelName = "JCWT Ultra Panel"
	}
	htmlBody := wrapWithLayout(renderedBody, panelName)

	// Build email message
	from := fromEmail
	if fromName != "" {
		from = fmt.Sprintf("%s <%s>", fromName, fromEmail)
	}

	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/html; charset=UTF-8\r\nDate: %s\r\n\r\n%s",
		from, to, subject, time.Now().Format(time.RFC1123Z), htmlBody)

	addr := fmt.Sprintf("%s:%d", host, port)

	var auth smtp.Auth
	if authEnabled && username != "" && password != "" {
		auth = smtp.PlainAuth("", username, password, host)
	}

	if err := sendMail(addr, host, encryption, auth, fromEmail, to, []byte(msg)); err != nil {
		return fmt.Errorf("send failed: %w", err)
	}

	log.Printf("[email] Sent %q to %s", slug, to)
	return nil
}

func renderTemplate(tmplStr string, vars map[string]string) string {
	t, err := template.New("email").Parse(tmplStr)
	if err != nil {
		return tmplStr
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, vars); err != nil {
		return tmplStr
	}
	return buf.String()
}

func wrapWithLayout(bodyContent, panelName string) string {
	year := time.Now().Format("2006")
	var sb strings.Builder
	sb.WriteString(`<!DOCTYPE html><html><head><meta charset="UTF-8"><meta name="viewport" content="width=device-width,initial-scale=1.0"></head>`)
	sb.WriteString(`<body style="margin:0;padding:0;background:#f4f4f5;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif;">`)
	sb.WriteString(`<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="background:#f4f4f5;padding:32px 16px;">`)
	sb.WriteString(`<tr><td align="center">`)
	sb.WriteString(`<table role="presentation" width="600" cellpadding="0" cellspacing="0" style="max-width:600px;width:100%;">`)

	// Header
	sb.WriteString(`<tr><td style="background:#6366f1;padding:24px 32px;border-radius:8px 8px 0 0;text-align:center;">`)
	sb.WriteString(fmt.Sprintf(`<h1 style="margin:0;color:#ffffff;font-size:22px;font-weight:700;">%s</h1>`, panelName))
	sb.WriteString(`</td></tr>`)

	// Body
	sb.WriteString(`<tr><td style="background:#ffffff;padding:32px;border-left:1px solid #e5e7eb;border-right:1px solid #e5e7eb;">`)
	sb.WriteString(bodyContent)
	sb.WriteString(`</td></tr>`)

	// Footer
	sb.WriteString(`<tr><td style="background:#f9fafb;padding:20px 32px;border:1px solid #e5e7eb;border-top:none;border-radius:0 0 8px 8px;text-align:center;">`)
	sb.WriteString(fmt.Sprintf(`<p style="margin:0;color:#9ca3af;font-size:12px;">&copy; %s %s &mdash; This is an automated message.</p>`, year, panelName))
	sb.WriteString(`</td></tr>`)

	sb.WriteString(`</table></td></tr></table></body></html>`)
	return sb.String()
}

func sendMail(addr, host, encryption string, auth smtp.Auth, from, to string, msg []byte) error {
	tlsConf := &tls.Config{ServerName: host}

	switch encryption {
	case "ssl":
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
				return fmt.Errorf("auth failed: %w", err)
			}
		}
		if err := c.Mail(from); err != nil {
			return err
		}
		if err := c.Rcpt(to); err != nil {
			return err
		}
		w, err := c.Data()
		if err != nil {
			return err
		}
		w.Write(msg)
		w.Close()
		return c.Quit()

	case "tls":
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
				return fmt.Errorf("auth failed: %w", err)
			}
		}
		if err := c.Mail(from); err != nil {
			return err
		}
		if err := c.Rcpt(to); err != nil {
			return err
		}
		w, err := c.Data()
		if err != nil {
			return err
		}
		w.Write(msg)
		w.Close()
		return c.Quit()

	default:
		return smtp.SendMail(addr, auth, from, []string{to}, msg)
	}
}
