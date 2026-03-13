package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"
)

const keyFileName = "encryption.key"
const encPrefix = "enc:"

var (
	derivedKey []byte
	keyOnce    sync.Once
	keyErr     error
)

// Init initialises the encryption key from the data directory.
// The key file is created automatically on first run.
func Init(dataDir string) error {
	keyOnce.Do(func() {
		keyPath := filepath.Join(dataDir, keyFileName)
		raw, err := os.ReadFile(keyPath)
		if errors.Is(err, os.ErrNotExist) {
			raw = make([]byte, 32)
			if _, err = io.ReadFull(rand.Reader, raw); err != nil {
				keyErr = err
				return
			}
			if err = os.WriteFile(keyPath, raw, 0600); err != nil {
				keyErr = err
				return
			}
		} else if err != nil {
			keyErr = err
			return
		}
		// Derive a fixed-length key via SHA-256 (handles any raw length gracefully)
		h := sha256.Sum256(raw)
		derivedKey = h[:]
	})
	return keyErr
}

// Encrypt encrypts plaintext with AES-256-GCM and returns a base64 string
// prefixed with "enc:" so callers can distinguish encrypted values.
func Encrypt(plaintext string) (string, error) {
	if len(derivedKey) == 0 {
		return "", errors.New("encryption not initialised")
	}
	block, err := aes.NewCipher(derivedKey)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return encPrefix + base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt decrypts a value produced by Encrypt. If the value does not have the
// "enc:" prefix it is returned as-is (backward compatibility with plaintext).
func Decrypt(ciphertext string) (string, error) {
	if len(ciphertext) <= len(encPrefix) || ciphertext[:len(encPrefix)] != encPrefix {
		// Not encrypted — return plaintext as-is (migration path)
		return ciphertext, nil
	}
	if len(derivedKey) == 0 {
		return "", errors.New("encryption not initialised")
	}
	data, err := base64.StdEncoding.DecodeString(ciphertext[len(encPrefix):])
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(derivedKey)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", errors.New("ciphertext too short")
	}
	plaintext, err := gcm.Open(nil, data[:nonceSize], data[nonceSize:], nil)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

// IsEncrypted reports whether a value looks like it was produced by Encrypt.
func IsEncrypted(s string) bool {
	return len(s) > len(encPrefix) && s[:len(encPrefix)] == encPrefix
}
