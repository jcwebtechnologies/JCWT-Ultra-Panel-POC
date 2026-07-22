#!/usr/bin/env bash
# ==============================================================================
#  JCWT Ultra Panel — Automatic Update & Patching Script
# ==============================================================================
#  This script updates the panel binary, system helpers, and permissions
#  without wiping databases, SSL certificates, site files, or custom vhosts.
#
#  Usage:
#    sudo bash update.sh
#    or
#    sudo bash installer/update.sh
# ==============================================================================

set -euo pipefail

# ---- Styling Tokens ----
BOLD='\033[1m'
GREEN='\033[0;32m'
CYAN='\033[0;36m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
PURPLE='\033[0;35m'
BLUE='\033[0;34m'
NC='\033[0m'

# ---- Logging Helpers ----
log_info()   { echo -e "  ${BLUE}ℹ${NC}  $*"; }
log_ok()     { echo -e "  ${GREEN}✔${NC}  $*"; }
log_warn()   { echo -e "  ${YELLOW}⚠${NC}  $*"; }
log_error()  { echo -e "  ${RED}✖  $*${NC}"; }
log_detail() { echo -e "     ${CYAN}↳${NC} $*"; }

step_header() {
    echo ""
    echo -e "${BOLD}${BLUE}═══ $* ═══${NC}"
    echo ""
}

die() {
    log_error "$*"
    exit 1
}

# ---- Pre-flight Checks ----
if [ "$(id -u)" -ne 0 ]; then
    die "This update script must be run as root (e.g. sudo bash update.sh)"
fi

if [ -f /etc/os-release ]; then
    . /etc/os-release
    if [ "$ID" != "ubuntu" ] && [ "$ID" != "debian" ]; then
        die "Unsupported operating system ($NAME). JCWT Ultra Panel requires Ubuntu or Debian."
    fi
fi

# Locate directories
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR=""
if [ -f "$SCRIPT_DIR/cmd/jcwt-panel/main.go" ]; then
    PROJECT_DIR="$SCRIPT_DIR"
elif [ -f "$SCRIPT_DIR/../cmd/jcwt-panel/main.go" ]; then
    PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
fi

DATA_DIR="/var/lib/jcwt-panel"
PANEL_BIN="/usr/local/bin/jcwt-panel"
PANEL_USER="jcwt-panel"
PANEL_PORT="8443"

# Clear terminal screen if interactive
if [ -t 1 ]; then
    clear || true
fi

echo -e "${BOLD}${PURPLE}"
echo "  ┌──────────────────────────────────────────────────────────┐"
echo "  │                                                          │"
echo "  │             JCWT ULTRA PANEL — UPDATER                   │"
echo "  │           Updating Panel Engine & System Services        │"
echo "  │                                                          │"
echo "  └──────────────────────────────────────────────────────────┘"
echo -e "${NC}"

# ---- Step 1: Build / Update Panel Binary ----
step_header "Updating Panel Binary"

export PATH=$PATH:/usr/local/go/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin

UPDATED=false

# Remove any stale local pre-built binary files so they don't override compilation
rm -f "$PROJECT_DIR/jcwt-panel" "./jcwt-panel" "$SCRIPT_DIR/jcwt-panel" "/tmp/jcwt-panel-new" 2>/dev/null || true

if [ -n "$PROJECT_DIR" ] && [ -f "$PROJECT_DIR/cmd/jcwt-panel/main.go" ]; then
    log_info "Source code found ($PROJECT_DIR/cmd/jcwt-panel/main.go) — building latest binary..."
    if ! command -v go >/dev/null 2>&1; then
        log_warn "Go compiler not found in PATH — installing Go..."
        case "$(uname -m)" in
            aarch64|arm64) GO_ARCH="arm64" ;;
            x86_64)        GO_ARCH="amd64" ;;
            *)             GO_ARCH="amd64" ;;
        esac
        GOVERSION="1.22.5"
        GO_URL="https://go.dev/dl/go${GOVERSION}.linux-${GO_ARCH}.tar.gz"
        wget -q --show-progress "$GO_URL" -O /tmp/go.tar.gz
        tar -C /usr/local -xzf /tmp/go.tar.gz
        export PATH=$PATH:/usr/local/go/bin
        rm -f /tmp/go.tar.gz
    fi

    cd "$PROJECT_DIR"
    go mod tidy 2>/dev/null || true
    if CGO_ENABLED=1 go build -o "$PANEL_BIN" ./cmd/jcwt-panel/; then
        UPDATED=true
        log_ok "Compiled and installed latest panel binary"
    else
        die "Compilation failed — see errors above"
    fi
