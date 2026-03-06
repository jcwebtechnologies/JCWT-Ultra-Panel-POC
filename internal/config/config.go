package config

import (
	"os"
	"path/filepath"
)

// Config holds all panel configuration
type Config struct {
	ListenAddr string
	DataDir    string
	TLSCert    string
	TLSKey     string

	// System paths
	NginxSitesAvailable string
	NginxSitesEnabled   string
	PHPFPMBaseDir       string
	SSLBaseDir          string
	WebRootBase         string

	// Panel settings
	PanelName string
	PanelPort string
}

// DefaultConfig returns config with sane defaults for production
func DefaultConfig() *Config {
	dataDir := getEnv("JCWT_DATA_DIR", "/var/lib/jcwt-panel")
	return &Config{
		ListenAddr:          getEnv("JCWT_LISTEN", "[::]:8443"),
		DataDir:             dataDir,
		TLSCert:             filepath.Join(dataDir, "tls", "panel.crt"),
		TLSKey:              filepath.Join(dataDir, "tls", "panel.key"),
		NginxSitesAvailable: "/etc/nginx/sites-available",
		NginxSitesEnabled:   "/etc/nginx/sites-enabled",
		PHPFPMBaseDir:       "/etc/php",
		SSLBaseDir:          filepath.Join(dataDir, "ssl"),
		WebRootBase:         "/home",
		PanelName:           "JCWT Ultra Panel",
		PanelPort:           "8443",
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
