#!/usr/bin/env bash
# ============================================================================
# JCWT Ultra Panel — One-Command Installer
# IPv6-Native Lightweight Web Hosting Control Panel for ARM64 EC2
# ============================================================================
set -uo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
BOLD='\033[1m'
DIM='\033[2m'
NC='\033[0m'

PANEL_USER="jcwt-panel"
PANEL_PORT="8443"
DATA_DIR="/var/lib/jcwt-panel"
PANEL_BIN="/usr/local/bin/jcwt-panel"
CONFIG_DIR="/etc/jcwt-panel"
LOG_DIR="/var/log/jcwt-panel"

# Step tracking
STEP_CURRENT=0
STEP_TOTAL=13

# Verbose mode (pass -v or --verbose)
VERBOSE=false
for arg in "$@"; do
    case "$arg" in
        -v|--verbose) VERBOSE=true ;;
    esac
done

# Helper: run apt-get with or without verbose output
apt_run() {
    if [ "$VERBOSE" = true ]; then
        "$@"
    else
        "$@" 2>&1 | { grep -E "^Setting up|^Unpacking|^E:" || true; } | while read -r line; do
            log_detail "$line"
        done
    fi
}

log_info()    { echo -e "  ${BLUE}INFO${NC}  $1"; }
log_ok()      { echo -e "  ${GREEN} OK ${NC}  $1"; }
log_warn()    { echo -e "  ${YELLOW}WARN${NC}  $1"; }
log_error()   { echo -e "  ${RED}FAIL${NC}  $1"; }
log_detail()  { echo -e "        ${DIM}→ $1${NC}"; }
log_pkg()     { echo -e "        ${DIM}  ├─ $1${NC}"; }
log_pkg_last(){ echo -e "        ${DIM}  └─ $1${NC}"; }

step_header() {
    STEP_CURRENT=$((STEP_CURRENT + 1))
    echo ""
    echo -e "${BOLD}${PURPLE}[$STEP_CURRENT/$STEP_TOTAL]${NC} ${BOLD}$1${NC}"
    echo -e "${DIM}$(printf '%.0s─' $(seq 1 60))${NC}"
}

# ---- Pre-flight checks ----
preflight() {
    echo ""
    echo -e "${PURPLE}${BOLD}"
    echo "  ╔══════════════════════════════════════════════╗"
    echo "  ║                                              ║"
    echo "  ║        JCWT Ultra Panel Installer            ║"
    echo "  ║      IPv6-Native ARM64 Hosting Panel         ║"
    echo "  ║                                              ║"
    echo "  ╚══════════════════════════════════════════════╝"
    echo -e "${NC}"

    if [[ $EUID -ne 0 ]]; then
        log_error "This script must be run as root (use: sudo bash install.sh)"
        exit 1
    fi

    step_header "Pre-flight Checks"

    # Check architecture
    ARCH=$(uname -m)
    if [[ "$ARCH" == "aarch64" || "$ARCH" == "arm64" ]]; then
        log_ok "Architecture: ${BOLD}$ARCH${NC} (ARM64)"
    else
        log_warn "Expected ARM64 (aarch64), got: $ARCH"
        log_warn "Continuing anyway, but this panel is optimized for ARM64"
    fi

    # Check OS
    if [ -f /etc/os-release ]; then
        . /etc/os-release
        log_ok "Operating System: ${BOLD}$PRETTY_NAME${NC}"
    fi

    # Kernel
    log_info "Kernel: $(uname -r)"

    # Detect IPv6
    if ip -6 addr show scope global 2>/dev/null | grep -q "inet6"; then
        IPV6_ADDR=$(ip -6 addr show scope global | grep "inet6" | head -1 | awk '{print $2}' | cut -d'/' -f1)
        log_ok "IPv6 detected: ${BOLD}$IPV6_ADDR${NC}"
    else
        log_warn "No global IPv6 address detected — using ::1 as fallback"
        IPV6_ADDR="::1"
    fi

    # Check IPv4
    if ip -4 addr show scope global 2>/dev/null | grep -q "inet "; then
        IPV4_ADDR=$(ip -4 addr show scope global | grep "inet " | head -1 | awk '{print $2}' | cut -d'/' -f1)
        log_info "IPv4 also available: $IPV4_ADDR (panel will bind IPv6 only)"
    else
        log_ok "IPv6-only environment confirmed"
    fi

    # Memory
    TOTAL_MEM_MB=$(awk '/MemTotal/ {printf "%d", $2/1024}' /proc/meminfo 2>/dev/null || echo "unknown")
    log_info "Total RAM: ${TOTAL_MEM_MB} MB"

    # Create swap if RAM < 1024MB and no swap exists (prevents OOM on t4g.nano)
    SWAP_TOTAL=$(awk '/SwapTotal/ {printf "%d", $2/1024}' /proc/meminfo 2>/dev/null || echo "0")
    if [ "$TOTAL_MEM_MB" -lt 1024 ] 2>/dev/null && [ "$SWAP_TOTAL" -lt 100 ] 2>/dev/null; then
        if [ ! -f /swapfile ]; then
            log_warn "Low RAM detected (${TOTAL_MEM_MB}MB). Creating 1GB swap file to prevent OOM..."
            dd if=/dev/zero of=/swapfile bs=1M count=1024 status=none 2>/dev/null
            chmod 600 /swapfile
            mkswap /swapfile >/dev/null 2>&1
            swapon /swapfile
            # Make persistent
            if ! grep -q '/swapfile' /etc/fstab 2>/dev/null; then
                echo '/swapfile none swap sw 0 0' >> /etc/fstab
            fi
            log_ok "1GB swap file created and activated"
        else
            swapon /swapfile 2>/dev/null || true
            log_ok "Swap file already exists"
        fi
    fi

    # Disk
    DISK_AVAIL=$(df -h / | awk 'NR==2 {print $4}')
    log_info "Disk available: $DISK_AVAIL"
}

