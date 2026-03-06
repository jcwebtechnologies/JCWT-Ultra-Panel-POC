// JCWT Ultra Panel — Site Detail Page (SSL, PHP Settings, Cron, Files)
import { sites, phpVersions, ssl, phpSettings, cron, files, databases } from '../api.js';
import { icons, showToast, showModal, closeModal, escapeHtml, formatBytes, showConfirm } from '../app.js';
import { request } from '../api.js';

export async function render(container, siteId) {
    document.getElementById('page-title').textContent = 'Site Management';
    container.innerHTML = '<div class="loading-screen"><div class="loading-spinner"></div></div>';

    if (!siteId) { container.innerHTML = '<p>No site selected</p>'; return; }

    try {
        const [site, versions] = await Promise.all([
            sites.get(siteId),
            phpVersions.list()
        ]);

        let activeTab = 'overview';

        function renderTabs() {
            container.innerHTML = `
            <div class="page-header">
                <div class="page-header-left">
                    <h2><a href="http://${escapeHtml(site.domain)}" target="_blank" rel="noopener" style="color: inherit; text-decoration: none;" title="Open site in new tab">${escapeHtml(site.domain)} <span style="font-size: var(--font-size-xs); opacity: 0.5;">↗</span></a></h2>
                    <p>User: <span class="mono">${escapeHtml(site.system_user)}</span> · Web root: <span class="mono">${escapeHtml(site.web_root)}</span></p>
                </div>
                <a href="#/sites" class="btn btn-secondary">← Back to Sites</a>
            </div>

            <div class="tabs">
                <button class="tab ${activeTab === 'overview' ? 'active' : ''}" data-tab="overview">Overview</button>
                ${site.site_type === 'php' ? `<button class="tab ${activeTab === 'php' ? 'active' : ''}" data-tab="php">PHP Settings</button>` : ''}
                <button class="tab ${activeTab === 'databases' ? 'active' : ''}" data-tab="databases">Databases</button>
                <button class="tab ${activeTab === 'ssl' ? 'active' : ''}" data-tab="ssl">SSL</button>
                <button class="tab ${activeTab === 'cron' ? 'active' : ''}" data-tab="cron">Cron Jobs</button>
                <button class="tab ${activeTab === 'security' ? 'active' : ''}" data-tab="security">Security</button>
                <button class="tab ${activeTab === 'files' ? 'active' : ''}" data-tab="files">File Manager</button>
            </div>

            <div id="tab-content"></div>`;

            container.querySelectorAll('.tab').forEach(tab => {
                tab.addEventListener('click', () => {
                    activeTab = tab.dataset.tab;
                    renderTabs();
                });
            });

            const tabContent = document.getElementById('tab-content');
            switch (activeTab) {
                case 'overview': renderOverview(tabContent, site, versions, siteId); break;
                case 'php': renderPHP(tabContent, siteId); break;
                case 'databases': renderDatabases(tabContent, siteId, site, renderTabs); break;
                case 'ssl': renderSSL(tabContent, site, siteId); break;
                case 'cron': renderCron(tabContent, siteId); break;
                case 'security': renderSecurity(tabContent, site, siteId, renderTabs); break;
                case 'files': renderFiles(tabContent, siteId); break;
            }
        }
        renderTabs();

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
            mod.render(document.getElementById('page-content'), siteId);

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
        el.innerHTML = `
        <div class="card">
            <h3 class="card-title" style="margin-bottom: var(--space-4);">PHP Pool Settings</h3>
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
                showToast('PHP settings saved & FPM restarted!', 'success');
            } catch (err) { showToast(err.message, 'error'); }
        });
    } catch (err) { el.innerHTML = `<p>Error: ${err.message}</p>`; }
}

function renderSSL(el, site, siteId) {
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
            ${site.ssl_type !== 'none' ? `<button class="btn btn-danger" id="ssl-disable">Disable SSL</button>` : ''}
        </div>
    </div>`;

    document.getElementById('ssl-self-signed')?.addEventListener('click', async () => {
        try {
            await ssl.selfSigned(siteId);
            showToast('Self-signed certificate generated!', 'success');
            // Reload page
            const mod = await import('./site-detail.js');
            mod.render(document.getElementById('page-content'), siteId);
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
                mod.render(document.getElementById('page-content'), siteId);
            } catch (err) { showToast(err.message, 'error'); }
        });
    });

    document.getElementById('ssl-disable')?.addEventListener('click', async () => {
        if (!await showConfirm('Disable SSL', 'Disable SSL for this site? The site will no longer be served over HTTPS.', 'Disable SSL', 'btn-danger')) return;
        try {
            await ssl.disable(siteId);
            showToast('SSL disabled', 'success');
            const mod = await import('./site-detail.js');
            mod.render(document.getElementById('page-content'), siteId);
        } catch (err) { showToast(err.message, 'error'); }
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