elif [ -f "$PANEL_BIN" ]; then
    log_ok "Panel binary found at $PANEL_BIN (re-asserting permissions)"
    UPDATED=true
else
    die "No source code or Go binary found! Run updater from project source root."
fi

chmod +x "$PANEL_BIN"
chown root:root "$PANEL_BIN"

BIN_SIZE=$(du -h "$PANEL_BIN" | awk '{print $1}')
log_detail "Binary path: $PANEL_BIN ($BIN_SIZE)"

# ---- Step 2: Update System Helpers & Sudoers ----
step_header "Updating System Helpers & Sudoers"

log_info "Installing/updating privileged filesystem helper (/usr/local/sbin/panel-fsctl)..."
cat > /usr/local/sbin/panel-fsctl << 'FSCTL_EOF'
#!/bin/bash
# /usr/local/sbin/panel-fsctl
# Privileged filesystem operations helper for JCWT Ultra Panel.
set -euo pipefail

COMMAND="${1:-}"
shift || true

die() {
    echo "panel-fsctl: error: $*" >&2
    exit 1
}

validate_user() {
    local u="$1"
    [[ "$u" =~ ^[a-z][a-z0-9_]{2,15}$ ]] || die "invalid username: $u"
    id "$u" &>/dev/null || die "user does not exist: $u"
}

assert_under() {
    local path="$1" prefix="$2"
    local real
    real=$(realpath -m "$path" 2>/dev/null) || die "cannot resolve path: $path"
    [[ "$real" == "$prefix"* ]] || die "access denied: $path is outside allowed root $prefix"
}

safe_rm() {
    local target="$1"
    rm -rf --one-file-system --preserve-root=all "$target"
}

case "$COMMAND" in
    delete-home)
        USER="${1:-}"
        validate_user "$USER"
        HOME_DIR="/home/$USER"
        assert_under "$HOME_DIR" "/home/"
        pkill -u "$USER" -9 2>/dev/null || true
        userdel -f -r "$USER" 2>/dev/null || safe_rm "$HOME_DIR"
        groupdel "$USER" 2>/dev/null || true
        ;;

    delete-backup)
        USER="${1:-}"
        FILENAME="${2:-}"
        validate_user "$USER"
        HOME_DIR="/home/$USER"
        TARGET="$HOME_DIR/backups/$FILENAME"
        assert_under "$TARGET" "$HOME_DIR/backups/"
        [[ -f "$TARGET" ]] || exit 0
        safe_rm "$TARGET"
        ;;

    delete-staging)
        USER="${1:-}"
        RELPATH="${2:-}"
        validate_user "$USER"
        HOME_DIR="/home/$USER"
        TARGET="$HOME_DIR/$RELPATH"
        REAL=$(realpath -m "$TARGET" 2>/dev/null) || die "cannot resolve path: $TARGET"
        ALLOWED=false
        [[ "$REAL" == "$HOME_DIR/tmp/"* ]] && ALLOWED=true
        [[ "$REAL" == "$HOME_DIR/backups/staging-"* ]] && ALLOWED=true
        $ALLOWED || die "path $REAL not in an allowed staging area"
        [[ -e "$REAL" ]] || exit 0
        safe_rm "$REAL"
        ;;

    write-authkeys)
        USER="${1:-}"
        validate_user "$USER"
        HOME_DIR="/home/$USER"
        SSH_DIR="$HOME_DIR/.ssh"
        AUTH_KEYS="$SSH_DIR/authorized_keys"
        mkdir -p "$SSH_DIR"
        chmod 0700 "$SSH_DIR"
        chown "$USER:$USER" "$SSH_DIR"
        cat > "$AUTH_KEYS"
        chmod 0600 "$AUTH_KEYS"
        chown "$USER:$USER" "$AUTH_KEYS"
        ;;

    ensure-panel-dir)
        USER="${1:-}"
        WEB_ROOT="${2:-}"
        validate_user "$USER"
        HOME_DIR="/home/$USER"
        PANEL_DIR="$WEB_ROOT/.panel"
        assert_under "$PANEL_DIR" "$HOME_DIR" >/dev/null
        mkdir -p "$PANEL_DIR"
        chmod 0700 "$PANEL_DIR"
        chown "$USER:$USER" "$PANEL_DIR"
        ;;

    *)
        die "unknown command '${COMMAND}'. Valid: delete-home, delete-backup, delete-staging, write-authkeys, ensure-panel-dir"
        ;;
