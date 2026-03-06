package models

import "time"

// AdminUser represents a panel admin
type AdminUser struct {
	ID           int64     `json:"id"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"created_at"`
}

// Site represents a hosted website
type Site struct {
	ID          int64     `json:"id"`
	Domain      string    `json:"domain"`
	Aliases     string    `json:"aliases"`
	SystemUser  string    `json:"system_user"`
	PHPVersion  string    `json:"php_version"`
	WebRoot     string    `json:"web_root"`
	SSLType     string    `json:"ssl_type"`
	SSLCertPath string    `json:"ssl_cert_path"`
	SSLKeyPath  string    `json:"ssl_key_path"`
	CreatedAt   time.Time `json:"created_at"`
}

// PHPSettings stores per-site PHP pool configuration
type PHPSettings struct {
	ID                int64  `json:"id"`
	SiteID            int64  `json:"site_id"`
	MemoryLimit       string `json:"memory_limit"`
	MaxExecutionTime  int    `json:"max_execution_time"`
	MaxInputTime      int    `json:"max_input_time"`
	MaxInputVars      int    `json:"max_input_vars"`
	PostMaxSize       string `json:"post_max_size"`
	UploadMaxFilesize string `json:"upload_max_filesize"`
	CustomDirectives  string `json:"custom_directives"`
}

// Database represents a MariaDB database managed by the panel
type Database struct {
	ID        int64     `json:"id"`
	SiteID    *int64    `json:"site_id"`
	DBName    string    `json:"db_name"`
	CreatedAt time.Time `json:"created_at"`
}

// DBUser represents a MariaDB user
type DBUser struct {
	ID         int64     `json:"id"`
	Username   string    `json:"username"`
	DatabaseID int64     `json:"database_id"`
	CreatedAt  time.Time `json:"created_at"`
}

// CronJob represents a cron entry for a site
type CronJob struct {
	ID        int64     `json:"id"`
	SiteID    int64     `json:"site_id"`
	Schedule  string    `json:"schedule"`
	Command   string    `json:"command"`
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
}

// PanelSettings stores branding and panel configuration
type PanelSettings struct {
	ID             int64  `json:"id"`
	PanelName      string `json:"panel_name"`
	PanelTagline   string `json:"panel_tagline"`
	LogoURL        string `json:"logo_url"`
	FaviconURL     string `json:"favicon_url"`
	PrimaryColor   string `json:"primary_color"`
	AccentColor    string `json:"accent_color"`
	FooterText     string `json:"footer_text"`
	SessionTimeout int    `json:"session_timeout"`
}

// FileEntry represents a file or directory in the file manager
type FileEntry struct {
	Name     string    `json:"name"`
	Path     string    `json:"path"`
	IsDir    bool      `json:"is_dir"`
	Size     int64     `json:"size"`
	Modified time.Time `json:"modified"`
	Perms    string    `json:"perms"`
}

// DashboardStats for the dashboard overview
type DashboardStats struct {
	TotalSites     int     `json:"total_sites"`
	TotalDatabases int     `json:"total_databases"`
	TotalCronJobs  int     `json:"total_cron_jobs"`
	DiskUsedPct    float64 `json:"disk_used_pct"`
	MemoryUsedPct  float64 `json:"memory_used_pct"`
	Uptime         string  `json:"uptime"`
	PHPVersions    []string `json:"php_versions"`
}

// APIResponse is a standard JSON response wrapper
type APIResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}
