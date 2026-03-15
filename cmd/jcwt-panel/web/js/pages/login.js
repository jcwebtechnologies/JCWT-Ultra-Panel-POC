// JCWT Ultra Panel — Login Page (Dynamic Branding + reCAPTCHA + 2FA)
import { auth, setCsrfToken, request, twofa } from '../api.js';
import { escapeHtml } from '../app.js';
import { ROUTES, navigateTo } from '../routes.js';

let recaptchaLoaded = false;

export async function render(container) {
    // Fetch public settings for branding + reCAPTCHA
    let branding = {};
    try {
        branding = await request('/api/settings/public');
    } catch (e) { /* use defaults */ }

    const panelName = branding.panel_name || 'JCWT Ultra Panel';
    const panelTagline = branding.panel_tagline || 'IPv6-Native Hosting Control Panel';
    const logoLight = branding.logo_url || '';
    const logoDark = branding.logo_url_dark || '';
    const theme = document.documentElement.getAttribute('data-theme') || 'dark';
    const logoUrl = theme === 'dark' ? (logoDark || logoLight) : (logoLight || logoDark);
    const recaptchaSiteKey = branding.recaptcha_site_key || '';

    container.innerHTML = `
    <div class="login-page">
        <div class="login-card">
            <div class="login-logo" id="login-logo-container">
                <h1>${escapeHtml(panelName)}</h1>
                <p>${escapeHtml(panelTagline)}</p>
            </div>
            <div class="login-error" id="login-error"></div>
            <form class="login-form" id="login-form">
                <div class="form-group">
                    <label class="form-label">Username</label>
                    <input type="text" class="form-input" id="login-username" placeholder="admin" autocomplete="username" required autofocus maxlength="100">
                </div>
                <div class="form-group">
                    <label class="form-label">Password</label>
                    <div style="position: relative;">
                        <input type="password" class="form-input" id="login-password" placeholder="••••••••" autocomplete="current-password" required maxlength="200" style="padding-right: 2.8rem;">
                        <button type="button" id="pwd-toggle" style="position: absolute; right: 8px; top: 50%; transform: translateY(-50%); background: none; border: none; cursor: pointer; padding: 4px; color: var(--text-tertiary); display: flex; align-items: center;" title="Toggle password visibility">
                            <svg id="pwd-eye" viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"/><circle cx="12" cy="12" r="3"/></svg>
                            <svg id="pwd-eye-off" viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" style="display:none;"><path d="M17.94 17.94A10.07 10.07 0 0 1 12 20c-7 0-11-8-11-8a18.45 18.45 0 0 1 5.06-5.94M9.9 4.24A9.12 9.12 0 0 1 12 4c7 0 11 8 11 8a18.5 18.5 0 0 1-2.16 3.19m-6.72-1.07a3 3 0 1 1-4.24-4.24"/><line x1="1" y1="1" x2="23" y2="23"/></svg>
                        </button>
                    </div>
                </div>
                ${recaptchaSiteKey ? `<div id="recaptcha-container" style="margin-bottom: var(--space-3); display: flex; justify-content: center;"></div>` : ''}
                <button type="submit" class="btn btn-primary login-btn" id="login-submit">
                    Sign In
                </button>
            </form>
        </div>
    </div>`;

    // Insert logo via DOM APIs to avoid XSS from logo URL
    const logoContainer = document.getElementById('login-logo-container');
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

    // Load reCAPTCHA if configured
    if (recaptchaSiteKey && !recaptchaLoaded) {
        const script = document.createElement('script');
        script.src = `https://www.google.com/recaptcha/api.js?render=explicit`;
        script.onload = () => {
            recaptchaLoaded = true;
            if (window.grecaptcha) {
                window.grecaptcha.ready(() => {
                    window.grecaptcha.render('recaptcha-container', {
                        sitekey: recaptchaSiteKey,
                        theme: document.documentElement.getAttribute('data-theme') === 'dark' ? 'dark' : 'light',
                    });
                });
            }
        };
        document.head.appendChild(script);
    } else if (recaptchaSiteKey && recaptchaLoaded && window.grecaptcha) {
        window.grecaptcha.ready(() => {
            window.grecaptcha.render('recaptcha-container', {
                sitekey: recaptchaSiteKey,
                theme: document.documentElement.getAttribute('data-theme') === 'dark' ? 'dark' : 'light',
            });
        });
    }

    // Password visibility toggle
    document.getElementById('pwd-toggle')?.addEventListener('click', () => {
        const pwdInput = document.getElementById('login-password');
        const eyeIcon = document.getElementById('pwd-eye');
        const eyeOffIcon = document.getElementById('pwd-eye-off');
        if (pwdInput.type === 'password') {
            pwdInput.type = 'text';
            eyeIcon.style.display = 'none';
            eyeOffIcon.style.display = '';
        } else {
            pwdInput.type = 'password';
            eyeIcon.style.display = '';
            eyeOffIcon.style.display = 'none';
        }
    });

    document.getElementById('login-form').addEventListener('submit', async (e) => {
        e.preventDefault();
        const username = document.getElementById('login-username').value.trim();
        const password = document.getElementById('login-password').value;
        const errorEl = document.getElementById('login-error');
        const submitBtn = document.getElementById('login-submit');

        // Client-side validation
        if (!username || !password) {
            errorEl.textContent = 'Username and password are required';
            errorEl.style.display = 'block';
            return;
        }

        // Get reCAPTCHA token if configured
        let captchaToken = '';
        if (recaptchaSiteKey && window.grecaptcha) {
            captchaToken = window.grecaptcha.getResponse();
            if (!captchaToken) {
                errorEl.textContent = 'Please complete the captcha verification';
                errorEl.style.display = 'block';
                return;
            }
        }

        submitBtn.disabled = true;
        submitBtn.textContent = 'Signing in...';
        errorEl.style.display = 'none';

        try {
            const data = await auth.login(username, password, captchaToken);

            // Check if 2FA is required
            if (data.requires_2fa) {
                show2FAStep(container, data.twofa_token, panelName, logoUrl);
                return;
            }

            setCsrfToken(data.csrf_token);
            navigateTo(ROUTES.DASHBOARD);
        } catch (err) {
            errorEl.textContent = err.message || 'Invalid credentials';
            errorEl.style.display = 'block';
            submitBtn.disabled = false;
            submitBtn.textContent = 'Sign In';
            // Reset reCAPTCHA on failure
            if (window.grecaptcha) {
                try { window.grecaptcha.reset(); } catch(e) {}
            }
        }
    });
}

