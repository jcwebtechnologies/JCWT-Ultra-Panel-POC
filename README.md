# JCWT Ultra Panel

A lightweight, IPv6-native web hosting control panel designed for ARM64 EC2 environments. A production-ready alternative to CloudPanel/cPanel that works perfectly on IPv6-only infrastructure.

## Features

- **Multi-PHP Version Manager** — PHP 8.2, 8.3, 8.4, 8.5 with per-site pool configs
- **VHost Management** — Create, edit, delete Nginx vhosts with isolated system users
- **Database Management** — MariaDB database and user CRUD with IPv6 bindings
- **SSL Management** — Self-signed generation and custom certificate upload
- **Cron Job Manager** — Per-site scheduled tasks with enable/disable toggle
- **File Manager** — Upload, download, edit files scoped to each site's web root
- **PHP Settings UI** — Visual editor for memory_limit, max_execution_time, custom directives
- **Panel Settings** — Branding, logos, colors, session timeout, custom footer
- **Dark/Light Theme** — Modern glassmorphism UI with responsive design
- **Secure by Default** — bcrypt auth, CSRF protection, rate limiting, isolated users

## Architecture

```
Go Backend (single binary, ~10MB)
├── SQLite (panel metadata)
├── Embedded SPA frontend (HTML/CSS/JS)
├── System modules (nginx, php-fpm, mariadb, ssl, cron)
└── REST API with session auth
```

## Quick Start

### On a fresh Ubuntu 24.04 ARM64 EC2 instance:

```bash
# Clone or upload the project
git clone <repo-url> /opt/jcwt-panel
cd /opt/jcwt-panel

# Run the installer
sudo bash installer/install.sh
```

### Build from source (requires Go 1.22+):

```bash
CGO_ENABLED=1 GOOS=linux GOARCH=arm64 go build -o jcwt-panel ./cmd/jcwt-panel/
```

## Target Environment

| Component | Specification |
|---|---|
| Instance | AWS EC2 t4g.small (ARM64) |
| OS | Ubuntu 24.04 LTS ARM64 |
| Network | IPv6-only public subnet |
| Storage | 8GB gp3 EBS |
| RAM | 2GB |

## Default Credentials

- **URL**: `https://[your-ipv6]:8443`
- **Username**: `admin`
- **Password**: `admin`

> ⚠️ Change the default password immediately after first login!

## Project Structure

```
├── cmd/jcwt-panel/       # Go entry point + embedded web assets
│   ├── main.go
│   └── web/              # Frontend SPA
├── internal/
│   ├── auth/             # Session, CSRF, bcrypt auth
│   ├── config/           # Configuration
│   ├── db/               # SQLite schema + queries
│   ├── handlers/         # REST API handlers
│   ├── models/           # Data structures
│   ├── nginx/            # Nginx config generator
│   ├── php/              # PHP-FPM pool generator
│   ├── router/           # HTTP router
│   └── system/           # OS-level operations
├── installer/            # One-command installer
├── configs/              # Systemd service
└── README.md
```

## License

Private / Internal Use
