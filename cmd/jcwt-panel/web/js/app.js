// JCWT Ultra Panel — SPA Core
import { auth, setCsrfToken, settings as settingsApi, request, twofa, setup } from './api.js';

// ---- State ----
let currentUser = null;
let currentRole = null;
let panelSettings = null;

// ---- Icons (inline SVG) ----
export const icons = {
    dashboard: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="3" width="7" height="7" rx="1"/><rect x="14" y="3" width="7" height="7" rx="1"/><rect x="3" y="14" width="7" height="7" rx="1"/><rect x="14" y="14" width="7" height="7" rx="1"/></svg>',
    sites: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><line x1="2" y1="12" x2="22" y2="12"/><path d="M12 2a15.3 15.3 0 0 1 4 10 15.3 15.3 0 0 1-4 10 15.3 15.3 0 0 1-4-10 15.3 15.3 0 0 1 4-10z"/></svg>',
    database: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><ellipse cx="12" cy="5" rx="9" ry="3"/><path d="M21 12c0 1.66-4 3-9 3s-9-1.34-9-3"/><path d="M3 5v14c0 1.66 4 3 9 3s9-1.34 9-3V5"/></svg>',
    lock: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="11" width="18" height="11" rx="2" ry="2"/><path d="M7 11V7a5 5 0 0 1 10 0v4"/></svg>',
    clock: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><polyline points="12 6 12 12 16 14"/></svg>',
    folder: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"/></svg>',
    settings: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1 0 2.83 2 2 0 0 1-2.83 0l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-2 2 2 2 0 0 1-2-2v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83 0 2 2 0 0 1 0-2.83l.06-.06A1.65 1.65 0 0 0 4.68 15a1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1-2-2 2 2 0 0 1 2-2h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 0-2.83 2 2 0 0 1 2.83 0l.06.06A1.65 1.65 0 0 0 9 4.68a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 2-2 2 2 0 0 1 2 2v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 0 2 2 0 0 1 0 2.83l-.06.06A1.65 1.65 0 0 0 19.4 9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 2 2 2 2 0 0 1-2 2h-.09a1.65 1.65 0 0 0-1.51 1z"/></svg>',
    logout: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4"/><polyline points="16 17 21 12 16 7"/><line x1="21" y1="12" x2="9" y2="12"/></svg>',
    plus: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><line x1="12" y1="5" x2="12" y2="19"/><line x1="5" y1="12" x2="19" y2="12"/></svg>',
    edit: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7"/><path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z"/></svg>',
    trash: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="3 6 5 6 21 6"/><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/></svg>',
    file: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M13 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V9z"/><polyline points="13 2 13 9 20 9"/></svg>',
    upload: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="16 16 12 12 8 16"/><line x1="12" y1="12" x2="12" y2="21"/><path d="M20.39 18.39A5 5 0 0 0 18 9h-1.26A8 8 0 1 0 3 16.3"/></svg>',
    download: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="8 17 12 21 16 17"/><line x1="12" y1="12" x2="12" y2="21"/><path d="M20.88 18.09A5 5 0 0 0 18 9h-1.26A8 8 0 1 0 3 16.3"/></svg>',
    menu: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><line x1="3" y1="12" x2="21" y2="12"/><line x1="3" y1="6" x2="21" y2="6"/><line x1="3" y1="18" x2="21" y2="18"/></svg>',
    code: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="16 18 22 12 16 6"/><polyline points="8 6 2 12 8 18"/></svg>',
    sun: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="5"/><line x1="12" y1="1" x2="12" y2="3"/><line x1="12" y1="21" x2="12" y2="23"/><line x1="4.22" y1="4.22" x2="5.64" y2="5.64"/><line x1="18.36" y1="18.36" x2="19.78" y2="19.78"/><line x1="1" y1="12" x2="3" y2="12"/><line x1="21" y1="12" x2="23" y2="12"/><line x1="4.22" y1="19.78" x2="5.64" y2="18.36"/><line x1="18.36" y1="5.64" x2="19.78" y2="4.22"/></svg>',
    moon: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z"/></svg>',
    server: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="2" y="2" width="20" height="8" rx="2" ry="2"/><rect x="2" y="14" width="20" height="8" rx="2" ry="2"/><line x1="6" y1="6" x2="6.01" y2="6"/><line x1="6" y1="18" x2="6.01" y2="18"/></svg>',
    key: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 2l-2 2m-7.61 7.61a5.5 5.5 0 1 1-7.778 7.778 5.5 5.5 0 0 1 7.777-7.777zm0 0L15.5 7.5m0 0l3 3L22 7l-3-3m-3.5 3.5L19 4"/></svg>',
    refresh: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="23 4 23 10 17 10"/><polyline points="1 20 1 14 7 14"/><path d="M3.51 9a9 9 0 0 1 14.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0 0 20.49 15"/></svg>',
    palette: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="13.5" cy="6.5" r="0.5" fill="currentColor"/><circle cx="17.5" cy="10.5" r="0.5" fill="currentColor"/><circle cx="8.5" cy="7.5" r="0.5" fill="currentColor"/><circle cx="6.5" cy="12" r="0.5" fill="currentColor"/><path d="M12 2C6.5 2 2 6.5 2 12s4.5 10 10 10c.926 0 1.648-.746 1.648-1.688 0-.437-.18-.835-.437-1.125-.29-.289-.438-.652-.438-1.125a1.64 1.64 0 0 1 1.668-1.668h1.996c3.051 0 5.555-2.503 5.555-5.554C21.965 6.012 17.461 2 12 2z"/></svg>',
    target: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><circle cx="12" cy="12" r="6"/><circle cx="12" cy="12" r="2"/></svg>',
    shield: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"/></svg>',
    bot: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="11" width="18" height="10" rx="2"/><circle cx="12" cy="5" r="2"/><path d="M12 7v4"/><line x1="8" y1="16" x2="8" y2="16"/><line x1="16" y1="16" x2="16" y2="16"/></svg>',
    copy: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="9" y="9" width="13" height="13" rx="2" ry="2"/><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"/></svg>',
    mail: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M4 4h16c1.1 0 2 .9 2 2v12c0 1.1-.9 2-2 2H4c-1.1 0-2-.9-2-2V6c0-1.1.9-2 2-2z"/><polyline points="22,6 12,13 2,6"/></svg>',
    play: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polygon points="5 3 19 12 5 21 5 3"/></svg>',
    stop: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="3" width="18" height="18" rx="2" ry="2"/></svg>',
    refreshCw: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="23 4 23 10 17 10"/><path d="M20.49 15a9 9 0 1 1-2.12-9.36L23 10"/></svg>',
    search: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="11" cy="11" r="8"/><line x1="21" y1="21" x2="16.65" y2="16.65"/></svg>',
    alertTriangle: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"/><line x1="12" y1="9" x2="12" y2="13"/><line x1="12" y1="17" x2="12.01" y2="17"/></svg>',
    users: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2"/><circle cx="9" cy="7" r="4"/><path d="M23 21v-2a4 4 0 0 0-3-3.87"/><path d="M16 3.13a4 4 0 0 1 0 7.75"/></svg>',
    chevronLeft: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="15 18 9 12 15 6"/></svg>',
    pma: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="3" width="18" height="18" rx="2"/><path d="M3 9h18"/><path d="M3 15h18"/><path d="M9 3v18"/></svg>',
    eye: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"/><circle cx="12" cy="12" r="3"/></svg>',
    eyeOff: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M17.94 17.94A10.07 10.07 0 0 1 12 20c-7 0-11-8-11-8a18.45 18.45 0 0 1 5.06-5.94M9.9 4.24A9.12 9.12 0 0 1 12 4c7 0 11 8 11 8a18.5 18.5 0 0 1-2.16 3.19m-6.72-1.07a3 3 0 1 1-4.24-4.24"/><line x1="1" y1="1" x2="23" y2="23"/></svg>',
};

