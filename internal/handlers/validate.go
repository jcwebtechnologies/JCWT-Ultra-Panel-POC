package handlers

import (
	"path/filepath"
	"strings"
)

// isValidDBName validates that a database name contains only safe characters.
func isValidDBName(name string) bool {
	return dbNameRegex.MatchString(name)
}

// isValidWebRoot validates a web root path is under /home/ and clean.
func isValidWebRoot(path string) bool {
	cleaned := filepath.Clean(path)
	if cleaned != path {
		return false
	}
	if !strings.HasPrefix(cleaned, "/home/") {
		return false
	}
	if strings.Contains(cleaned, "..") {
		return false
	}
	// Reject shell metacharacters
	for _, c := range cleaned {
		switch c {
		case ';', '&', '|', '$', '`', '(', ')', '{', '}', '<', '>', '\'', '"', '\\', '\n', '\r', '\t', ' ':
			return false
		}
	}
	return true
}
