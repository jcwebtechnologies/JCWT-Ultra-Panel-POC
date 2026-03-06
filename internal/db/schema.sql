-- JCWT Ultra Panel Schema
-- SQLite database for panel metadata

CREATE TABLE IF NOT EXISTS admin_users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS sites (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    domain TEXT UNIQUE NOT NULL,
    aliases TEXT DEFAULT '',
    system_user TEXT UNIQUE NOT NULL,
    site_type TEXT NOT NULL DEFAULT 'php',
    php_version TEXT NOT NULL DEFAULT '8.3',
    proxy_url TEXT DEFAULT '',
    web_root TEXT NOT NULL,
    ssl_type TEXT DEFAULT 'none',
    ssl_cert_path TEXT DEFAULT '',
    ssl_key_path TEXT DEFAULT '',
    basic_auth_enabled INTEGER DEFAULT 0,
    basic_auth_users TEXT DEFAULT '',
    delete_protection INTEGER DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS php_settings (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    site_id INTEGER NOT NULL UNIQUE REFERENCES sites(id) ON DELETE CASCADE,
    memory_limit TEXT DEFAULT '256M',
    max_execution_time INTEGER DEFAULT 30,
    max_input_time INTEGER DEFAULT 60,
    max_input_vars INTEGER DEFAULT 1000,
    post_max_size TEXT DEFAULT '64M',
    upload_max_filesize TEXT DEFAULT '64M',
    custom_directives TEXT DEFAULT ''
);

CREATE TABLE IF NOT EXISTS databases (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    site_id INTEGER NOT NULL REFERENCES sites(id) ON DELETE CASCADE,
    db_name TEXT UNIQUE NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS db_users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT UNIQUE NOT NULL,
    database_id INTEGER REFERENCES databases(id) ON DELETE CASCADE,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS cron_jobs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    site_id INTEGER NOT NULL REFERENCES sites(id) ON DELETE CASCADE,
    schedule TEXT NOT NULL,
    command TEXT NOT NULL,
    enabled INTEGER DEFAULT 1,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS panel_settings (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    panel_name TEXT DEFAULT 'JCWT Ultra Panel',
    panel_tagline TEXT DEFAULT 'Lightweight IPv6-Native Hosting Panel',
    logo_url TEXT DEFAULT '',
    favicon_url TEXT DEFAULT '',
    primary_color TEXT DEFAULT '#6366f1',
    accent_color TEXT DEFAULT '#818cf8',
    footer_text TEXT DEFAULT '© 2026 JCWT Ultra Panel',
    session_timeout INTEGER DEFAULT 30,
    allow_signup INTEGER DEFAULT 0,
    recaptcha_site_key TEXT DEFAULT '',
    recaptcha_secret_key TEXT DEFAULT '',
    timezone TEXT DEFAULT 'UTC'
);

-- Ensure there is always one row in panel_settings
INSERT OR IGNORE INTO panel_settings (id) VALUES (1);
