// JCWT Ultra Panel — Site Detail Page (SSL, PHP Settings, Cron, Files)
import { sites, phpVersions, ssl, phpSettings, cron, files, databases, dbUsers } from '../api.js';
import { icons, showToast, showModal, closeModal, escapeHtml, formatBytes, showConfirm } from '../app.js';
import { request } from '../api.js';

export async function render(container, siteToken, section) {
    document.getElementById('page-title').textContent = 'Site Management';
    container.innerHTML = '<div class="loading-screen"><div class="loading-spinner"></div></div>';

    if (!siteToken) { container.innerHTML = '<p>No site selected</p>'; return; }

    try {
        const [site, versions] = await Promise.all([
            sites.getByToken(siteToken),
            phpVersions.list()
        ]);

        const siteId = site.id;
        let activeSection = section || null;

        function renderPage() {
            container.innerHTML = `
            <div class="page-header" style="margin-bottom: var(--space-5);">
                <div class="page-header-left">
                    <h2 style="display: flex; align-items: center; gap: var(--space-2);">
                        <a href="http://${escapeHtml(site.domain)}" target="_blank" rel="noopener" style="color: inherit; text-decoration: none;" title="Open site in new tab">${escapeHtml(site.domain)} <span style="font-size: var(--font-size-xs); opacity: 0.4;">↗</span></a>
                    </h2>
                    <div style="display: flex; align-items: center; gap: var(--space-4); margin-top: var(--space-1); color: var(--text-secondary); font-size: var(--font-size-sm);">
                        <span style="display: flex; align-items: center; gap: var(--space-1);"><span class="nav-icon" style="width:14px;height:14px;opacity:0.5;">${icons.key}</span> ${escapeHtml(site.system_user)}</span>
                        <span style="display: flex; align-items: center; gap: var(--space-1);"><span class="nav-icon" style="width:14px;height:14px;opacity:0.5;">${icons.folder}</span> ${escapeHtml(site.web_root)}</span>
                    </div>
                </div>
                <a href="#/sites" class="btn btn-secondary" style="display: inline-flex; align-items: center; gap: var(--space-2);"><span class="nav-icon" style="width:16px;height:16px;">${icons.sites}</span> All Sites</a>
            </div>

            ${activeSection ? `<div style="margin-bottom: var(--space-4);"><a href="#/sites/${escapeHtml(siteToken)}" class="btn btn-sm btn-ghost" style="display: inline-flex; align-items: center; gap: var(--space-2);"><span class="nav-icon" style="width:14px;height:14px;">${icons.dashboard}</span> Site Overview</a></div>` : `
            <div class="site-cards-section">
                <div class="site-cards-section-title">Configuration</div>
                <div class="site-cards-grid">
                    <div class="site-card" data-section="overview">
                        <div class="site-card-icon blue"><span class="nav-icon" style="width:28px;height:28px">${icons.settings}</span></div>
                        <div class="site-card-title">Site Settings</div>
                    </div>
                    ${site.site_type === 'php' ? `
                    <div class="site-card" data-section="php">
                        <div class="site-card-icon purple"><span class="nav-icon" style="width:28px;height:28px">${icons.code}</span></div>
                        <div class="site-card-title">PHP Settings</div>
                    </div>` : ''}
                    <div class="site-card" data-section="vhost">
                        <div class="site-card-icon"><span class="nav-icon" style="width:28px;height:28px">${icons.file}</span></div>
                        <div class="site-card-title">Vhost Editor</div>
                    </div>
                </div>
            </div>

            <div class="site-cards-section">
                <div class="site-cards-section-title">Content</div>
                <div class="site-cards-grid">
                    <div class="site-card" data-section="files">
                        <div class="site-card-icon green"><span class="nav-icon" style="width:28px;height:28px">${icons.folder}</span></div>
                        <div class="site-card-title">File Manager</div>
                    </div>
                    <div class="site-card" data-section="databases">
                        <div class="site-card-icon blue"><span class="nav-icon" style="width:28px;height:28px">${icons.database}</span></div>
                        <div class="site-card-title">Databases</div>
                    </div>
                    <div class="site-card" data-section="dbusers">
                        <div class="site-card-icon purple"><span class="nav-icon" style="width:28px;height:28px">${icons.users}</span></div>
                        <div class="site-card-title">Database Users</div>
                    </div>
                    <div class="site-card" data-section="cron">
                        <div class="site-card-icon orange"><span class="nav-icon" style="width:28px;height:28px">${icons.clock}</span></div>
                        <div class="site-card-title">Cron Jobs</div>
                    </div>
                    <div class="site-card" data-section="backups">
                        <div class="site-card-icon purple"><span class="nav-icon" style="width:28px;height:28px">${icons.download}</span></div>
                        <div class="site-card-title">Backups</div>
                    </div>
                </div>
            </div>

            <div class="site-cards-section">
                <div class="site-cards-section-title">Security & SSL</div>
                <div class="site-cards-grid">
                    <div class="site-card" data-section="ssl">
                        <div class="site-card-icon green"><span class="nav-icon" style="width:28px;height:28px">${icons.lock}</span></div>
                        <div class="site-card-title">SSL Certificates</div>
                    </div>
                    <div class="site-card" data-section="security">
                        <div class="site-card-icon red"><span class="nav-icon" style="width:28px;height:28px">${icons.shield}</span></div>
                        <div class="site-card-title">Security</div>
                    </div>
                </div>
            </div>

            <div class="site-cards-section">
                <div class="site-cards-section-title">Monitoring</div>
                <div class="site-cards-grid">
                    <div class="site-card" data-section="logs">
                        <div class="site-card-icon orange"><span class="nav-icon" style="width:28px;height:28px">${icons.search}</span></div>
                        <div class="site-card-title">Logs</div>
                    </div>
                </div>
            </div>
            `}

            <div id="section-content"></div>`;

            // Bind card clicks — navigate to URL-based sections
            container.querySelectorAll('.site-card').forEach(card => {
                card.addEventListener('click', () => {
                    window.location.hash = `#/sites/${siteToken}/${card.dataset.section}`;
                });
            });

            // Render active section content
            const sectionContent = document.getElementById('section-content');
            if (activeSection && sectionContent) {
                switch (activeSection) {
                    case 'overview': renderOverview(sectionContent, site, versions, siteId); break;
                    case 'php': renderPHP(sectionContent, siteId); break;
                    case 'databases': renderDatabases(sectionContent, siteId, site, renderPage); break;
                    case 'dbusers': renderDBUsers(sectionContent, siteId, site, renderPage); break;
                    case 'ssl': renderSSL(sectionContent, site, siteId); break;
                    case 'cron': renderCron(sectionContent, siteId); break;
                    case 'security': renderSecurity(sectionContent, site, siteId, renderPage); break;
                    case 'files': renderFiles(sectionContent, siteId); break;
                    case 'vhost': renderVhost(sectionContent, site, siteId); break;
                    case 'backups': renderBackups(sectionContent, site, siteId); break;
                    case 'logs': renderLogs(sectionContent, site, siteId); break;
                }
            }
        }
        renderPage();

    } catch (err) {
        container.innerHTML = `<div class="empty-state"><div class="empty-state-title">Error: ${err.message}</div></div>`;
    }
}

