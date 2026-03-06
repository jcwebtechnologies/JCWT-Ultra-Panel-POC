package auth

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// Session represents an authenticated session
type Session struct {
	UserID    int64
	Username  string
	Role      string
	CSRFToken string
	ExpiresAt time.Time
}

// Manager handles authentication and sessions
type Manager struct {
	sessions map[string]*Session
	mu       sync.RWMutex
	timeout  time.Duration
}

// NewManager creates a new auth manager
func NewManager(timeoutMinutes int) *Manager {
	m := &Manager{
		sessions: make(map[string]*Session),
		timeout:  time.Duration(timeoutMinutes) * time.Minute,
	}
	go m.cleanup()
	return m
}

// HashPassword hashes a password with bcrypt
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(hash), err
}

// CheckPassword verifies a password against a hash
func CheckPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// CreateSession creates a new session for a user
func (m *Manager) CreateSession(userID int64, username, role string) (sessionID, csrfToken string) {
	sessionID = generateToken(32)
	csrfToken = generateToken(32)

	m.mu.Lock()
	m.sessions[sessionID] = &Session{
		UserID:    userID,
		Username:  username,
		Role:      role,
		CSRFToken: csrfToken,
		ExpiresAt: time.Now().Add(m.timeout),
	}
	m.mu.Unlock()

	return sessionID, csrfToken
}

// GetSession retrieves and validates a session
func (m *Manager) GetSession(sessionID string) (*Session, bool) {
	m.mu.RLock()
	sess, ok := m.sessions[sessionID]
	m.mu.RUnlock()

	if !ok || time.Now().After(sess.ExpiresAt) {
		if ok {
			m.DestroySession(sessionID)
		}
		return nil, false
	}

	// Extend session on activity
	m.mu.Lock()
	sess.ExpiresAt = time.Now().Add(m.timeout)
	m.mu.Unlock()

	return sess, true
}

// ValidateCSRF checks the CSRF token
func (m *Manager) ValidateCSRF(sessionID, token string) bool {
	sess, ok := m.GetSession(sessionID)
	if !ok {
		return false
	}
	return sess.CSRFToken == token
}

// DestroySession removes a session
func (m *Manager) DestroySession(sessionID string) {
	m.mu.Lock()
	delete(m.sessions, sessionID)
	m.mu.Unlock()
}

// SetSessionCookie sets the session cookie on the response
func SetSessionCookie(w http.ResponseWriter, sessionID string) {
	http.SetCookie(w, &http.Cookie{
		Name:     "jcwt_session",
		Value:    sessionID,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   86400, // 24 hours
	})
}

// ClearSessionCookie clears the session cookie
func ClearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     "jcwt_session",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
	})
}

// GetSessionID extracts session ID from request cookie
func GetSessionID(r *http.Request) string {
	cookie, err := r.Cookie("jcwt_session")
	if err != nil {
		return ""
	}
	return cookie.Value
}

func generateToken(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func (m *Manager) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	for range ticker.C {
		now := time.Now()
		m.mu.Lock()
		for id, sess := range m.sessions {
			if now.After(sess.ExpiresAt) {
				delete(m.sessions, id)
			}
		}
		m.mu.Unlock()
	}
}
