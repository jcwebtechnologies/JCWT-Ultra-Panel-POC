package system

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// GenerateSelfSignedCert generates a self-signed SSL certificate for a domain
func GenerateSelfSignedCert(sslBaseDir, domain string) (certPath, keyPath string, err error) {
	certDir := filepath.Join(sslBaseDir, domain)

	cmd := exec.Command("sudo", "mkdir", "-p", certDir)
	if output, err := cmd.CombinedOutput(); err != nil {
		return "", "", fmt.Errorf("create ssl dir: %s: %s", err, string(output))
	}

	certPath = filepath.Join(certDir, "cert.pem")
	keyPath = filepath.Join(certDir, "key.pem")

	cmd = exec.Command("sudo", "openssl", "req",
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

	exec.Command("sudo", "chmod", "0600", keyPath).Run()
	exec.Command("sudo", "chmod", "0644", certPath).Run()

	return certPath, keyPath, nil
}

// SaveCustomCert saves uploaded certificate and key files
func SaveCustomCert(sslBaseDir, domain string, certData, keyData []byte) (certPath, keyPath string, err error) {
	certDir := filepath.Join(sslBaseDir, domain)

	// Use sudo to create directory since panel user may not have permission
	cmd := exec.Command("sudo", "mkdir", "-p", certDir)
	if output, err := cmd.CombinedOutput(); err != nil {
		return "", "", fmt.Errorf("create ssl dir: %s: %s", err, string(output))
	}

	certPath = filepath.Join(certDir, "cert.pem")
	keyPath = filepath.Join(certDir, "key.pem")

	// Write cert via sudo tee
	cmd = exec.Command("sudo", "tee", certPath)
	cmd.Stdin = strings.NewReader(string(certData))
	cmd.Stdout = nil
	if output, err := cmd.CombinedOutput(); err != nil {
		return "", "", fmt.Errorf("write cert: %s: %s", err, string(output))
	}

	// Write key via sudo tee
	cmd = exec.Command("sudo", "tee", keyPath)
	cmd.Stdin = strings.NewReader(string(keyData))
	cmd.Stdout = nil
	if output, err := cmd.CombinedOutput(); err != nil {
		return "", "", fmt.Errorf("write key: %s: %s", err, string(output))
	}

	// Set permissions
	exec.Command("sudo", "chmod", "0644", certPath).Run()
	exec.Command("sudo", "chmod", "0600", keyPath).Run()

	return certPath, keyPath, nil
}

// RemoveCert removes SSL certificate files for a domain
func RemoveCert(sslBaseDir, domain string) error {
	certDir := filepath.Join(sslBaseDir, domain)
	cmd := exec.Command("sudo", "rm", "-rf", certDir)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("remove certs: %s: %s", err, string(output))
	}
	return nil
}

// GenerateRandomPassword generates a random password
func GenerateRandomPassword(length int) string {
	b := make([]byte, length)
	rand.Read(b)
	return hex.EncodeToString(b)[:length]
}