function renderOverview(el, site, versions, siteId) {
    const versionOpts = versions.map(v => `<option value="${v}" ${v === site.php_version ? 'selected' : ''}>PHP ${v}</option>`).join('');
    el.innerHTML = `
    <div class="card">
        <h3 class="card-title" style="margin-bottom: var(--space-4);">Site Configuration</h3>
        <form id="update-site-form">
            <div class="form-row">
                <div class="form-group">
                    <label class="form-label">Domain</label>
                    <input type="text" class="form-input" id="edit-domain" value="${escapeHtml(site.domain)}">
                </div>
                <div class="form-group">
                    <label class="form-label">Domain Aliases</label>
                    <input type="text" class="form-input" id="edit-aliases" value="${escapeHtml(site.aliases || '')}" placeholder="www.example.com alias.com">
                </div>
            </div>
            <div class="form-row">
                <div class="form-group">
                    <label class="form-label">Site Type</label>
                    <select class="form-select" id="edit-type">
                        <option value="php" ${site.site_type === 'php' ? 'selected' : ''}>PHP Application</option>
                        <option value="html" ${site.site_type === 'html' ? 'selected' : ''}>Static HTML</option>
                        <option value="proxy" ${site.site_type === 'proxy' ? 'selected' : ''}>Reverse Proxy</option>
                    </select>
                </div>
                <div class="form-group" id="edit-php-group" style="${site.site_type === 'php' ? '' : 'display: none;'}">
                    <label class="form-label">PHP Version</label>
                    <select class="form-select" id="edit-php">${versionOpts}</select>
                </div>
            </div>
            <div class="form-group" id="edit-proxy-group" style="${site.site_type === 'proxy' ? '' : 'display: none;'}">
                <label class="form-label">Backend URL</label>
                <input type="url" class="form-input" id="edit-proxy" value="${escapeHtml(site.proxy_url || '')}" placeholder="http://127.0.0.1:3000">
            </div>
            <div class="form-group" id="edit-webroot-group" style="${site.site_type !== 'proxy' ? '' : 'display: none;'}">
                <label class="form-label">Web Root</label>
                <input type="text" class="form-input mono" id="edit-webroot" value="${escapeHtml(site.web_root)}" placeholder="/home/user/htdocs">
                <small style="color: var(--text-tertiary); font-size: var(--font-size-xs);">Must be under /home/. Changing this only updates the config — the directory must already exist.</small>
            </div>
            <div class="info-grid" style="margin-bottom: var(--space-4);">
                <div class="info-item"><span class="info-label">System User</span><span class="info-value mono">${escapeHtml(site.system_user)}</span></div>
                <div class="info-item"><span class="info-label">SSL</span><span class="info-value"><span class="badge ${site.ssl_type === 'none' ? 'badge-warning' : 'badge-success'}">${site.ssl_type}</span></span></div>
                <div class="info-item"><span class="info-label">Disk Usage</span><span class="info-value" id="site-disk-usage"><span style="color:var(--text-tertiary);font-size:var(--font-size-xs);">Loading...</span></span></div>
                <div class="info-item"><span class="info-label">Created</span><span class="info-value">${new Date(site.created_at).toLocaleDateString()}</span></div>
            </div>
            <button type="submit" class="btn btn-primary" style="width: auto;">Save Changes</button>
        </form>
    </div>`;

    document.getElementById('edit-type')?.addEventListener('change', (e) => {
        const type = e.target.value;
        document.getElementById('edit-php-group').style.display = type === 'php' ? 'block' : 'none';
        document.getElementById('edit-proxy-group').style.display = type === 'proxy' ? 'block' : 'none';
        document.getElementById('edit-webroot-group').style.display = type !== 'proxy' ? 'block' : 'none';
    });

    // Deferred disk usage load
    sites.diskUsage(siteId).then(data => {
        const el2 = document.getElementById('site-disk-usage');
        if (el2) el2.innerHTML = `<span class="badge badge-info">${escapeHtml(data.size)}</span>`;
    }).catch(() => {
        const el2 = document.getElementById('site-disk-usage');
        if (el2) el2.textContent = 'N/A';
    });

    document.getElementById('update-site-form').addEventListener('submit', async (e) => {
        e.preventDefault();

        const siteType = document.getElementById('edit-type').value;
        const proxyUrl = (document.getElementById('edit-proxy')?.value || '').trim();
        const domain = document.getElementById('edit-domain').value.trim();
        const webRoot = (document.getElementById('edit-webroot')?.value || '').trim();

        // Client-side validation
        if (!domain) {
            showToast('Domain is required', 'error');
            return;
        }
        if (!/^[a-zA-Z0-9]([a-zA-Z0-9\-]*[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9\-]*[a-zA-Z0-9])?)*$/.test(domain)) {
            showToast('Invalid domain format', 'error');
            return;
        }
        if (siteType === 'proxy' && !proxyUrl) {
            showToast('Backend URL is required for proxy sites', 'error');
            return;
        }
        if (siteType === 'proxy' && proxyUrl && !proxyUrl.startsWith('http')) {
            showToast('Backend URL must start with http:// or https://', 'error');
            return;
        }
        if (webRoot && !webRoot.startsWith('/home/')) {
            showToast('Web root must be under /home/', 'error');
            return;
        }

        try {
            await sites.update({
                id: parseInt(siteId),
                domain: domain,
                aliases: (document.getElementById('edit-aliases')?.value || '').trim(),
                site_type: siteType,
                php_version: document.getElementById('edit-php')?.value || '',
                proxy_url: proxyUrl,
                web_root: webRoot,
                ssl_type: site.ssl_type,
            });
            showToast('Site updated!', 'success');

            // Reload page
            const mod = await import('./site-detail.js');
            mod.render(document.getElementById('page-content'), site.token, 'overview');

        } catch (err) { showToast(err.message, 'error'); }
    });
}

async function renderPHP(el, siteId) {
    const phpOpts = {
        memory_limit: ['128M', '256M', '512M', '768M', '1024M', '2048M'],
        max_execution_time: ['30', '60', '120', '300', '600', '900'],
        max_input_time: ['60', '120', '300', '600', '900'],
        max_input_vars: ['1000', '2000', '3000', '5000', '10000'],
        post_max_size: ['2M', '4M', '8M', '16M', '32M', '64M', '128M', '256M', '512M', '1024M'],
        upload_max_filesize: ['2M', '4M', '8M', '16M', '32M', '64M', '128M', '256M', '512M', '1024M'],
    };
    function opts(key, current) {
        return phpOpts[key].map(v => `<option value="${v}" ${String(v) === String(current) ? 'selected' : ''}>${v}</option>`).join('');
    }
    try {
        const settings = await phpSettings.get(siteId);
        const site = await request(`/api/sites?id=${siteId}`);
        const poolPath = site.php_version && site.system_user ? `/etc/php/${site.php_version}/fpm/pool.d/${site.system_user}.conf` : '';
        el.innerHTML = `
        <div class="card">
            <h3 class="card-title" style="margin-bottom: var(--space-4);">PHP Pool Settings</h3>
            ${poolPath ? `<div class="info-item" style="margin-bottom:var(--space-4);"><span class="info-label">Pool Config</span><span class="info-value mono" style="font-size:var(--font-size-xs);">${escapeHtml(poolPath)}</span></div>` : ''}
            <form id="php-settings-form">
                <div class="form-row">
                    <div class="form-group">
                        <label class="form-label">memory_limit</label>
                        <select class="form-select" id="php-memory">${opts('memory_limit', settings.memory_limit || '256M')}</select>
                    </div>
                    <div class="form-group">
                        <label class="form-label">max_execution_time <span style="font-weight:400;color:var(--text-tertiary)">(seconds)</span></label>
                        <select class="form-select" id="php-exec-time">${opts('max_execution_time', settings.max_execution_time || 30)}</select>
                    </div>
                </div>
                <div class="form-row">
                    <div class="form-group">
                        <label class="form-label">max_input_time <span style="font-weight:400;color:var(--text-tertiary)">(seconds)</span></label>
                        <select class="form-select" id="php-input-time">${opts('max_input_time', settings.max_input_time || 60)}</select>
                    </div>
                    <div class="form-group">
                        <label class="form-label">max_input_vars</label>
                        <select class="form-select" id="php-input-vars">${opts('max_input_vars', settings.max_input_vars || 1000)}</select>
                    </div>
                </div>
                <div class="form-row">
                    <div class="form-group">
                        <label class="form-label">post_max_size</label>
                        <select class="form-select" id="php-post-max">${opts('post_max_size', settings.post_max_size || '64M')}</select>
                    </div>
                    <div class="form-group">
                        <label class="form-label">upload_max_filesize</label>
                        <select class="form-select" id="php-upload-max">${opts('upload_max_filesize', settings.upload_max_filesize || '64M')}</select>
                    </div>
                </div>
                <div class="form-group">
                    <label class="form-label">Additional Directives</label>
                    <textarea class="form-textarea" id="php-custom" placeholder="date.timezone=UTC;&#10;display_errors=off;">${escapeHtml(settings.custom_directives || '')}</textarea>
                    <div class="form-help">Semi-colon separated key=value pairs</div>
                </div>
                <button type="submit" class="btn btn-primary">Save PHP Settings</button>
            </form>
        </div>`;

        document.getElementById('php-settings-form').addEventListener('submit', async (e) => {
            e.preventDefault();
            try {
                await phpSettings.update({
                    site_id: parseInt(siteId),
                    memory_limit: document.getElementById('php-memory').value,
                    max_execution_time: parseInt(document.getElementById('php-exec-time').value),
                    max_input_time: parseInt(document.getElementById('php-input-time').value),
                    max_input_vars: parseInt(document.getElementById('php-input-vars').value),
                    post_max_size: document.getElementById('php-post-max').value,
                    upload_max_filesize: document.getElementById('php-upload-max').value,
                    custom_directives: document.getElementById('php-custom').value,
                });
                showToast('PHP settings saved & FPM reloaded!', 'success');
            } catch (err) { showToast(err.message, 'error'); }
        });
    } catch (err) { el.innerHTML = `<p>Error: ${err.message}</p>`; }
}

