// JCWT Ultra Panel — Route Constants
// Centralised route definitions to avoid scattered magic strings.

export const ROUTES = {
    SETUP:               '/setup',
    LOGIN:               '/login',
    DASHBOARD:           '/dashboard',
    SITES:               '/sites',
    SITE_DETAIL:         '/sites/:token',
    SERVICES:            '/services',
    FIREWALL:            '/firewall',
    DISK_USAGE:          '/disk-usage',
    USERS:               '/users',
    SETTINGS:            '/settings',
    BRANDING:            '/branding-appearance',
    LOGIN_SECURITY:      '/login-security',
    SMTP_SETTINGS:       '/smtp-settings',
    EMAIL_NOTIFICATIONS: '/email-notifications',
    BACKUP_CONFIG:       '/backup-config',
};

// Mapping from pageName (derived from URL) → page title for the header bar.
// Pages that set their own title (e.g. site-detail) can still override.
export const PAGE_TITLES = {
    'dashboard':           'Dashboard',
    'sites':               'Sites',
    'site-detail':         'Site Management',
    'services':            'Services',
    'firewall':            'Firewall',
    'disk-usage':          'Site Disk Usage',
    'users':               'User Management',
    'branding-appearance': 'Branding & Appearance',
    'login-security':      'Login Security',
    'smtp-settings':       'SMTP Settings',
    'email-notifications': 'Email Notifications',
    'backup-config':       'Backup Configuration',
    'databases':           'Databases',
    'settings':            'Panel Settings',
};

// Settings sub-page names for sidebar highlight logic
export const SETTINGS_PAGES = [
    'branding-appearance',
    'login-security',
    'backup-config',
    'smtp-settings',
    'email-notifications',
];

/**
 * Navigate to a hash route.
 * @param {string} route - One of ROUTES values
 * @param {string} [param] - Optional path parameter (e.g. site token)
 */
export function navigateTo(route, param) {
    const hash = param ? `#${route.replace(':token', param)}` : `#${route}`;
    window.location.hash = hash;
}

/**
 * Build a hash path for a site detail page.
 * @param {string} token - Site token
 * @param {string} [section] - Optional sub-section (e.g. 'ssl', 'cron')
 * @returns {string} Hash path like "#/sites/abc123" or "#/sites/abc123/ssl"
 */
export function siteHref(token, section) {
    const base = `#${ROUTES.SITES}/${token}`;
    return section ? `${base}/${section}` : base;
}

/**
 * Set the page title in the header bar.
 * @param {string} pageName - The page identifier from the route
 */
export function setPageTitle(pageName) {
    const el = document.getElementById('page-title');
    if (el && PAGE_TITLES[pageName]) {
        el.textContent = PAGE_TITLES[pageName];
    }
}
