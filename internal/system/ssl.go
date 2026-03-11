package system

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// GenerateSelfSignedCert creates a self-signed TLS certificate for domain,
// storing it under sslBaseDir/{domain}/. Returns the cert and key paths.
func GenerateSelfSignedCert(sslBaseDir, domain string) (certPath, keyPath string, err error) {
	dir := filepath.Join(sslBaseDir, domain)
	if err = os.MkdirAll(dir, 0755); err != nil {
		return "", "", fmt.Errorf("create ssl dir: %w", err)
	}
	certPath = filepath.Join(dir, "cert.pem")
	keyPath = filepath.Join(dir, "key.pem")

	out, cmdErr := exec.Command("sudo", "openssl", "req", "-x509", "-nodes",
		"-days", "3650",
		"-newkey", "rsa:2048",
		"-keyout", keyPath,
		"-out", certPath,
		"-subj", fmt.Sprintf("/CN=%s", domain),
	).CombinedOutput()
	if cmdErr != nil {
		return "", "", fmt.Errorf("openssl: %s", strings.TrimSpace(string(out)))
	}
	return certPath, keyPath, nil
}

// SaveCustomCert writes user-supplied certificate and private key bytes under
// sslBaseDir/{domain}/. Returns the file paths.
func SaveCustomCert(sslBaseDir, domain string, certData, keyData []byte) (certPath, keyPath string, err error) {
	dir := filepath.Join(sslBaseDir, domain)
	if err = os.MkdirAll(dir, 0755); err != nil {
		return "", "", fmt.Errorf("create ssl dir: %w", err)
	}
	certPath = filepath.Join(dir, "cert.pem")
	keyPath = filepath.Join(dir, "key.pem")

	if err = os.WriteFile(certPath, certData, 0644); err != nil {
		return "", "", fmt.Errorf("write cert: %w", err)
	}
	if err = os.WriteFile(keyPath, keyData, 0600); err != nil {
		return "", "", fmt.Errorf("write key: %w", err)
	}
	return certPath, keyPath, nil
}

// ObtainLetsEncryptCert runs certbot to obtain a Let's Encrypt certificate
// for the given domains using the webroot HTTP-01 challenge.
// Returns the fullchain and privkey paths under /etc/letsencrypt/live/.
func ObtainLetsEncryptCert(sslBaseDir, webRoot string, domains []string) (certPath, keyPath string, err error) {
	if len(domains) == 0 {
		return "", "", fmt.Errorf("no domains specified")
	}

	args := []string{
		"certbot", "certonly", "--webroot",
		"-w", webRoot,
		"--agree-tos", "--non-interactive", "--quiet",
		"--email", "admin@" + domains[0],
	}
	for _, d := range domains {
		args = append(args, "-d", d)
	}

	out, cmdErr := exec.Command("sudo", args...).CombinedOutput()
	if cmdErr != nil {
		return "", "", fmt.Errorf("certbot: %s", strings.TrimSpace(string(out)))
	}

	// Certbot places certs at /etc/letsencrypt/live/{primaryDomain}/
	lePath := filepath.Join("/etc/letsencrypt/live", domains[0])
	return filepath.Join(lePath, "fullchain.pem"), filepath.Join(lePath, "privkey.pem"), nil
}

// RemoveCert deletes the SSL certificate directory for domain under sslBaseDir.
func RemoveCert(sslBaseDir, domain string) {
	os.RemoveAll(filepath.Join(sslBaseDir, domain))
}