function renderSSL(el, site, siteId) {
    el.innerHTML = '<div class="loading-screen"><div class="loading-spinner"></div></div>';

    request(`/api/ssl-certs?site_id=${siteId}`).then(data => {
        const certs = data.certificates || [];
        const activeCert = certs.find(c => c.active);
        const hasSelfSigned = certs.some(c => c.type === 'self-signed');

        el.innerHTML = `
        <div class="card" style="margin-bottom: var(--space-4);">
            <div class="card-header">
                <h3 class="card-title">SSL Certificates</h3>
            </div>
            <div style="padding: var(--space-4);">
                <div class="info-item" style="margin-bottom: var(--space-4);">
                    <span class="info-label">Active Certificate</span>
                    <span class="info-value"><span class="badge ${site.ssl_type === 'none' ? 'badge-warning' : 'badge-success'}">${site.ssl_type === 'none' ? 'None' : site.ssl_type}</span></span>
                </div>
                ${activeCert && activeCert.cert_path ? `<div class="info-item" style="margin-bottom: var(--space-4);"><span class="info-label">Certificate Path</span><span class="info-value mono" style="font-size: var(--font-size-xs);">${escapeHtml(activeCert.cert_path)}</span></div>` : ''}

                <div style="display: flex; gap: var(--space-3); flex-wrap: wrap; margin-bottom: var(--space-4);">
                    ${!hasSelfSigned ? `<button class="btn btn-primary" id="ssl-self-signed">${icons.lock} Generate Self-Signed</button>` : ''}
                    <button class="btn btn-secondary" id="ssl-custom">${icons.upload} Upload Certificate</button>
                </div>
            </div>
        </div>

        ${certs.length > 0 ? `
        <div class="card">
            <div class="card-header">
                <h3 class="card-title">Uploaded Certificates</h3>
            </div>
            <div class="table-responsive">
                <table class="data-table">
                    <thead><tr><th>Type</th><th>Label</th><th>Uploaded</th><th>Status</th><th>Actions</th></tr></thead>
                    <tbody>
                        ${certs.map(c => `
                        <tr>
                            <td><span class="badge ${c.type === 'self-signed' ? 'badge-warning' : 'badge-info'}">${escapeHtml(c.type)}</span></td>
                            <td>${escapeHtml(c.label || c.type)}</td>
                            <td style="font-size: var(--font-size-xs);">${c.created_at ? new Date(c.created_at).toLocaleDateString() : 'N/A'}</td>
                            <td>${c.active ? '<span class="badge badge-success">Active</span>' : '<span class="badge" style="background:var(--bg-tertiary);color:var(--text-tertiary)">Inactive</span>'}</td>
                            <td>
                                <div class="table-actions">
                                    ${!c.active ? `<button class="btn btn-sm btn-primary activate-cert" data-id="${c.id}">Activate</button>` : ''}
                                    ${!c.active ? `<button class="btn btn-sm btn-danger delete-cert" data-id="${c.id}">Delete</button>` : ''}
                                </div>
                            </td>
                        </tr>`).join('')}
                    </tbody>
                </table>
            </div>
        </div>` : ''}`;

        // Generate self-signed
        document.getElementById('ssl-self-signed')?.addEventListener('click', async () => {
            try {
                await ssl.selfSigned(siteId);
                showToast('Self-signed certificate generated!', 'success');
                const mod = await import('./site-detail.js');
                mod.render(document.getElementById('page-content'), site.token, 'ssl');
            } catch (err) { showToast(err.message, 'error'); }
        });

        // Upload custom cert
        document.getElementById('ssl-custom')?.addEventListener('click', () => {
            showModal('Upload SSL Certificate', `
                <form id="upload-cert-form">
                    <div class="form-group">
                        <label class="form-label">Label (optional)</label>
                        <input type="text" class="form-input" id="cert-label" placeholder="e.g. Let's Encrypt 2025">
                    </div>
                    <div class="form-group">
                        <label class="form-label">Certificate (.pem, .crt)</label>
                        <input type="file" class="form-input" id="cert-file" accept=".pem,.crt" required>
                    </div>
                    <div class="form-group">
                        <label class="form-label">Private Key (.pem, .key)</label>
                        <input type="file" class="form-input" id="key-file" accept=".pem,.key" required>
                    </div>
                </form>
            `, `
                <button class="btn btn-secondary" onclick="document.getElementById('modal-overlay').remove()">Cancel</button>
                <button class="btn btn-primary" id="submit-cert">Upload</button>
            `);

            document.getElementById('submit-cert')?.addEventListener('click', async () => {
                const certInput = document.getElementById('cert-file');
                const keyInput = document.getElementById('key-file');
                if (!certInput.files[0] || !keyInput.files[0]) { showToast('Both files required', 'error'); return; }
                const formData = new FormData();
                formData.append('certificate', certInput.files[0]);
                formData.append('private_key', keyInput.files[0]);
                formData.append('label', document.getElementById('cert-label')?.value || '');
                try {
                    await ssl.custom(siteId, formData);
                    closeModal();
                    showToast('Certificate uploaded & activated!', 'success');
                    const mod = await import('./site-detail.js');
                    mod.render(document.getElementById('page-content'), site.token, 'ssl');
                } catch (err) { showToast(err.message, 'error'); }
            });
        });

        // Activate cert
        el.querySelectorAll('.activate-cert').forEach(btn => {
            btn.addEventListener('click', async () => {
                try {
                    await request(`/api/ssl-certs?action=activate`, {
                        method: 'POST',
                        body: JSON.stringify({ cert_id: parseInt(btn.dataset.id), site_id: parseInt(siteId) }),
                    });
                    showToast('Certificate activated!', 'success');
                    const mod = await import('./site-detail.js');
                    mod.render(document.getElementById('page-content'), site.token, 'ssl');
                } catch (err) { showToast(err.message, 'error'); }
            });
        });

        // Delete cert
        el.querySelectorAll('.delete-cert').forEach(btn => {
            btn.addEventListener('click', async () => {
                if (!await showConfirm('Delete Certificate', 'Delete this certificate permanently?', 'Delete', 'btn-danger')) return;
                try {
                    await request(`/api/ssl-certs?id=${btn.dataset.id}&site_id=${siteId}`, { method: 'DELETE' });
                    showToast('Certificate deleted', 'success');
                    const mod = await import('./site-detail.js');
                    mod.render(document.getElementById('page-content'), site.token, 'ssl');
                } catch (err) { showToast(err.message, 'error'); }
            });
        });
    }).catch(err => {
        // Fallback to old-style SSL card if ssl-certs endpoint not available
        el.innerHTML = `
        <div class="card">
            <h3 class="card-title" style="margin-bottom: var(--space-4);">SSL Certificate</h3>
            <div class="info-item" style="margin-bottom: var(--space-4);">
                <span class="info-label">Current Status</span>
                <span class="info-value"><span class="badge ${site.ssl_type === 'none' ? 'badge-warning' : 'badge-success'}">${site.ssl_type}</span></span>
            </div>
            ${site.ssl_cert_path ? `<div class="info-item" style="margin-bottom: var(--space-4);"><span class="info-label">Certificate Path</span><span class="info-value mono">${escapeHtml(site.ssl_cert_path)}</span></div>` : ''}
            <div style="display: flex; gap: var(--space-3); flex-wrap: wrap;">
                <button class="btn btn-primary" id="ssl-self-signed">${icons.lock} Generate Self-Signed</button>
                <button class="btn btn-secondary" id="ssl-custom">${icons.upload} Upload Custom Certificate</button>
            </div>
        </div>`;

        document.getElementById('ssl-self-signed')?.addEventListener('click', async () => {
            try {
                await ssl.selfSigned(siteId);
                showToast('Self-signed certificate generated!', 'success');
                const mod = await import('./site-detail.js');
                mod.render(document.getElementById('page-content'), site.token, 'ssl');
            } catch (err) { showToast(err.message, 'error'); }
        });

        document.getElementById('ssl-custom')?.addEventListener('click', () => {
            showModal('Upload SSL Certificate', `
                <form id="upload-cert-form">
                    <div class="form-group">
                        <label class="form-label">Certificate (.pem, .crt)</label>
                        <input type="file" class="form-input" id="cert-file" accept=".pem,.crt" required>
                    </div>
                    <div class="form-group">
                        <label class="form-label">Private Key (.pem, .key)</label>
                        <input type="file" class="form-input" id="key-file" accept=".pem,.key" required>
                    </div>
                </form>
            `, `
                <button class="btn btn-secondary" onclick="document.getElementById('modal-overlay').remove()">Cancel</button>
                <button class="btn btn-primary" id="submit-cert">Upload</button>
            `);

            document.getElementById('submit-cert')?.addEventListener('click', async () => {
                const certInput = document.getElementById('cert-file');
                const keyInput = document.getElementById('key-file');
                if (!certInput.files[0] || !keyInput.files[0]) { showToast('Both files required', 'error'); return; }
                const formData = new FormData();
                formData.append('certificate', certInput.files[0]);
                formData.append('private_key', keyInput.files[0]);
                try {
                    await ssl.custom(siteId, formData);
                    closeModal();
                    showToast('Certificate uploaded!', 'success');
                    const mod = await import('./site-detail.js');
                    mod.render(document.getElementById('page-content'), site.token, 'ssl');
                } catch (err) { showToast(err.message, 'error'); }
            });
        });

    });
}

