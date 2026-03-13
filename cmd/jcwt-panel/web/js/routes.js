// JCWT Ultra Panel — Route Constants
// Centralised route definitions to avoid scattered magic strings.

export const ROUTES = {
    LOGIN:               '/login',
    DASHBOARD:           '/dashboard',
    SITES:               '/sites',
    SITE_DETAIL:         '/sites/:id',
    DATABASES:           '/databases',
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

/**
 * Navigate to a hash route.
 * @param {string} route - One of ROUTES values
 * @param {string} [param] - Optional path parameter (e.g. site id)
 */
export function navigateTo(route, param) {
    const hash = param ? `#${route.replace(':id', param)}` : `#${route}`;
    window.location.hash = hash;
}
