package main

import (
	"crypto/rand"
	"embed"
	"encoding/hex"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/jcwt/ultra-panel/internal/auth"
	"github.com/jcwt/ultra-panel/internal/config"
	"github.com/jcwt/ultra-panel/internal/crypto"
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

	var allowHTTP bool
	flag.StringVar(&cfg.ListenAddr, "listen", cfg.ListenAddr, "Listen address (default [::]:8443)")
	flag.StringVar(&cfg.DataDir, "data-dir", cfg.DataDir, "Data directory")
	flag.BoolVar(&allowHTTP, "allow-http", false, "Allow plaintext HTTP (dev only)")
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

	// Initialize encryption for secrets at rest
	if err := crypto.Init(cfg.DataDir); err != nil {
		log.Fatalf("Failed to initialize encryption: %v", err)
	}

	// Bootstrap: if no admin exists, generate a one-time setup token
	count, _ := database.AdminCount()
	var setupToken string
	if count == 0 {
		tokenBytes := make([]byte, 24)
		if _, err := rand.Read(tokenBytes); err != nil {
			log.Fatalf("Failed to generate setup token: %v", err)
		}
		setupToken = hex.EncodeToString(tokenBytes)
		if err := database.SetSetupToken(setupToken); err != nil {
			log.Fatalf("Failed to store setup token: %v", err)
		}
		log.Println("╔════════════════════════════════════════════════════════════╗")
		log.Println("║           FIRST-TIME SETUP REQUIRED                       ║")
		log.Println("╠════════════════════════════════════════════════════════════╣")
		log.Printf("║  Setup Token: %s  ║", setupToken)
		log.Println("║                                                            ║")
		log.Println("║  Open the panel in your browser and create your admin      ║")
		log.Println("║  account. This token can only be used once.                ║")
		log.Println("╚════════════════════════════════════════════════════════════╝")
	}

	// Initialize auth manager with session timeout from DB (default 30 minutes)
	sessionTimeout := 30
	if settings, err := database.GetPanelSettings(); err == nil {
		if t, ok := settings["session_timeout"].(int); ok && t >= 5 {
			sessionTimeout = t
		}
	}
	authMgr := auth.NewManager(sessionTimeout)

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

	srv := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      120 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20, // 1MB
	}

	if _, err := os.Stat(certFile); os.IsNotExist(err) {
		if !allowHTTP {
			log.Fatalf("No TLS certificate found at %s. Use --allow-http flag to run without TLS (dev only).", certFile)
		}
		log.Printf("WARNING: Running in plaintext HTTP mode (--allow-http). Do NOT use in production.")
		log.Printf("Panel URL: http://[server-ip]:%s", cfg.PanelPort)

		if err := srv.ListenAndServe(); err != nil {
			log.Fatalf("Server failed: %v", err)
		}
	} else {
		log.Printf("TLS enabled")
		log.Printf("Panel URL: https://[server-ip]:%s", cfg.PanelPort)

		if err := srv.ListenAndServeTLS(certFile, keyFile); err != nil {
			log.Fatalf("Server failed: %v", err)
		}
	}

	fmt.Println("Server stopped")
}