async function renderCron(el, siteId) {
    try {
        const jobs = await cron.list(siteId);
        el.innerHTML = `
        <div class="card">
            <div class="card-header">
                <h3 class="card-title">Cron Jobs</h3>
                <button class="btn btn-primary btn-sm" id="add-cron-btn">${icons.plus} Add</button>
            </div>
            ${jobs.length === 0 ? `
                <div class="empty-state" style="padding: var(--space-6);">
                    <div class="empty-state-title">No cron jobs</div>
                    <div class="empty-state-text">Add scheduled tasks for this site.</div>
                </div>
            ` : `
                <div class="table-container" style="border: none;">
                    <table class="data-table">
                        <thead><tr><th>Schedule</th><th>Command</th><th>Status</th><th>Actions</th></tr></thead>
                        <tbody>
                            ${jobs.map(j => `
                            <tr>
                                <td class="mono">${escapeHtml(j.schedule)}</td>
                                <td class="mono truncate" style="max-width: 300px;">${escapeHtml(j.command)}</td>
                                <td><span class="badge ${j.enabled ? 'badge-success' : 'badge-warning'}">${j.enabled ? 'Active' : 'Paused'}</span></td>
                                <td>
                                    <div class="table-actions">
                                        <button class="btn btn-sm btn-ghost toggle-cron" data-id="${j.id}" data-enabled="${j.enabled}" data-schedule="${escapeHtml(j.schedule)}" data-command="${escapeHtml(j.command)}">${j.enabled ? 'Pause' : 'Enable'}</button>
                                        <button class="btn btn-sm btn-danger delete-cron" data-id="${j.id}">Delete</button>
                                    </div>
                                </td>
                            </tr>`).join('')}
                        </tbody>
                    </table>
                </div>
            `}
        </div>`;

        document.getElementById('add-cron-btn')?.addEventListener('click', () => {
            showModal('Add Cron Job', `
                <div class="form-group">
                    <label class="form-label">Schedule</label>
                    <input type="text" class="form-input" id="cron-schedule" placeholder="*/5 * * * *">
                    <div class="form-help">Cron expression: min hour day month weekday</div>
                </div>
                <div class="form-group">
                    <label class="form-label">Command</label>
                    <input type="text" class="form-input" id="cron-command" placeholder="/usr/bin/php /home/user/htdocs/cron.php">
                </div>
            `, `
                <button class="btn btn-secondary" onclick="document.getElementById('modal-overlay').remove()">Cancel</button>
                <button class="btn btn-primary" id="submit-cron">Add Job</button>
            `);

            document.getElementById('submit-cron')?.addEventListener('click', async () => {
                try {
                    await cron.create({
                        site_id: parseInt(siteId),
                        schedule: document.getElementById('cron-schedule').value,
                        command: document.getElementById('cron-command').value,
                    });
                    closeModal();
                    showToast('Cron job added!', 'success');
                    renderCron(el, siteId);
                } catch (err) { showToast(err.message, 'error'); }
            });
        });

        el.querySelectorAll('.toggle-cron').forEach(btn => {
            btn.addEventListener('click', async () => {
                try {
                    await cron.update({
                        id: parseInt(btn.dataset.id),
                        site_id: parseInt(siteId),
                        schedule: btn.dataset.schedule,
                        command: btn.dataset.command,
                        enabled: btn.dataset.enabled !== 'true',
                    });
                    showToast('Cron job updated', 'success');
                    renderCron(el, siteId);
                } catch (err) { showToast(err.message, 'error'); }
            });
        });

        el.querySelectorAll('.delete-cron').forEach(btn => {
            btn.addEventListener('click', async () => {
                try {
                    await cron.delete(btn.dataset.id, siteId);
                    showToast('Cron job deleted', 'success');
                    renderCron(el, siteId);
                } catch (err) { showToast(err.message, 'error'); }
            });
        });

    } catch (err) { el.innerHTML = `<p>Error: ${err.message}</p>`; }
}

async function renderFiles(el, siteId) {
    el.innerHTML = `
    <div class="card" style="padding: var(--space-4);">
        <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: var(--space-3);">
            <h3 class="card-title" style="margin: 0;">File Manager</h3>
            <div>
                <button class="btn btn-sm btn-ghost" id="fb-reload">↻ Reload</button>
            </div>
        </div>
        <div id="fb-container" style="min-height: 500px; display: flex; align-items: center; justify-content: center;">
            <div class="empty-state">
                <div class="loading-spinner" style="margin: 0 auto var(--space-3);"></div>
                <div class="empty-state-title">Starting File Browser...</div>
                <div class="empty-state-text">Please wait while the file manager initializes.</div>
            </div>
        </div>
    </div>`;

    try {
        const data = await files.list(siteId);
        const fbUrl = data.url || `/fb/${siteId}/`;

        // Load iframe with retry logic
        let retries = 0;
        const maxRetries = 5;

        function loadIframe() {
            const container = document.getElementById('fb-container');
            if (!container) return;

            container.innerHTML = `
                <iframe src="${fbUrl}"
                        style="width: 100%; height: 70vh; border: 1px solid var(--border-primary); border-radius: var(--radius-md);"
                        id="fb-iframe"></iframe>`;

            const iframe = document.getElementById('fb-iframe');
            if (!iframe) return;

            // Set a timeout — if iframe doesn't load in 5 seconds, retry
            const loadTimeout = setTimeout(() => {
                if (retries < maxRetries) {
                    retries++;
                    container.innerHTML = `
                        <div class="empty-state" style="padding: var(--space-4);">
                            <div class="loading-spinner" style="margin: 0 auto var(--space-3);"></div>
                            <div class="empty-state-text">File Browser is starting up... (attempt ${retries + 1}/${maxRetries + 1})</div>
                        </div>`;
                    setTimeout(loadIframe, 2000);
                } else {
                    container.innerHTML = `
                        <div class="empty-state" style="padding: var(--space-6);">
                            <div class="empty-state-title">File Browser Unavailable</div>
                            <div class="empty-state-text">Could not connect after ${maxRetries + 1} attempts. The file browser binary may not be installed or has failed to start.</div>
                            <button class="btn btn-primary btn-sm" id="fb-manual-retry">Retry</button>
                        </div>`;
                    document.getElementById('fb-manual-retry')?.addEventListener('click', () => {
                        retries = 0;
                        loadIframe();
                    });
                }
            }, 5000);

            iframe.addEventListener('load', () => clearTimeout(loadTimeout));
        }

        // Small delay to let File Browser fully bind
        setTimeout(loadIframe, 500);

        document.getElementById('fb-reload')?.addEventListener('click', () => {
            const iframe = document.getElementById('fb-iframe');
            if (iframe) iframe.src = iframe.src;
        });
    } catch (err) {
        document.getElementById('fb-container').innerHTML = `
            <div class="empty-state" style="padding: var(--space-6);">
                <div class="empty-state-title">File Browser Error</div>
                <div class="empty-state-text">${escapeHtml(err.message)}</div>
                <button class="btn btn-primary btn-sm" onclick="location.reload()">Retry</button>
            </div>`;
    }
}

