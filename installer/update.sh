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
if [ -f "$SCRIPT_DIR/../cmd/jcwt-panel/main.go" ]; then
    PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
elif [ -f "$SCRIPT_DIR/cmd/jcwt-panel/main.go" ]; then
    PROJECT_DIR="$SCRIPT_DIR"
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

UPDATED=false

if [ -n "$PROJECT_DIR" ] && command -v go >/dev/null 2>&1; then
    log_info "Building latest binary from Go source ($PROJECT_DIR)..."
    cd "$PROJECT_DIR"
    if go build -o /tmp/jcwt-panel-new ./cmd/jcwt-panel/; then
        mv /tmp/jcwt-panel-new "$PANEL_BIN"
        UPDATED=true
        log_ok "Compiled and installed latest panel binary"
    else
        log_warn "Compilation failed — preserving existing panel binary"
    fi
elif [ -f "$SCRIPT_DIR/jcwt-panel" ]; then
    log_info "Installing pre-built binary from $SCRIPT_DIR/jcwt-panel..."
    cp "$SCRIPT_DIR/jcwt-panel" "$PANEL_BIN"
    UPDATED=true
    log_ok "Updated panel binary from pre-built package"
elif [ -f "./jcwt-panel" ]; then
    log_info "Installing pre-built binary from ./jcwt-panel..."
    cp "./jcwt-panel" "$PANEL_BIN"
    UPDATED=true
    log_ok "Updated panel binary"
else
    if [ -f "$PANEL_BIN" ]; then
        log_ok "Panel binary found at $PANEL_BIN (re-asserting permissions)"
        UPDATED=true
    else
        die "No binary or Go compiler found! Place binary at $PANEL_BIN or run from source root."
    fi
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

    *)
        die "unknown command '${COMMAND}'. Valid: delete-home, delete-backup, delete-staging"
        ;;
esac
FSCTL_EOF

chmod 0755 /usr/local/sbin/panel-fsctl
chown root:root /usr/local/sbin/panel-fsctl
log_ok "Filesystem helper updated"

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

# Nginx config test
jcwt-panel ALL=(root) NOPASSWD: /usr/sbin/nginx -t

# File management helper
jcwt-panel ALL=(root) NOPASSWD: /usr/local/sbin/panel-fsctl *

# Network & firewall
jcwt-panel ALL=(root) NOPASSWD: /usr/sbin/ufw *

# SSL cert management (certbot)
jcwt-panel ALL=(root) NOPASSWD: /usr/bin/certbot certonly *
jcwt-panel ALL=(root) NOPASSWD: /usr/bin/certbot renew *

# Package & service version queries
jcwt-panel ALL=(root) NOPASSWD: /usr/bin/apt-get update
jcwt-panel ALL=(root) NOPASSWD: /usr/bin/apt-get upgrade -y
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
