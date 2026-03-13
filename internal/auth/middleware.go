package auth

import (
	"net/http"
	"strings"
	"sync"
	"time"
)

// Middleware provides HTTP middleware for authentication
type Middleware struct {
	authManager *Manager
	rateLimiter *rateLimiter
}

// NewMiddleware creates auth middleware
func NewMiddleware(am *Manager) *Middleware {
	return &Middleware{
		authManager: am,
		rateLimiter: newRateLimiter(),
	}
}

// RequireAuth middleware checks for valid session
func (m *Middleware) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sessionID := GetSessionID(r)
		if sessionID == "" {
			http.Error(w, `{"success":false,"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}

		_, ok := m.authManager.GetSession(sessionID)
		if !ok {
			ClearSessionCookie(w)
			http.Error(w, `{"success":false,"error":"session expired"}`, http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// RequireCSRF middleware validates CSRF token for mutating requests
func (m *Middleware) RequireCSRF(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only check CSRF for mutating methods
		if r.Method == "GET" || r.Method == "HEAD" || r.Method == "OPTIONS" {
			next.ServeHTTP(w, r)
			return
		}

		sessionID := GetSessionID(r)
		csrfToken := r.Header.Get("X-CSRF-Token")

		if csrfToken == "" || !m.authManager.ValidateCSRF(sessionID, csrfToken) {
			http.Error(w, `{"success":false,"error":"invalid csrf token"}`, http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// RequireRole middleware restricts access to users with specific roles
func (m *Middleware) RequireRole(allowedRoles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sessionID := GetSessionID(r)
			sess, ok := m.authManager.GetSession(sessionID)
			if !ok {
				http.Error(w, `{"success":false,"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}

			for _, role := range allowedRoles {
				if sess.Role == role {
					next.ServeHTTP(w, r)
					return
				}
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte(`{"success":false,"error":"insufficient permissions"}`))
		})
	}
}

// GetSessionFromRequest returns the session for the current request (for handlers)
func (m *Middleware) GetSessionFromRequest(r *http.Request) *Session {
	sessionID := GetSessionID(r)
	sess, ok := m.authManager.GetSession(sessionID)
	if !ok {
		return nil
	}
	return sess
}

// RateLimit middleware for login endpoint
func (m *Middleware) RateLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := extractIP(r)
		if !m.rateLimiter.allow(ip) {
			http.Error(w, `{"success":false,"error":"too many requests"}`, http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// SecurityHeaders adds security headers to all responses
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Frame-Options", "SAMEORIGIN")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		// HSTS: enforce HTTPS for 1 year, include subdomains
		w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		// Skip CSP for filebrowser routes — filebrowser's Ace editor loads themes/modes from cdn.jsdelivr.net
		if !strings.HasPrefix(r.URL.Path, "/fb/") {
			w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline' https://www.google.com https://www.gstatic.com; style-src 'self' 'unsafe-inline' https://fonts.googleapis.com; font-src 'self' https://fonts.gstatic.com; frame-src 'self' https://www.google.com; img-src 'self' data: blob: https:;")
		}
		w.Header().Set("X-Robots-Tag", "noindex, nofollow")

		// No caching for API responses
		if strings.HasPrefix(r.URL.Path, "/api/") {
			w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
			w.Header().Set("Pragma", "no-cache")
		}

		next.ServeHTTP(w, r)
	})
}

// BodySizeLimit limits request body size (1MB default)
func BodySizeLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip for file uploads (handled individually)
		if r.Header.Get("Content-Type") != "" && strings.Contains(r.Header.Get("Content-Type"), "multipart/form-data") {
			next.ServeHTTP(w, r)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1MB
		next.ServeHTTP(w, r)
	})
}

func extractIP(r *http.Request) string {
	// For IPv6, strip port
	addr := r.RemoteAddr
	if idx := strings.LastIndex(addr, ":"); idx != -1 {
		addr = addr[:idx]
	}
	return strings.Trim(addr, "[]")
}

// Simple rate limiter: max 10 attempts per minute per IP
type rateLimiter struct {
	attempts map[string][]time.Time
	mu       sync.Mutex
}

func newRateLimiter() *rateLimiter {
	return &rateLimiter{attempts: make(map[string][]time.Time)}
}

func (rl *rateLimiter) allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-1 * time.Minute)

	// Clean old entries
	var valid []time.Time
	for _, t := range rl.attempts[ip] {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}
	rl.attempts[ip] = valid

	if len(valid) >= 10 {
		return false
	}

	rl.attempts[ip] = append(rl.attempts[ip], now)
	return true
}