// ---- Databases Tab ----
async function renderDatabases(container, siteId, site, refreshTabs) {
    container.innerHTML = '<div class="loading-screen"><div class="loading-spinner"></div></div>';

    try {
        const allDbs = await databases.list();
        const siteDbs = (allDbs || []).filter(db => String(db.site_id) === String(siteId));
        const allUsers = await request('/api/db-users');

        container.innerHTML = `
        <div class="card">
            <div class="card-header">
                <h3 class="card-title">Databases for ${escapeHtml(site.domain)}</h3>
                <button class="btn btn-primary btn-sm" id="add-site-db">${icons.plus} Create Database</button>
            </div>
            ${siteDbs.length === 0 ? `
            <div class="empty-state" style="padding: var(--space-6);">
                <div class="empty-state-title">No databases</div>
                <div class="empty-state-text">Create a database linked to this site.</div>
            </div>` : `
            <div class="table-responsive">
                <table class="data-table">
                    <thead><tr><th>Database Name</th><th>Users</th><th>Created</th><th>Actions</th></tr></thead>
                    <tbody>
                        ${siteDbs.map(db => {
            const dbUsers = (allUsers || []).filter(u => u.database_id === db.id);
            return `<tr>
                                <td><span class="mono">${escapeHtml(db.db_name)}</span></td>
                                <td>${dbUsers.length > 0 ? dbUsers.map(u => `<span class="badge badge-info">${escapeHtml(u.username)}</span>`).join(' ') : '<span style="color: var(--text-tertiary);">None</span>'}</td>
                                <td>${db.created_at ? new Date(db.created_at).toLocaleDateString() : 'N/A'}</td>
                                <td style="display: flex; gap: var(--space-1);">
                                    <button class="btn btn-sm btn-secondary open-pma" data-id="${db.id}" data-name="${escapeHtml(db.db_name)}" title="Open phpMyAdmin">⛁ phpMyAdmin</button>
                                    <button class="btn btn-sm btn-danger delete-site-db" data-id="${db.id}" data-name="${escapeHtml(db.db_name)}">Delete</button>
                                </td>
                            </tr>`;
        }).join('')}
                    </tbody>
                </table>
            </div>`}
        </div>`;

        // Create DB button
        document.getElementById('add-site-db')?.addEventListener('click', () => {
            showModal('Create Database', `
                <div class="form-group">
                    <label class="form-label">Database Name</label>
                    <input type="text" class="form-input" id="new-db-name" placeholder="myapp_db" required pattern="^[a-zA-Z][a-zA-Z0-9_]*$" maxlength="64">
                    <small style="color: var(--text-tertiary); font-size: var(--font-size-xs);">Letters, numbers, underscore only. Must start with a letter.</small>
                </div>
            `, `
                <button class="btn btn-secondary" onclick="document.getElementById('modal-overlay').remove()">Cancel</button>
                <button class="btn btn-primary" id="confirm-create-db">Create</button>
            `);

            document.getElementById('confirm-create-db')?.addEventListener('click', async () => {
                const dbName = document.getElementById('new-db-name').value.trim();
                if (!dbName) { showToast('Name required', 'error'); return; }
                if (!/^[a-zA-Z][a-zA-Z0-9_]*$/.test(dbName)) {
                    showToast('Invalid name: use letters, numbers, underscore only (start with a letter)', 'error'); return;
                }
                try {
                    await databases.create({ db_name: dbName, site_id: parseInt(siteId) });
                    closeModal();
                    showToast('Database created!', 'success');
                    renderDatabases(container, siteId, site, refreshTabs);
                } catch (err) { showToast(err.message, 'error'); }
            });
        });

        // Delete DB buttons
        container.querySelectorAll('.delete-site-db').forEach(btn => {
            btn.addEventListener('click', async () => {
                const id = btn.dataset.id;
                const name = btn.dataset.name;
                if (!await showConfirm('Delete Database', `Delete database "${name}"? This will also drop it from MariaDB.`, 'Delete', 'btn-danger')) return;
                try {
                    await databases.delete(id);
                    showToast(`Database ${name} deleted`, 'success');
                    renderDatabases(container, siteId, site, refreshTabs);
                } catch (err) { showToast(err.message, 'error'); }
            });
        });

        // phpMyAdmin access buttons
        container.querySelectorAll('.open-pma').forEach(btn => {
            btn.addEventListener('click', async () => {
                const dbId = btn.dataset.id;
                const dbName = btn.dataset.name;
                btn.disabled = true;
                btn.textContent = '⏳ Opening...';
                try {
                    const data = await request('/api/pma', {
                        method: 'POST',
                        body: JSON.stringify({ database_id: parseInt(dbId) })
                    });
                    if (data.url) {
                        window.open(data.url, '_blank');
                        showToast(`phpMyAdmin opened for ${dbName}`, 'success');
                    }
                } catch (err) {
                    showToast(`phpMyAdmin error: ${err.message}`, 'error');
                } finally {
                    btn.disabled = false;
                    btn.textContent = '⛁ phpMyAdmin';
                }
            });
        });
    } catch (err) {
        container.innerHTML = `<div class="empty-state"><div class="empty-state-title">Error: ${escapeHtml(err.message)}</div></div>`;
    }
}