esac
FSCTL_EOF

chmod 0755 /usr/local/sbin/panel-fsctl
chown root:root /usr/local/sbin/panel-fsctl
log_ok "Filesystem helper updated"

# Configure iptables alternatives to ensure UFW operates correctly
update-alternatives --set iptables /usr/sbin/iptables-nft 2>/dev/null || update-alternatives --set iptables /sbin/iptables-legacy 2>/dev/null || true
update-alternatives --set ip6tables /usr/sbin/ip6tables-nft 2>/dev/null || update-alternatives --set ip6tables /sbin/ip6tables-legacy 2>/dev/null || true

log_info "Updating sudoers configuration (/etc/sudoers.d/jcwt-panel)..."
cat > /etc/sudoers.d/jcwt-panel << 'EOF'
# JCWT Ultra Panel - Scoped privileges for system management
Defaults:jcwt-panel secure_path="/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"

# User management
jcwt-panel ALL=(root) NOPASSWD: /usr/sbin/useradd -m -d /home/[a-z]* -s /bin/bash [a-z]*
jcwt-panel ALL=(root) NOPASSWD: /usr/sbin/useradd -m -d /home/[a-z]* -s /usr/sbin/nologin [a-z]*
jcwt-panel ALL=(root) NOPASSWD: /usr/sbin/usermod -s /bin/bash [a-z]*
jcwt-panel ALL=(root) NOPASSWD: /usr/sbin/usermod -s /usr/sbin/nologin [a-z]*
jcwt-panel ALL=(root) NOPASSWD: /usr/sbin/usermod -aG [a-z]* www-data
jcwt-panel ALL=(root) NOPASSWD: /usr/sbin/groupdel [a-z]*

# PHP CLI & WP-CLI execution (run as root or as site user via sudo -u)
jcwt-panel ALL=(ALL) NOPASSWD: /usr/bin/php*
jcwt-panel ALL=(ALL) NOPASSWD: /usr/bin/php
jcwt-panel ALL=(ALL) NOPASSWD: /usr/bin/zip *
jcwt-panel ALL=(ALL) NOPASSWD: /usr/bin/unzip *

