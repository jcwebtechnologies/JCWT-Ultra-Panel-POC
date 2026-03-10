package main

import (
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"

	"github.com/jcwt/ultra-panel/internal/auth"
	"github.com/jcwt/ultra-panel/internal/config"
	"github.com/jcwt/ultra-panel/internal/db"
	"github.com/jcwt/ultra-panel/internal/router"
)

const Version = "1.0.0"

//go:embed all:web
var webEmbed embed.FS

func main() {
	// Redirect all log output to stdout so systemd's StandardOutput captures it in panel.log.
	// By default Go's log package writes to stderr, which would end up in panel-error.log only.
	log.SetOutput(os.Stdout)

	cfg := config.DefaultConfig()

	flag.StringVar(&cfg.ListenAddr, "listen", cfg.ListenAddr, "Listen address (default [::]:8443)")
	flag.StringVar(&cfg.DataDir, "data-dir", cfg.DataDir, "Data directory")
	flag.Parse()

	// Override from environment
	if v := os.Getenv("JCWT_LISTEN"); v != "" {
		cfg.ListenAddr = v
	}
	if v := os.Getenv("JCWT_DATA_DIR"); v != "" {
		cfg.DataDir = v
	}

	log.Printf("JCWT Ultra Panel starting...")
	log.Printf("Data directory: %s", cfg.DataDir)
	log.Printf("Listen address: %s", cfg.ListenAddr)

	// Initialize database
	database, err := db.Open(cfg.DataDir)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer database.Close()

	// Ensure at least one admin user exists
	count, _ := database.AdminCount()
	if count == 0 {
		password := "admin" // Default password, changed on first login
		hash, _ := auth.HashPassword(password)
		if err := database.CreateAdmin("admin", hash); err != nil {
			log.Fatalf("Failed to create default admin: %v", err)
		}
		log.Printf("Created default admin user: admin / admin")
		log.Printf("⚠️  CHANGE THE DEFAULT PASSWORD IMMEDIATELY!")
	}

	// Initialize auth manager
	authMgr := auth.NewManager(30) // 30-minute session timeout

	// Setup embedded web filesystem
	webFS, err := fs.Sub(webEmbed, "web")
	if err != nil {
		log.Fatalf("Failed to load web assets: %v", err)
	}

	// Setup router
	handler := router.Setup(database, cfg, authMgr, http.FS(webFS), Version)

	// Check for TLS certificates
	certFile := cfg.TLSCert
	keyFile := cfg.TLSKey

	if _, err := os.Stat(certFile); os.IsNotExist(err) {
		// No TLS certs - run in HTTP mode (for development or behind reverse proxy)
		log.Printf("No TLS certificate found at %s", certFile)
		log.Printf("Running in HTTP mode on %s", cfg.ListenAddr)
		log.Printf("Panel URL: http://[server-ip]:%s", cfg.PanelPort)

		if err := http.ListenAndServe(cfg.ListenAddr, handler); err != nil {
			log.Fatalf("Server failed: %v", err)
		}
	} else {
		// TLS mode
		log.Printf("TLS enabled")
		log.Printf("Panel URL: https://[server-ip]:%s", cfg.PanelPort)

		if err := http.ListenAndServeTLS(cfg.ListenAddr, certFile, keyFile, handler); err != nil {
			log.Fatalf("Server failed: %v", err)
		}
	}

	fmt.Println("Server stopped")
}
