package system

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// EnableSSH changes the user's shell from /usr/sbin/nologin to /bin/bash
func EnableSSH(username string) error {
	if !safeDomainRegex.MatchString(username) {
		return fmt.Errorf("invalid username")
	}
	cmd := exec.Command("sudo", "usermod", "--shell", "/bin/bash", username)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("enable SSH: %s", strings.TrimSpace(string(output)))
	}
	return nil
}

// DisableSSH changes the user's shell back to /usr/sbin/nologin
func DisableSSH(username string) error {
	if !safeDomainRegex.MatchString(username) {
		return fmt.Errorf("invalid username")
	}
	cmd := exec.Command("sudo", "usermod", "--shell", "/usr/sbin/nologin", username)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("disable SSH: %s", strings.TrimSpace(string(output)))
	}
	return nil
}

// IsSSHEnabled checks if the user has a login shell (not nologin)
func IsSSHEnabled(username string) bool {
	if !safeDomainRegex.MatchString(username) {
		return false
	}
	out, err := exec.Command("getent", "passwd", username).Output()
	if err != nil {
		return false
	}
	fields := strings.Split(strings.TrimSpace(string(out)), ":")
	if len(fields) < 7 {
		return false
	}
	shell := fields[6]
	return shell != "/usr/sbin/nologin" && shell != "/bin/false"
}

// GenerateSSHKeyPair generates an SSH key pair and returns (publicKey, privateKey, fingerprint, error)
func GenerateSSHKeyPair(keyType string, bits int, passphrase string) (string, string, string, error) {
	tmpDir, err := exec.Command("mktemp", "-d").Output()
	if err != nil {
		return "", "", "", fmt.Errorf("create temp dir: %v", err)
	}
	dir := strings.TrimSpace(string(tmpDir))
	defer exec.Command("rm", "-rf", dir).Run()

	keyPath := filepath.Join(dir, "key")

	var args []string
	switch keyType {
	case "rsa":
		args = []string{"-t", "rsa", "-b", fmt.Sprintf("%d", bits), "-f", keyPath, "-N", passphrase, "-C", "jcwt-panel-generated"}
	case "ed25519":
		args = []string{"-t", "ed25519", "-f", keyPath, "-N", passphrase, "-C", "jcwt-panel-generated"}
	default:
		return "", "", "", fmt.Errorf("unsupported key type: %s", keyType)
	}

	cmd := exec.Command("ssh-keygen", args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		return "", "", "", fmt.Errorf("ssh-keygen: %s", strings.TrimSpace(string(output)))
	}

	privKey, err := exec.Command("cat", keyPath).Output()
	if err != nil {
		return "", "", "", fmt.Errorf("read private key: %v", err)
	}

	pubKey, err := exec.Command("cat", keyPath+".pub").Output()
	if err != nil {
		return "", "", "", fmt.Errorf("read public key: %v", err)
	}

	fpOut, err := exec.Command("ssh-keygen", "-lf", keyPath+".pub").Output()
	if err != nil {
		return "", "", "", fmt.Errorf("fingerprint: %v", err)
	}
	fingerprint := strings.TrimSpace(string(fpOut))

	return strings.TrimSpace(string(pubKey)), strings.TrimSpace(string(privKey)), fingerprint, nil
}

// GetSSHFingerprint returns the fingerprint of a public key string
func GetSSHFingerprint(pubKey string) (string, error) {
	tmpDir, err := exec.Command("mktemp", "-d").Output()
	if err != nil {
		return "", fmt.Errorf("create temp dir: %v", err)
	}
	dir := strings.TrimSpace(string(tmpDir))
	defer exec.Command("rm", "-rf", dir).Run()

	keyFile := filepath.Join(dir, "key.pub")
	cmd := exec.Command("bash", "-c", fmt.Sprintf("cat > %s", keyFile))
	cmd.Stdin = strings.NewReader(pubKey)
	if output, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("write key: %s", string(output))
	}

	fpOut, err := exec.Command("ssh-keygen", "-lf", keyFile).Output()
	if err != nil {
		return "", fmt.Errorf("fingerprint: %v", err)
	}
	return strings.TrimSpace(string(fpOut)), nil
}

// EnsureSSHDir creates the .ssh directory for a user if it doesn't exist
func EnsureSSHDir(username, homeDir string) error {
	if !safeDomainRegex.MatchString(username) {
		return fmt.Errorf("invalid username")
	}
	sshDir := filepath.Join(homeDir, ".ssh")
	exec.Command("sudo", "mkdir", "-p", sshDir).Run()
	exec.Command("sudo", "chmod", "700", sshDir).Run()
	exec.Command("sudo", "chown", username+":"+username, sshDir).Run()
	return nil
}

// SyncAuthorizedKeys writes all authorized public keys to the user's ~/.ssh/authorized_keys
func SyncAuthorizedKeys(username, homeDir string, publicKeys []string) error {
	if !safeDomainRegex.MatchString(username) {
		return fmt.Errorf("invalid username")
	}

	if err := EnsureSSHDir(username, homeDir); err != nil {
		return err
	}

	sshDir := filepath.Join(homeDir, ".ssh")
	authKeysPath := filepath.Join(sshDir, "authorized_keys")
	content := strings.Join(publicKeys, "\n")
	if len(publicKeys) > 0 {
		content += "\n"
	}

	cmd := exec.Command("sudo", "tee", authKeysPath)
	cmd.Stdin = strings.NewReader(content)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("write authorized_keys: %s", strings.TrimSpace(string(output)))
	}

	exec.Command("sudo", "chmod", "600", authKeysPath).Run()
	exec.Command("sudo", "chown", username+":"+username, authKeysPath).Run()

	return nil
}
