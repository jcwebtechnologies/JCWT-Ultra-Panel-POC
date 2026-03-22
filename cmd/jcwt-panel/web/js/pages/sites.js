// JCWT Ultra Panel — Sites Page
import { sites, phpVersions, databases } from '../api.js';
import { icons, showToast, showModal, closeModal, escapeHtml, showConfirm } from '../app.js';
import { siteHref } from '../routes.js';
import { showLoading } from '../ui.js';
import { sslBadge } from '../css-classes.js';

export async function render(container) {
    showLoading(container);

    try {
        const [siteList, versions] = await Promise.all([sites.list(), phpVersions.list()]);

        container.innerHTML = `
        <div class="page-header">
            <div class="page-header-left">
                <h2>Managed Sites</h2>
                <p>Create and manage your websites</p>
            </div>
            <button class="btn btn-primary" id="add-site-btn">
                <span class="nav-icon nav-icon-sm">${icons.plus}</span>
                Add Site
            </button>
        </div>

        ${siteList.length === 0 ? `
            <div class="empty-state">
                <div class="empty-state-icon"><span class="nav-icon" style="width:32px;height:32px;color:var(--primary)">${icons.sites}</span></div>
                <div class="empty-state-title">No sites yet</div>
                <div class="empty-state-text">Create your first website to get started.</div>
            </div>
        ` : `
            <div class="table-container">
                <table class="data-table responsive-cards">
                    <thead>
                        <tr>
                            <th>Domain</th>
                            <th>System User</th>
                            <th>Type</th>
                            <th>SSL</th>
                            <th>Created</th>
                            <th>Actions</th>
                        </tr>
                    </thead>
                    <tbody>
                        ${siteList.map(s => `
                        <tr>
                            <td data-label="Domain">
                                <strong>${escapeHtml(s.domain)}</strong>
                                ${s.aliases ? `<br><small style="color: var(--text-tertiary);">${escapeHtml(s.aliases)}</small>` : ''}
                            </td>
                            <td data-label="User"><span class="mono">${escapeHtml(s.system_user)}</span></td>
                            <td data-label="Type">
                                ${s.site_type === 'php' ? `<span class="badge badge-info">PHP ${escapeHtml(s.php_version)}</span>` : ''}
                                ${s.site_type === 'wordpress' ? `<span class="badge" style="background:#21759b;color:white">WordPress</span>` : ''}
                                ${s.site_type === 'html' ? `<span class="badge" style="background:var(--error);color:white">HTML</span>` : ''}
                                ${s.site_type === 'proxy' ? `<span class="badge" style="background:var(--text-secondary);color:white">Proxy</span>` : ''}
                            </td>
                            <td data-label="SSL"><span class="${sslBadge(s.ssl_type)}">${s.ssl_type === 'none' ? 'None' : s.ssl_type}</span></td>
                            <td data-label="Created" style="color: var(--text-tertiary); font-size: var(--font-size-xs);">${new Date(s.created_at).toLocaleDateString()}</td>
                            <td>
                                <div class="table-actions">
                                    <a href="${siteHref(s.token)}" class="btn btn-sm btn-secondary">Manage</a>
                                    <button class="btn btn-sm btn-danger delete-site" data-id="${s.id}" data-domain="${escapeHtml(s.domain)}">Delete</button>
                                </div>
                            </td>
                        </tr>
                        `).join('')}
                    </tbody>
                </table>
            </div>
        `}`;

        // Bind add site
        document.getElementById('add-site-btn')?.addEventListener('click', () => {
            const versionOptions = versions.map(v => `<option value="${v}"${v === '8.4' ? ' selected' : ''}>PHP ${v}</option>`).join('');
            showModal('Add New Site', `
                <form id="add-site-form" autocomplete="off">
                    <div class="form-row">
                        <div class="form-group">
                            <label class="form-label">Domain</label>
                            <input type="text" class="form-input" id="site-domain" placeholder="example.com" required>
                        </div>
                        <div class="form-group">
                            <label class="form-label">Aliases (optional)</label>
                            <input type="text" class="form-input" id="site-aliases" placeholder="www.example.com alias.com">
                            <div class="form-help">Space-separated domain aliases</div>
                        </div>
                    </div>
                    <div class="form-group">
                        <label class="form-label">System User</label>
                        <div id="sysuser-auto-wrap">
                            <div style="display:flex;gap:0;">
                                <select class="form-select" id="sysuser-mode" style="border-radius:var(--radius-md) 0 0 var(--radius-md);border-right:none;width:auto;min-width:140px;">
                                    <option value="auto">Auto Generate</option>
                                    <option value="custom">Custom</option>
                                </select>
                                <input type="text" class="form-input mono" id="site-user" readonly style="border-radius:0;flex:1;">
                                <button type="button" class="btn btn-secondary" id="sysuser-refresh" title="Regenerate username" style="border-radius:0 var(--radius-md) var(--radius-md) 0;border-left:none;padding:0 var(--space-3);display:flex;align-items:center;"><span class="nav-icon nav-icon-sm">${icons.refreshCw}</span></button>
                            </div>
                        </div>
                        <div class="form-help" id="sysuser-help">Auto-generated (u_ + 9 random chars)</div>
                    </div>
                    <div class="form-group">
                        <label class="form-label">Site Type</label>
                        <select class="form-select" id="site-type">
                            <option value="php">PHP Application</option>
                            <option value="wordpress">WordPress</option>
                            <option value="html">Static HTML</option>
                            <option value="proxy">Reverse Proxy</option>
                        </select>
                    </div>
                    <div class="form-row">
                        <div class="form-group" id="php-version-group">
                            <label class="form-label">PHP Version</label>
                            <select class="form-select" id="site-php">${versionOptions}</select>
                        </div>
                        <div class="form-group" id="proxy-url-group" style="display: none;">
                            <label class="form-label">Backend URL</label>
                            <input type="url" class="form-input" id="site-proxy" placeholder="http://127.0.0.1:3000">
                        </div>
                    </div>
                    <div id="wp-fields" style="display: none;">
                        <div style="border-top: 1px solid var(--border-primary); margin: var(--space-3) 0; padding-top: var(--space-3);">
                            <div style="font-weight: 600; margin-bottom: var(--space-2); font-size: var(--font-size-sm); color: var(--text-secondary);">WordPress Admin</div>
                            <div class="form-row">
                                <div class="form-group">
                                    <label class="form-label">Site Title</label>
                                    <input type="text" class="form-input" id="wp-site-title" placeholder="My WordPress Site">
                                </div>
                                <div class="form-group">
                                    <label class="form-label">Admin Username</label>
                                    <input type="text" class="form-input" id="wp-admin-user" placeholder="admin" value="admin" autocomplete="off">
                                </div>
                            </div>
                            <div class="form-row">
                                <div class="form-group">
                                    <label class="form-label">Admin Email</label>
                                    <input type="email" class="form-input" id="wp-admin-email" placeholder="admin@example.com" required>
                                </div>
                                <div class="form-group">
                                    <label class="form-label">Admin Password</label>
                                    <div style="position:relative;">
                                        <input type="password" class="form-input" id="wp-admin-pass" placeholder="Strong password" required autocomplete="new-password" style="padding-right:40px;">
                                        <button type="button" id="toggle-wp-pass" style="position:absolute;right:8px;top:50%;transform:translateY(-50%);background:none;border:none;cursor:pointer;padding:4px;color:var(--text-tertiary);display:flex;align-items:center;" title="Toggle password visibility"><span class="nav-icon nav-icon-md">${icons.eye}</span></button>
                                    </div>
                                </div>
                            </div>
                            <div class="form-group">
                                <label class="form-label">Table Prefix</label>
                                <input type="text" class="form-input" id="wp-table-prefix" placeholder="wp_" value="wp_" autocomplete="off" maxlength="20" pattern="^[a-zA-Z_][a-zA-Z0-9_]*_$">
                                <div class="form-help">Must end with underscore. Letters, numbers, underscore only.</div>
                            </div>
                        </div>
                    </div>
                </form>
            `, `
                <button class="btn btn-secondary" id="cancel-site-btn">Cancel</button>
                <button class="btn btn-primary" id="submit-site">Create Site</button>
            `, { persistent: true });

            // Cancel button handler
            document.getElementById('cancel-site-btn')?.addEventListener('click', () => closeModal());

            // Toggle WP admin password visibility
            document.getElementById('toggle-wp-pass')?.addEventListener('click', () => {
                const passInput = document.getElementById('wp-admin-pass');
                const toggleBtn = document.getElementById('toggle-wp-pass');
                if (passInput.type === 'password') {
                    passInput.type = 'text';
                    toggleBtn.innerHTML = `<span class="nav-icon nav-icon-md">${icons.eyeOff}</span>`;
                } else {
                    passInput.type = 'password';
                    toggleBtn.innerHTML = `<span class="nav-icon nav-icon-md">${icons.eye}</span>`;
                }
            });

            // Toggle visibility of type specifics
            document.getElementById('site-type')?.addEventListener('change', (e) => {
                const type = e.target.value;
                document.getElementById('php-version-group').style.display = (type === 'php' || type === 'wordpress') ? 'block' : 'none';
                document.getElementById('proxy-url-group').style.display = type === 'proxy' ? 'block' : 'none';
                document.getElementById('wp-fields').style.display = type === 'wordpress' ? 'block' : 'none';
            });

            // --- System User: auto-generate / custom toggle ---
            const RESERVED = ['root','admin','mysql','www','nginx','apache','ftp','user','test','panel','daemon','bin','sys','nobody','www_data'];
            function genUser() {
                const letters = 'abcdefghijklmnopqrstuvwxyz';
                const chars = 'abcdefghijklmnopqrstuvwxyz0123456789';
                // first char is a letter, then 8 more alphanumeric = 9 total chars after u_
                let s = letters[Math.floor(Math.random() * 26)];
                for (let i = 0; i < 8; i++) s += chars[Math.floor(Math.random() * chars.length)];
                return 'u_' + s;
            }
            function validateUser(val) {
                if (!/^[a-z][a-z0-9_]{2,15}$/.test(val)) return 'Must be 3-16 chars, start with letter, lowercase letters/numbers/underscore only';
                if (val.includes('__')) return 'No consecutive underscores';
                if (RESERVED.includes(val)) return 'Reserved name';
                return '';
            }
            const userInput = document.getElementById('site-user');
            const modeSelect = document.getElementById('sysuser-mode');
            const refreshBtn = document.getElementById('sysuser-refresh');
            const helpText = document.getElementById('sysuser-help');
            // Set initial auto value
            userInput.value = genUser();
            refreshBtn.addEventListener('click', () => {
                if (modeSelect.value === 'auto') userInput.value = genUser();
            });
            modeSelect.addEventListener('change', () => {
                if (modeSelect.value === 'auto') {
                    userInput.readOnly = true;
                    userInput.value = genUser();
                    refreshBtn.style.display = '';
                    helpText.textContent = 'Auto-generated (u_ + 9 random chars)';
                    helpText.style.color = '';
                } else {
                    userInput.readOnly = false;
                    userInput.value = '';
                    userInput.placeholder = 'my_site_user';
                    refreshBtn.style.display = 'none';
                    helpText.textContent = '3-16 chars, start with letter, lowercase a-z 0-9 _ only';
                    helpText.style.color = '';
                }
            });
            userInput.addEventListener('input', () => {
                if (modeSelect.value === 'custom') {
                    const err = validateUser(userInput.value);
                    helpText.textContent = err || '3-16 chars, start with letter, lowercase a-z 0-9 _ only';
                    helpText.style.color = err ? 'var(--status-error)' : '';
                }
            });

            document.getElementById('submit-site')?.addEventListener('click', async () => {
                const domain = document.getElementById('site-domain').value.trim();
                const aliases = document.getElementById('site-aliases').value.trim();
                const systemUser = document.getElementById('site-user').value.trim();
                const siteType = document.getElementById('site-type').value;
                const phpVersion = document.getElementById('site-php').value;
                const proxyUrl = document.getElementById('site-proxy').value.trim();

                if (!domain || !systemUser) {
                    showToast('Domain and system user are required', 'error');
                    return;
                }

                const userErr = validateUser(systemUser);
                if (userErr) {
                    showToast('System user: ' + userErr, 'error');
                    return;
                }
                
                if (siteType === 'proxy' && !proxyUrl) {
                    showToast('Backend URL is required for proxy sites', 'error');
                    return;
                }

                const data = { domain, aliases, system_user: systemUser, site_type: siteType, php_version: phpVersion, proxy_url: proxyUrl };

                // WordPress-specific fields & validation
                if (siteType === 'wordpress') {
                    const wpEmail = document.getElementById('wp-admin-email').value.trim();
                    const wpPass = document.getElementById('wp-admin-pass').value.trim();
                    const wpUser = document.getElementById('wp-admin-user').value.trim() || 'admin';
                    const wpTitle = document.getElementById('wp-site-title').value.trim() || domain;

                    if (!wpEmail || !wpPass) {
                        showToast('WordPress admin email and password are required', 'error');
                        return;
                    }
                    const wpTablePrefix = document.getElementById('wp-table-prefix').value.trim() || 'wp_';
                    if (!/^[a-zA-Z_][a-zA-Z0-9_]*_$/.test(wpTablePrefix) || wpTablePrefix.length > 20) {
                        showToast('Table prefix must end with underscore, contain only letters/numbers/underscore, and be at most 20 characters', 'error');
                        return;
                    }
                    data.wp_admin_user = wpUser;
                    data.wp_admin_email = wpEmail;
                    data.wp_admin_password = wpPass;
                    data.wp_site_title = wpTitle;
                    data.wp_table_prefix = wpTablePrefix;
                }

                // Show progress spinner — disable all modal interactions
                const submitBtn = document.getElementById('submit-site');
                const cancelBtn = document.getElementById('cancel-site-btn');
                const origText = submitBtn.textContent;
                submitBtn.disabled = true;
                submitBtn.innerHTML = '<span class="loading-spinner btn-spinner"></span>' + (siteType === 'wordpress' ? 'Installing WordPress...' : 'Creating Site...');
                cancelBtn.disabled = true;
                cancelBtn.style.opacity = '0.5';
                cancelBtn.style.pointerEvents = 'none';

                // Disable all form inputs
                document.querySelectorAll('#add-site-form input, #add-site-form select').forEach(el => el.disabled = true);

                try {
                    await sites.create(data);
                    closeModal();
                    showToast(siteType === 'wordpress' ? 'WordPress site created successfully!' : 'Site created successfully!', 'success');
                    render(container);
                } catch (err) {
                    // Re-enable everything on error
                    submitBtn.disabled = false;
                    submitBtn.textContent = origText;
                    cancelBtn.disabled = false;
                    cancelBtn.style.opacity = '';
                    cancelBtn.style.pointerEvents = '';
                    document.querySelectorAll('#add-site-form input, #add-site-form select').forEach(el => el.disabled = false);
                    showToast(err.message, 'error');
                }
            });
        });

        // Bind delete buttons
        container.querySelectorAll('.delete-site').forEach(btn => {
            btn.addEventListener('click', async () => {
                const id = btn.dataset.id;
                const domain = btn.dataset.domain;
                let siteDbs = [];
                try {
                    const allDbs = await databases.list();
                    siteDbs = (allDbs || []).filter(db => String(db.site_id) === String(id));
                } catch {}
                const dbList = siteDbs.length > 0
                    ? `<div style="margin-top: var(--space-3); padding: var(--space-2) var(--space-3); background: var(--bg-secondary); border: 1px solid var(--border-primary); border-radius: var(--radius-md);"><div style="font-weight: 600; margin-bottom: var(--space-1); font-size: var(--font-size-sm);">Databases that will also be deleted:</div>${siteDbs.map(db => `<div style="font-size: var(--font-size-xs); color: var(--text-secondary);">• <span class="mono">${escapeHtml(db.db_name)}</span></div>`).join('')}</div>`
                    : '';
                showModal('Delete Site', `
                    <p style="color: var(--text-secondary); font-size: var(--font-size-sm); line-height: 1.6;">Delete site "<strong>${escapeHtml(domain)}</strong>"? This will remove all configs, the system user, web files, databases and backups. This action cannot be undone.</p>
                    ${dbList}
                `, `
                    <button class="btn btn-secondary" onclick="document.getElementById('modal-overlay').remove()">Cancel</button>
                    <button class="btn btn-danger" id="confirm-delete-site">Delete Site</button>
                `);
                document.getElementById('confirm-delete-site')?.addEventListener('click', async () => {
                    const delBtn = document.getElementById('confirm-delete-site');
                    delBtn.disabled = true;
                    delBtn.innerHTML = '<span class="loading-spinner btn-spinner"></span> Deleting...';
                    try {
                        await sites.delete(id);
                        closeModal();
                        showToast('Site deleted', 'success');
                        render(container);
                    } catch (err) {
                        delBtn.disabled = false;
                        delBtn.textContent = 'Delete Site';
                        showToast(err.message, 'error');
                    }
                });
            });
        });

    } catch (err) {
        container.innerHTML = `<div class="empty-state">
            <div class="empty-state-icon"><span class="nav-icon" style="width:48px;height:48px;color:var(--status-warning)">${icons.alertTriangle}</span></div>
            <div class="empty-state-title">Error</div>
            <div class="empty-state-text">${err.message}</div>
        </div>`;
    }
}
