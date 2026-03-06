// JCWT Ultra Panel — Login Page (Dynamic Branding + reCAPTCHA)
import { auth, setCsrfToken, request } from '../api.js';

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
