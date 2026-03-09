// JCWT Ultra Panel — API Client
// Handles all API communication with CSRF token management

let csrfToken = '';

export function setCsrfToken(token) {
    csrfToken = token;
}

export function getCsrfToken() {
    return csrfToken;
}

export async function request(url, options = {}) {
    const defaults = {
        headers: {
            'Content-Type': 'application/json',
        },
        credentials: 'same-origin',
    };

    if (options.method && options.method !== 'GET') {
        defaults.headers['X-CSRF-Token'] = csrfToken;
    }

    // Don't set Content-Type for FormData
    if (options.body instanceof FormData) {
        delete defaults.headers['Content-Type'];
    }

    const config = { ...defaults, ...options };
    if (options.headers) {
        config.headers = { ...defaults.headers, ...options.headers };
    }

    const response = await fetch(url, config);

    if (response.status === 401) {
        window.location.hash = '#/login';
        throw new Error('Unauthorized');
    }

    // Handle non-JSON responses (file downloads)
    const contentType = response.headers.get('content-type');
    if (contentType && !contentType.includes('application/json')) {
        return response;
    }

    const data = await response.json();
    if (!data.success) {
        throw new Error(data.error || 'Unknown error');
    }

    return data.data;
}

// Auth
export const auth = {
    check: () => request('/api/auth/check'),
    login: (username, password, captchaToken = '') => request('/api/auth/login', {
        method: 'POST',
        body: JSON.stringify({ username, password, captcha_token: captchaToken }),
    }),
    logout: () => request('/api/auth/logout', { method: 'POST' }),
    changePassword: (data) => request('/api/auth/change-password', { method: 'POST', body: JSON.stringify(data) }),
};

// Dashboard
export const dashboard = {
    stats: () => request('/api/dashboard'),
};

// Sites
export const sites = {
    list: () => request('/api/sites'),
    get: (id) => request(`/api/sites?id=${id}`),
    getByToken: (token) => request(`/api/sites?token=${encodeURIComponent(token)}`),
    create: (data) => request('/api/sites', { method: 'POST', body: JSON.stringify(data) }),
    update: (data) => request('/api/sites', { method: 'PUT', body: JSON.stringify(data) }),
    delete: (id) => request(`/api/sites?id=${id}`, { method: 'DELETE' }),
    diskUsage: (id) => request(`/api/sites?action=disk-usage&id=${id}`),
};

// Databases
export const databases = {
    list: () => request('/api/databases'),
    create: (data) => request('/api/databases', { method: 'POST', body: JSON.stringify(data) }),
    delete: (id) => request(`/api/databases?id=${id}`, { method: 'DELETE' }),
};

// DB Users
export const dbUsers = {
    list: () => request('/api/db-users'),
    create: (data) => request('/api/db-users', { method: 'POST', body: JSON.stringify(data) }),
    changePassword: (data) => request('/api/db-users', { method: 'PUT', body: JSON.stringify(data) }),
    updatePrivilege: (data) => request('/api/db-users?action=privilege', { method: 'PUT', body: JSON.stringify(data) }),
    delete: (id) => request(`/api/db-users?id=${id}`, { method: 'DELETE' }),
};

// SSL
export const ssl = {
    selfSigned: (siteId) => request(`/api/ssl-certs?type=self-signed&site_id=${siteId}`, { method: 'POST' }),
    custom: (siteId, formData) => request(`/api/ssl-certs?type=custom&site_id=${siteId}`, { method: 'POST', body: formData }),
    letsEncrypt: (siteId, domains) => request(`/api/ssl-certs?type=letsencrypt&site_id=${siteId}`, { method: 'POST', body: JSON.stringify({ domains }) }),
};

// Cron
export const cron = {
    list: (siteId) => request(`/api/cron?site_id=${siteId}`),
    create: (data) => request('/api/cron', { method: 'POST', body: JSON.stringify(data) }),
    update: (data) => request('/api/cron', { method: 'PUT', body: JSON.stringify(data) }),
    delete: (id, siteId) => request(`/api/cron?id=${id}&site_id=${siteId}`, { method: 'DELETE' }),
};

// Files (File Browser)
export const files = {
    list: (siteId) => request(`/api/files?site_id=${siteId}`),
    stop: (siteId) => request(`/api/files?site_id=${siteId}`, { method: 'DELETE' }),
};

// PHP Settings
export const phpSettings = {
    get: (siteId) => request(`/api/php-settings?site_id=${siteId}`),
    update: (data) => request('/api/php-settings', { method: 'PUT', body: JSON.stringify(data) }),
};

// PHP Versions
export const phpVersions = {
    list: () => request('/api/php-versions'),
};