# Systemd service control
jcwt-panel ALL=(root) NOPASSWD: /usr/bin/systemctl reload nginx
jcwt-panel ALL=(root) NOPASSWD: /usr/bin/systemctl reload php*-fpm
jcwt-panel ALL=(root) NOPASSWD: /usr/bin/systemctl restart nginx
jcwt-panel ALL=(root) NOPASSWD: /usr/bin/systemctl restart php*-fpm
jcwt-panel ALL=(root) NOPASSWD: /usr/bin/systemctl restart mariadb
jcwt-panel ALL=(root) NOPASSWD: /usr/bin/systemctl restart redis-server
jcwt-panel ALL=(root) NOPASSWD: /usr/bin/systemctl stop nginx
jcwt-panel ALL=(root) NOPASSWD: /usr/bin/systemctl stop php*-fpm
jcwt-panel ALL=(root) NOPASSWD: /usr/bin/systemctl stop mariadb
jcwt-panel ALL=(root) NOPASSWD: /usr/bin/systemctl stop redis-server
jcwt-panel ALL=(root) NOPASSWD: /usr/bin/systemctl start nginx
jcwt-panel ALL=(root) NOPASSWD: /usr/bin/systemctl start php*-fpm
jcwt-panel ALL=(root) NOPASSWD: /usr/bin/systemctl start mariadb
jcwt-panel ALL=(root) NOPASSWD: /usr/bin/systemctl start redis-server
jcwt-panel ALL=(root) NOPASSWD: /usr/bin/systemctl is-active nginx
jcwt-panel ALL=(root) NOPASSWD: /usr/bin/systemctl is-active php*-fpm
jcwt-panel ALL=(root) NOPASSWD: /usr/bin/systemctl is-active mariadb
jcwt-panel ALL=(root) NOPASSWD: /usr/bin/systemctl is-active redis-server
jcwt-panel ALL=(root) NOPASSWD: /usr/bin/systemctl is-active jcwt-panel
jcwt-panel ALL=(root) NOPASSWD: /usr/bin/systemctl is-active ufw
jcwt-panel ALL=(root) NOPASSWD: /usr/bin/systemctl show nginx --property=MemoryCurrent --value
jcwt-panel ALL=(root) NOPASSWD: /usr/bin/systemctl show nginx --property=ActiveEnterTimestamp --value
jcwt-panel ALL=(root) NOPASSWD: /usr/bin/systemctl show php*-fpm --property=MemoryCurrent --value
jcwt-panel ALL=(root) NOPASSWD: /usr/bin/systemctl show php*-fpm --property=ActiveEnterTimestamp --value
jcwt-panel ALL=(root) NOPASSWD: /usr/bin/systemctl show mariadb --property=MemoryCurrent --value
jcwt-panel ALL=(root) NOPASSWD: /usr/bin/systemctl show mariadb --property=ActiveEnterTimestamp --value
jcwt-panel ALL=(root) NOPASSWD: /usr/bin/systemctl show redis-server --property=MemoryCurrent --value
jcwt-panel ALL=(root) NOPASSWD: /usr/bin/systemctl show redis-server --property=ActiveEnterTimestamp --value
jcwt-panel ALL=(root) NOPASSWD: /usr/bin/systemctl show jcwt-panel --property=MemoryCurrent --value
jcwt-panel ALL=(root) NOPASSWD: /usr/bin/systemctl show jcwt-panel --property=ActiveEnterTimestamp --value

# Nginx config test
jcwt-panel ALL=(root) NOPASSWD: /usr/sbin/nginx -t

# MariaDB client
jcwt-panel ALL=(root) NOPASSWD: /usr/bin/mysql -e *
jcwt-panel ALL=(root) NOPASSWD: /usr/bin/mysql [a-zA-Z]*
jcwt-panel ALL=(root) NOPASSWD: /usr/bin/mysqldump --single-transaction [a-zA-Z]*

# Crontab management
jcwt-panel ALL=(root) NOPASSWD: /usr/bin/crontab -u [a-z]* -
jcwt-panel ALL=(root) NOPASSWD: /usr/bin/crontab -r -u [a-z]*

# SSL/TLS
jcwt-panel ALL=(root) NOPASSWD: /usr/bin/openssl req *
jcwt-panel ALL=(root) NOPASSWD: /usr/bin/openssl x509 *
jcwt-panel ALL=(root) NOPASSWD: /usr/bin/certbot certonly *
jcwt-panel ALL=(root) NOPASSWD: /usr/bin/certbot renew *