function show2FAStep(container, twofaToken, panelName, logoUrl) {
    container.innerHTML = `
    <div class="login-page">
        <div class="login-card">
            <div class="login-logo" id="twofa-logo-container">
                <h1>Two-Factor Authentication</h1>
                <p>Enter the 6-digit code from your authenticator app</p>
            </div>
            <div class="login-error" id="twofa-error"></div>
            <form class="login-form" id="twofa-form">
                <div class="form-group">
                    <label class="form-label">Verification Code</label>
                    <input type="text" class="form-input" id="twofa-code" placeholder="000000" autocomplete="one-time-code" required maxlength="6" pattern="[0-9]{6}" inputmode="numeric" autofocus
                        style="text-align: center; font-size: var(--font-size-2xl); letter-spacing: 0.5em; font-weight: 600;">
                </div>
                <button type="submit" class="btn btn-primary login-btn" id="twofa-submit">
                    Verify
                </button>
            </form>
            <div style="margin-top: var(--space-4); text-align: center;">
                <a href="#${ROUTES.LOGIN}" style="color: var(--text-tertiary); font-size: var(--font-size-sm); text-decoration: none;">← Back to Login</a>
            </div>
        </div>
    </div>`;

    // Insert logo via DOM APIs to avoid XSS
    const logoContainer = document.getElementById('twofa-logo-container');
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

    document.getElementById('twofa-form').addEventListener('submit', async (e) => {
        e.preventDefault();
        const code = document.getElementById('twofa-code').value.trim();
        const errorEl = document.getElementById('twofa-error');
        const submitBtn = document.getElementById('twofa-submit');

        if (!code || code.length !== 6) {
            errorEl.textContent = 'Please enter a 6-digit code';
            errorEl.style.display = 'block';
            return;
        }

        submitBtn.disabled = true;
        submitBtn.textContent = 'Verifying...';
        errorEl.style.display = 'none';

        try {
            const data = await twofa.verify(twofaToken, code);
            setCsrfToken(data.csrf_token);
            navigateTo(ROUTES.DASHBOARD);
        } catch (err) {
            errorEl.textContent = err.message || 'Invalid code';
            errorEl.style.display = 'block';
            submitBtn.disabled = false;
            submitBtn.textContent = 'Verify';
            document.getElementById('twofa-code').value = '';
            document.getElementById('twofa-code').focus();
        }
    });
}