// Panel Settings
export const settings = {
    get: () => request('/api/settings'),
    getPublic: () => request('/api/settings/public'),
    update: (data) => request('/api/settings', { method: 'PUT', body: JSON.stringify(data) }),
    uploadLogo: (formData) => request('/api/settings?action=upload-logo', { method: 'POST', body: formData }),
    uploadFavicon: (formData) => request('/api/settings?action=upload-favicon', { method: 'POST', body: formData }),
};

// Services
export const services = {
    list: () => request('/api/services'),
    start: (name) => request('/api/services?action=start', { method: 'POST', body: JSON.stringify({ service: name }) }),
    stop: (name) => request('/api/services?action=stop', { method: 'POST', body: JSON.stringify({ service: name }) }),
    restart: (name) => request('/api/services', { method: 'POST', body: JSON.stringify({ service: name }) }),
    reload: (name) => request('/api/services?action=reload', { method: 'POST', body: JSON.stringify({ service: name }) }),
};

// SMTP Settings
export const smtpSettings = {
    get: () => request('/api/smtp'),
    update: (data) => request('/api/smtp', { method: 'PUT', body: JSON.stringify(data) }),
    testEmail: (to, contentType = 'plain') => request('/api/smtp?action=test', { method: 'POST', body: JSON.stringify({ to, content_type: contentType }) }),
};

// Two-Factor Authentication
export const twofa = {
    status: () => request('/api/auth/2fa?action=status'),
    setup: () => request('/api/auth/2fa?action=setup', { method: 'POST' }),
    enable: (secret, code) => request('/api/auth/2fa?action=enable', { method: 'POST', body: JSON.stringify({ secret, code }) }),
    disable: (password) => request('/api/auth/2fa?action=disable', { method: 'POST', body: JSON.stringify({ password }) }),
    verify: (token, code) => request('/api/auth/2fa/verify', { method: 'POST', body: JSON.stringify({ token, code }) }),
};

// Vhost Editor
export const vhost = {
    get: (siteId) => request(`/api/vhost?site_id=${siteId}`),
    update: (data) => request('/api/vhost', { method: 'PUT', body: JSON.stringify(data) }),
    reset: (data) => request('/api/vhost', { method: 'POST', body: JSON.stringify(data) }),
};

// Site Backups
export const backups = {
    list: (siteId) => request(`/api/backups?site_id=${siteId}`),
    create: (data) => request('/api/backups?action=create', { method: 'POST', body: JSON.stringify(data) }),
    restore: (data) => request('/api/backups?action=restore', { method: 'POST', body: JSON.stringify(data) }),
    delete: (id) => request(`/api/backups?id=${id}`, { method: 'DELETE' }),
    updateSchedule: (data) => request('/api/backups?action=schedule', { method: 'POST', body: JSON.stringify(data) }),
};

// Backup Methods (panel-wide)
export const backupMethods = {
    list: () => request('/api/backup-methods'),
    create: (data) => request('/api/backup-methods', { method: 'POST', body: JSON.stringify(data) }),
    update: (data) => request('/api/backup-methods', { method: 'PUT', body: JSON.stringify(data) }),
    delete: (id) => request(`/api/backup-methods?id=${id}`, { method: 'DELETE' }),
};

// Site Logs
export const logs = {
    get: (siteId, type = 'access', lines = 100) => request(`/api/logs?site_id=${siteId}&type=${type}&lines=${lines}`),
};

// SSL Certificates (multi-cert)
export const sslCerts = {
    list: (siteId) => request(`/api/ssl-certs?site_id=${siteId}`),
    createSelfSigned: (siteId) => request(`/api/ssl-certs?site_id=${siteId}&type=self-signed`, { method: 'POST' }),
    createCustom: (siteId, formData) => request(`/api/ssl-certs?site_id=${siteId}&type=custom`, { method: 'POST', body: formData }),
    activate: (data) => request('/api/ssl-certs?action=activate', { method: 'POST', body: JSON.stringify(data) }),
    delete: (id) => request(`/api/ssl-certs?id=${id}`, { method: 'DELETE' }),
};

// Firewall
export const firewall = {
    list: () => request('/api/firewall'),
    create: (data) => request('/api/firewall', { method: 'POST', body: JSON.stringify(data) }),
    update: (data) => request('/api/firewall', { method: 'PUT', body: JSON.stringify(data) }),
    delete: (id) => request(`/api/firewall?id=${id}`, { method: 'DELETE' }),
    toggle: (enable) => request('/api/firewall?action=toggle', { method: 'POST', body: JSON.stringify({ enable }) }),
};