// ---- Toast Notifications ----
export function showToast(message, type = 'info') {
    let container = document.querySelector('.toast-container');
    if (!container) {
        container = document.createElement('div');
        container.className = 'toast-container';
        document.body.appendChild(container);
    }

    const toast = document.createElement('div');
    toast.className = `toast ${type}`;
    toast.innerHTML = `
        <span class="toast-message">${escapeHtml(message)}</span>
        <button class="toast-close" onclick="this.parentElement.remove()">×</button>
    `;
    container.appendChild(toast);

    setTimeout(() => toast.remove(), 4000);
}

// ---- Modal ----
export function showModal(title, content, footer = '', options = {}) {
    closeModal();
    const overlay = document.createElement('div');
    overlay.className = 'modal-overlay';
    overlay.id = 'modal-overlay';
    overlay.innerHTML = `
        <div class="modal">
            <div class="modal-header">
                <h3 class="modal-title">${escapeHtml(title)}</h3>
                <button class="modal-close" onclick="document.getElementById('modal-overlay').remove()">×</button>
            </div>
            <div class="modal-body">${content}</div>
            ${footer ? `<div class="modal-footer">${footer}</div>` : ''}
        </div>
    `;
    if (!options.persistent) {
        overlay.addEventListener('click', (e) => {
            if (e.target === overlay) overlay.remove();
        });
    } else {
        // Hide close button for persistent modals
        const closeBtn = overlay.querySelector('.modal-close');
        if (closeBtn) closeBtn.style.display = 'none';
    }
    document.body.appendChild(overlay);
    return overlay;
}