# File operations (scoped)
jcwt-panel ALL=(root) NOPASSWD: /usr/bin/chown [a-z]*\:[a-z]* /home/[a-z]*
jcwt-panel ALL=(root) NOPASSWD: /usr/bin/chown -R [a-z]*\:[a-z]* /home/[a-z]*
jcwt-panel ALL=(root) NOPASSWD: /usr/bin/chmod [0-9]* /home/[a-z]*
jcwt-panel ALL=(root) NOPASSWD: /usr/bin/chmod [0-9]* /etc/nginx/*
jcwt-panel ALL=(root) NOPASSWD: /usr/bin/chmod [0-9]* /etc/logrotate.d/*
jcwt-panel ALL=(root) NOPASSWD: /usr/bin/mkdir -p /home/[a-z]*
jcwt-panel ALL=(root) NOPASSWD: /usr/bin/mkdir -p /etc/nginx/*
jcwt-panel ALL=(root) NOPASSWD: /usr/bin/rm -f /etc/nginx/sites-available/*
jcwt-panel ALL=(root) NOPASSWD: /usr/bin/rm -f /etc/nginx/sites-enabled/*
jcwt-panel ALL=(root) NOPASSWD: /usr/bin/rm -f /etc/nginx/htpasswd/*
jcwt-panel ALL=(root) NOPASSWD: /usr/local/sbin/panel-fsctl *
jcwt-panel ALL=(root) NOPASSWD: /usr/bin/rm -f /var/lib/jcwt-panel/ssl/*
jcwt-panel ALL=(root) NOPASSWD: /usr/bin/rm -f /etc/logrotate.d/*
jcwt-panel ALL=(root) NOPASSWD: /usr/bin/rm -f /run/php/php*-fpm-*.sock
jcwt-panel ALL=(root) NOPASSWD: /usr/bin/rm -f /etc/php/*/fpm/pool.d/*.conf
jcwt-panel ALL=(root) NOPASSWD: /usr/bin/ln -sf /etc/nginx/sites-available/* /etc/nginx/sites-enabled/*
jcwt-panel ALL=(root) NOPASSWD: /usr/bin/tee /etc/nginx/sites-available/*
jcwt-panel ALL=(root) NOPASSWD: /usr/bin/tee /etc/php/*/fpm/pool.d/*.conf
jcwt-panel ALL=(root) NOPASSWD: /usr/bin/tee /etc/logrotate.d/*
jcwt-panel ALL=(root) NOPASSWD: /usr/bin/tee /etc/default/ufw
jcwt-panel ALL=(root) NOPASSWD: /usr/bin/tee /usr/share/phpmyadmin/signon_*.php
jcwt-panel ALL=(root) NOPASSWD: /usr/bin/tee /home/[a-z]*
jcwt-panel ALL=(root) NOPASSWD: /usr/bin/rm -f /usr/share/phpmyadmin/signon_*.php

# Tar/archive operations (scoped)
jcwt-panel ALL=(root) NOPASSWD: /usr/bin/tar -czf /home/[a-z]*
jcwt-panel ALL=(root) NOPASSWD: /usr/bin/tar -czf /home/[a-z]* -C /home/[a-z]* *
jcwt-panel ALL=(root) NOPASSWD: /usr/bin/tar -czf /home/[a-z]* -C /home/[a-z]* .
jcwt-panel ALL=(root) NOPASSWD: /usr/bin/tar -xzf /home/[a-z]*
jcwt-panel ALL=(root) NOPASSWD: /usr/bin/tar -xzf /tmp/* -C /home/[a-z]*
jcwt-panel ALL=(root) NOPASSWD: /usr/bin/tar -xzf /tmp/* -C /home/[a-z]*/tmp
jcwt-panel ALL=(root) NOPASSWD: /usr/bin/tar -tzf /home/[a-z]*
jcwt-panel ALL=(root) NOPASSWD: /usr/bin/tar cf - -C /home/[a-z]*
jcwt-panel ALL=(root) NOPASSWD: /usr/bin/tar xf - -C /home/[a-z]*

# Rsync (for backup restore & staging)
jcwt-panel ALL=(root) NOPASSWD: /usr/bin/rsync -a --delete /home/[a-z]*
jcwt-panel ALL=(root) NOPASSWD: /usr/bin/rsync -a --delete /home/[a-z]*/* /home/[a-z]*/*
jcwt-panel ALL=(root) NOPASSWD: /usr/bin/rsync -a --delete /home/[a-z]* /home/[a-z]*

# Disk usage (read-only)
jcwt-panel ALL=(root) NOPASSWD: /usr/bin/du -sh /home/[a-z]*
jcwt-panel ALL=(root) NOPASSWD: /usr/bin/du -b /home/[a-z]*
jcwt-panel ALL=(root) NOPASSWD: /usr/bin/du -sb /home/[a-z]*
jcwt-panel ALL=(root) NOPASSWD: /usr/bin/du -b --max-depth=3 /home/[a-z]*
jcwt-panel ALL=(root) NOPASSWD: /usr/bin/test -f /home/[a-z]*

