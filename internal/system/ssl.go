package system

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// uniqueCertSuffix returns a short random hex string for unique cert filenames.
func uniqueCertSuffix() string {
	b := make([]byte, 4)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// GenerateSelfSignedCert generates a self-signed SSL certificate for a domain
func GenerateSelfSignedCert(sslBaseDir, domain string) (certPath, keyPath string, err error) {
	certDir := filepath.Join(sslBaseDir, domain)

	cmd := exec.Command("sudo", "mkdir", "-p", certDir)
	if output, err := cmd.CombinedOutput(); err != nil {
		return "", "", fmt.Errorf("create ssl dir: %s: %s", err, string(output))
	}

	suffix := uniqueCertSuffix()
	certPath = filepath.Join(certDir, fmt.Sprintf("cert_%s.pem", suffix))
	keyPath = filepath.Join(certDir, fmt.Sprintf("key_%s.pem", suffix))

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

	suffix := uniqueCertSuffix()
	certPath = filepath.Join(certDir, fmt.Sprintf("cert_%s.pem", suffix))
	keyPath = filepath.Join(certDir, fmt.Sprintf("key_%s.pem", suffix))

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

// ObtainLetsEncryptCert obtains a Let's Encrypt certificate via certbot webroot challenge.
// domains should include the main domain and any aliases to include in the SAN.
func ObtainLetsEncryptCert(sslBaseDir, webRoot string, domains []string) (certPath, keyPath string, err error) {
	if len(domains) == 0 {
		return "", "", fmt.Errorf("at least one domain is required")
	}

	mainDomain := domains[0]
	certDir := filepath.Join(sslBaseDir, mainDomain)

	mkdirCmd := exec.Command("sudo", "mkdir", "-p", certDir)
	if output, err := mkdirCmd.CombinedOutput(); err != nil {
		return "", "", fmt.Errorf("create ssl dir: %s: %s", err, string(output))
	}

	// Build certbot args
	args := []string{
		"certbot", "certonly",
		"--webroot",
		"-w", webRoot,
		"--agree-tos",
		"--non-interactive",
		"--register-unsafely-without-email",
		"--cert-name", mainDomain,
	}
	for _, d := range domains {
		args = append(args, "-d", d)
	}

	cmd := exec.Command("sudo", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", "", fmt.Errorf("certbot failed: %s: %s", err, string(output))
	}

	// Certbot stores certs in /etc/letsencrypt/live/<domain>/
	leCertPath := fmt.Sprintf("/etc/letsencrypt/live/%s/fullchain.pem", mainDomain)
	leKeyPath := fmt.Sprintf("/etc/letsencrypt/live/%s/privkey.pem", mainDomain)

	// Copy to our SSL dir so we have a consistent path and can manage permissions
	suffix := uniqueCertSuffix()
	certPath = filepath.Join(certDir, fmt.Sprintf("cert_%s.pem", suffix))
	keyPath = filepath.Join(certDir, fmt.Sprintf("key_%s.pem", suffix))

	cpCert := exec.Command("sudo", "cp", leCertPath, certPath)
	if out, err := cpCert.CombinedOutput(); err != nil {
		return "", "", fmt.Errorf("copy cert: %s: %s", err, string(out))
	}
	cpKey := exec.Command("sudo", "cp", leKeyPath, keyPath)
	if out, err := cpKey.CombinedOutput(); err != nil {
		return "", "", fmt.Errorf("copy key: %s: %s", err, string(out))
	}

	exec.Command("sudo", "chmod", "0644", certPath).Run()
	exec.Command("sudo", "chmod", "0600", keyPath).Run()

	return certPath, keyPath, nil
}

// GenerateRandomPassword generates a random password
func GenerateRandomPassword(length int) string {
	b := make([]byte, length)
	rand.Read(b)
	return hex.EncodeToString(b)[:length]
}