export function closeModal() {
    const existing = document.getElementById('modal-overlay');
    if (existing) existing.remove();
}

// ---- Confirm Dialog (replaces window.confirm) ----
export function showConfirm(title, message, confirmText = 'Confirm', confirmClass = 'btn-danger', { html = false } = {}) {
    return new Promise((resolve) => {
        closeModal();
        const overlay = document.createElement('div');
        overlay.className = 'modal-overlay';
        overlay.id = 'modal-overlay';
        overlay.innerHTML = `
            <div class="modal" style="max-width: 440px;">
                <div class="modal-header">
                    <h3 class="modal-title">${escapeHtml(title)}</h3>
                    <button class="modal-close" id="confirm-close-btn">×</button>
                </div>
                <div class="modal-body">
                    <p style="color: var(--text-secondary); font-size: var(--font-size-sm); line-height: 1.6;">${html ? message : escapeHtml(message)}</p>
                </div>
                <div class="modal-footer">
                    <button class="btn btn-secondary" id="confirm-cancel-btn">Cancel</button>
                    <button class="btn ${confirmClass}" id="confirm-ok-btn">${escapeHtml(confirmText)}</button>
                </div>
            </div>
        `;
        const cleanup = (result) => { overlay.remove(); resolve(result); };
        overlay.addEventListener('click', (e) => { if (e.target === overlay) cleanup(false); });
        overlay.querySelector('#confirm-close-btn').addEventListener('click', () => cleanup(false));
        overlay.querySelector('#confirm-cancel-btn').addEventListener('click', () => cleanup(false));
        overlay.querySelector('#confirm-ok-btn').addEventListener('click', () => cleanup(true));
        document.body.appendChild(overlay);
    });
}

/**
 * Show a styled prompt dialog (replaces native prompt()).
 * @param {string} title - Dialog title
 * @param {string} message - Description text
 * @param {string} [defaultValue=''] - Pre-filled input value
 * @returns {Promise<string|null>} The entered value, or null if cancelled
 */
export function showPrompt(title, message, defaultValue = '') {
    return new Promise((resolve) => {
        closeModal();
        const overlay = document.createElement('div');
        overlay.className = 'modal-overlay';
        overlay.id = 'modal-overlay';
        overlay.innerHTML = `
            <div class="modal" style="max-width: 440px;">
                <div class="modal-header">
                    <h3 class="modal-title">${escapeHtml(title)}</h3>
                    <button class="modal-close" id="prompt-close-btn">×</button>
                </div>
                <div class="modal-body">
                    <p style="color: var(--text-secondary); font-size: var(--font-size-sm); line-height: 1.6; margin-bottom: var(--space-3);">${escapeHtml(message)}</p>
                    <input type="text" class="form-input" id="prompt-input" value="${escapeHtml(defaultValue)}" autofocus>
                </div>
                <div class="modal-footer">
                    <button class="btn btn-secondary" id="prompt-cancel-btn">Cancel</button>
                    <button class="btn btn-primary" id="prompt-ok-btn">OK</button>
                </div>
            </div>
        `;
        const cleanup = (result) => { overlay.remove(); resolve(result); };
        overlay.addEventListener('click', (e) => { if (e.target === overlay) cleanup(null); });
        overlay.querySelector('#prompt-close-btn').addEventListener('click', () => cleanup(null));
        overlay.querySelector('#prompt-cancel-btn').addEventListener('click', () => cleanup(null));
        overlay.querySelector('#prompt-ok-btn').addEventListener('click', () => {
            cleanup(overlay.querySelector('#prompt-input').value);
        });
        overlay.querySelector('#prompt-input').addEventListener('keydown', (e) => {
            if (e.key === 'Enter') {
                e.preventDefault();
                cleanup(overlay.querySelector('#prompt-input').value);
            }
        });
        document.body.appendChild(overlay);
    });
}

// ---- Utilities ----
export function escapeHtml(str) {
    if (!str) return '';
    const div = document.createElement('div');
    div.textContent = str;
    return div.innerHTML;
}

