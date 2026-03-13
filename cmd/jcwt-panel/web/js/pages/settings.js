// JCWT Ultra Panel — Settings Page (Panel Branding & Configuration)
import { settings, backupMethods } from '../api.js';
import { icons, showToast, escapeHtml, showConfirm, showPrompt } from '../app.js';

export async function render(container) {
    document.getElementById('page-title').textContent = 'Panel Settings';
    container.innerHTML = '<div class="loading-screen"><div class="loading-spinner"></div></div>';

    try {
        const cfg = await settings.get();

        container.innerHTML = `
        <div class="page-header">
            <div class="page-header-left">
                <h2>Panel Settings</h2>
                <p>Customize branding, appearance, and behavior</p>
            </div>
        </div>

        <form id="settings-form">
            <!-- Branding Section -->
            <div class="card" style="margin-bottom: var(--space-6);">
                <h3 class="settings-section-title"><span class="nav-icon section-icon">${icons.palette}</span> Branding</h3>

                <div class="settings-row">
                    <div class="settings-row-label">Panel Name<small>Displayed in sidebar and title</small></div>
                    <div><input type="text" class="form-input" id="s-panel-name" value="${escapeHtml(cfg.panel_name || '')}"></div>
                </div>
                <div class="settings-row">
                    <div class="settings-row-label">Tagline<small>Shown under the panel name</small></div>
                    <div><input type="text" class="form-input" id="s-tagline" value="${escapeHtml(cfg.panel_tagline || '')}"></div>
                </div>
                <div class="settings-row">
                    <div class="settings-row-label">Footer Text<small>Displayed at the bottom</small></div>
                    <div><input type="text" class="form-input" id="s-footer" value="${escapeHtml(cfg.footer_text || '')}"></div>
                </div>
                <div class="settings-row">
                    <div class="settings-row-label">☀️ Logo URL (Light Mode)<small>Displayed when light theme is active</small></div>
                    <div>
                        <div style="display: flex; gap: var(--space-2); align-items: center;">
                            <input type="text" class="form-input" id="s-logo" value="${escapeHtml(cfg.logo_url || '')}" placeholder="/api/uploads/logo.png">
                            <button type="button" class="btn btn-sm btn-secondary" id="upload-logo-btn"><span class="nav-icon">${icons.upload}</span></button>
                        </div>
                        ${cfg.logo_url ? `<img src="${escapeHtml(cfg.logo_url)}" style="max-height: 40px; margin-top: var(--space-2); border-radius: var(--radius-sm); background: #f8f8f8; padding: 4px;">` : ''}
                    </div>
                </div>
                <div class="settings-row">
                    <div class="settings-row-label">🌙 Logo URL (Dark Mode)<small>Displayed when dark theme is active</small></div>
                    <div>
                        <div style="display: flex; gap: var(--space-2); align-items: center;">
                            <input type="text" class="form-input" id="s-logo-dark" value="${escapeHtml(cfg.logo_url_dark || '')}" placeholder="/api/uploads/logo-dark.png">
                            <button type="button" class="btn btn-sm btn-secondary" id="upload-logo-dark-btn"><span class="nav-icon">${icons.upload}</span></button>
                        </div>
                        ${cfg.logo_url_dark ? `<img src="${escapeHtml(cfg.logo_url_dark)}" style="max-height: 40px; margin-top: var(--space-2); border-radius: var(--radius-sm); background: #1a1a2e; padding: 4px;">` : ''}
                    </div>
                </div>
                <div class="settings-row">
                    <div class="settings-row-label">Favicon URL<small>Browser tab icon</small></div>
                    <div>
                        <div style="display: flex; gap: var(--space-2); align-items: center;">
                            <input type="text" class="form-input" id="s-favicon" value="${escapeHtml(cfg.favicon_url || '')}" placeholder="/api/uploads/favicon.ico">
                            <button type="button" class="btn btn-sm btn-secondary" id="upload-favicon-btn"><span class="nav-icon">${icons.upload}</span></button>
                        </div>
                    </div>
                </div>
            </div>

            <!-- Appearance Section -->
            <div class="card" style="margin-bottom: var(--space-6);">
                <h3 class="settings-section-title"><span class="nav-icon section-icon">${icons.target}</span> Appearance</h3>

                <div class="settings-row">
                    <div class="settings-row-label">Primary Color<small>Used for buttons, links, accent</small></div>
                    <div>
                        <div class="color-input-group">
                            <input type="color" class="color-swatch" id="s-primary-color" value="${cfg.primary_color || '#6366f1'}">
                            <input type="text" class="form-input" id="s-primary-hex" value="${escapeHtml(cfg.primary_color || '#6366f1')}" style="max-width: 120px;">
                        </div>
                    </div>
                </div>
                <div class="settings-row">
                    <div class="settings-row-label">Accent Color<small>Secondary highlight color</small></div>
                    <div>
                        <div class="color-input-group">
                            <input type="color" class="color-swatch" id="s-accent-color" value="${cfg.accent_color || '#818cf8'}">
                            <input type="text" class="form-input" id="s-accent-hex" value="${escapeHtml(cfg.accent_color || '#818cf8')}" style="max-width: 120px;">
                        </div>
                    </div>
                </div>
            </div>

            <!-- Security Section -->
            <div class="card" style="margin-bottom: var(--space-6);">
                <h3 class="settings-section-title"><span class="nav-icon section-icon">${icons.shield}</span> Security & Session</h3>

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

            <!-- Backup Configuration Section -->
            <div class="card" style="margin-bottom: var(--space-6);">
                <h3 class="settings-section-title"><span class="nav-icon section-icon">${icons.database}</span> Backup Configuration</h3>
                <p style="color: var(--text-tertiary); font-size: var(--font-size-sm); margin-bottom: var(--space-4);">Configure backup storage methods available for site backups.</p>
                <div id="backup-methods-list" style="margin-bottom: var(--space-4);"></div>
                <button type="button" class="btn btn-sm btn-secondary" id="add-backup-method-btn">+ Add Backup Method</button>
            </div>

            <!-- reCAPTCHA Section -->
            <div class="card" style="margin-bottom: var(--space-6);">
                <h3 class="settings-section-title"><span class="nav-icon section-icon">${icons.bot}</span> reCAPTCHA (Login Protection)</h3>
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

        // Color sync
        const syncColor = (colorId, hexId) => {
            document.getElementById(colorId)?.addEventListener('input', (e) => {
                document.getElementById(hexId).value = e.target.value;
            });
            document.getElementById(hexId)?.addEventListener('input', (e) => {
                if (/^#[0-9a-fA-F]{6}$/.test(e.target.value)) {
                    document.getElementById(colorId).value = e.target.value;
                }
            });
        };
        syncColor('s-primary-color', 's-primary-hex');
        syncColor('s-accent-color', 's-accent-hex');

        // Logo upload
        const logoInput = document.createElement('input');
        logoInput.type = 'file';
        logoInput.accept = 'image/*';
        logoInput.style.display = 'none';
        container.appendChild(logoInput);
        document.getElementById('upload-logo-btn')?.addEventListener('click', () => logoInput.click());
        logoInput.addEventListener('change', async () => {
            if (!logoInput.files[0]) return;
            const fd = new FormData();
            fd.append('file', logoInput.files[0]);
            try {
                const result = await settings.uploadLogo(fd);
                document.getElementById('s-logo').value = result.url;
                showToast('Logo uploaded!', 'success');
            } catch (err) { showToast(err.message, 'error'); }
        });

        // Dark logo upload
        const logoDarkInput = document.createElement('input');
        logoDarkInput.type = 'file';
        logoDarkInput.accept = 'image/*';
        logoDarkInput.style.display = 'none';
        container.appendChild(logoDarkInput);
        document.getElementById('upload-logo-dark-btn')?.addEventListener('click', () => logoDarkInput.click());
        logoDarkInput.addEventListener('change', async () => {
            if (!logoDarkInput.files[0]) return;
            const fd = new FormData();
            fd.append('file', logoDarkInput.files[0]);
            try {
                const result = await settings.uploadLogoDark(fd);
                document.getElementById('s-logo-dark').value = result.url;
                showToast('Dark mode logo uploaded!', 'success');
            } catch (err) { showToast(err.message, 'error'); }
        });

        // Favicon upload
        const favInput = document.createElement('input');
        favInput.type = 'file';
        favInput.accept = '.ico,.png,.svg';
        favInput.style.display = 'none';
        container.appendChild(favInput);
        document.getElementById('upload-favicon-btn')?.addEventListener('click', () => favInput.click());
        favInput.addEventListener('change', async () => {
            if (!favInput.files[0]) return;
            const fd = new FormData();
            fd.append('file', favInput.files[0]);
            try {
                const result = await settings.uploadFavicon(fd);
                document.getElementById('s-favicon').value = result.url;
                showToast('Favicon uploaded!', 'success');
            } catch (err) { showToast(err.message, 'error'); }
        });

        // Backup Methods management
        async function loadBackupMethods() {
            const listEl = document.getElementById('backup-methods-list');
            if (!listEl) return;
            try {
                const data = await backupMethods.list();
                const methods = Array.isArray(data) ? data : (data?.methods || []);
                if (methods.length === 0) {
                    listEl.innerHTML = '<div style="color:var(--text-tertiary);font-size:var(--font-size-sm);">No backup methods configured. A default "Local" method will be used for site backups.</div>';
                    return;
                }
                listEl.innerHTML = methods.map(m => `
                    <div class="settings-row" style="padding:var(--space-3);border:1px solid var(--border-primary);border-radius:var(--radius-md);margin-bottom:var(--space-2);">
                        <div class="settings-row-label" style="min-width:auto;">
                            <strong>${escapeHtml(m.name)}</strong>
                            <small>Type: ${escapeHtml(m.type)} ${m.enabled ? '(Active)' : '(Disabled)'}</small>
                        </div>
                        <div style="display:flex;gap:var(--space-2);">
                            <button type="button" class="btn btn-sm ${m.enabled ? 'btn-secondary' : 'btn-primary'}" data-toggle-method="${m.id}">${m.enabled ? 'Disable' : 'Enable'}</button>
                            <button type="button" class="btn btn-sm btn-danger" data-delete-method="${m.id}">Delete</button>
                        </div>
                    </div>
                `).join('');

                listEl.querySelectorAll('[data-toggle-method]').forEach(btn => {
                    btn.addEventListener('click', async () => {
                        const mid = parseInt(btn.dataset.toggleMethod);
                        const method = methods.find(m => m.id === mid);
                        if (!method) return;
                        try {
                            await backupMethods.update({ id: mid, name: method.name, type: method.type, config: method.config || '{}', enabled: !method.enabled });
                            showToast('Backup method updated', 'success');
                            loadBackupMethods();
                        } catch (err) { showToast(err.message, 'error'); }
                    });
                });

                listEl.querySelectorAll('[data-delete-method]').forEach(btn => {
                    btn.addEventListener('click', async () => {
                        const confirmed = await showConfirm('Delete this backup method?');
                        if (!confirmed) return;
                        try {
                            await backupMethods.delete(parseInt(btn.dataset.deleteMethod));
                            showToast('Backup method deleted', 'success');
                            loadBackupMethods();
                        } catch (err) { showToast(err.message, 'error'); }
                    });
                });
            } catch (err) {
                listEl.innerHTML = '<div style="color:var(--status-error);font-size:var(--font-size-sm);">Failed to load backup methods</div>';
            }
        }
        loadBackupMethods();

        document.getElementById('add-backup-method-btn')?.addEventListener('click', async () => {
            const name = await showPrompt('Backup Method Name', 'Enter a name for the backup method (e.g. "Local Backups")');
            if (!name) return;
            const type = await showPrompt('Backup Method Type', 'Enter the type (local, s3, sftp, gdrive, dropbox)', 'local');
            if (!type) return;
            try {
                await backupMethods.create({ name, type, config: '{}' });
                showToast('Backup method added', 'success');
                loadBackupMethods();
            } catch (err) { showToast(err.message, 'error'); }
        });

        // Save settings
        document.getElementById('settings-form')?.addEventListener('submit', async (e) => {
            e.preventDefault();
            try {
                await settings.update({
                    panel_name: document.getElementById('s-panel-name').value,
                    panel_tagline: document.getElementById('s-tagline').value,
                    logo_url: document.getElementById('s-logo').value,
                    logo_url_dark: document.getElementById('s-logo-dark').value,
                    favicon_url: document.getElementById('s-favicon').value,
                    primary_color: document.getElementById('s-primary-hex').value,
                    accent_color: document.getElementById('s-accent-hex').value,
                    footer_text: document.getElementById('s-footer').value,
                    session_timeout: parseInt(document.getElementById('s-timeout').value),
                    recaptcha_site_key: document.getElementById('s-recaptcha-site').value.trim(),
                    recaptcha_secret_key: document.getElementById('s-recaptcha-secret').value.trim(),
                    timezone: document.getElementById('s-timezone').value,
                });
                showToast('Settings saved! Refresh to see branding changes.', 'success');
            } catch (err) { showToast(err.message, 'error'); }
        });

    } catch (err) {
        container.innerHTML = `<div class="empty-state"><div class="empty-state-title">Error: ${err.message}</div></div>`;
    }
}