# ---- Install system packages ----
install_packages() {
    export DEBIAN_FRONTEND=noninteractive

    # Clean up any broken repo files from a previous failed install
    if [ -f /etc/apt/sources.list.d/ondrej-php.list ]; then
        log_warn "Removing leftover ondrej-php.list from previous run..."
        rm -f /etc/apt/sources.list.d/ondrej-php.list
    fi
    # Also remove native DEB822 format (from add-apt-repository) to avoid duplicates on re-run
    for f in /etc/apt/sources.list.d/ondrej-ubuntu-php-*.sources; do
        [ -f "$f" ] && rm -f "$f" && log_warn "Removed $(basename $f) from previous run..."
    done
    if [ -f /etc/apt/trusted.gpg.d/ondrej-php.gpg ]; then
        rm -f /etc/apt/trusted.gpg.d/ondrej-php.gpg
    fi
    if [ -f /usr/share/keyrings/ondrej-php.gpg ]; then
        rm -f /usr/share/keyrings/ondrej-php.gpg
    fi

    step_header "Updating System Packages"
    log_info "Running apt-get update..."
    if [ "$VERBOSE" = true ]; then
        apt-get update || true
    else
        apt-get update -qq 2>&1 | tail -3 || true
    fi
    log_ok "Package lists updated"

    # Upgrade existing packages
    log_info "Checking for system upgrades..."
    UPGRADABLE=$(apt list --upgradable 2>/dev/null | grep -v "^Listing" | grep -c "/" || true)
    UPGRADABLE=${UPGRADABLE:-0}
    if [ "$UPGRADABLE" -gt 0 ] 2>/dev/null; then
        log_info "Upgrading $UPGRADABLE packages..."
        apt_run apt-get upgrade -y
        log_ok "System packages upgraded"
    else
        log_ok "System already up to date"
    fi

    step_header "Adding PHP Repository"
    log_info "Installing software-properties-common..."
    apt_run apt-get install -y software-properties-common
    log_ok "software-properties-common ready"

    log_info "Adding ppa:ondrej/php repository..."
    # Try add-apt-repository with a 30s timeout (hangs on IPv6-only due to keyserver)
    if timeout 30 add-apt-repository -y ppa:ondrej/php >/dev/null 2>&1; then
        log_detail "Successfully added via add-apt-repository"
        # Remove manual fallback files to avoid duplicate sources
        rm -f /etc/apt/sources.list.d/ondrej-php.list
        rm -f /etc/apt/trusted.gpg.d/ondrej-php.gpg
    else
        log_warn "add-apt-repository failed or timed out (common on IPv6-only EC2). Falling back to manual method..."
        # Download GPG key directly via HTTPS (works on IPv6)
        curl -fsSL "https://keyserver.ubuntu.com/pks/lookup?op=get&search=0x14aa40ec0831756756d7f66c4f4ea0aae5267a6c" \
            | gpg --batch --yes --dearmor -o /etc/apt/trusted.gpg.d/ondrej-php.gpg 2>/dev/null
        chmod 644 /etc/apt/trusted.gpg.d/ondrej-php.gpg
        OS_CODENAME=$(. /etc/os-release && echo "$VERSION_CODENAME")
        echo "deb https://ppa.launchpadcontent.net/ondrej/php/ubuntu $OS_CODENAME main" > /etc/apt/sources.list.d/ondrej-php.list
        log_detail "Added repo for Ubuntu $OS_CODENAME via manual fallback"
        # Remove native format file to avoid duplicate sources
        for f in /etc/apt/sources.list.d/ondrej-ubuntu-php-*.sources; do
            [ -f "$f" ] && rm -f "$f"
        done
    fi

    log_info "Refreshing package lists..."
    if [ "$VERBOSE" = true ]; then
        apt-get update || true
    else
        apt-get update -qq 2>&1 | tail -3 || true
    fi
    log_ok "Ondrej PHP PPA added"

    step_header "Installing Core Services"

    # ---- Nginx ----
    log_info "Installing Nginx web server..."
    apt_run apt-get install -y nginx libnginx-mod-http-headers-more-filter
    NGINX_VER=$(nginx -v 2>&1 | awk -F/ '{print $2}' || echo "unknown")
    log_ok "Nginx ${BOLD}v${NGINX_VER}${NC} installed"

    # ---- MariaDB ----
    log_info "Installing MariaDB server and client..."
    apt_run apt-get install -y mariadb-server mariadb-client
    MARIA_VER=$(mariadbd --version 2>/dev/null | awk '{print $3}' || mysql --version 2>/dev/null | awk '{print $5}' | tr -d ',' || echo "unknown")
    log_ok "MariaDB ${BOLD}${MARIA_VER}${NC} installed"

    # ---- phpMyAdmin ----
    log_info "Installing phpMyAdmin..."
    echo "phpmyadmin phpmyadmin/reconfigure-webserver multiselect" | debconf-set-selections 2>/dev/null || true
    echo "phpmyadmin phpmyadmin/dbconfig-install boolean false" | debconf-set-selections 2>/dev/null || true
    apt_run apt-get install -y phpmyadmin

    # Remove any phpMyAdmin nginx configs the package may drop in conf.d
    # (these contain bare 'location' blocks that are invalid at http{} level)
    rm -f /etc/nginx/conf.d/phpmyadmin.conf
    rm -f /etc/nginx/conf.d/jcwt-phpmyadmin.conf
    rm -f /etc/nginx/conf.d/*phpmyadmin*

    if [ -d /usr/share/phpmyadmin ]; then
        # Configure signon auth (auto-login from panel)
        mkdir -p /etc/phpmyadmin/conf.d
        cat > /etc/phpmyadmin/conf.d/jcwt-signon.php << 'PMACONF'
<?php
$cfg['Servers'][1]['auth_type'] = 'signon';
$cfg['Servers'][1]['SignonSession'] = 'SignonSession';
$cfg['Servers'][1]['SignonURL'] = '/pma/jcwt_signon.php';
$cfg['Servers'][1]['LogoutURL'] = '/pma/jcwt_signon.php';
$cfg['Servers'][1]['host'] = 'localhost';
$cfg['LoginCookieValidity'] = 1800;
$cfg['SendErrorReports'] = 'never';
$cfg['Servers'][1]['hide_db'] = '^(information_schema|performance_schema|mysql|sys|phpmyadmin)$';
// Suppress configuration storage warnings
$cfg['PmaNoRelation_DisableWarning'] = true;
$cfg['SuhosinDisableWarning'] = true;
$cfg['LoginCookieDeleteAll'] = true;
PMACONF

        # Set session.gc_maxlifetime to match cookie validity (1800s)
        for PHPINI in /etc/php/*/fpm/php.ini /etc/php/*/cli/php.ini; do
            if [ -f "$PHPINI" ]; then
                sed -i 's/^session\.gc_maxlifetime.*/session.gc_maxlifetime = 1800/' "$PHPINI" 2>/dev/null || true
            fi
        done

        # Create persistent signon landing page (shown on logout / expired session)
        cat > /usr/share/phpmyadmin/jcwt_signon.php << 'SIGNONPHP'
<?php
session_name('SignonSession');
session_start();
// Clear any stale session data
$_SESSION = array();
session_destroy();
?>
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>phpMyAdmin — Session Ended</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            min-height: 100vh; display: flex; align-items: center; justify-content: center;
            background: linear-gradient(135deg, #f8fafc 0%, #e2e8f0 100%); color: #334155; }
        .card { text-align: center; background: #fff; padding: 3rem 2.5rem; border-radius: 16px;
            box-shadow: 0 4px 24px rgba(0,0,0,0.08); max-width: 440px; }
        .icon { font-size: 3rem; margin-bottom: 1rem; }
        h1 { font-size: 1.4rem; font-weight: 600; margin-bottom: 0.5rem; color: #1e293b; }
        p { color: #64748b; line-height: 1.6; margin-bottom: 1.5rem; }
        .btn { display: inline-block; padding: 0.6rem 1.5rem; background: #6366f1; color: #fff;
            border: none; border-radius: 8px; font-weight: 500; font-size: 0.9rem;
            cursor: pointer; transition: background 0.2s; margin-bottom: 1rem; }
        .btn:hover { background: #4f46e5; }
        .badge { display: inline-block; padding: 0.35rem 0.9rem; background: #f1f5f9;
            border-radius: 999px; font-size: 0.75rem; color: #94a3b8; }
    </style>
</head>
<body>
    <div class="card">
        <div class="icon">🔒</div>
        <h1>Session Ended</h1>
        <p>Your phpMyAdmin session has ended.<br>You may close this tab and open phpMyAdmin again from the panel when needed.</p>
        <button class="btn" onclick="window.close()">Close This Window</button>
        <div class="badge">Powered by JCWT Ultra Panel</div>
    </div>
</body>
</html>
SIGNONPHP

        # Nginx snippet for /pma/ URL (included inside server blocks via 'include')
        mkdir -p /etc/nginx/snippets
        cat > /etc/nginx/snippets/phpmyadmin.conf << 'PMANGINX'
# JCWT Ultra Panel — phpMyAdmin location block (included inside server{})
location /pma/ {
    alias /usr/share/phpmyadmin/;
    index index.php;
    location ~ ^/pma/(.*\.php)$ {
        alias /usr/share/phpmyadmin/$1;
        fastcgi_pass unix:/run/php/php8.3-fpm.sock;
        fastcgi_index index.php;
        fastcgi_param SCRIPT_FILENAME /usr/share/phpmyadmin/$1;
        include fastcgi_params;
    }
    location ~* ^/pma/(.+\.(jpg|jpeg|gif|css|png|js|ico|html|xml|txt))$ {
        alias /usr/share/phpmyadmin/$1;
    }
}
PMANGINX
        log_ok "phpMyAdmin installed (signon auth via panel)"
    else
        log_warn "phpMyAdmin directory not found"
        # Create empty snippet so nginx include doesn't fail
        mkdir -p /etc/nginx/snippets
        echo "# phpMyAdmin not installed" > /etc/nginx/snippets/phpmyadmin.conf
    fi

    step_header "Installing PHP Versions"

    PHP_EXTENSIONS="fpm cli mysql curl gd mbstring xml zip intl bcmath opcache readline redis sqlite3 imagick igbinary soap exif"

    for VER in 8.2 8.3 8.4; do
        log_info "Installing PHP ${BOLD}$VER${NC} Core..."
        
        # Install Core first to reduce apt memory pressure on 512MB instances (t4g.nano)
        apt_run apt-get install -y php${VER}-fpm php${VER}-cli

        log_info "Installing PHP ${BOLD}$VER${NC} Extensions..."
        
        # Build extension package list
        PKG_LIST=""
        for EXT in $PHP_EXTENSIONS; do
            if [[ "$EXT" != "fpm" && "$EXT" != "cli" ]]; then
                PKG_LIST="$PKG_LIST php${VER}-${EXT}"
            fi
        done

        # Show what we're installing
        EXT_COUNT=$(echo "$PHP_EXTENSIONS" | wc -w | tr -d ' ')
        log_detail "Installing extensions: $(echo $PKG_LIST | sed 's/php[0-9.]*-//g' | sed 's/ /, /g')"

        # Install extensions in a separate transaction
        apt_run apt-get install -y $PKG_LIST

        # Verify
        PHP_FULL_VER=$(php${VER} -v 2>/dev/null | head -1 | awk '{print $2}' || echo "$VER.x")
        log_ok "PHP ${BOLD}${PHP_FULL_VER}${NC} installed with ${EXT_COUNT} extensions"
    done

    # Reload systemd units after PHP package installs to prevent
    # "unit file changed on disk" warnings during trigger processing
    systemctl daemon-reload 2>/dev/null || true

    # PHP 8.5 — try but don't fail (some extensions may not exist yet)
    if apt-cache show php8.5-fpm > /dev/null 2>&1; then
        log_info "Installing PHP ${BOLD}8.5${NC} with extensions..."
        PHP85_INSTALLED=0
        PHP85_SKIPPED=0
        for EXT in $PHP_EXTENSIONS; do
            PKG="php8.5-${EXT}"
            if apt-cache show "$PKG" > /dev/null 2>&1; then
                if apt-get install -y "$PKG" > /dev/null 2>&1; then
                    log_pkg "$PKG"
                    PHP85_INSTALLED=$((PHP85_INSTALLED + 1))
                else
                    log_pkg "$PKG (failed)"
                    PHP85_SKIPPED=$((PHP85_SKIPPED + 1))
                fi
            else
                PHP85_SKIPPED=$((PHP85_SKIPPED + 1))
            fi
        done
        if [ "$PHP85_INSTALLED" -gt 0 ]; then
            PHP85_VER=$(php8.5 -v 2>/dev/null | head -1 | awk '{print $2}' || echo "8.5.x")
            log_ok "PHP ${BOLD}${PHP85_VER}${NC} installed ($PHP85_INSTALLED extensions, $PHP85_SKIPPED skipped)"
        else
            log_warn "PHP 8.5 packages exist but none installed successfully"
        fi
    else
        log_warn "PHP 8.5 is not yet available in the repository — skipping"
    fi

    # ---- Utilities ----
    log_info "Installing utilities (openssl, ufw, curl, wget, jq, gcc, certbot)..."
    UTIL_PKGS="openssl ufw curl wget jq build-essential apache2-utils certbot zip unzip imagemagick ghostscript"
    apt_run apt-get install -y $UTIL_PKGS
    log_ok "Utilities installed"

    # ---- Redis Server ----
    step_header "Installing Redis Server"
    log_info "Installing Redis..."
    apt_run apt-get install -y redis-server
    # Harden Redis configuration
    REDIS_CONF="/etc/redis/redis.conf"
    if [ -f "$REDIS_CONF" ]; then
        # Bind to localhost only (no external access)
        sed -i 's/^bind .*/bind 127.0.0.1 ::1/' "$REDIS_CONF"
        # Set memory limit and eviction policy
        sed -i 's/^# maxmemory .*/maxmemory 128mb/' "$REDIS_CONF"
        if ! grep -q "^maxmemory " "$REDIS_CONF"; then
            echo "maxmemory 128mb" >> "$REDIS_CONF"
        fi
        sed -i 's/^# maxmemory-policy .*/maxmemory-policy allkeys-lru/' "$REDIS_CONF"
        if ! grep -q "^maxmemory-policy " "$REDIS_CONF"; then
            echo "maxmemory-policy allkeys-lru" >> "$REDIS_CONF"
        fi
        # Use systemd supervision
        sed -i 's/^supervised .*/supervised systemd/' "$REDIS_CONF"
        # Disable dangerous commands
        if ! grep -q "^rename-command FLUSHDB" "$REDIS_CONF"; then
            echo 'rename-command FLUSHDB ""' >> "$REDIS_CONF"
            echo 'rename-command FLUSHALL ""' >> "$REDIS_CONF"
            echo 'rename-command DEBUG ""' >> "$REDIS_CONF"
        fi
        # Disable RDB snapshots for cache-only use (reduce disk I/O)
        sed -i 's/^save /#save /' "$REDIS_CONF"
        if ! grep -q "^save \"\"" "$REDIS_CONF"; then
            echo 'save ""' >> "$REDIS_CONF"
        fi
    fi
    systemctl restart redis-server
    systemctl enable redis-server > /dev/null 2>&1
    REDIS_VER=$(redis-server --version 2>/dev/null | awk '{print $3}' | sed 's/v=//' || echo "unknown")
    log_ok "Redis ${BOLD}${REDIS_VER}${NC} installed and secured (localhost only, 128MB limit)"

    # ---- File Browser ----
    # Detect IPv4/IPv6 connectivity for download fallback logic
    HAS_IPV4=$(ip -4 addr show scope global 2>/dev/null | grep -c inet || echo "0")
    HAS_IPV6=$(ip -6 addr show scope global 2>/dev/null | grep -c inet6 || echo "0")

    log_info "Installing File Browser (file manager)..."
    if [ -f /usr/local/bin/filebrowser ]; then
        FB_VER=$(/usr/local/bin/filebrowser version 2>/dev/null | head -1 || echo "unknown")
        # Check if version supports syntax highlighting (v2.25+)
        FB_MAJOR=$(echo "$FB_VER" | grep -oP 'v?\K[0-9]+' | head -1 || echo "0")
        FB_MINOR=$(echo "$FB_VER" | grep -oP 'v?[0-9]+\.\K[0-9]+' | head -1 || echo "0")
        if [ "$FB_MAJOR" -ge 2 ] 2>/dev/null && [ "$FB_MINOR" -ge 25 ] 2>/dev/null; then
            log_ok "File Browser already installed: $FB_VER (syntax highlighting supported)"
        else
            log_warn "File Browser $FB_VER is outdated (need v2.25+ for syntax highlighting). Upgrading..."
            rm -f /usr/local/bin/filebrowser
        fi
    fi
    if [ ! -f /usr/local/bin/filebrowser ]; then
        FB_INSTALLED=false

        # Method 1: Official install script (may hang on IPv6-only)
        log_info "  → Trying official install script..."
        timeout 30 bash -c 'curl -fsSL https://raw.githubusercontent.com/filebrowser/get/master/get.sh | bash' >/dev/null 2>&1
        if [ -f /usr/local/bin/filebrowser ]; then
            FB_INSTALLED=true
        fi

        # Method 2: Direct binary download from GitHub releases
        if [ "$FB_INSTALLED" = false ]; then
            log_warn "  → Official script failed/timed out. Trying direct binary download..."
            FB_ARCH="linux-amd64"
            if [ "$(uname -m)" = "aarch64" ]; then
                FB_ARCH="linux-arm64"
            fi
            FB_URL="https://github.com/filebrowser/filebrowser/releases/latest/download/${FB_ARCH}-filebrowser.tar.gz"
            if timeout 30 wget -q -O /tmp/filebrowser.tar.gz "$FB_URL" 2>/dev/null || \
               timeout 30 curl -fsSL -o /tmp/filebrowser.tar.gz "$FB_URL" 2>/dev/null; then
                tar -xzf /tmp/filebrowser.tar.gz -C /tmp/ filebrowser 2>/dev/null
                if [ -f /tmp/filebrowser ]; then
                    mv /tmp/filebrowser /usr/local/bin/filebrowser
                    chmod +x /usr/local/bin/filebrowser
                    FB_INSTALLED=true
                fi
                rm -f /tmp/filebrowser.tar.gz
            fi
        fi

        # Method 3: Try via DNS64/NAT64 (for pure IPv6 instances)
        if [ "$FB_INSTALLED" = false ] && [ "$HAS_IPV6" -gt 0 ] && [ "$HAS_IPV4" -eq 0 ]; then
            log_warn "  → Direct download failed. Trying via DNS64 NAT64 proxy..."
            FB_ARCH="linux-amd64"
            if [ "$(uname -m)" = "aarch64" ]; then
                FB_ARCH="linux-arm64"
            fi
            # Use dns64.dns.google as resolver for NAT64
            FB_URL="https://github.com/filebrowser/filebrowser/releases/latest/download/${FB_ARCH}-filebrowser.tar.gz"
            if timeout 30 wget -q --dns-servers=2001:4860:4860::6464 -O /tmp/filebrowser.tar.gz "$FB_URL" 2>/dev/null; then
                tar -xzf /tmp/filebrowser.tar.gz -C /tmp/ filebrowser 2>/dev/null
                if [ -f /tmp/filebrowser ]; then
                    mv /tmp/filebrowser /usr/local/bin/filebrowser
                    chmod +x /usr/local/bin/filebrowser
                    FB_INSTALLED=true
                fi
                rm -f /tmp/filebrowser.tar.gz
            fi
        fi

        if [ "$FB_INSTALLED" = true ] && [ -f /usr/local/bin/filebrowser ]; then
            FB_VER=$(/usr/local/bin/filebrowser version 2>/dev/null | head -1 || echo "unknown")
            log_ok "File Browser ${BOLD}${FB_VER}${NC} installed"
        else
            log_warn "File Browser installation failed — file manager will be unavailable"
            log_warn "  → You can install manually later: curl -fsSL https://raw.githubusercontent.com/filebrowser/get/master/get.sh | bash"
        fi
    fi
}

# ---- Configure MariaDB for IPv6 ----
configure_mariadb() {
    step_header "Configuring MariaDB for IPv6"

    log_info "Writing IPv6 bind configuration..."
    cat > /etc/mysql/mariadb.conf.d/60-jcwt-ipv6.cnf << 'EOF'
[mysqld]
# JCWT Ultra Panel - IPv6 Configuration
bind-address = ::1
skip-name-resolve
character-set-server = utf8mb4
collation-server = utf8mb4_unicode_ci
innodb_buffer_pool_size = 128M
innodb_log_file_size = 32M
max_connections = 50
EOF
    log_detail "Config written: /etc/mysql/mariadb.conf.d/60-jcwt-ipv6.cnf"
    log_detail "bind-address = ::1 (IPv6 localhost only)"
    log_detail "charset = utf8mb4, max_connections = 50"

    log_info "Restarting MariaDB..."
    systemctl restart mariadb
    systemctl enable mariadb > /dev/null 2>&1
    log_ok "MariaDB restarted and enabled"

    log_info "Securing MariaDB installation..."
    mysql -u root -e "DELETE FROM mysql.user WHERE User='';" 2>/dev/null || true
    log_detail "Removed anonymous users"
    mysql -u root -e "DELETE FROM mysql.user WHERE User='root' AND Host NOT IN ('localhost', '127.0.0.1', '::1');" 2>/dev/null || true
    log_detail "Restricted root to local connections only"
    mysql -u root -e "DROP DATABASE IF EXISTS test;" 2>/dev/null || true
    log_detail "Removed test database"
    mysql -u root -e "FLUSH PRIVILEGES;" 2>/dev/null || true

    log_ok "MariaDB secured and configured for IPv6"
}

# ---- Configure Nginx for IPv6 ----
configure_nginx() {
    step_header "Configuring Nginx for IPv6"

    log_info "Removing default site..."
    rm -f /etc/nginx/sites-enabled/default
    log_detail "Disabled: /etc/nginx/sites-enabled/default"

    # Comment out conflicting directives in the default nginx.conf
    # Ubuntu's nginx.conf already contains: gzip, server_tokens, client_max_body_size, etc.
    log_info "Patching default nginx.conf to avoid directive conflicts..."

    # Comment out gzip in default nginx.conf to avoid duplicate
    if grep -q "^[[:space:]]*gzip on;" /etc/nginx/nginx.conf 2>/dev/null; then
        sed -i 's/^\([[:space:]]*\)gzip on;/\1# gzip on; # Managed by JCWT Panel/' /etc/nginx/nginx.conf
        log_detail "Commented out default 'gzip on' in nginx.conf"
    fi
    if grep -q "^[[:space:]]*gzip_" /etc/nginx/nginx.conf 2>/dev/null; then
        sed -i 's/^\([[:space:]]*\)gzip_\(.*\)/\1# gzip_\2 # Managed by JCWT Panel/' /etc/nginx/nginx.conf
        log_detail "Commented out default gzip_* directives in nginx.conf"
    fi
    if grep -q "^[[:space:]]*server_tokens" /etc/nginx/nginx.conf 2>/dev/null; then
        sed -i 's/^\([[:space:]]*\)server_tokens\(.*\)/\1# server_tokens\2 # Managed by JCWT Panel/' /etc/nginx/nginx.conf
        log_detail "Commented out default 'server_tokens' in nginx.conf"
    fi
    # Comment out any ssl_* directives in nginx.conf (managed by jcwt-optimization.conf)
    if grep -q "^[[:space:]]*ssl_protocols" /etc/nginx/nginx.conf 2>/dev/null; then
        sed -i 's/^\([[:space:]]*\)ssl_protocols\(.*\)/\1# ssl_protocols\2 # Managed by JCWT Panel/' /etc/nginx/nginx.conf
        log_detail "Commented out default 'ssl_protocols' in nginx.conf"
    fi
    if grep -q "^[[:space:]]*ssl_prefer_server_ciphers" /etc/nginx/nginx.conf 2>/dev/null; then
        sed -i 's/^\([[:space:]]*\)ssl_prefer_server_ciphers\(.*\)/\1# ssl_prefer_server_ciphers\2 # Managed by JCWT Panel/' /etc/nginx/nginx.conf
        log_detail "Commented out default 'ssl_prefer_server_ciphers' in nginx.conf"
    fi
    if grep -q "^[[:space:]]*ssl_ciphers" /etc/nginx/nginx.conf 2>/dev/null; then
        sed -i 's/^\([[:space:]]*\)ssl_ciphers\(.*\)/\1# ssl_ciphers\2 # Managed by JCWT Panel/' /etc/nginx/nginx.conf
        log_detail "Commented out default 'ssl_ciphers' in nginx.conf"
    fi
    if grep -q "^[[:space:]]*ssl_session_cache" /etc/nginx/nginx.conf 2>/dev/null; then
        sed -i 's/^\([[:space:]]*\)ssl_session_cache\(.*\)/\1# ssl_session_cache\2 # Managed by JCWT Panel/' /etc/nginx/nginx.conf
        log_detail "Commented out default 'ssl_session_cache' in nginx.conf"
    fi

    # On re-install: strip duplicate SSL directives from existing vhost files
    # Clean both sites-available and sites-enabled (in case of non-symlink copies)
    log_info "Cleaning SSL/include directives from existing vhost files..."
    for VHOST_DIR in /etc/nginx/sites-available /etc/nginx/sites-enabled; do
        for VHOST in "$VHOST_DIR"/*.conf; do
            [ -f "$VHOST" ] || continue
            [ "$(basename "$VHOST")" = "000-default.conf" ] && continue
            # Remove per-server SSL directives (now in jcwt-optimization.conf at http{} level)
            sed -i '/^[[:space:]]*ssl_protocols/d' "$VHOST"
            sed -i '/^[[:space:]]*ssl_ciphers[[:space:]]/d' "$VHOST"
            sed -i '/^[[:space:]]*ssl_prefer_server_ciphers/d' "$VHOST"
            sed -i '/^[[:space:]]*ssl_session_cache/d' "$VHOST"
            sed -i '/^[[:space:]]*ssl_session_timeout/d' "$VHOST"
            sed -i '/^[[:space:]]*ssl_stapling/d' "$VHOST"
            sed -i '/^[[:space:]]*ssl_stapling_verify/d' "$VHOST"
            # Revert standalone 'http2 on;' back to listen-line format (compatible with nginx < 1.25.1)
            if grep -q 'http2 on;' "$VHOST"; then
                sed -i '/^[[:space:]]*http2 on;/d' "$VHOST"
                sed -i 's|listen \[::\]:443 ssl;|listen [::]:443 ssl http2;|' "$VHOST"
            fi
            # Migrate old phpMyAdmin include to new common snippet
            sed -i 's|include /etc/nginx/snippets/phpmyadmin\.conf;|include /etc/nginx/snippets/jcwt-server-common.conf;|g' "$VHOST"
        done
    done
    log_ok "Existing vhosts cleaned up"

    log_info "Writing JCWT optimization config..."
    cat > /etc/nginx/conf.d/jcwt-optimization.conf << 'EOF'
# JCWT Ultra Panel - Nginx Optimizations
# This file manages all compression, security headers, and limits

# Upload size
client_max_body_size 100M;

# Hide server version and Server header
server_tokens off;
more_clear_headers Server;

# Compression
gzip on;
gzip_types text/plain text/css application/json application/javascript text/xml application/xml application/xml+rss text/javascript image/svg+xml;
gzip_min_length 256;
gzip_vary on;
gzip_comp_level 5;
gzip_proxied any;

# Security headers (applied globally)
add_header X-Frame-Options "SAMEORIGIN" always;
add_header X-Content-Type-Options "nosniff" always;
add_header X-XSS-Protection "1; mode=block" always;
add_header Referrer-Policy "strict-origin-when-cross-origin" always;

# SSL settings (applied globally — do not repeat in server blocks)
ssl_protocols TLSv1.2 TLSv1.3;
ssl_ciphers HIGH:!aNULL:!MD5;
ssl_prefer_server_ciphers on;
ssl_session_cache shared:SSL:10m;
EOF
    log_detail "Config written: /etc/nginx/conf.d/jcwt-optimization.conf"
    log_detail "  • client_max_body_size = 100M"
    log_detail "  • gzip compression enabled (level 5)"
    log_detail "  • Security headers: X-Frame-Options, X-Content-Type-Options, etc."

    # Create common server-level snippet (included inside each server{} block)
    log_info "Writing common server snippet..."
    cat > /etc/nginx/snippets/jcwt-server-common.conf << 'SRVEOF'
# JCWT Ultra Panel — common server-block includes
# Add per-server-block directives here (included inside every server{} block)

# phpMyAdmin (if installed)
include /etc/nginx/snippets/phpmyadmin.conf;
SRVEOF
    log_detail "Config written: /etc/nginx/snippets/jcwt-server-common.conf"

    # Generate self-signed cert for nginx default HTTPS catch-all
    log_info "Generating nginx default SSL certificate..."
    mkdir -p /etc/nginx/ssl
    openssl req -x509 -nodes -days 3650 -newkey rsa:2048 \
        -keyout /etc/nginx/ssl/default.key \
        -out /etc/nginx/ssl/default.crt \
        -subj "/CN=localhost/O=Default/C=US" 2>/dev/null
    chmod 600 /etc/nginx/ssl/default.key
    chmod 644 /etc/nginx/ssl/default.crt
    log_ok "Nginx default SSL certificate generated"

    # Default catch-all vhost — shows a welcome page for DNS pointing to this server
    # that doesn't match any configured site (prevents panel login from showing)
    log_info "Creating default catch-all vhost..."
    mkdir -p /var/www/default
    cat > /var/www/default/index.html << 'DEFAULTHTML'
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Welcome</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            min-height: 100vh;
            display: flex;
            align-items: center;
            justify-content: center;
            background: linear-gradient(135deg, #f8fafc 0%, #e2e8f0 100%);
            color: #334155;
        }
        .container {
            text-align: center;
            padding: 3rem 2rem;
            max-width: 520px;
        }
        .icon {
            width: 80px; height: 80px;
            margin: 0 auto 1.5rem;
            background: linear-gradient(135deg, #6366f1, #8b5cf6);
            border-radius: 20px;
            display: flex;
            align-items: center;
            justify-content: center;
            box-shadow: 0 8px 32px rgba(99,102,241,0.25);
        }
        .icon svg { width: 40px; height: 40px; }
        h1 { font-size: 1.75rem; font-weight: 700; margin-bottom: 0.75rem; color: #1e293b; }
        p { font-size: 1.05rem; line-height: 1.7; color: #64748b; margin-bottom: 0.5rem; }
        .badge {
            display: inline-block; margin-top: 1.5rem; padding: 0.4rem 1rem;
            background: #f1f5f9; border-radius: 999px;
            font-size: 0.8rem; color: #94a3b8; letter-spacing: 0.02em;
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="icon">
            <svg viewBox="0 0 24 24" fill="none" stroke="white" stroke-width="2">
                <circle cx="12" cy="12" r="10"/>
                <line x1="2" y1="12" x2="22" y2="12"/>
                <path d="M12 2a15.3 15.3 0 0 1 4 10 15.3 15.3 0 0 1-4 10 15.3 15.3 0 0 1-4-10 15.3 15.3 0 0 1 4-10z"/>
            </svg>
        </div>
        <h1>Welcome</h1>
        <p>This server is operational and ready to serve your website.</p>
        <p>If you are the owner, please configure your domain in the hosting panel.</p>
        <div class="badge">Powered by JCWT Ultra Panel</div>
    </div>
</body>
</html>
DEFAULTHTML

    cat > /etc/nginx/sites-available/000-default.conf << 'DEFAULTVHOST'
# JCWT Ultra Panel — Default catch-all vhost
# Serves a welcome page for any domain not matching a configured site
server {
    listen [::]:80 default_server;
    server_name _;

    root /var/www/default;
    index index.html;

    location / {
        try_files $uri $uri/ =404;
    }

    # Common server-level includes (phpMyAdmin, etc.)
    include /etc/nginx/snippets/jcwt-server-common.conf;

    # Prevent access to hidden files
    location ~ /\. {
        deny all;
    }
}

# HTTPS catch-all — returns the same welcome page for unrecognized domains on 443
server {
    listen [::]:443 ssl default_server;
    server_name _;

    ssl_certificate     /etc/nginx/ssl/default.crt;
    ssl_certificate_key /etc/nginx/ssl/default.key;

    root /var/www/default;
    index index.html;

    location / {
        try_files $uri $uri/ =404;
    }

    include /etc/nginx/snippets/jcwt-server-common.conf;

    location ~ /\. {
        deny all;
    }
}
DEFAULTVHOST
    ln -sf /etc/nginx/sites-available/000-default.conf /etc/nginx/sites-enabled/000-default.conf
    log_ok "Default catch-all vhost created"

    log_info "Testing Nginx configuration..."
    if nginx -t 2>&1; then
        log_ok "Nginx config test passed"
    else
        log_error "Nginx config test FAILED — dumping error:"
        nginx -t 2>&1 || true
        log_warn "Attempting to fix by removing problematic configs and retesting..."
        # Remove configs that may have bare location blocks or conflicts
        rm -f /etc/nginx/conf.d/jcwt-phpmyadmin.conf
        rm -f /etc/nginx/conf.d/phpmyadmin.conf
        rm -f /etc/nginx/conf.d/*phpmyadmin*
        if nginx -t 2>/dev/null; then
            log_warn "Removed conflicting phpMyAdmin config from conf.d — Nginx works now"
        else
            rm -f /etc/nginx/conf.d/jcwt-optimization.conf
            if nginx -t 2>/dev/null; then
                log_warn "Removed jcwt-optimization.conf — Nginx works with defaults"
            fi
        fi
    fi

    log_info "Restarting Nginx..."
    systemctl restart nginx
    systemctl enable nginx > /dev/null 2>&1
    log_ok "Nginx configured and running"
}

# ---- Configure PHP-FPM ----
configure_php() {
    step_header "Configuring PHP-FPM Services"

    mkdir -p /var/log/php

    for VER in 8.2 8.3 8.4 8.5; do
        if [ -d "/etc/php/$VER/fpm" ]; then
            log_info "Starting PHP-FPM ${BOLD}$VER${NC}..."

            # Apply security hardening to php.ini (FPM + CLI)
            for PHPINI in /etc/php/$VER/fpm/php.ini /etc/php/$VER/cli/php.ini; do
                if [ -f "$PHPINI" ]; then
                    # Session security
                    sed -i 's/^session\.cookie_httponly.*/session.cookie_httponly = 1/' "$PHPINI"
                    sed -i 's/^;session\.cookie_httponly.*/session.cookie_httponly = 1/' "$PHPINI"
                    sed -i 's/^session\.cookie_secure.*/session.cookie_secure = 1/' "$PHPINI"
                    sed -i 's/^;session\.cookie_secure.*/session.cookie_secure = 1/' "$PHPINI"
                    sed -i 's/^session\.use_strict_mode.*/session.use_strict_mode = 1/' "$PHPINI"
                    sed -i 's/^;session\.use_strict_mode.*/session.use_strict_mode = 1/' "$PHPINI"
                    sed -i 's/^session\.use_only_cookies.*/session.use_only_cookies = 1/' "$PHPINI"
                    sed -i 's/^session\.cookie_samesite.*/session.cookie_samesite = Lax/' "$PHPINI"
                    sed -i 's/^;session\.cookie_samesite.*/session.cookie_samesite = Lax/' "$PHPINI"
                    # Hide PHP version
                    sed -i 's/^expose_php.*/expose_php = Off/' "$PHPINI"
                    # Disable dangerous URL include (allow_url_fopen left enabled for WordPress remote calls)
                    sed -i 's/^allow_url_include.*/allow_url_include = Off/' "$PHPINI"
                    # Limit request data
                    sed -i 's/^max_input_vars.*/max_input_vars = 5000/' "$PHPINI"
                fi
            done

            systemctl restart php${VER}-fpm 2>/dev/null || true
            systemctl enable php${VER}-fpm 2>/dev/null || true

            # Show socket path
            SOCK_PATH="/run/php/php${VER}-fpm.sock"
            if [ -S "$SOCK_PATH" ]; then
                log_detail "Socket: $SOCK_PATH"
                log_ok "PHP-FPM $VER active"
            else
                log_detail "Socket not found at $SOCK_PATH (may use TCP)"
                log_ok "PHP-FPM $VER started"
            fi
        fi
    done
}

# ---- Create panel user and directories ----
setup_panel() {
    step_header "Setting Up Panel User & Directories"

    # Create panel system user
    if ! id "$PANEL_USER" > /dev/null 2>&1; then
        log_info "Creating system user: ${BOLD}$PANEL_USER${NC}..."
        useradd --system --shell /usr/sbin/nologin --home-dir "$DATA_DIR" "$PANEL_USER"
        log_ok "System user '$PANEL_USER' created"
    else
        log_ok "System user '$PANEL_USER' already exists"
    fi

    # Create directories
    log_info "Creating directory structure..."
    mkdir -p "$DATA_DIR"/{tls,ssl,uploads}
    mkdir -p "$CONFIG_DIR"
    mkdir -p "$LOG_DIR"
    log_detail "$DATA_DIR/"
    log_detail "├── tls/     (panel TLS certificates)"
    log_detail "├── ssl/     (site SSL certificates)"
    log_detail "└── uploads/ (logo, favicon uploads)"
    log_detail "$LOG_DIR/ (panel logs)"

    # Generate self-signed TLS cert for panel
    log_info "Generating panel TLS certificate (10-year self-signed)..."
    openssl req -x509 -nodes -days 3650 -newkey rsa:2048 \
        -keyout "$DATA_DIR/tls/panel.key" \
        -out "$DATA_DIR/tls/panel.crt" \
        -subj "/CN=JCWT-Ultra-Panel/O=JCWT/C=US" 2>/dev/null

    chmod 600 "$DATA_DIR/tls/panel.key"
    chmod 644 "$DATA_DIR/tls/panel.crt"
    log_detail "Certificate: $DATA_DIR/tls/panel.crt"
    log_detail "Private Key: $DATA_DIR/tls/panel.key (mode 600)"

    chown -R "$PANEL_USER:$PANEL_USER" "$DATA_DIR"
    chown -R "$PANEL_USER:$PANEL_USER" "$LOG_DIR"

    log_ok "Panel directories created and secured"
}

# ---- Install panel binary ----
install_binary() {
    step_header "Installing Panel Binary"

    SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
    PROJECT_DIR="$(cd "$SCRIPT_DIR/.." 2>/dev/null && pwd || echo "")"

    # Check if the binary exists in the current directory or build it
    if [ -f "./jcwt-panel" ]; then
        log_info "Found pre-built binary in current directory"
        cp ./jcwt-panel "$PANEL_BIN"
        log_ok "Binary copied to $PANEL_BIN"
    elif [ -f "$PROJECT_DIR/jcwt-panel" ]; then
        log_info "Found pre-built binary in project root"
        cp "$PROJECT_DIR/jcwt-panel" "$PANEL_BIN"
        log_ok "Binary copied to $PANEL_BIN"
    elif [ -f "$PROJECT_DIR/cmd/jcwt-panel/main.go" ]; then
        log_info "Source code found — building from source..."

        if ! command -v go > /dev/null 2>&1; then
            log_info "Go not found — installing Go toolchain..."

            # Detect architecture for Go download
            case "$(uname -m)" in
                aarch64|arm64) GO_ARCH="arm64" ;;
                x86_64)        GO_ARCH="amd64" ;;
                *)             GO_ARCH="amd64" ;;
            esac

            GOVERSION="1.22.5"
            GO_URL="https://go.dev/dl/go${GOVERSION}.linux-${GO_ARCH}.tar.gz"
            log_detail "Downloading: $GO_URL"
            wget -q --show-progress "$GO_URL" -O /tmp/go.tar.gz
            tar -C /usr/local -xzf /tmp/go.tar.gz
            export PATH=$PATH:/usr/local/go/bin
            rm /tmp/go.tar.gz
            log_ok "Go ${GOVERSION} (${GO_ARCH}) installed"
        else
            GO_INSTALLED=$(go version | awk '{print $3}')
            log_ok "Go already installed: $GO_INSTALLED"
        fi

        export PATH=$PATH:/usr/local/go/bin

        cd "$PROJECT_DIR"

        log_info "Downloading Go dependencies..."
        go mod tidy 2>&1 | { grep -v "^$" || true; } | while read -r line; do
            log_detail "$line"
        done
        log_ok "Dependencies resolved (go.sum generated)"

        log_info "Building JCWT Ultra Panel binary (this may take a minute)..."
        log_detail "CGO_ENABLED=1 go build -o $PANEL_BIN ./cmd/jcwt-panel/"
        if CGO_ENABLED=1 go build -o "$PANEL_BIN" ./cmd/jcwt-panel/ 2>&1; then
            log_ok "Compilation successful"
        else
            log_error "Compilation failed — see errors above"
        fi

        cd - > /dev/null

        if [ ! -f "$PANEL_BIN" ]; then
            log_error "Build failed — binary not created at $PANEL_BIN"
            log_error "Check: cd $PROJECT_DIR && go build -v ./cmd/jcwt-panel/"
            exit 1
        fi
        log_ok "Panel binary built successfully"
    else
        log_error "Panel binary not found!"
        log_error "Either:"
        log_error "  • Place a pre-built 'jcwt-panel' binary in the current directory"
        log_error "  • Or run this installer from the project root (with cmd/jcwt-panel/main.go)"
        exit 1
    fi

    chmod +x "$PANEL_BIN"
    chown root:root "$PANEL_BIN"

    BIN_SIZE=$(du -h "$PANEL_BIN" | awk '{print $1}')
    log_ok "Binary installed: $PANEL_BIN ($BIN_SIZE)"
}

# ---- Install systemd service ----
install_service() {
    step_header "Installing Systemd Service"

    log_info "Writing service unit file..."
    cat > /etc/systemd/system/jcwt-panel.service << EOF
[Unit]
Description=JCWT Ultra Panel - IPv6-Native Hosting Control Panel
After=network.target mariadb.service nginx.service
Wants=mariadb.service nginx.service

[Service]
Type=simple
User=$PANEL_USER
Group=$PANEL_USER
ExecStart=$PANEL_BIN --data-dir $DATA_DIR --listen [::]:$PANEL_PORT
Restart=always
RestartSec=5
StandardOutput=append:$LOG_DIR/panel.log
StandardError=append:$LOG_DIR/panel.log

# Security hardening (panel uses sudo for privileged operations)
NoNewPrivileges=false
ProtectHome=false
PrivateTmp=true
ProtectSystem=yes
ReadWritePaths=$DATA_DIR /etc/nginx /etc/php /home /etc/logrotate.d $LOG_DIR /etc/default /usr/share/phpmyadmin
ProtectKernelTunables=true
ProtectKernelModules=true
ProtectKernelLogs=true
ProtectControlGroups=true
RestrictSUIDSGID=true
LockPersonality=true
RestrictNamespaces=true
SystemCallArchitectures=native
RestrictAddressFamilies=AF_INET AF_INET6 AF_UNIX
MemoryDenyWriteExecute=true
RemoveIPC=true
AmbientCapabilities=CAP_NET_BIND_SERVICE

[Install]
WantedBy=multi-user.target
EOF
    log_detail "Service: /etc/systemd/system/jcwt-panel.service"
    log_detail "User: $PANEL_USER"
    log_detail "Exec: $PANEL_BIN --data-dir $DATA_DIR --listen [::]:$PANEL_PORT"

    log_info "Configuring sudo privileges..."
    cat > /etc/sudoers.d/jcwt-panel << 'EOF'
# JCWT Ultra Panel - Scoped privileges for system management
# NO wildcard bash, rm, cat, tee — all file ops use specific arg patterns

# User management (only useradd/userdel/usermod with controlled args)
jcwt-panel ALL=(root) NOPASSWD: /usr/sbin/useradd -m -d /home/[a-z]* -s /bin/bash [a-z]*
jcwt-panel ALL=(root) NOPASSWD: /usr/sbin/useradd -m -d /home/[a-z]* -s /usr/sbin/nologin [a-z]*
jcwt-panel ALL=(root) NOPASSWD: /usr/sbin/userdel -r [a-z]*
jcwt-panel ALL=(root) NOPASSWD: /usr/sbin/usermod -s /bin/bash [a-z]*
jcwt-panel ALL=(root) NOPASSWD: /usr/sbin/usermod -s /usr/sbin/nologin [a-z]*
jcwt-panel ALL=(root) NOPASSWD: /usr/sbin/usermod -aG [a-z]* www-data
jcwt-panel ALL=(root) NOPASSWD: /usr/sbin/groupdel [a-z]*

# Systemd service control (only allowed services)
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

# Nginx
jcwt-panel ALL=(root) NOPASSWD: /usr/sbin/nginx -t

# MariaDB client (admin socket auth, no shell needed)
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

# File operations (scoped to allowed paths only)
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
jcwt-panel ALL=(root) NOPASSWD: /usr/bin/rm -rf /home/[a-z]*
jcwt-panel ALL=(root) NOPASSWD: /usr/bin/rm -f /var/lib/jcwt-panel/ssl/*
jcwt-panel ALL=(root) NOPASSWD: /usr/bin/rm -f /etc/logrotate.d/*
jcwt-panel ALL=(root) NOPASSWD: /usr/bin/rm -f /run/php/php*-fpm-*.sock
jcwt-panel ALL=(root) NOPASSWD: /usr/bin/rm -f /etc/php/*/fpm/pool.d/*.conf
jcwt-panel ALL=(root) NOPASSWD: /usr/bin/ln -sf /etc/nginx/sites-available/* /etc/nginx/sites-enabled/*
jcwt-panel ALL=(root) NOPASSWD: /usr/bin/tee /etc/nginx/sites-available/*
jcwt-panel ALL=(root) NOPASSWD: /usr/bin/tee /etc/php/*/fpm/pool.d/*.conf
jcwt-panel ALL=(root) NOPASSWD: /usr/bin/tee /etc/logrotate.d/*
jcwt-panel ALL=(root) NOPASSWD: /usr/bin/tee /etc/default/ufw
jcwt-panel ALL=(root) NOPASSWD: /usr/bin/tee /home/[a-z]*
jcwt-panel ALL=(root) NOPASSWD: /usr/bin/tee /usr/share/phpmyadmin/signon_*.php
jcwt-panel ALL=(root) NOPASSWD: /usr/bin/rm -f /usr/share/phpmyadmin/signon_*.php

# Tar/archive operations (scoped)
jcwt-panel ALL=(root) NOPASSWD: /usr/bin/tar -czf /home/[a-z]*
jcwt-panel ALL=(root) NOPASSWD: /usr/bin/tar -xzf /home/[a-z]*
jcwt-panel ALL=(root) NOPASSWD: /usr/bin/tar -tzf /home/[a-z]*
jcwt-panel ALL=(root) NOPASSWD: /usr/bin/tar cf - -C /home/[a-z]*
jcwt-panel ALL=(root) NOPASSWD: /usr/bin/tar xf - -C /home/[a-z]*

# Disk usage (read-only)
jcwt-panel ALL=(root) NOPASSWD: /usr/bin/du -sh /home/[a-z]*
jcwt-panel ALL=(root) NOPASSWD: /usr/bin/du -b /home/[a-z]*
jcwt-panel ALL=(root) NOPASSWD: /usr/bin/test -f /home/[a-z]*

# Log viewing
jcwt-panel ALL=(root) NOPASSWD: /usr/bin/tail -n [0-9]* /home/[a-z]*/logs/*

# Timezone
jcwt-panel ALL=(root) NOPASSWD: /usr/bin/timedatectl set-timezone *

# htpasswd
jcwt-panel ALL=(root) NOPASSWD: /usr/bin/htpasswd -c -B -b /etc/nginx/htpasswd/*
jcwt-panel ALL=(root) NOPASSWD: /usr/bin/htpasswd -B -b /etc/nginx/htpasswd/*

# Firewall
jcwt-panel ALL=(root) NOPASSWD: /usr/sbin/ufw status *
jcwt-panel ALL=(root) NOPASSWD: /usr/sbin/ufw allow *
jcwt-panel ALL=(root) NOPASSWD: /usr/sbin/ufw deny *
jcwt-panel ALL=(root) NOPASSWD: /usr/sbin/ufw delete *
jcwt-panel ALL=(root) NOPASSWD: /usr/sbin/ufw --force enable
jcwt-panel ALL=(root) NOPASSWD: /usr/sbin/ufw --force reset
jcwt-panel ALL=(root) NOPASSWD: /usr/sbin/ufw default *
jcwt-panel ALL=(root) NOPASSWD: /usr/sbin/ufw reload

# Rsync (for backup restore)
jcwt-panel ALL=(root) NOPASSWD: /usr/bin/rsync -a --delete /home/[a-z]*

# Wget (only to /home and /tmp paths)
jcwt-panel ALL=(root) NOPASSWD: /usr/bin/wget -q https\://wordpress.org/* -O /home/[a-z]*
jcwt-panel ALL=(root) NOPASSWD: /usr/bin/wget -q https\://raw.githubusercontent.com/wp-cli/* -O /usr/local/bin/wp

# WP-CLI and PHP (run as site user only)
jcwt-panel ALL=(ALL) NOPASSWD: /usr/local/bin/filebrowser *
jcwt-panel ALL=(ALL) NOPASSWD: /usr/bin/php8.2 /usr/local/bin/wp *
jcwt-panel ALL=(ALL) NOPASSWD: /usr/bin/php8.3 /usr/local/bin/wp *
jcwt-panel ALL=(ALL) NOPASSWD: /usr/bin/php8.4 /usr/local/bin/wp *
jcwt-panel ALL=(ALL) NOPASSWD: /usr/bin/php8.5 /usr/local/bin/wp *
EOF
    chmod 440 /etc/sudoers.d/jcwt-panel
    log_detail "Sudoers: /etc/sudoers.d/jcwt-panel (mode 440)"

    systemctl daemon-reload
    systemctl enable jcwt-panel > /dev/null 2>&1

    # Fix any PHP-FPM pool files written by older panel versions that incorrectly
    # combined pm = dynamic with pm.process_idle_timeout (fatal for PHP-FPM 8.x).
    log_info "Validating PHP-FPM pool configurations..."
    for phpver_dir in /etc/php/*/fpm/pool.d; do
        [ -d "$phpver_dir" ] || continue
        for f in "$phpver_dir"/*.conf; do
            [ -f "$f" ] || continue
            if grep -q '^pm = dynamic' "$f" && grep -q '^pm\.process_idle_timeout' "$f"; then
                sed -i '/^pm\.process_idle_timeout/d' "$f"
                log_detail "Fixed pool: $f"
            fi
        done
    done

    log_info "Starting JCWT Ultra Panel service..."
    if systemctl start jcwt-panel; then
        sleep 2
        if systemctl is-active --quiet jcwt-panel; then
            log_ok "Panel service is running!"
        else
            log_warn "Panel service started but may have issues"
            log_warn "Check logs: journalctl -u jcwt-panel -n 20"
        fi
    else
        log_error "Failed to start panel service"
        log_error "Check: journalctl -u jcwt-panel -n 20"
    fi
}

# ---- Configure Firewall ----
configure_firewall() {
    step_header "Configuring IPv6 Firewall"

    log_info "Ensuring UFW supports IPv6..."
    sed -i 's/IPV6=no/IPV6=yes/' /etc/default/ufw 2>/dev/null || true
    log_detail "SET IPV6=yes in /etc/default/ufw"

    log_info "Resetting firewall rules..."
    ufw --force reset > /dev/null 2>&1

    log_info "Setting default policies..."
    ufw default deny incoming > /dev/null 2>&1
    ufw default allow outgoing > /dev/null 2>&1
    log_detail "Default: deny incoming, allow outgoing"

    log_info "Adding firewall rules..."
    ufw allow 22/tcp > /dev/null 2>&1
    log_detail "ALLOW 22/tcp   — SSH"
    ufw allow 80/tcp > /dev/null 2>&1
    log_detail "ALLOW 80/tcp   — HTTP"
    ufw allow 443/tcp > /dev/null 2>&1
    log_detail "ALLOW 443/tcp  — HTTPS"
    ufw allow ${PANEL_PORT}/tcp > /dev/null 2>&1
    log_detail "ALLOW ${PANEL_PORT}/tcp — JCWT Panel"

    ufw --force enable > /dev/null 2>&1
    log_ok "Firewall enabled with IPv6 support"
}

# ---- Harden SSH ----
harden_ssh() {
    step_header "Hardening SSH Configuration"

    SSHD_CONFIG="/etc/ssh/sshd_config"

    log_info "Disabling password authentication..."
    # Set PasswordAuthentication no
    if grep -q '^#\?PasswordAuthentication' "$SSHD_CONFIG"; then
        sed -i 's/^#\?PasswordAuthentication.*/PasswordAuthentication no/' "$SSHD_CONFIG"
    else
        echo 'PasswordAuthentication no' >> "$SSHD_CONFIG"
    fi
    log_detail "PasswordAuthentication no"

    # Disable challenge-response auth (another password vector)
    if grep -q '^#\?KbdInteractiveAuthentication' "$SSHD_CONFIG"; then
        sed -i 's/^#\?KbdInteractiveAuthentication.*/KbdInteractiveAuthentication no/' "$SSHD_CONFIG"
    else
        echo 'KbdInteractiveAuthentication no' >> "$SSHD_CONFIG"
    fi
    if grep -q '^#\?ChallengeResponseAuthentication' "$SSHD_CONFIG"; then
        sed -i 's/^#\?ChallengeResponseAuthentication.*/ChallengeResponseAuthentication no/' "$SSHD_CONFIG"
    fi
    log_detail "KbdInteractiveAuthentication no"

    log_info "Disabling root SSH login..."
    if grep -q '^#\?PermitRootLogin' "$SSHD_CONFIG"; then
        sed -i 's/^#\?PermitRootLogin.*/PermitRootLogin no/' "$SSHD_CONFIG"
    else
        echo 'PermitRootLogin no' >> "$SSHD_CONFIG"
    fi
    log_detail "PermitRootLogin no"

    log_info "Enabling public key authentication..."
    if grep -q '^#\?PubkeyAuthentication' "$SSHD_CONFIG"; then
        sed -i 's/^#\?PubkeyAuthentication.*/PubkeyAuthentication yes/' "$SSHD_CONFIG"
    else
        echo 'PubkeyAuthentication yes' >> "$SSHD_CONFIG"
    fi
    log_detail "PubkeyAuthentication yes"

    # Ensure the running user's authorized_keys is preserved
    # (so the admin doesn't lock themselves out)
    log_info "Validating SSH config..."
    if sshd -t 2>/dev/null; then
        systemctl restart sshd > /dev/null 2>&1 || systemctl restart ssh > /dev/null 2>&1
        log_ok "SSH hardened: password auth disabled, root login disabled, key-based auth only"
    else
        log_error "SSH config validation failed — reverting changes"
        # Attempt to restore from backup if available
        if [ -f "${SSHD_CONFIG}.bak" ]; then
            cp "${SSHD_CONFIG}.bak" "$SSHD_CONFIG"
        fi
        log_warn "SSH configuration was not changed"
    fi
}

# ---- Print completion banner ----
print_banner() {
    echo ""
    echo ""

    # Get service statuses with colors
    get_status() {
        local status
        status=$(systemctl is-active "$1" 2>/dev/null || echo "inactive")
        if [ "$status" = "active" ]; then
            echo -e "${GREEN}● active${NC}"
        else
            echo -e "${RED}● $status${NC}"
        fi
    }

    echo -e "${GREEN}${BOLD}"
    echo "  ╔══════════════════════════════════════════════════════════╗"
    echo "  ║                                                          ║"
    echo -e "  ║   ${PURPLE}✨ JCWT Ultra Panel installed successfully! ✨${GREEN}       ║"
    echo "  ║                                                          ║"
    echo "  ╚══════════════════════════════════════════════════════════╝"
    echo -e "${NC}"

    echo -e "  ${BOLD}Access Your Panel${NC}"
    echo -e "  ─────────────────────────────────────────"
    echo -e "  ${CYAN}URL:${NC}       https://[${IPV6_ADDR}]:${PANEL_PORT}"
    echo ""

    # Check whether setup is still needed by querying the running panel API.
    # This is reliable even on re-installs because it checks live state, not log files.
    NEEDS_SETUP=true
    SETUP_STATUS=$(curl -sk --max-time 5 "https://[::1]:${PANEL_PORT}/api/setup/status" 2>/dev/null)
    if echo "$SETUP_STATUS" | grep -q '"needs_setup":false'; then
        NEEDS_SETUP=false
    fi

    if [ "$NEEDS_SETUP" = "true" ]; then
        # Extract the setup token from the log — this is freshly generated on
        # each start when no admin exists, so tail -1 gives the current token.
        SETUP_TOKEN=$(grep -oP 'Setup Token: \K[0-9a-f]+' "$LOG_DIR/panel.log" 2>/dev/null | tail -1)
        if [ -n "$SETUP_TOKEN" ]; then
            echo -e "  ${YELLOW}${BOLD}First-Time Setup Required${NC}"
            echo -e "  Open the URL above and use this one-time token"
            echo -e "  to create your admin account:"
            echo ""
            echo -e "  ${CYAN}Setup Token:${NC}  ${GREEN}${BOLD}${SETUP_TOKEN}${NC}"
            echo ""
            echo -e "  ${DIM}This token can only be used once.${NC}"
        fi
    else
        echo -e "  ${DIM}An admin account already exists. Log in with your credentials.${NC}"
    fi
    echo ""

    echo -e "  ${BOLD}Service Status${NC}"
    echo -e "  ─────────────────────────────────────────"
    echo -e "  Nginx .............. $(get_status nginx)"
    echo -e "  MariaDB ............ $(get_status mariadb)"
    echo -e "  Redis .............. $(get_status redis-server)"
    echo -e "  JCWT Panel ......... $(get_status jcwt-panel)"
    for VER in 8.2 8.3 8.4 8.5; do
        if systemctl list-unit-files "php${VER}-fpm.service" > /dev/null 2>&1; then
            echo -e "  PHP-FPM $VER ........ $(get_status php${VER}-fpm)"
        fi
    done
    echo ""

    echo -e "  ${BOLD}Important Paths${NC}"
    echo -e "  ─────────────────────────────────────────"
    echo -e "  ${DIM}Panel logs:${NC}    $LOG_DIR/panel.log"
    echo -e "  ${DIM}Panel data:${NC}    $DATA_DIR/"
    echo -e "  ${DIM}Nginx sites:${NC}   /etc/nginx/sites-available/"
    echo -e "  ${DIM}PHP configs:${NC}   /etc/php/"
    echo -e "  ${DIM}Web roots:${NC}     /home/<user>/htdocs/"
    echo ""

    echo -e "  ${BOLD}Useful Commands${NC}"
    echo -e "  ─────────────────────────────────────────"
    echo -e "  ${DIM}Panel status:${NC}  systemctl status jcwt-panel"
    echo -e "  ${DIM}Panel logs:${NC}    tail -f $LOG_DIR/panel.log"
    echo -e "  ${DIM}Restart:${NC}       systemctl restart jcwt-panel"
    echo ""
}

# ---- Main ----
main() {
    START_TIME=$(date +%s)

    preflight
    install_packages
    configure_mariadb
    configure_nginx
    configure_php
    setup_panel
    install_binary
    install_service
    configure_firewall
    harden_ssh

    END_TIME=$(date +%s)
    ELAPSED=$((END_TIME - START_TIME))
    MINUTES=$((ELAPSED / 60))
    SECONDS_REM=$((ELAPSED % 60))

    print_banner
    echo -e "  ${DIM}Installation completed in ${MINUTES}m ${SECONDS_REM}s${NC}"
    echo ""
}

main "$@"
