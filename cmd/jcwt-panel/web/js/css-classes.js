// JCWT Ultra Panel — CSS Class & Badge Constants
// Centralised class name mappings for badges, status indicators, and common patterns.
// Import and use these to reduce scattered magic strings.

export const BADGE = {
    SUCCESS: 'badge badge-success',
    WARNING: 'badge badge-warning',
    ERROR:   'badge badge-error',
    INFO:    'badge badge-info',
    PRIMARY: 'badge badge-primary',
    DEFAULT: 'badge badge-default',
};

/**
 * Return the appropriate badge class for an enabled/disabled state.
 */
export function enabledBadge(enabled) {
    return enabled ? BADGE.SUCCESS : BADGE.WARNING;
}

/**
 * Return badge class for SSL type.
 */
export function sslBadge(sslType) {
    return sslType === 'none' ? BADGE.WARNING : BADGE.SUCCESS;
}

/**
 * Return badge class for a service status.
 */
export function statusBadge(status) {
    switch (status) {
        case 'running': case 'active': return BADGE.SUCCESS;
        case 'stopped': case 'inactive': case 'failed': return BADGE.ERROR;
        default: return BADGE.WARNING;
    }
}
