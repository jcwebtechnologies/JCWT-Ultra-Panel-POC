package system

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// GenerateSelfSignedCert generates a self-signed SSL certificate for a domain
func GenerateSelfSignedCert(sslBaseDir, domain string) (certPath, keyPath string, err error) {
	certDir := filepath.Join(sslBaseDir, domain)
	if err := os.MkdirAll(certDir, 0700); err != nil {
		return "", "", fmt.Errorf("create ssl dir: %w", err)
	}

	certPath = filepath.Join(certDir, "cert.pem")
	keyPath = filepath.Join(certDir, "key.pem")

	cmd := exec.Command("openssl", "req",
		"-x509",
		"-nodes",
		"-days", "365",
		"-newkey", "rsa:2048",
		"-keyout", keyPath,
		"-out", certPath,
		"-subj", fmt.Sprintf("/CN=%s/O=JCWT Ultra Panel/C=US", domain),
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", "", fmt.Errorf("generate cert: %s: %s", err, string(output))
	}

	os.Chmod(keyPath, 0600)
	os.Chmod(certPath, 0644)

	return certPath, keyPath, nil
}

// SaveCustomCert saves uploaded certificate and key files
func SaveCustomCert(sslBaseDir, domain string, certData, keyData []byte) (certPath, keyPath string, err error) {
	certDir := filepath.Join(sslBaseDir, domain)
	if err := os.MkdirAll(certDir, 0700); err != nil {
		return "", "", fmt.Errorf("create ssl dir: %w", err)
	}

	certPath = filepath.Join(certDir, "cert.pem")
	keyPath = filepath.Join(certDir, "key.pem")

	if err := os.WriteFile(certPath, certData, 0644); err != nil {
		return "", "", fmt.Errorf("write cert: %w", err)
	}
	if err := os.WriteFile(keyPath, keyData, 0600); err != nil {
		return "", "", fmt.Errorf("write key: %w", err)
	}

	return certPath, keyPath, nil
}

// RemoveCert removes SSL certificate files for a domain
func RemoveCert(sslBaseDir, domain string) error {
	certDir := filepath.Join(sslBaseDir, domain)
	return os.RemoveAll(certDir)
}

// GenerateRandomPassword generates a random password
func GenerateRandomPassword(length int) string {
	b := make([]byte, length)
	rand.Read(b)
	return hex.EncodeToString(b)[:length]
}
