package totp

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"net/url"
	"strings"
	"time"
)

const (
	SecretSize = 20 // 160 bits
	CodeDigits = 6
	TimeStep   = 30 // seconds
	Window     = 1  // accept ±1 time step
)

// GenerateSecret creates a new random base32-encoded TOTP secret
func GenerateSecret() (string, error) {
	secret := make([]byte, SecretSize)
	if _, err := rand.Read(secret); err != nil {
		return "", err
	}
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(secret), nil
}

// GenerateCode computes a TOTP code for the given secret and time (RFC 6238)
func GenerateCode(secret string, t time.Time) (string, error) {
	key, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(strings.ToUpper(secret))
	if err != nil {
		return "", err
	}

	counter := uint64(t.Unix()) / TimeStep
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, counter)

	mac := hmac.New(sha1.New, key)
	mac.Write(buf)
	sum := mac.Sum(nil)

	offset := sum[len(sum)-1] & 0x0f
	code := binary.BigEndian.Uint32(sum[offset:offset+4]) & 0x7fffffff
	code = code % 1000000

	return fmt.Sprintf("%06d", code), nil
}

// ValidateCode checks if the code is valid for the secret (allows ±1 time step)
func ValidateCode(secret, code string) bool {
	if len(code) != CodeDigits {
		return false
	}
	now := time.Now()
	for i := -Window; i <= Window; i++ {
		t := now.Add(time.Duration(i*TimeStep) * time.Second)
		expected, err := GenerateCode(secret, t)
		if err != nil {
			continue
		}
		if hmac.Equal([]byte(expected), []byte(code)) {
			return true
		}
	}
	return false
}

// BuildOTPAuthURI creates an otpauth:// URI for authenticator apps
func BuildOTPAuthURI(secret, username, issuer string) string {
	label := url.PathEscape(fmt.Sprintf("%s:%s", issuer, username))
	q := url.Values{}
	q.Set("secret", secret)
	q.Set("issuer", issuer)
	q.Set("algorithm", "SHA1")
	q.Set("digits", "6")
	q.Set("period", "30")
	return fmt.Sprintf("otpauth://totp/%s?%s", label, q.Encode())
}
