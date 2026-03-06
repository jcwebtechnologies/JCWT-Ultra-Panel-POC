// JCWT Ultra Panel — Login Page (Dynamic Branding + reCAPTCHA + 2FA)
import { auth, setCsrfToken, request, twofa } from '../api.js';

let recaptchaLoaded = false;

export async function render(container) {
    // Fetch public settings for branding + reCAPTCHA
    let branding = {};
    try {
        branding = await request('/api/settings/public');
    } catch (e) { /* use defaults */ }

    const panelName = branding.panel_name || 'JCWT Ultra Panel';
    const panelTagline = branding.panel_tagline || 'IPv6-Native Hosting Control Panel';
    const logoUrl = branding.logo_url || '';
    const recaptchaSiteKey = branding.recaptcha_site_key || '';

    container.innerHTML = `
    <div class="login-page">
        <div class="login-card">
            <div class="login-logo">
                ${logoUrl
                    ? `<img src="${logoUrl}" alt="${panelName}" style="width: 48px; height: 48px; border-radius: 12px; margin-bottom: 8px;">`
                    : `<div class="logo-icon">${panelName.charAt(0)}</div>`
                }
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
                    <input type="password" class="form-input" id="login-password" placeholder="••••••••" autocomplete="current-password" required maxlength="200">
                </div>
                ${recaptchaSiteKey ? `<div id="recaptcha-container" style="margin-bottom: var(--space-3);"></div>` : ''}
                <button type="submit" class="btn btn-primary login-btn" id="login-submit">
                    Sign In
                </button>
            </form>
        </div>
    </div>`;

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
                        theme: document.body.classList.contains('dark-theme') ? 'dark' : 'light',
                    });
                });
            }
        };
        document.head.appendChild(script);
    } else if (recaptchaSiteKey && recaptchaLoaded && window.grecaptcha) {
        window.grecaptcha.ready(() => {
            window.grecaptcha.render('recaptcha-container', {
                sitekey: recaptchaSiteKey,
                theme: document.body.classList.contains('dark-theme') ? 'dark' : 'light',
            });
        });
    }

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
            window.location.hash = '#/dashboard';
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

function escapeHtml(str) {
    const div = document.createElement('div');
    div.textContent = str;
    return div.innerHTML;
}

function show2FAStep(container, twofaToken, panelName, logoUrl) {
    container.innerHTML = `
    <div class="login-page">
        <div class="login-card">
            <div class="login-logo">
                ${logoUrl
                    ? `<img src="${logoUrl}" alt="${panelName}" style="width: 48px; height: 48px; border-radius: 12px; margin-bottom: 8px;">`
                    : `<div class="logo-icon">${panelName.charAt(0)}</div>`
                }
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
                <a href="#/login" style="color: var(--text-tertiary); font-size: var(--font-size-sm); text-decoration: none;">← Back to Login</a>
            </div>
        </div>
    </div>`;

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
            window.location.hash = '#/dashboard';
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
