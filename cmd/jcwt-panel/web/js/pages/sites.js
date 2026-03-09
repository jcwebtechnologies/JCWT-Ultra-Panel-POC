// JCWT Ultra Panel — Sites Page
import { sites, phpVersions, databases } from '../api.js';
import { icons, showToast, showModal, closeModal, escapeHtml, showConfirm } from '../app.js';

export async function render(container) {
    document.getElementById('page-title').textContent = 'Sites';

    container.innerHTML = '<div class="loading-screen"><div class="loading-spinner"></div></div>';

    try {
        const [siteList, versions] = await Promise.all([sites.list(), phpVersions.list()]);

        container.innerHTML = `
        <div class="page-header">
            <div class="page-header-left">
                <h2>Managed Sites</h2>
                <p>Create and manage your websites</p>
            </div>
            <button class="btn btn-primary" id="add-site-btn">
                <span class="nav-icon" style="width:16px;height:16px;">${icons.plus}</span>
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
                            <td data-label="SSL"><span class="badge ${s.ssl_type === 'none' ? 'badge-warning' : 'badge-success'}">${s.ssl_type === 'none' ? 'None' : s.ssl_type}</span></td>
                            <td data-label="Created" style="color: var(--text-tertiary); font-size: var(--font-size-xs);">${new Date(s.created_at).toLocaleDateString()}</td>
                            <td>
                                <div class="table-actions">
                                    <a href="#/sites/${s.token}" class="btn btn-sm btn-secondary">Manage</a>
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
            const versionOptions = versions.map(v => `<option value="${v}">PHP ${v}</option>`).join('');
            showModal('Add New Site', `
                <form id="add-site-form" autocomplete="off">
                    <div class="form-group">
                        <label class="form-label">Domain</label>
                        <input type="text" class="form-input" id="site-domain" placeholder="example.com" required>
                    </div>
                    <div class="form-group">
                        <label class="form-label">Aliases (optional)</label>
                        <input type="text" class="form-input" id="site-aliases" placeholder="www.example.com alias.com">
                        <div class="form-help">Space-separated domain aliases</div>
                    </div>
                    <div class="form-row">
                        <div class="form-group">
                            <label class="form-label">System User</label>
                            <input type="text" class="form-input" id="site-user" placeholder="example_user" required autocomplete="off">
                            <div class="form-help">Lowercase, 2-31 chars</div>
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
                                    <input type="password" class="form-input" id="wp-admin-pass" placeholder="Strong password" required autocomplete="new-password">
                                </div>
                            </div>
                        </div>
                    </div>
                </form>
            `, `
                <button class="btn btn-secondary" id="cancel-site-btn">Cancel</button>
                <button class="btn btn-primary" id="submit-site">Create Site</button>
            `, { persistent: false });

            // Cancel button handler
            document.getElementById('cancel-site-btn')?.addEventListener('click', () => closeModal());

            // Toggle visibility of type specifics
            document.getElementById('site-type')?.addEventListener('change', (e) => {
                const type = e.target.value;
                document.getElementById('php-version-group').style.display = (type === 'php' || type === 'wordpress') ? 'block' : 'none';
                document.getElementById('proxy-url-group').style.display = type === 'proxy' ? 'block' : 'none';
                document.getElementById('wp-fields').style.display = type === 'wordpress' ? 'block' : 'none';
            });

            // Auto-generate user from domain
            document.getElementById('site-domain')?.addEventListener('input', (e) => {
                const domain = e.target.value;
                const user = domain.replace(/\./g, '_').replace(/[^a-z0-9_]/g, '').substring(0, 30);
                const userInput = document.getElementById('site-user');
                if (userInput && !userInput.dataset.manual) {
                    userInput.value = user;
                }
            });
            document.getElementById('site-user')?.addEventListener('input', (e) => {
                e.target.dataset.manual = 'true';
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
                    data.wp_admin_user = wpUser;
                    data.wp_admin_email = wpEmail;
                    data.wp_admin_password = wpPass;
                    data.wp_site_title = wpTitle;
                }

                // Show progress spinner — disable all modal interactions
                const submitBtn = document.getElementById('submit-site');
                const cancelBtn = document.getElementById('cancel-site-btn');
                const origText = submitBtn.textContent;
                submitBtn.disabled = true;
                submitBtn.innerHTML = '<span class="loading-spinner" style="width:14px;height:14px;border-width:2px;display:inline-block;vertical-align:middle;margin-right:6px;"></span>' + (siteType === 'wordpress' ? 'Installing WordPress...' : 'Creating Site...');
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
                    <p style="color: var(--text-secondary); font-size: var(--font-size-sm); line-height: 1.6;">Delete site "<strong>${escapeHtml(domain)}</strong>"? This will remove all configs, the system user, web files, databasesand backups. This action cannot be undone.</p>
                    ${dbList}
                `, `
                    <button class="btn btn-secondary" onclick="document.getElementById('modal-overlay').remove()">Cancel</button>
                    <button class="btn btn-danger" id="confirm-delete-site">Delete Site</button>
                `);
                document.getElementById('confirm-delete-site')?.addEventListener('click', async () => {
                    closeModal();
                    try {
                        await sites.delete(id);
                        showToast('Site deleted', 'success');
                        render(container);
                    } catch (err) {
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