# Log viewing
jcwt-panel ALL=(root) NOPASSWD: /usr/bin/tail -n [0-9]* /home/[a-z]*/logs/*
jcwt-panel ALL=(root) NOPASSWD: /usr/bin/cat /home/[a-z]*

# Timezone
jcwt-panel ALL=(root) NOPASSWD: /usr/bin/timedatectl set-timezone *

# htpasswd
jcwt-panel ALL=(root) NOPASSWD: /usr/bin/htpasswd -c -B -b /etc/nginx/htpasswd/*
jcwt-panel ALL=(root) NOPASSWD: /usr/bin/htpasswd -B -b /etc/nginx/htpasswd/*

# Firewall
jcwt-panel ALL=(root) NOPASSWD: /usr/sbin/ufw status
jcwt-panel ALL=(root) NOPASSWD: /usr/sbin/ufw status *
jcwt-panel ALL=(root) NOPASSWD: /usr/sbin/ufw allow *
jcwt-panel ALL=(root) NOPASSWD: /usr/sbin/ufw deny *
jcwt-panel ALL=(root) NOPASSWD: /usr/sbin/ufw delete *
jcwt-panel ALL=(root) NOPASSWD: /usr/sbin/ufw disable
jcwt-panel ALL=(root) NOPASSWD: /usr/sbin/ufw --force enable
jcwt-panel ALL=(root) NOPASSWD: /usr/sbin/ufw --force reset
jcwt-panel ALL=(root) NOPASSWD: /usr/sbin/ufw default *
jcwt-panel ALL=(root) NOPASSWD: /usr/sbin/ufw reload

# Filebrowser (run as any site user)
jcwt-panel ALL=(ALL) NOPASSWD: /usr/local/bin/filebrowser *
jcwt-panel ALL=(ALL) NOPASSWD: /usr/bin/filebrowser *
EOF

chmod 0440 /etc/sudoers.d/jcwt-panel
visudo -cf /etc/sudoers.d/jcwt-panel >/dev/null 2>&1 || die "Sudoers syntax error!"
log_ok "Sudoers rules updated and validated"

# ---- Step 3: Nginx Test & Panel Restart ----
step_header "Testing Configuration & Restarting Services"

log_info "Testing Nginx syntax..."
if nginx -t >/dev/null 2>&1; then
    log_ok "Nginx configuration valid"
    systemctl reload nginx 2>/dev/null || true
else
    log_warn "Nginx syntax warnings detected (check nginx -t)"
fi

log_info "Reloading systemd and restarting jcwt-panel service..."
systemctl daemon-reload
if systemctl restart jcwt-panel; then
    log_ok "JCWT Ultra Panel service restarted"
else
    die "Failed to restart jcwt-panel service! Check: journalctl -u jcwt-panel -n 50"
fi

# Query IP addresses for display banner
IPV4_ADDR=$(curl -4 -s --max-time 3 https://api.ipify.org 2>/dev/null || ip -4 addr show scope global | grep -oP 'inet \K[\d.]+' | head -1 || echo "")
IPV6_ADDR=$(curl -6 -s --max-time 3 https://api64.ipify.org 2>/dev/null || ip -6 addr show scope global | grep -v 'fe80' | grep -oP 'inet6 \K[0-9a-f:]+' | head -1 || echo "")

echo ""
echo -e "${BOLD}${GREEN}"
echo "  ╔══════════════════════════════════════════════════════════╗"
echo "  ║                                                          ║"
echo "  ║   ${PURPLE}✨ JCWT Ultra Panel updated successfully! ✨${GREEN}       ║"
echo "  ║                                                          ║"
echo "  ╚══════════════════════════════════════════════════════════╝"
echo -e "${NC}"

echo -e "  ${BOLD}Access Your Panel${NC}"
echo -e "  ─────────────────────────────────────────"
if [ -n "${IPV4_ADDR:-}" ] && [ "$IPV4_ADDR" != "none" ]; then
    echo -e "  ${CYAN}IPv4 URL:${NC}  https://${IPV4_ADDR}:${PANEL_PORT}"
fi
if [ -n "${IPV6_ADDR:-}" ] && [ "$IPV6_ADDR" != "::1" ]; then
    echo -e "  ${CYAN}IPv6 URL:${NC}  https://[${IPV6_ADDR}]:${PANEL_PORT}"
fi
echo ""