export function formatBytes(bytes) {
    if (bytes === 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
}

export function getPanelSettings() {
    return panelSettings;
}

// ---- Password Change ----
function showPasswordChangeModal() {
    const content = `
        <form id="change-password-form">
            <div class="form-group">
                <label class="form-label">Current Password</label>
                <input type="password" class="form-input" id="cp-current" required minlength="1" placeholder="Enter current password">
            </div>
            <div class="form-group">
                <label class="form-label">New Password</label>
                <input type="password" class="form-input" id="cp-new" required minlength="8" placeholder="Min. 8 characters">
            </div>
            <div class="form-group">
                <label class="form-label">Confirm New Password</label>
                <input type="password" class="form-input" id="cp-confirm" required minlength="8" placeholder="Repeat new password">
            </div>
        </form>
    `;
    const footer = `
        <button class="btn btn-secondary" onclick="document.getElementById('modal-overlay').remove()">Cancel</button>
        <button class="btn btn-primary" id="cp-submit-btn">Change Password</button>
    `;
    const modal = showModal('Change Password', content, footer);

    modal.querySelector('#cp-submit-btn')?.addEventListener('click', async () => {
        const current = modal.querySelector('#cp-current').value;
        const newPw = modal.querySelector('#cp-new').value;
        const confirm = modal.querySelector('#cp-confirm').value;

        if (!current || !newPw || !confirm) {
            showToast('Please fill in all fields', 'error');
            return;
        }
        if (newPw.length < 8) {
            showToast('New password must be at least 8 characters', 'error');
            return;
        }
        if (newPw !== confirm) {
            showToast('New passwords do not match', 'error');
            return;
        }

        try {
            await request('/api/auth/change-password', {
                method: 'POST',
                body: JSON.stringify({ current_password: current, new_password: newPw }),
            });
            closeModal();
            showToast('Password changed successfully!', 'success');
        } catch (err) {
            showToast(err.message || 'Failed to change password', 'error');
        }
    });
}

async function show2FAModal() {
    try {
        const status = await twofa.status();
        if (status.enabled) {
            // 2FA is enabled — show disable option
            const content = `
                <div style="text-align: center; margin-bottom: var(--space-4);">
                    <div style="width: 56px; height: 56px; background: var(--status-success-soft); border-radius: 50%; display: flex; align-items: center; justify-content: center; margin: 0 auto var(--space-3);"><span class="nav-icon" style="width:28px;height:28px;color:var(--status-success)">${icons.shield}</span></div>
                    <h3 style="font-weight: 600; margin-bottom: var(--space-2);">2FA is Enabled</h3>
                    <p style="color: var(--text-secondary); font-size: var(--font-size-sm);">Your account is protected with two-factor authentication.</p>
                </div>
                <div class="form-group">
                    <label class="form-label">Enter your password to disable 2FA</label>
                    <input type="password" class="form-input" id="twofa-disable-pwd" placeholder="Current password" required>
                </div>
            `;
            const footer = `
                <button class="btn btn-secondary" onclick="document.getElementById('modal-overlay').remove()">Cancel</button>
                <button class="btn btn-danger" id="twofa-disable-btn">Disable 2FA</button>
            `;
            const modal = showModal('Two-Factor Authentication', content, footer);
            modal.querySelector('#twofa-disable-btn')?.addEventListener('click', async () => {
                const pwd = modal.querySelector('#twofa-disable-pwd').value;
                if (!pwd) { showToast('Password is required', 'error'); return; }
                try {
                    await twofa.disable(pwd);
                    closeModal();
                    showToast('Two-factor authentication disabled', 'success');
                } catch (err) { showToast(err.message, 'error'); }
            });
        } else {
            // 2FA is disabled — show setup flow
            const setupContent = `
                <div style="text-align: center; margin-bottom: var(--space-4);">
                    <div style="width: 56px; height: 56px; background: var(--accent-primary-soft); border-radius: 50%; display: flex; align-items: center; justify-content: center; margin: 0 auto var(--space-3);"><span class="nav-icon" style="width:28px;height:28px;color:var(--accent-primary)">${icons.lock}</span></div>
                    <p style="color: var(--text-secondary); font-size: var(--font-size-sm);">Add an extra layer of security to your account using an authenticator app.</p>
                </div>
                <div id="twofa-setup-area" style="text-align: center;">
                    <button class="btn btn-primary" id="twofa-generate-btn">Generate Secret Key</button>
                </div>
            `;
            const footer = `
                <button class="btn btn-secondary" onclick="document.getElementById('modal-overlay').remove()">Cancel</button>
            `;
            const modal = showModal('Set Up Two-Factor Auth', setupContent, footer);

            modal.querySelector('#twofa-generate-btn')?.addEventListener('click', async () => {
                const btn = modal.querySelector('#twofa-generate-btn');
                btn.disabled = true;
                btn.textContent = 'Generating...';
                try {
                    const data = await twofa.setup();
                    const area = modal.querySelector('#twofa-setup-area');
                    area.innerHTML = `
                        <div style="text-align: left;">
                            <p style="font-size: var(--font-size-sm); color: var(--text-secondary); margin-bottom: var(--space-3);">
                                <strong>Step 1:</strong> Scan this QR code with your authenticator app (Google Authenticator, Authy, etc.), or enter the key manually.
                            </p>
                            <div style="text-align: center; margin-bottom: var(--space-4);">
                                <img src="https://api.qrserver.com/v1/create-qr-code/?size=200x200&data=${encodeURIComponent(data.uri)}"
                                     alt="2FA QR Code" style="border-radius: var(--radius-md); border: 1px solid var(--border-primary); padding: var(--space-2); background: white;">
                            </div>
                            <div style="margin-bottom: var(--space-4);">
                                <label class="form-label">Manual Entry Key:</label>
                                <div style="display: flex; gap: var(--space-2);">
                                    <input type="text" class="form-input mono" value="${data.secret}" readonly id="twofa-secret-display" style="font-size: var(--font-size-xs);">
                                    <button class="btn btn-sm btn-secondary" id="twofa-copy-secret" title="Copy"><span class="nav-icon nav-icon-xs">${icons.copy}</span></button>
                                </div>
                            </div>
                            <p style="font-size: var(--font-size-sm); color: var(--text-secondary); margin-bottom: var(--space-3);">
                                <strong>Step 2:</strong> Enter the 6-digit code from your authenticator app to verify.
                            </p>
                            <div class="form-group">
                                <input type="text" class="form-input" id="twofa-verify-code" placeholder="000000" maxlength="6" pattern="[0-9]{6}" inputmode="numeric" autocomplete="one-time-code"
                                    style="text-align: center; font-size: var(--font-size-lg); letter-spacing: 0.3em; font-weight: 600;">
                            </div>
                            <button class="btn btn-primary" id="twofa-enable-btn" style="width: 100%;">Verify & Enable 2FA</button>
                        </div>
                    `;
                    // Store secret for enable call
                    const secret = data.secret;

                    modal.querySelector('#twofa-copy-secret')?.addEventListener('click', () => {
                        navigator.clipboard.writeText(secret).then(() => showToast('Secret copied!', 'success')).catch(() => {
                            modal.querySelector('#twofa-secret-display').select();
                            showToast('Select and copy the key manually', 'info');
                        });
                    });

                    modal.querySelector('#twofa-enable-btn')?.addEventListener('click', async () => {
                        const code = modal.querySelector('#twofa-verify-code').value.trim();
                        if (!code || code.length !== 6) {
                            showToast('Enter a 6-digit code', 'error');
                            return;
                        }
                        const enableBtn = modal.querySelector('#twofa-enable-btn');
                        enableBtn.disabled = true;
                        enableBtn.textContent = 'Verifying...';
                        try {
                            await twofa.enable(secret, code);
                            closeModal();
                            showToast('Two-factor authentication enabled!', 'success');
                        } catch (err) {
                            showToast(err.message, 'error');
                            enableBtn.disabled = false;
                            enableBtn.textContent = 'Verify & Enable 2FA';
                        }
                    });
                } catch (err) {
                    showToast(err.message, 'error');
                    btn.disabled = false;
                    btn.textContent = 'Generate Secret Key';
                }
            });
        }
    } catch (err) {
        showToast('Failed to load 2FA status: ' + err.message, 'error');
    }
}

// ---- Theme ----
function initTheme() {
    const saved = localStorage.getItem('jcwt-theme') || 'light';
    document.documentElement.setAttribute('data-theme', saved);
}

function toggleTheme() {
    const current = document.documentElement.getAttribute('data-theme');
    const next = current === 'dark' ? 'light' : 'dark';
    document.documentElement.setAttribute('data-theme', next);
    localStorage.setItem('jcwt-theme', next);
}

// ---- Sidebar / Layout ----
function renderLayout(pageName) {
    const theme = document.documentElement.getAttribute('data-theme') || 'dark';
    const brandName = panelSettings?.panel_name || 'JCWT Ultra Panel';
    const tagline = panelSettings?.panel_tagline || 'IPv6-Native Hosting';
    const logoLight = panelSettings?.logo_url || '';
    const logoDark = panelSettings?.logo_url_dark || '';
    const logoUrl = theme === 'dark' ? (logoDark || logoLight) : (logoLight || logoDark);

    return `
    <div class="app-layout">
        <div class="sidebar-overlay" id="sidebar-overlay"></div>
        <aside class="sidebar" id="sidebar">
            <div class="sidebar-header">
                ${logoUrl
                    ? `<img src="${escapeHtml(logoUrl)}" alt="" class="sidebar-logo-img">`
                    : `<div class="sidebar-logo">${escapeHtml(brandName.charAt(0))}</div>`
                }
                <div class="sidebar-brand">
                    <span class="sidebar-brand-name">${escapeHtml(brandName)}</span>
                    <span class="sidebar-brand-tagline">${escapeHtml(tagline)}</span>
                </div>
            </div>
            <nav class="sidebar-nav">
                <div class="nav-section">
                    <div class="nav-section-title">Overview</div>
                    <a href="#/dashboard" class="nav-item ${pageName === 'dashboard' ? 'active' : ''}">
                        <span class="nav-icon">${icons.dashboard}</span> Dashboard
                    </a>
                </div>
                <div class="nav-section">
                    <div class="nav-section-title">Management</div>
                    <a href="#/sites" class="nav-item ${pageName === 'sites' || pageName === 'site-detail' ? 'active' : ''}">
                        <span class="nav-icon">${icons.sites}</span> Sites
                    </a>
                    ${currentRole === 'admin' || currentRole === 'manager' ? `
                    <a href="#/databases" class="nav-item ${pageName === 'databases' ? 'active' : ''}">
                        <span class="nav-icon">${icons.database}</span> Databases
                    </a>` : ''}
                    ${currentRole === 'admin' ? `
                    <a href="#/users" class="nav-item ${pageName === 'users' ? 'active' : ''}">
                        <span class="nav-icon">${icons.key}</span> Users
                    </a>` : ''}
                    ${currentRole === 'admin' ? `
                    <a href="#/disk-usage" class="nav-item ${pageName === 'disk-usage' ? 'active' : ''}">
                        <span class="nav-icon">${icons.database}</span> Site Disk Usage
                    </a>` : ''}
                </div>
                <div class="nav-section">
                    <div class="nav-section-title">System</div>
                    ${currentRole === 'admin' ? `
                    <a href="#/services" class="nav-item ${pageName === 'services' ? 'active' : ''}">
                        <span class="nav-icon">${icons.server}</span> Services
                    </a>` : ''}
                    ${currentRole === 'admin' ? `
                    <a href="#/firewall" class="nav-item ${pageName === 'firewall' ? 'active' : ''}">
                        <span class="nav-icon">${icons.shield}</span> Firewall
                    </a>` : ''}
                    ${currentRole === 'admin' || currentRole === 'manager' ? `
                    <div class="nav-item-group">
                        <a href="javascript:void(0)" class="nav-item ${['branding-appearance','login-security','backup-config','smtp-settings','email-notifications'].includes(pageName) ? 'active' : ''}" data-has-submenu="true">
                            <span class="nav-icon">${icons.settings}</span> Settings
                            <svg class="submenu-arrow ${['branding-appearance','login-security','backup-config','smtp-settings','email-notifications'].includes(pageName) ? 'open' : ''}" viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2"><polyline points="9 18 15 12 9 6"/></svg>
                        </a>
                        <div class="nav-submenu ${['branding-appearance','login-security','backup-config','smtp-settings','email-notifications'].includes(pageName) ? 'open' : ''}" id="settings-submenu">
                            <a href="#/branding-appearance" class="nav-item nav-subitem ${pageName === 'branding-appearance' ? 'active' : ''}">
                                <span style="color: var(--text-tertiary); margin-right: var(--space-1);">&mdash;</span> Branding & Appearance
                            </a>
                            <a href="#/login-security" class="nav-item nav-subitem ${pageName === 'login-security' ? 'active' : ''}">
                                <span style="color: var(--text-tertiary); margin-right: var(--space-1);">&mdash;</span> Login Security
                            </a>
                            ${currentRole === 'admin' ? `
                            <a href="#/backup-config" class="nav-item nav-subitem ${pageName === 'backup-config' ? 'active' : ''}">
                                <span style="color: var(--text-tertiary); margin-right: var(--space-1);">&mdash;</span> Backup Configuration
                            </a>
                            <a href="#/smtp-settings" class="nav-item nav-subitem ${pageName === 'smtp-settings' ? 'active' : ''}">
                                <span style="color: var(--text-tertiary); margin-right: var(--space-1);">&mdash;</span> SMTP
                            </a>
                            <a href="#/email-notifications" class="nav-item nav-subitem ${pageName === 'email-notifications' ? 'active' : ''}">
                                <span style="color: var(--text-tertiary); margin-right: var(--space-1);">&mdash;</span> Email Notifications
                            </a>` : ''}
                        </div>
                    </div>` : ''}
                </div>
            </nav>
            <div class="sidebar-footer">
                <a href="#" class="nav-item" id="logout-btn">
                    <span class="nav-icon">${icons.logout}</span> Logout
                </a>
            </div>
        </aside>
        <main class="main-content">
            <header class="main-header">
                <div class="main-header-left">
                    <button class="mobile-menu-btn" id="mobile-menu-btn">
                        <span class="nav-icon">${icons.menu}</span>
                    </button>
                    <h1 class="page-title" id="page-title"></h1>
                </div>
                <div class="main-header-right">
                    <button class="theme-toggle" id="theme-toggle" title="Toggle theme"></button>
                    <div class="user-menu-wrapper" style="position: relative;">
                        <div class="user-menu" id="user-menu" style="cursor:pointer" title="Account menu">
                            <div class="user-avatar">${currentUser ? currentUser[0].toUpperCase() : 'A'}</div>
                            <span class="user-menu-name">${escapeHtml(currentUser || 'Admin')}</span>
                            <svg width="12" height="12" viewBox="0 0 12 12" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M3 5l3 3 3-3"/></svg>
                        </div>
                        <div class="user-dropdown" id="user-dropdown">
                            <div class="user-dropdown-header">
                                <div class="user-dropdown-avatar">${currentUser ? currentUser[0].toUpperCase() : 'A'}</div>
                                <div>
                                    <div style="font-weight: 600; font-size: var(--font-size-sm);">${escapeHtml(currentUser || 'Admin')}</div>
                                    <div style="font-size: var(--font-size-xs); color: var(--text-tertiary);">Administrator</div>
                                </div>
                            </div>
                            <div class="user-dropdown-divider"></div>
                            <a class="user-dropdown-item" id="dropdown-change-pwd">
                                <span class="nav-icon">${icons.key}</span> Change Password
                            </a>
                            <a class="user-dropdown-item" id="dropdown-2fa">
                                <span class="nav-icon">${icons.lock}</span> Two-Factor Auth
                            </a>
                            <div class="user-dropdown-divider"></div>
                            <a class="user-dropdown-item" id="dropdown-logout" style="color: var(--status-error);">
                                <span class="nav-icon">${icons.logout}</span> Sign Out
                            </a>
                        </div>
                    </div>
                </div>
            </header>
            <div class="main-body" id="page-content">
                <div class="loading-screen"><div class="loading-spinner"></div></div>
            </div>
            ${`<footer class="panel-footer">${panelSettings?.footer_text ? escapeHtml(panelSettings.footer_text).replace('{year}', new Date().getFullYear()) : `&copy; ${new Date().getFullYear()} ${escapeHtml(panelSettings?.panel_name || 'JCWT Ultra Panel')}`}${panelSettings?.version ? ` &mdash; v${escapeHtml(panelSettings.version)}` : ''}</footer>`}
        </main>
    </div>`;
}

// ---- Router ----
const routes = {};

export function registerPage(path, renderFn) {
    routes[path] = renderFn;
}

async function navigate() {
    const hash = window.location.hash || '#/dashboard';
    const path = hash.replace('#', '');

    // Check if first-time setup is needed
    try {
        const { needs_setup } = await setup.status();
        if (needs_setup) {
            if (path !== '/setup') {
                window.location.hash = '#/setup';
                return;
            }
            const setupModule = await import('./pages/setup.js');
            const app = document.getElementById('app');
            app.innerHTML = '';
            setupModule.render(app);
            return;
        }
    } catch {
        // If status check fails, fall through to normal auth
    }

    // Setup page after setup is done → go to login
    if (path === '/setup') {
        window.location.hash = '#/login';
        return;
    }

    // Check authentication
    try {
        const authData = await auth.check();
        if (!authData.authenticated) {
            if (path !== '/login') {
                window.location.hash = '#/login';
                return;
            }
        } else {
            currentUser = authData.username;
            currentRole = authData.role || 'admin';
            setCsrfToken(authData.csrf_token);

            if (path === '/login') {
                window.location.hash = '#/dashboard';
                return;
            }
        }
    } catch {
        if (path !== '/login') {
            window.location.hash = '#/login';
            return;
        }
    }

    // Login page special case
    if (path === '/login') {
        const loginModule = await import('./pages/login.js');
        const app = document.getElementById('app');
        app.innerHTML = '';
        loginModule.render(app);
        return;
    }

    // Redirect legacy /settings to first sub-page
    if (path === '/settings') {
        window.location.hash = '#/branding-appearance';
        return;
    }

    // Load panel settings if not loaded
    if (!panelSettings) {
        try {
            panelSettings = await settingsApi.get();
        } catch {
            panelSettings = {};
        }
    }

    // Determine page name from path
    let pageName = path.split('/')[1] || 'dashboard';
    let param = path.split('/')[2] || null;
    let param2 = path.split('/')[3] || null;

    // Routes like /sites/{token} and /sites/{token}/{section} → load site-detail
    if (pageName === 'sites' && param) {
        pageName = 'site-detail';
    }

    const app = document.getElementById('app');
    app.innerHTML = renderLayout(pageName);

    // Bind layout events
    document.getElementById('logout-btn')?.addEventListener('click', async (e) => {
        e.preventDefault();
        await auth.logout();
        window.location.hash = '#/login';
    });

    document.getElementById('theme-toggle')?.addEventListener('click', toggleTheme);

    document.getElementById('mobile-menu-btn')?.addEventListener('click', () => {
        document.getElementById('sidebar').classList.toggle('open');
        document.getElementById('sidebar-overlay').classList.toggle('active');
    });

    document.getElementById('sidebar-overlay')?.addEventListener('click', () => {
        document.getElementById('sidebar').classList.remove('open');
        document.getElementById('sidebar-overlay').classList.remove('active');
    });

    // User dropdown menu
    document.getElementById('user-menu')?.addEventListener('click', (e) => {
        e.stopPropagation();
        const dropdown = document.getElementById('user-dropdown');
        dropdown?.classList.toggle('open');
    });

    // Close dropdown on outside click
    document.addEventListener('click', (e) => {
        const dropdown = document.getElementById('user-dropdown');
        if (dropdown && !e.target.closest('.user-menu-wrapper')) {
            dropdown.classList.remove('open');
        }
    });

    document.getElementById('dropdown-change-pwd')?.addEventListener('click', () => {
        document.getElementById('user-dropdown')?.classList.remove('open');
        showPasswordChangeModal();
    });

    document.getElementById('dropdown-2fa')?.addEventListener('click', () => {
        document.getElementById('user-dropdown')?.classList.remove('open');
        show2FAModal();
    });

    document.getElementById('dropdown-logout')?.addEventListener('click', async () => {
        await auth.logout();
        window.location.hash = '#/login';
    });

    // Close sidebar on nav click (mobile) — but not when toggling a submenu
    document.querySelectorAll('.nav-item').forEach(item => {
        if (item.hasAttribute('data-has-submenu')) return;
        item.addEventListener('click', () => {
            document.getElementById('sidebar')?.classList.remove('open');
            document.getElementById('sidebar-overlay')?.classList.remove('active');
        });
    });

    // Settings submenu toggle
    document.querySelectorAll('[data-has-submenu]').forEach(item => {
        const arrow = item.querySelector('.submenu-arrow');
        const submenu = item.parentElement.querySelector('.nav-submenu');
        if (!arrow || !submenu) return;
        const toggle = (e) => {
            e.preventDefault();
            e.stopPropagation();
            arrow.classList.toggle('open');
            submenu.classList.toggle('open');
        };
        item.addEventListener('click', toggle);
    });

    // Render page content
    const pageContent = document.getElementById('page-content');
    const routeKey = pageName === 'site-detail' ? '/site-detail' : `/${pageName}`;

    if (routes[routeKey]) {
        await routes[routeKey](pageContent, param, param2);
    } else {
        // Lazy load page modules
        try {
            const module = await import(`./pages/${pageName}.js`);
            routes[routeKey] = module.render;
            await module.render(pageContent, param, param2);
        } catch (err) {
            pageContent.innerHTML = `
                <div class="empty-state">
                    <div class="empty-state-icon"><span class="nav-icon" style="width:48px;height:48px;color:var(--text-tertiary)">${icons.search}</span></div>
                    <div class="empty-state-title">Page Not Found</div>
                    <div class="empty-state-text">The page you're looking for doesn't exist.</div>
                    <a href="#/dashboard" class="btn btn-primary">Go to Dashboard</a>
                </div>`;
        }
    }
}

// ---- Global Autofill Prevention ----
// MutationObserver sets autocomplete="off" on all form inputs and
// autocomplete="new-password" on password fields. This runs automatically
// for every dynamically-added form. To allow autofill later, remove this block.
function disableAutofill(root) {
    root.querySelectorAll('input:not([data-allow-autofill]), textarea:not([data-allow-autofill])').forEach(el => {
        if (!el.hasAttribute('autocomplete')) {
            el.setAttribute('autocomplete', el.type === 'password' ? 'new-password' : 'off');
        }
    });
    root.querySelectorAll('form:not([data-allow-autofill])').forEach(form => {
        if (!form.hasAttribute('autocomplete')) {
            form.setAttribute('autocomplete', 'off');
        }
    });
}
const _autofillObserver = new MutationObserver((mutations) => {
    for (const m of mutations) {
        for (const node of m.addedNodes) {
            if (node.nodeType === 1) disableAutofill(node);
        }
    }
});
_autofillObserver.observe(document.documentElement, { childList: true, subtree: true });

// ---- Init ----
document.addEventListener('DOMContentLoaded', () => {
    disableAutofill(document);
    initTheme();
    window.addEventListener('hashchange', navigate);
    navigate();
});