// ---- Database Users Tab ----
async function renderDBUsers(container, siteId, site, refreshTabs) {
    container.innerHTML = '<div class="loading-screen"><div class="loading-spinner"></div></div>';

    const privilegeLabels = {
        'readonly': 'Read Only',
        'readwrite': 'Read / Write',
        'full': 'Full',
        'administrator': 'Administrator'
    };
    const privilegeBadge = {
        'readonly': 'badge-warning',
        'readwrite': 'badge-info',
        'full': 'badge-success',
        'administrator': 'badge-primary'
    };

    try {
        const allDbs = await databases.list();
        const siteDbs = (allDbs || []).filter(db => String(db.site_id) === String(siteId));
        const allUsers = await dbUsers.list();
        const siteDbIds = new Set(siteDbs.map(db => db.id));
        const siteUsers = (allUsers || []).filter(u => siteDbIds.has(u.database_id));

        container.innerHTML = `
        <div class="card">
            <div class="card-header">
                <h3 class="card-title">Database Users for ${escapeHtml(site.domain)}</h3>
                <button class="btn btn-primary btn-sm" id="add-db-user" ${siteDbs.length === 0 ? 'disabled title="Create a database first"' : ''}>${icons.plus} Create User</button>
            </div>
            ${siteUsers.length === 0 ? `
            <div class="empty-state" style="padding: var(--space-6);">
                <div class="empty-state-title">No database users</div>
                <div class="empty-state-text">${siteDbs.length === 0 ? 'Create a database first, then add users.' : 'Create a user with specific privileges for a database.'}</div>
            </div>` : `
            <div class="table-responsive">
                <table class="data-table">
                    <thead><tr><th>Username</th><th>Database</th><th>Privileges</th><th>Created</th><th>Actions</th></tr></thead>
                    <tbody>
                        ${siteUsers.map(u => `<tr>
                                <td>${escapeHtml(u.username)}</td>
                                <td><span class="badge badge-info">${escapeHtml(u.db_name)}</span></td>
                                <td>
                                    <select class="form-select form-select-sm change-privilege" data-id="${u.id}" data-username="${escapeHtml(u.username)}" style="width: auto; padding: var(--space-1) var(--space-2); font-size: var(--font-size-xs);">
                                        ${Object.entries(privilegeLabels).map(([val, label]) => `<option value="${val}" ${u.privilege_level === val ? 'selected' : ''}>${label}</option>`).join('')}
                                    </select>
                                </td>
                                <td>${u.created_at ? new Date(u.created_at).toLocaleDateString() : 'N/A'}</td>
                                <td style="display: flex; gap: var(--space-1);">
                                    <button class="btn btn-sm btn-secondary change-pwd" data-username="${escapeHtml(u.username)}" title="Change Password">${icons.key} Password</button>
                                    <button class="btn btn-sm btn-secondary open-pma-user" data-id="${u.database_id}" data-name="${escapeHtml(u.db_name)}" title="Open phpMyAdmin">⛁ phpMyAdmin</button>
                                    <button class="btn btn-sm btn-danger delete-db-user" data-id="${u.id}" data-username="${escapeHtml(u.username)}">Delete</button>
                                </td>
                            </tr>`).join('')}
                    </tbody>
                </table>
            </div>`}
        </div>`;

        // Create User button
        document.getElementById('add-db-user')?.addEventListener('click', () => {
            const dbOpts = siteDbs.map(db => `<option value="${db.id}">${escapeHtml(db.db_name)}</option>`).join('');
            showModal('Create Database User', `
                <div class="form-group">
                    <label class="form-label">Username</label>
                    <input type="text" class="form-input" id="new-dbuser-name" placeholder="app_user" required pattern="^[a-zA-Z][a-zA-Z0-9_]*$" maxlength="32">
                    <small style="color: var(--text-tertiary); font-size: var(--font-size-xs);">Letters, numbers, underscore only. Must start with a letter.</small>
                </div>
                <div class="form-group">
                    <label class="form-label">Password</label>
                    <input type="password" class="form-input" id="new-dbuser-pass" placeholder="Min 8 characters" required minlength="8">
                </div>
                <div class="form-group">
                    <label class="form-label">Database</label>
                    <select class="form-select" id="new-dbuser-db">${dbOpts}</select>
                </div>
                <div class="form-group">
                    <label class="form-label">Privilege Level</label>
                    <select class="form-select" id="new-dbuser-priv">
                        <option value="administrator">Administrator — Full control</option>
                        <option value="full">Full — All DML + DDL operations</option>
                        <option value="readwrite">Read / Write — SELECT, INSERT, UPDATE, DELETE</option>
                        <option value="readonly">Read Only — SELECT only</option>
                    </select>
                </div>
            `, `
                <button class="btn btn-secondary" onclick="document.getElementById('modal-overlay').remove()">Cancel</button>
                <button class="btn btn-primary" id="confirm-create-dbuser">Create</button>
            `);

            document.getElementById('confirm-create-dbuser')?.addEventListener('click', async () => {
                const username = document.getElementById('new-dbuser-name').value.trim();
                const password = document.getElementById('new-dbuser-pass').value;
                const databaseId = parseInt(document.getElementById('new-dbuser-db').value);
                const privilegeLevel = document.getElementById('new-dbuser-priv').value;
                if (!username) { showToast('Username required', 'error'); return; }
                if (password.length < 8) { showToast('Password must be at least 8 characters', 'error'); return; }
                try {
                    await dbUsers.create({ username, password, database_id: databaseId, privilege_level: privilegeLevel });
                    closeModal();
                    showToast('Database user created!', 'success');
                    renderDBUsers(container, siteId, site, refreshTabs);
                } catch (err) { showToast(err.message, 'error'); }
            });
        });

        // Privilege change dropdowns
        container.querySelectorAll('.change-privilege').forEach(sel => {
            sel.addEventListener('change', async () => {
                const id = parseInt(sel.dataset.id);
                const newLevel = sel.value;
                try {
                    await dbUsers.updatePrivilege({ id, privilege_level: newLevel });
                    showToast(`Privileges updated for ${sel.dataset.username}`, 'success');
                } catch (err) {
                    showToast(err.message, 'error');
                    renderDBUsers(container, siteId, site, refreshTabs);
                }
            });
        });

        // Change Password buttons
        container.querySelectorAll('.change-pwd').forEach(btn => {
            btn.addEventListener('click', () => {
                const username = btn.dataset.username;
                showModal('Change Password', `
                    <p style="margin-bottom: var(--space-3); color: var(--text-secondary);">Change password for <strong>${escapeHtml(username)}</strong></p>
                    <div class="form-group">
                        <label class="form-label">New Password</label>
                        <input type="password" class="form-input" id="dbuser-new-pwd" placeholder="Min 8 characters" required minlength="8">
                    </div>
                `, `
                    <button class="btn btn-secondary" onclick="document.getElementById('modal-overlay').remove()">Cancel</button>
                    <button class="btn btn-primary" id="confirm-change-pwd">Change Password</button>
                `);
                document.getElementById('confirm-change-pwd')?.addEventListener('click', async () => {
                    const newPwd = document.getElementById('dbuser-new-pwd').value;
                    if (newPwd.length < 8) { showToast('Password must be at least 8 characters', 'error'); return; }
                    try {
                        await dbUsers.changePassword({ username, new_password: newPwd });
                        closeModal();
                        showToast('Password changed', 'success');
                    } catch (err) { showToast(err.message, 'error'); }
                });
            });
        });

        // phpMyAdmin buttons
        container.querySelectorAll('.open-pma-user').forEach(btn => {
            btn.addEventListener('click', async () => {
                const dbId = btn.dataset.id;
                const dbName = btn.dataset.name;
                btn.disabled = true;
                btn.textContent = '⏳ Opening...';
                try {
                    const data = await request('/api/pma', {
                        method: 'POST',
                        body: JSON.stringify({ database_id: parseInt(dbId) })
                    });
                    if (data.url) {
                        window.open(data.url, '_blank');
                        showToast(`phpMyAdmin opened for ${dbName}`, 'success');
                    }
                } catch (err) {
                    showToast(`phpMyAdmin error: ${err.message}`, 'error');
                } finally {
                    btn.disabled = false;
                    btn.textContent = '⛁ phpMyAdmin';
                }
            });
        });

        // Delete buttons
        container.querySelectorAll('.delete-db-user').forEach(btn => {
            btn.addEventListener('click', async () => {
                const id = btn.dataset.id;
                const username = btn.dataset.username;
                if (!await showConfirm('Delete User', `Delete database user "${username}"? This will also drop the MariaDB user.`, 'Delete', 'btn-danger')) return;
                try {
                    await dbUsers.delete(id);
                    showToast(`User ${username} deleted`, 'success');
                    renderDBUsers(container, siteId, site, refreshTabs);
                } catch (err) { showToast(err.message, 'error'); }
            });
        });
    } catch (err) {
        container.innerHTML = `<div class="empty-state"><div class="empty-state-title">Error: ${escapeHtml(err.message)}</div></div>`;
    }
}

// ---- Security Tab (Basic Auth + Deletion Protection) ----
async function renderSecurity(container, site, siteId, refreshTabs) {
    const basicAuthEnabled = site.basic_auth_enabled === 1 || site.basic_auth_enabled === true;
    const deleteProtected = site.delete_protection === 1 || site.delete_protection === true;
    let authUsers = [];
    try {
        authUsers = site.basic_auth_users ? JSON.parse(site.basic_auth_users) : [];
    } catch { authUsers = []; }

    container.innerHTML = `
    <div class="card" style="margin-bottom: var(--space-4);">
        <div class="card-header">
            <h3 class="card-title">Basic Authentication</h3>
        </div>
        <div style="padding: var(--space-4);">
            <p style="color: var(--text-secondary); margin-bottom: var(--space-4); font-size: var(--font-size-sm);">
                Protect this site with HTTP Basic Authentication. Visitors will need to enter a username and password before accessing the site.
            </p>
            <div class="settings-row" style="margin-bottom: var(--space-4);">
                <div class="settings-row-label">Enable Basic Auth<small>Require login to access site</small></div>
                <div>
                    <label class="toggle">
                        <input type="checkbox" id="basic-auth-toggle" ${basicAuthEnabled ? 'checked' : ''}>
                        <span class="toggle-slider"></span>
                    </label>
                </div>
            </div>

            <div id="auth-users-section" style="${basicAuthEnabled ? '' : 'display:none;'}">
                <h4 style="margin-bottom: var(--space-3); font-size: var(--font-size-sm); font-weight: 600;">Authorized Users</h4>
                <div id="auth-users-list">
                    ${authUsers.length === 0 ? '<p style="color: var(--text-tertiary); font-size: var(--font-size-sm);">No users configured. Add at least one user to enable basic auth.</p>' : ''}
                    ${authUsers.map((u, i) => `
                    <div class="auth-user-row" style="display: flex; gap: var(--space-2); align-items: center; margin-bottom: var(--space-2);">
                        <input type="text" class="form-input auth-username" value="${escapeHtml(u.username || '')}" placeholder="Username" style="flex: 1;">
                        <div style="flex: 1; position: relative; display: flex;">
                            <input type="password" class="form-input auth-password" value="${escapeHtml(u.password || '')}" placeholder="Password" style="flex: 1; padding-right: 2.5rem;">
                            <button type="button" class="btn btn-sm toggle-pwd-btn" style="position: absolute; right: 4px; top: 50%; transform: translateY(-50%); padding: 2px 6px; font-size: 14px; background: none; border: none; cursor: pointer; opacity: 0.6;" title="Toggle password">👁</button>
                        </div>
                        <button class="btn btn-sm btn-danger remove-auth-user" data-index="${i}">✕</button>
                    </div>`).join('')}
                </div>
                <button class="btn btn-sm btn-secondary" id="add-auth-user" style="margin-top: var(--space-2);">${icons.plus} Add User</button>
            </div>
        </div>
    </div>
    <div class="card" style="margin-bottom: var(--space-4);">
        <div class="card-header">
            <h3 class="card-title">Deletion Protection</h3>
        </div>
        <div style="padding: var(--space-4);">
            <p style="color: var(--text-secondary); margin-bottom: var(--space-4); font-size: var(--font-size-sm);">
                Prevent accidental deletion of this site. While enabled, the site cannot be deleted until this toggle is turned off.
            </p>
            <div class="settings-row">
                <div class="settings-row-label">Enable Deletion Protection<small>Block site deletion</small></div>
                <div>
                    <label class="toggle">
                        <input type="checkbox" id="delete-protection-toggle" ${deleteProtected ? 'checked' : ''}>
                        <span class="toggle-slider"></span>
                    </label>
                </div>
            </div>
        </div>
    </div>
    <button class="btn btn-primary" id="save-basic-auth">Save Security Settings</button>`;

    // Toggle show/hide users section
    document.getElementById('basic-auth-toggle')?.addEventListener('change', (e) => {
        document.getElementById('auth-users-section').style.display = e.target.checked ? '' : 'none';
    });

    // Add user row
    document.getElementById('add-auth-user')?.addEventListener('click', () => {
        const list = document.getElementById('auth-users-list');
        const index = list.querySelectorAll('.auth-user-row').length;
        const row = document.createElement('div');
        row.className = 'auth-user-row';
        row.style = 'display: flex; gap: var(--space-2); align-items: center; margin-bottom: var(--space-2);';
        row.innerHTML = `
            <input type="text" class="form-input auth-username" placeholder="Username" style="flex: 1;">
            <div style="flex: 1; position: relative; display: flex;">
                <input type="password" class="form-input auth-password" placeholder="Password" style="flex: 1; padding-right: 2.5rem;">
                <button type="button" class="btn btn-sm toggle-pwd-btn" style="position: absolute; right: 4px; top: 50%; transform: translateY(-50%); padding: 2px 6px; font-size: 14px; background: none; border: none; cursor: pointer; opacity: 0.6;" title="Toggle password">👁</button>
            </div>
            <button class="btn btn-sm btn-danger remove-auth-user" data-index="${index}">✕</button>`;
        // Remove "No users" message if present
        const noUsers = list.querySelector('p');
        if (noUsers) noUsers.remove();
        list.appendChild(row);
        row.querySelector('.remove-auth-user').addEventListener('click', () => row.remove());
        bindToggle(row.querySelector('.toggle-pwd-btn'));
    });

    // Password toggle binding
    function bindToggle(btn) {
        btn.addEventListener('click', () => {
            const pwdInput = btn.parentElement.querySelector('.auth-password');
            if (pwdInput.type === 'password') {
                pwdInput.type = 'text';
                btn.textContent = '👁‍🗨';
            } else {
                pwdInput.type = 'password';
                btn.textContent = '👁';
            }
        });
    }
    container.querySelectorAll('.toggle-pwd-btn').forEach(bindToggle);

    // Remove user buttons
    container.querySelectorAll('.remove-auth-user').forEach(btn => {
        btn.addEventListener('click', () => btn.closest('.auth-user-row').remove());
    });

    // Save
    document.getElementById('save-basic-auth')?.addEventListener('click', async () => {
        const enabled = document.getElementById('basic-auth-toggle').checked;
        const users = [];
        container.querySelectorAll('.auth-user-row').forEach(row => {
            const username = row.querySelector('.auth-username').value.trim();
            const password = row.querySelector('.auth-password').value.trim();
            if (username && password) {
                users.push({ username, password });
            }
        });

        if (enabled && users.length === 0) {
            showToast('Add at least one user to enable basic auth', 'error');
            return;
        }

        try {
            await request('/api/sites?action=update-security', {
                method: 'PUT',
                body: JSON.stringify({
                    site_id: parseInt(siteId),
                    basic_auth_enabled: enabled,
                    basic_auth_users: users,
                    delete_protection: document.getElementById('delete-protection-toggle').checked,
                }),
            });
            showToast('Security settings saved!', 'success');
            // Refresh site data
            const updatedSite = await sites.get(siteId);
            Object.assign(site, updatedSite);
        } catch (err) { showToast(err.message, 'error'); }
    });
}

