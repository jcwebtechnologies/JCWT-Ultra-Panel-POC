// JCWT Ultra Panel — SMTP Settings Page
import { smtpSettings } from '../api.js';
import { showToast, showModal, closeModal, icons, escapeHtml } from '../app.js';

export async function render(container) {
    document.getElementById('page-title').textContent = 'SMTP Settings';
    container.innerHTML = `
        <div class="page-header">
            <div>
                <h2>SMTP Settings</h2>
                <p>Configure outgoing email for panel notifications</p>
            </div>
        </div>
        <div class="card" style="max-width: 640px;">
            <div class="card-body" id="smtp-form-area">
                <div class="loading-screen"><div class="loading-spinner"></div></div>
            </div>
        </div>`;

    await loadForm();
}

async function loadForm() {
    const area = document.getElementById('smtp-form-area');
    try {
        const data = await smtpSettings.get();

        area.innerHTML = `
            <form id="smtp-form" autocomplete="off">
                <div class="form-group">
                    <label class="form-label">SMTP Host</label>
                    <input type="text" class="form-input" id="smtp-host" value="${escapeHtml(data.host || '')}" placeholder="smtp.example.com">
                </div>
                <div style="display: grid; grid-template-columns: 1fr 1fr; gap: var(--space-4);">
                    <div class="form-group">
                        <label class="form-label">Port</label>
                        <input type="number" class="form-input" id="smtp-port" value="${data.port || 587}" min="1" max="65535">
                    </div>
                    <div class="form-group">
                        <label class="form-label">Encryption</label>
                        <select class="form-input" id="smtp-encryption">
                            <option value="tls" ${data.encryption === 'tls' ? 'selected' : ''}>STARTTLS (587)</option>
                            <option value="ssl" ${data.encryption === 'ssl' ? 'selected' : ''}>SSL/TLS (465)</option>
                            <option value="none" ${data.encryption === 'none' ? 'selected' : ''}>None (25)</option>
                        </select>
                    </div>
                </div>
                <div class="form-group" style="margin-bottom: var(--space-3);">
                    <div style="display: flex; align-items: center; gap: var(--space-3);">
                        <label class="toggle">
                            <input type="checkbox" id="smtp-auth" ${data.auth_enabled ? 'checked' : ''}>
                            <span class="toggle-slider"></span>
                        </label>
                        <span class="form-label" style="margin: 0;">Require Authentication</span>
                    </div>
                </div>
                <div id="smtp-auth-fields" style="${data.auth_enabled ? '' : 'display:none;'}">
                    <div class="form-group">
                        <label class="form-label">Username</label>
                        <input type="text" class="form-input" id="smtp-username" value="${escapeHtml(data.username || '')}" placeholder="user@example.com" autocomplete="off">
                    </div>
                    <div class="form-group">
                        <label class="form-label">Password</label>
                        <input type="password" class="form-input" id="smtp-password" value="${escapeHtml(data.password || '')}" placeholder="SMTP password" autocomplete="new-password">
                    </div>
                </div>
                <hr style="border: none; border-top: 1px solid var(--border-primary); margin: var(--space-4) 0;">
                <div class="form-group">
                    <label class="form-label">From Email</label>
                    <input type="email" class="form-input" id="smtp-from-email" value="${escapeHtml(data.from_email || '')}" placeholder="noreply@example.com">
                </div>
                <div class="form-group">
                    <label class="form-label">From Name</label>
                    <input type="text" class="form-input" id="smtp-from-name" value="${escapeHtml(data.from_name || '')}" placeholder="JCWT Ultra Panel">
                </div>
                <div style="display: flex; gap: var(--space-3); margin-top: var(--space-4);">
                    <button type="submit" class="btn btn-primary" id="smtp-save-btn">
                        Save Settings
                    </button>
                    <button type="button" class="btn btn-secondary" id="smtp-test-btn">
                        <span class="nav-icon">${icons.mail}</span> Send Test Email
                    </button>
                </div>
            </form>`;

        // Toggle auth fields
        document.getElementById('smtp-auth')?.addEventListener('change', (e) => {
            document.getElementById('smtp-auth-fields').style.display = e.target.checked ? '' : 'none';
        });

        // Auto-update port on encryption change
        document.getElementById('smtp-encryption')?.addEventListener('change', (e) => {
            const portMap = { tls: 587, ssl: 465, none: 25 };
            const portInput = document.getElementById('smtp-port');
            const currentPort = parseInt(portInput.value);
            // Only auto-set if port matches a known default
            if ([25, 465, 587].includes(currentPort) || !currentPort) {
                portInput.value = portMap[e.target.value] || 587;
            }
        });

        // Save
        document.getElementById('smtp-form')?.addEventListener('submit', async (e) => {
            e.preventDefault();
            const btn = document.getElementById('smtp-save-btn');
            btn.disabled = true;
            btn.textContent = 'Saving...';
            try {
                await smtpSettings.update({
                    host: document.getElementById('smtp-host').value.trim(),
                    port: parseInt(document.getElementById('smtp-port').value) || 587,
                    encryption: document.getElementById('smtp-encryption').value,
                    auth_enabled: document.getElementById('smtp-auth').checked,
                    username: document.getElementById('smtp-username').value.trim(),
                    password: document.getElementById('smtp-password').value,
                    from_email: document.getElementById('smtp-from-email').value.trim(),
                    from_name: document.getElementById('smtp-from-name').value.trim(),
                });
                showToast('SMTP settings saved', 'success');
            } catch (err) {
                showToast(err.message || 'Failed to save', 'error');
            }
            btn.disabled = false;
            btn.textContent = 'Save Settings';
        });

        // Test email
        document.getElementById('smtp-test-btn')?.addEventListener('click', () => {
            const content = `
                <div class="form-group">
                    <label class="form-label">Recipient Email</label>
                    <input type="email" class="form-input" id="test-email-to" placeholder="you@example.com" required>
                </div>
                <div class="form-group">
                    <label class="form-label">Email Format</label>
                    <div style="display: flex; gap: var(--space-3); margin-top: var(--space-2);">
                        <label style="display: flex; align-items: center; gap: var(--space-2); cursor: pointer;">
                            <input type="radio" name="email-format" value="plain" checked> Plain Text
                        </label>
                        <label style="display: flex; align-items: center; gap: var(--space-2); cursor: pointer;">
                            <input type="radio" name="email-format" value="html"> HTML
                        </label>
                    </div>
                </div>
            `;
            const footer = `
                <button class="btn btn-secondary" onclick="document.getElementById('modal-overlay').remove()">Cancel</button>
                <button class="btn btn-primary" id="send-test-btn"><span class="nav-icon">${icons.mail}</span> Send</button>
            `;
            const modal = showModal('Send Test Email', content, footer);

            modal.querySelector('#send-test-btn')?.addEventListener('click', async () => {
                const to = modal.querySelector('#test-email-to')?.value?.trim();
                if (!to) { showToast('Enter a recipient email', 'error'); return; }
                const format = modal.querySelector('input[name="email-format"]:checked')?.value || 'plain';
                const btn = modal.querySelector('#send-test-btn');
                btn.disabled = true;
                btn.innerHTML = `<span class="nav-icon">${icons.mail}</span> Sending...`;
                try {
                    await smtpSettings.testEmail(to, format);
                    closeModal();
                    showToast('Test email sent successfully!', 'success');
                } catch (err) {
                    showToast(err.message || 'Test failed', 'error');
                    btn.disabled = false;
                    btn.innerHTML = `<span class="nav-icon">${icons.mail}</span> Send`;
                }
            });
        });

    } catch (err) {
        area.innerHTML = `<div class="empty-state"><div class="empty-state-title">Failed to load SMTP settings</div><div class="empty-state-text">${escapeHtml(err.message)}</div></div>`;
    }
}
