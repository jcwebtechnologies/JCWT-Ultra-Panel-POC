// JCWT Ultra Panel — First-Time Setup Page
import { setup, request } from '../api.js';
import { escapeHtml } from '../app.js';

export async function render(container) {
    // Fetch public settings for branding
    let branding = {};
    try {
        branding = await request('/api/settings/public');
    } catch { /* use defaults */ }

    const panelName = branding.panel_name || 'JCWT Ultra Panel';
    const logoLight = branding.logo_url || '';
    const logoDark = branding.logo_url_dark || '';
    const theme = document.documentElement.getAttribute('data-theme') || 'dark';
    const logoUrl = theme === 'dark' ? (logoDark || logoLight) : (logoLight || logoDark);

    container.innerHTML = `
    <div class="login-page">
        <div class="login-card">
            <div class="login-logo" id="setup-logo-container">
                <h1>${escapeHtml(panelName)}</h1>
                <p>First-Time Setup</p>
            </div>
            <div style="background: var(--bg-secondary); border: 1px solid var(--border-primary); border-radius: var(--radius-md); padding: var(--space-3); margin-bottom: var(--space-4); font-size: var(--font-size-sm); color: var(--text-secondary);">
                Create the first admin account. You'll need the setup token shown in the server output or installer log.
            </div>
            <div class="login-error" id="setup-error"></div>
            <form class="login-form" id="setup-form">
                <div class="form-group">
                    <label class="form-label">Setup Token</label>
                    <input type="text" class="form-input mono" id="setup-token" placeholder="Paste the token from installer output" autocomplete="off" required autofocus maxlength="200">
                </div>
                <div class="form-group">
                    <label class="form-label">Admin Username</label>
                    <input type="text" class="form-input" id="setup-username" placeholder="admin" autocomplete="off" required minlength="3" maxlength="31">
                </div>
                <div class="form-group">
                    <label class="form-label">Password</label>
                    <input type="password" class="form-input" id="setup-password" placeholder="Minimum 10 characters" autocomplete="new-password" required minlength="10" maxlength="200">
                </div>
                <div class="form-group">
                    <label class="form-label">Confirm Password</label>
                    <input type="password" class="form-input" id="setup-password-confirm" placeholder="Re-enter password" autocomplete="new-password" required minlength="10" maxlength="200">
                </div>
                <button type="submit" class="btn btn-primary login-btn" id="setup-submit">
                    Create Admin Account
                </button>
            </form>
        </div>
    </div>`;

    // Insert logo via DOM APIs
    const logoContainer = document.getElementById('setup-logo-container');
    if (logoUrl) {
        const img = document.createElement('img');
        img.src = logoUrl;
        img.alt = panelName;
        img.style.cssText = 'width: 48px; height: 48px; border-radius: 12px; margin-bottom: 8px;';
        logoContainer.insertBefore(img, logoContainer.firstChild);
    } else {
        const iconDiv = document.createElement('div');
        iconDiv.className = 'logo-icon';
        iconDiv.textContent = panelName.charAt(0);
        logoContainer.insertBefore(iconDiv, logoContainer.firstChild);
    }

    document.getElementById('setup-form').addEventListener('submit', async (e) => {
        e.preventDefault();
        const token = document.getElementById('setup-token').value.trim();
        const username = document.getElementById('setup-username').value.trim();
        const password = document.getElementById('setup-password').value;
        const confirm = document.getElementById('setup-password-confirm').value;
        const errorEl = document.getElementById('setup-error');
        const submitBtn = document.getElementById('setup-submit');

        errorEl.style.display = 'none';

        if (!token || !username || !password) {
            errorEl.textContent = 'All fields are required';
            errorEl.style.display = 'block';
            return;
        }
        if (username.length < 3 || username.length > 31) {
            errorEl.textContent = 'Username must be 3-31 characters';
            errorEl.style.display = 'block';
            return;
        }
        if (password.length < 10) {
            errorEl.textContent = 'Password must be at least 10 characters';
            errorEl.style.display = 'block';
            return;
        }
        if (password !== confirm) {
            errorEl.textContent = 'Passwords do not match';
            errorEl.style.display = 'block';
            return;
        }

        submitBtn.disabled = true;
        submitBtn.textContent = 'Creating account...';

        try {
            await setup.create({ setup_token: token, username, password });
            // Show success, then redirect to login
            container.innerHTML = `
            <div class="login-page">
                <div class="login-card" style="text-align: center;">
                    <div style="color: var(--status-success); margin-bottom: var(--space-3);">
                        <svg viewBox="0 0 24 24" width="48" height="48" fill="none" stroke="currentColor" stroke-width="2"><path d="M22 11.08V12a10 10 0 1 1-5.93-9.14"/><polyline points="22 4 12 14.01 9 11.01"/></svg>
                    </div>
                    <h2 style="margin-bottom: var(--space-2);">Admin Account Created!</h2>
                    <p style="color: var(--text-secondary); margin-bottom: var(--space-4);">You can now sign in with your new credentials.</p>
                    <a href="#/login" class="btn btn-primary" style="display: inline-block;">Go to Login</a>
                </div>
            </div>`;
        } catch (err) {
            errorEl.textContent = err.message || 'Setup failed';
            errorEl.style.display = 'block';
            submitBtn.disabled = false;
            submitBtn.textContent = 'Create Admin Account';
        }
    });
}