// ---- Vhost Editor ----
async function renderVhost(container, site, siteId) {
    container.innerHTML = '<div class="loading-screen"><div class="loading-spinner"></div></div>';
    try {
        const data = await request(`/api/vhost?site_id=${siteId}`);
        container.innerHTML = `
        <div class="card">
            <div class="card-header">
                <h3 class="card-title">Nginx Vhost Configuration</h3>
            </div>
            <div style="padding: var(--space-4);">
                <p style="color: var(--text-secondary); font-size: var(--font-size-sm); margin-bottom: var(--space-3);">
                    Edit the Nginx virtual host configuration for this site. Changes will be validated before applying.
                </p>
                <div class="form-group">
                    <textarea class="form-textarea mono" id="vhost-editor" style="min-height: 400px; font-size: var(--font-size-xs); line-height: 1.6; tab-size: 4; white-space: pre; overflow-x: auto;">${escapeHtml(data.config)}</textarea>
                </div>
                <div style="display: flex; gap: var(--space-3);">
                    <button class="btn btn-primary" id="save-vhost">Save & Apply</button>
                    <button class="btn btn-secondary" id="reset-vhost">Reset to Default</button>
                </div>
            </div>
        </div>`;

        document.getElementById('vhost-editor')?.addEventListener('keydown', (e) => {
            if (e.key === 'Tab') {
                e.preventDefault();
                const ta = e.target;
                const start = ta.selectionStart;
                ta.value = ta.value.substring(0, start) + '    ' + ta.value.substring(ta.selectionEnd);
                ta.selectionStart = ta.selectionEnd = start + 4;
            }
        });

        document.getElementById('save-vhost')?.addEventListener('click', async () => {
            const config = document.getElementById('vhost-editor').value;
            try {
                await request('/api/vhost', {
                    method: 'PUT',
                    body: JSON.stringify({ site_id: parseInt(siteId), config }),
                });
                showToast('Vhost configuration saved & applied!', 'success');
            } catch (err) { showToast(err.message, 'error'); }
        });

        document.getElementById('reset-vhost')?.addEventListener('click', async () => {
            if (!await showConfirm('Reset Vhost', 'Reset the vhost configuration to its default generated state? This will overwrite any manual changes.', 'Reset', 'btn-danger')) return;
            try {
                const data = await request(`/api/vhost?site_id=${siteId}&action=reset`, { method: 'POST' });
                document.getElementById('vhost-editor').value = data.config;
                showToast('Vhost reset to default!', 'success');
            } catch (err) { showToast(err.message, 'error'); }
        });
    } catch (err) {
        container.innerHTML = `<div class="empty-state"><div class="empty-state-title">Error: ${escapeHtml(err.message)}</div></div>`;
    }
}

