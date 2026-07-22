package system

import (
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

// TestSFTPConnection tests SSH/SFTP connectivity and remote directory write access
func TestSFTPConnection(host string, port int, user, authType, password, privateKey, passphrase, remotePath string) error {
	client, err := dialSSH(host, port, user, authType, password, privateKey, passphrase)
	if err != nil {
		return err
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("create SSH session: %w", err)
	}
	defer session.Close()

	if remotePath == "" {
		remotePath = "/backups/jcwt-panel"
	}

	// Ensure remote directory exists and is writable
	probePath := filepath.ToSlash(filepath.Join(remotePath, ".jcwt_probe"))
	remoteDir := filepath.ToSlash(remotePath)
	cmd := fmt.Sprintf("mkdir -p %s && touch %s && rm -f %s",
		shellQuote(remoteDir), shellQuote(probePath), shellQuote(probePath))

	out, err := session.CombinedOutput(cmd)
	if err != nil {
		return fmt.Errorf("remote directory test failed (%s): %s", err, strings.TrimSpace(string(out)))
	}

	return nil
}

// UploadViaSFTP transfers a local file to a remote path over SFTP/SSH
func UploadViaSFTP(host string, port int, user, authType, password, privateKey, passphrase, remotePath, localFilePath string) error {
	client, err := dialSSH(host, port, user, authType, password, privateKey, passphrase)
	if err != nil {
		return err
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("create SSH session: %w", err)
	}
	defer session.Close()

	localFile, err := os.Open(localFilePath)
	if err != nil {
		return fmt.Errorf("open local file: %w", err)
	}
	defer localFile.Close()

	fileName := filepath.Base(localFilePath)
	if remotePath == "" {
		remotePath = "/backups/jcwt-panel"
	}
	remoteDir := filepath.ToSlash(remotePath)
	destPath := filepath.ToSlash(filepath.Join(remoteDir, fileName))

	cmd := fmt.Sprintf("mkdir -p %s && cat > %s", shellQuote(remoteDir), shellQuote(destPath))

	stdin, err := session.StdinPipe()
	if err != nil {
		return fmt.Errorf("session stdin: %w", err)
	}

	if err := session.Start(cmd); err != nil {
		return fmt.Errorf("start remote upload: %w", err)
	}

	if _, err := io.Copy(stdin, localFile); err != nil {
		stdin.Close()
		return fmt.Errorf("upload stream failed: %w", err)
	}
	stdin.Close()

	if err := session.Wait(); err != nil {
		return fmt.Errorf("remote save failed: %w", err)
	}

	return nil
}

func dialSSH(host string, port int, user, authType, password, privateKey, passphrase string) (*ssh.Client, error) {
	if host == "" {
		return nil, fmt.Errorf("SFTP host is required")
	}
	if port <= 0 {
		port = 22
	}
	if user == "" {
		return nil, fmt.Errorf("SFTP username is required")
	}

	var authMethods []ssh.AuthMethod

	if authType == "key" && privateKey != "" {
		var signer ssh.Signer
		var err error
		if passphrase != "" {
			signer, err = ssh.ParsePrivateKeyWithPassphrase([]byte(privateKey), []byte(passphrase))
		} else {
			signer, err = ssh.ParsePrivateKey([]byte(privateKey))
		}
		if err != nil {
			return nil, fmt.Errorf("parse SSH private key: %w", err)
		}
		authMethods = append(authMethods, ssh.PublicKeys(signer))
	} else if password != "" {
		authMethods = append(authMethods, ssh.Password(password))
	} else {
		return nil, fmt.Errorf("password or private key required for authentication")
	}

	config := &ssh.ClientConfig{
		User:            user,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         15 * time.Second,
	}

	addr := net.JoinHostPort(host, strconv.Itoa(port))
	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return nil, fmt.Errorf("connect to SFTP server %s: %w", addr, err)
	}

	return client, nil
}

func shellQuote(s string) string {
	if s == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}
