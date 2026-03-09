// JCWT Ultra Panel — Login Security Settings
import { settings } from '../api.js';
import { icons, showToast, escapeHtml } from '../app.js';

export async function render(container) {
    document.getElementById('page-title').textContent = 'Login Security';
    container.innerHTML = '<div class="loading-screen"><div class="loading-spinner"></div></div>';

    try {
        const cfg = await settings.get();

        container.innerHTML = `
        <div class="page-header">
            <div class="page-header-left">
                <h2>Login Security</h2>
                <p>Session management and login protection</p>
            </div>
        </div>

        <form id="security-form">
            <div class="card" style="margin-bottom: var(--space-6);">
                <h3 class="settings-section-title"><span class="nav-icon" style="width:18px;height:18px;color:var(--accent-primary)">${icons.shield}</span> Security & Session</h3>

                <div class="settings-row">
                    <div class="settings-row-label">Session Timeout<small>Minutes of inactivity before logout</small></div>
                    <div>
                        <input type="number" class="form-input" id="s-timeout" value="${cfg.session_timeout || 30}" min="5" max="1440" style="max-width: 120px;">
                        <div class="form-help">5 - 1440 minutes</div>
                    </div>
                </div>
                <div class="settings-row">
                    <div class="settings-row-label">Server Timezone<small>Timezone for the server and logs</small></div>
                    <div>
                        <select class="form-select" id="s-timezone" style="max-width: 280px;">
                            ${['UTC', 'US/Eastern', 'US/Central', 'US/Mountain', 'US/Pacific', 'US/Hawaii',
                'America/New_York', 'America/Chicago', 'America/Denver', 'America/Los_Angeles', 'America/Toronto', 'America/Sao_Paulo',
                'Europe/London', 'Europe/Paris', 'Europe/Berlin', 'Europe/Moscow', 'Europe/Istanbul',
                'Asia/Kolkata', 'Asia/Dubai', 'Asia/Shanghai', 'Asia/Tokyo', 'Asia/Singapore', 'Asia/Seoul', 'Asia/Hong_Kong',
                'Australia/Sydney', 'Australia/Melbourne', 'Australia/Perth',
                'Pacific/Auckland', 'Africa/Cairo', 'Africa/Johannesburg', 'Africa/Lagos'
            ].map(tz => `<option value="${tz}" ${(cfg.timezone || 'UTC') === tz ? 'selected' : ''}>${tz}</option>`).join('')}
                        </select>
                    </div>
                </div>
            </div>

            <div class="card" style="margin-bottom: var(--space-6);">
                <h3 class="settings-section-title"><span class="nav-icon" style="width:18px;height:18px;color:var(--accent-primary)">${icons.bot}</span> reCAPTCHA (Login Protection)</h3>
                <p style="color: var(--text-tertiary); font-size: var(--font-size-sm); margin-bottom: var(--space-4);">Add Google reCAPTCHA v2 to the login page. Get keys from <a href="https://www.google.com/recaptcha/admin" target="_blank" style="color: var(--primary);">Google reCAPTCHA Admin</a>.</p>

                <div class="settings-row">
                    <div class="settings-row-label">Site Key<small>Public key shown on login page</small></div>
                    <div><input type="text" class="form-input mono" id="s-recaptcha-site" value="${escapeHtml(cfg.recaptcha_site_key || '')}" placeholder="6Lc..."></div>
                </div>
                <div class="settings-row">
                    <div class="settings-row-label">Secret Key<small>Private key for server verification</small></div>
                    <div><input type="password" class="form-input mono" id="s-recaptcha-secret" value="${escapeHtml(cfg.recaptcha_secret_key || '')}" placeholder="6Lc..."></div>
                </div>
                <div style="font-size: var(--font-size-xs); color: var(--text-tertiary); margin-top: var(--space-2);">
                    Leave both fields empty to disable reCAPTCHA on the login page.
                </div>
            </div>

            <div style="display: flex; justify-content: flex-end;">
                <button type="submit" class="btn btn-primary" style="min-width: 160px;">Save Settings</button>
            </div>
        </form>`;

        // Save
        document.getElementById('security-form')?.addEventListener('submit', async (e) => {
            e.preventDefault();
            try {
                const current = await settings.get();
                await settings.update({
                    ...current,
                    session_timeout: parseInt(document.getElementById('s-timeout').value),
                    recaptcha_site_key: document.getElementById('s-recaptcha-site').value.trim(),
                    recaptcha_secret_key: document.getElementById('s-recaptcha-secret').value.trim(),
                    timezone: document.getElementById('s-timezone').value,
                });
                showToast('Settings saved!', 'success');
            } catch (err) { showToast(err.message, 'error'); }
        });

    } catch (err) {
        container.innerHTML = `<div class="empty-state"><div class="empty-state-title">Error: ${err.message}</div></div>`;
    }
}
