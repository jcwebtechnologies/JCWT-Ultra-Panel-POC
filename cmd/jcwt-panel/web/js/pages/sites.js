// JCWT Ultra Panel — Sites Page
import { sites, phpVersions } from '../api.js';
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
                <table class="data-table">
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
                            <td>
                                <strong>${escapeHtml(s.domain)}</strong>
                                ${s.aliases ? `<br><small style="color: var(--text-tertiary);">${escapeHtml(s.aliases)}</small>` : ''}
                            </td>
                            <td><span class="mono">${escapeHtml(s.system_user)}</span></td>
                            <td>
                                ${s.site_type === 'php' ? `<span class="badge badge-info">PHP ${escapeHtml(s.php_version)}</span>` : ''}
                                ${s.site_type === 'html' ? `<span class="badge" style="background:var(--error);color:white">HTML</span>` : ''}
                                ${s.site_type === 'proxy' ? `<span class="badge" style="background:var(--text-secondary);color:white">Proxy</span>` : ''}
                            </td>
                            <td><span class="badge ${s.ssl_type === 'none' ? 'badge-warning' : 'badge-success'}">${s.ssl_type === 'none' ? 'None' : s.ssl_type}</span></td>
                            <td style="color: var(--text-tertiary); font-size: var(--font-size-xs);">${new Date(s.created_at).toLocaleDateString()}</td>
                            <td>
                                <div class="table-actions">
                                    <a href="#/site-detail/${s.id}" class="btn btn-sm btn-secondary">Manage</a>
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
                <form id="add-site-form">
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
                            <input type="text" class="form-input" id="site-user" placeholder="example_user" required>
                            <div class="form-help">Lowercase, 2-31 chars</div>
                        </div>
                        <div class="form-group">
                            <label class="form-label">Site Type</label>
                            <select class="form-select" id="site-type">
                                <option value="php">PHP Application</option>
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
                </form>
            `, `
                <button class="btn btn-secondary" onclick="document.getElementById('modal-overlay').remove()">Cancel</button>
                <button class="btn btn-primary" id="submit-site">Create Site</button>
            `);

            // Toggle visibility of type specifics
            document.getElementById('site-type')?.addEventListener('change', (e) => {
                const type = e.target.value;
                document.getElementById('php-version-group').style.display = type === 'php' ? 'block' : 'none';
                document.getElementById('proxy-url-group').style.display = type === 'proxy' ? 'block' : 'none';
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

                try {
                    await sites.create({ domain, aliases, system_user: systemUser, site_type: siteType, php_version: phpVersion, proxy_url: proxyUrl });
                    closeModal();
                    showToast('Site created successfully!', 'success');
                    render(container);
                } catch (err) {
                    showToast(err.message, 'error');
                }
            });
        });

        // Bind delete buttons
        container.querySelectorAll('.delete-site').forEach(btn => {
            btn.addEventListener('click', async () => {
                const id = btn.dataset.id;
                const domain = btn.dataset.domain;
                if (await showConfirm('Delete Site', `Delete site "${domain}"? This will remove all configs, the system user, and web files. This action cannot be undone.`, 'Delete Site', 'btn-danger')) {
                    try {
                        await sites.delete(id);
                        showToast('Site deleted', 'success');
                        render(container);
                    } catch (err) {
                        showToast(err.message, 'error');
                    }
                }
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