// ---- Backups ----
async function renderBackups(container, site, siteId) {
    container.innerHTML = '<div class="loading-screen"><div class="loading-spinner"></div></div>';
    try {
        const data = await request(`/api/backups?site_id=${siteId}`);
        const backups = data.backups || [];
        const schedule = data.schedule || {};

        container.innerHTML = `
        <div class="card" style="margin-bottom: var(--space-4);">
            <div class="card-header">
                <h3 class="card-title">Backup Schedule</h3>
            </div>
            <div style="padding: var(--space-4);">
                <div class="form-row">
                    <div class="form-group">
                        <label class="form-label">Auto Backup</label>
                        <select class="form-select" id="backup-frequency">
                            <option value="disabled" ${schedule.frequency === 'disabled' || !schedule.frequency ? 'selected' : ''}>Disabled</option>
                            <option value="daily" ${schedule.frequency === 'daily' ? 'selected' : ''}>Daily</option>
                            <option value="weekly" ${schedule.frequency === 'weekly' ? 'selected' : ''}>Weekly</option>
                            <option value="monthly" ${schedule.frequency === 'monthly' ? 'selected' : ''}>Monthly</option>
                        </select>
                    </div>
                    <div class="form-group">
                        <label class="form-label">Retention (keep last N)</label>
                        <select class="form-select" id="backup-retention">
                            ${[3,5,7,10,14,30].map(n => `<option value="${n}" ${(schedule.retention || 7) === n ? 'selected' : ''}>${n} backups</option>`).join('')}
                        </select>
                    </div>
                    <div class="form-group">
                        <label class="form-label">Backup Method</label>
                        <select class="form-select" id="backup-method">
                            <option value="local" ${schedule.method === 'local' || !schedule.method ? 'selected' : ''}>Local Storage</option>
                            ${(data.methods || []).map(m => `<option value="${escapeHtml(m.id)}" ${schedule.method === m.id ? 'selected' : ''}>${escapeHtml(m.name)}</option>`).join('')}
                        </select>
                    </div>
                </div>
                <button class="btn btn-primary btn-sm" id="save-backup-schedule">Save Schedule</button>
            </div>
        </div>

        <div class="card">
            <div class="card-header">
                <h3 class="card-title">Backups</h3>
                <button class="btn btn-primary btn-sm" id="create-backup-btn">${icons.plus} Create Backup Now</button>
            </div>
            ${backups.length === 0 ? `
                <div class="empty-state" style="padding: var(--space-6);">
                    <div class="empty-state-title">No backups yet</div>
                    <div class="empty-state-text">Create a manual backup or configure automated backups above.</div>
                </div>
            ` : `
                <div class="table-responsive">
                    <table class="data-table">
                        <thead><tr><th>Date</th><th>Size</th><th>Type</th><th>Method</th><th>Actions</th></tr></thead>
                        <tbody>
                            ${backups.map(b => `
                            <tr>
                                <td>${new Date(b.created_at).toLocaleString()}</td>
                                <td>${b.size ? formatBytes(parseInt(b.size)) : 'N/A'}</td>
                                <td><span class="badge ${b.type === 'auto' ? 'badge-info' : 'badge-primary'}">${b.type}</span></td>
                                <td>${escapeHtml(b.method || 'local')}</td>
                                <td>
                                    <div class="table-actions">
                                        <button class="btn btn-sm btn-secondary download-backup" data-id="${b.id}" title="Download"><span class="nav-icon" style="width:14px;height:14px;">${icons.download}</span></button>
                                        <button class="btn btn-sm btn-secondary restore-backup" data-id="${b.id}">Restore</button>
                                        <button class="btn btn-sm btn-danger delete-backup" data-id="${b.id}">Delete</button>
                                    </div>
                                </td>
                            </tr>`).join('')}
                        </tbody>
                    </table>
                </div>
            `}
        </div>`;

        document.getElementById('save-backup-schedule')?.addEventListener('click', async () => {
            try {
                await request('/api/backups?action=schedule', {
                    method: 'POST',
                    body: JSON.stringify({
                        site_id: parseInt(siteId),
                        frequency: document.getElementById('backup-frequency').value,
                        retention: parseInt(document.getElementById('backup-retention').value),
                        method: document.getElementById('backup-method').value,
                    }),
                });
                showToast('Backup schedule saved!', 'success');
            } catch (err) { showToast(err.message, 'error'); }
        });

        document.getElementById('create-backup-btn')?.addEventListener('click', async () => {
            if (!await showConfirm('Create Backup', `Create a backup of ${escapeHtml(site.domain)} now? This may take a moment for large sites.`, 'Create Backup', 'btn-primary')) return;
            try {
                showToast('Backup started...', 'info');
                await request('/api/backups', {
                    method: 'POST',
                    body: JSON.stringify({ site_id: parseInt(siteId) }),
                });
                showToast('Backup created successfully!', 'success');
                renderBackups(container, site, siteId);
            } catch (err) { showToast(err.message, 'error'); }
        });

        container.querySelectorAll('.download-backup').forEach(btn => {
            btn.addEventListener('click', async () => {
                try {
                    const data = await request('/api/backups?action=download-token', {
                        method: 'POST',
                        body: JSON.stringify({ backup_id: parseInt(btn.dataset.id) }),
                    });
                    // Open download in new tab/trigger using the one-time token
                    const a = document.createElement('a');
                    a.href = `/api/backups?action=download&token=${encodeURIComponent(data.token)}`;
                    a.download = '';
                    document.body.appendChild(a);
                    a.click();
                    a.remove();
                } catch (err) { showToast(err.message, 'error'); }
            });
        });

        container.querySelectorAll('.restore-backup').forEach(btn => {
            btn.addEventListener('click', async () => {
                const backupId = parseInt(btn.dataset.id);
                showModal('Restore Backup', `
                    <p style="margin-bottom: var(--space-3); color: var(--text-secondary);">Select which components to restore. Current data will be replaced.</p>
                    <div class="settings-row" style="margin-bottom: var(--space-3);">
                        <div class="settings-row-label">Web Files<small>Restore website files to web root</small></div>
                        <div><label class="toggle"><input type="checkbox" id="restore-files" checked><span class="toggle-slider"></span></label></div>
                    </div>
                    <div class="settings-row" style="margin-bottom: var(--space-3);">
                        <div class="settings-row-label">Databases<small>Restore database SQL dumps</small></div>
                        <div><label class="toggle"><input type="checkbox" id="restore-dbs" checked><span class="toggle-slider"></span></label></div>
                    </div>
                    <div class="settings-row" style="margin-bottom: var(--space-3);">
                        <div class="settings-row-label">Cron Jobs<small>Restore scheduled tasks</small></div>
                        <div><label class="toggle"><input type="checkbox" id="restore-cron" checked><span class="toggle-slider"></span></label></div>
                    </div>
                    <p style="color: var(--text-tertiary); font-size: var(--font-size-xs); margin-top: var(--space-2);">Components not included in the backup will be skipped automatically.</p>
                `, `
                    <button class="btn btn-secondary" onclick="document.getElementById('modal-overlay').remove()">Cancel</button>
                    <button class="btn btn-danger" id="confirm-restore">Restore</button>
                `);

                document.getElementById('confirm-restore')?.addEventListener('click', async () => {
                    const restoreFiles = document.getElementById('restore-files').checked;
                    const restoreDBs = document.getElementById('restore-dbs').checked;
                    const restoreCron = document.getElementById('restore-cron').checked;
                    if (!restoreFiles && !restoreDBs && !restoreCron) {
                        showToast('Select at least one component to restore', 'error');
                        return;
                    }
                    closeModal();
                    try {
                        showToast('Restoring backup...', 'info');
                        const result = await request('/api/backups?action=restore', {
                            method: 'POST',
                            body: JSON.stringify({
                                backup_id: backupId,
                                restore_files: restoreFiles,
                                restore_databases: restoreDBs,
                                restore_cron: restoreCron,
                            }),
                        });
                        const restored = (result.restored || []).join(', ');
                        showToast(`Backup restored: ${restored}`, 'success');
                    } catch (err) { showToast(err.message, 'error'); }
                });
            });
        });

        container.querySelectorAll('.delete-backup').forEach(btn => {
            btn.addEventListener('click', async () => {
                if (!await showConfirm('Delete Backup', 'Delete this backup permanently?', 'Delete', 'btn-danger')) return;
                try {
                    await request(`/api/backups?id=${btn.dataset.id}&site_id=${siteId}`, { method: 'DELETE' });
                    showToast('Backup deleted', 'success');
                    renderBackups(container, site, siteId);
                } catch (err) { showToast(err.message, 'error'); }
            });
        });
    } catch (err) {
        container.innerHTML = `<div class="empty-state"><div class="empty-state-title">Error: ${escapeHtml(err.message)}</div></div>`;
    }
}

// ---- Logs Viewer ----
async function renderLogs(container, site, siteId) {
    let activeLog = 'access';
    let logLines = 25;

    async function loadLog() {
        container.innerHTML = '<div class="loading-screen"><div class="loading-spinner"></div></div>';
        try {
            const data = await request(`/api/logs?site_id=${siteId}&type=${activeLog}&lines=${logLines}`);
            container.innerHTML = `
            <div class="card">
                <div class="card-header">
                    <h3 class="card-title">Site Logs</h3>
                    <div style="display: flex; gap: var(--space-2); align-items: center;">
                        <select class="form-select" id="log-type" style="width: auto; min-width: 140px; padding: var(--space-1) var(--space-2); font-size: var(--font-size-xs);">
                            <option value="access" ${activeLog === 'access' ? 'selected' : ''}>Access Log</option>
                            <option value="error" ${activeLog === 'error' ? 'selected' : ''}>Error Log</option>
                            ${site.site_type === 'php' ? `<option value="php-fpm" ${activeLog === 'php-fpm' ? 'selected' : ''}>PHP-FPM Error</option>` : ''}
                        </select>
                        <select class="form-select" id="log-lines" style="width: auto; min-width: 80px; padding: var(--space-1) var(--space-2); font-size: var(--font-size-xs);">
                            ${[25,50,100,200,500].map(n => `<option value="${n}" ${logLines === n ? 'selected' : ''}>${n} lines</option>`).join('')}
                        </select>
                        <button class="btn btn-sm btn-ghost" id="refresh-logs">${icons.refresh} Refresh</button>
                    </div>
                </div>
                <div style="padding: var(--space-3);">
                    <pre class="mono" style="background: var(--bg-tertiary); border: 1px solid var(--border-primary); border-radius: var(--radius-md); padding: var(--space-3); overflow-x: auto; max-height: 500px; overflow-y: auto; font-size: var(--font-size-xs); line-height: 1.5; white-space: pre-wrap; word-break: break-all;">${escapeHtml(data.content || 'No log data available.')}</pre>
                </div>
            </div>`;

            document.getElementById('log-type')?.addEventListener('change', (e) => {
                activeLog = e.target.value;
                loadLog();
            });
            document.getElementById('log-lines')?.addEventListener('change', (e) => {
                logLines = parseInt(e.target.value);
                loadLog();
            });
            document.getElementById('refresh-logs')?.addEventListener('click', () => loadLog());
        } catch (err) {
            container.innerHTML = `<div class="empty-state"><div class="empty-state-title">Error: ${escapeHtml(err.message)}</div></div>`;
        }
    }

    await loadLog();
}
