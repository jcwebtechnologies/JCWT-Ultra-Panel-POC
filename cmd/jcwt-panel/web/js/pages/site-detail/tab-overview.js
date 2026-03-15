import { sites, phpSettings } from '../../api.js';
import { icons, showToast, showConfirm, escapeHtml } from '../../app.js';
import { request } from '../../api.js';
import { showLoading } from '../../ui.js';

export function renderOverview(el, site, versions, siteId) {
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
                    <div style="display:flex;align-items:center;gap:var(--space-2);padding:var(--space-2) 0;">
                        <span class="badge badge-info">${escapeHtml(site.site_type)}</span>
                        <span style="font-size:var(--font-size-xs);color:var(--text-tertiary);">Site type cannot be changed after creation</span>
                    </div>
                </div>
                <div class="form-group" id="edit-php-group" style="${(site.site_type === 'php' || site.site_type === 'wordpress') ? '' : 'display: none;'}">
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
                <div style="display: flex; align-items: center; gap: 0;">
                    <span style="background: var(--bg-tertiary); border: 1px solid var(--border-primary); border-right: none; border-radius: var(--radius-md) 0 0 var(--radius-md); padding: var(--space-2) var(--space-3); font-size: var(--font-size-sm); color: var(--text-tertiary); white-space: nowrap;">/home/${escapeHtml(site.system_user)}/</span>
                    <input type="text" class="form-input mono" id="edit-webroot" value="${escapeHtml(site.web_root.replace('/home/' + site.system_user + '/', ''))}" placeholder="htdocs" style="border-radius: 0 var(--radius-md) var(--radius-md) 0;">
                </div>
                <small style="color: var(--text-tertiary); font-size: var(--font-size-xs);">Changing this only updates the config — the directory must already exist.</small>
            </div>
            <button type="submit" class="btn btn-primary" style="width: auto;">Save Changes</button>
        </form>
    </div>
    <div class="card" style="margin-top: var(--space-4);">
        <h3 class="card-title" style="margin-bottom: var(--space-4);">Logging</h3>
        <div class="settings-row" style="margin-bottom: var(--space-3);">
            <div class="settings-row-label">Access Log<small>Log all incoming requests</small></div>
            <div><label class="toggle"><input type="checkbox" id="toggle-access-log" ${site.access_log == 1 ? 'checked' : ''}><span class="toggle-slider"></span></label></div>
        </div>
        <div class="settings-row">
            <div class="settings-row-label">Error Log<small>Log server errors and warnings</small></div>
            <div><label class="toggle"><input type="checkbox" id="toggle-error-log" ${site.error_log == 1 ? 'checked' : ''}><span class="toggle-slider"></span></label></div>
        </div>
        <div style="margin-top: var(--space-3);">
            <button class="btn btn-primary" id="save-log-settings" style="width: auto;">Save Log Settings</button>
        </div>
    </div>`;

    (function() {
        const type = site.site_type;
        document.getElementById('edit-php-group').style.display = (type === 'php' || type === 'wordpress') ? 'block' : 'none';
        document.getElementById('edit-proxy-group').style.display = type === 'proxy' ? 'block' : 'none';
        document.getElementById('edit-webroot-group').style.display = type !== 'proxy' ? 'block' : 'none';
    })();

    document.getElementById('update-site-form').addEventListener('submit', async (e) => {
        e.preventDefault();

        const siteType = site.site_type;
        const proxyUrl = (document.getElementById('edit-proxy')?.value || '').trim();
        const domain = document.getElementById('edit-domain').value.trim();
        const webRootSuffix = (document.getElementById('edit-webroot')?.value || '').trim();
        const webRoot = webRootSuffix ? `/home/${site.system_user}/${webRootSuffix}` : '';

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

        try {
            await sites.update({
                id: parseInt(siteId),
                domain: domain,
                aliases: (document.getElementById('edit-aliases')?.value || '').trim(),
                php_version: document.getElementById('edit-php')?.value || '',
                proxy_url: proxyUrl,
                web_root: webRoot,
                ssl_type: site.ssl_type,
            });
            showToast('Site updated!', 'success');

            const mod = await import('../site-detail.js');
            mod.render(document.getElementById('page-content'), site.token, 'overview');

        } catch (err) { showToast(err.message, 'error'); }
    });

    document.getElementById('save-log-settings')?.addEventListener('click', async () => {
        const accessLog = document.getElementById('toggle-access-log').checked;
        const errorLog = document.getElementById('toggle-error-log').checked;
        try {
            await request(`/api/sites?action=update-logs`, { method: 'PUT', body: JSON.stringify({ site_id: parseInt(siteId), access_log: accessLog, error_log: errorLog }) });
            showToast('Log settings updated!', 'success');
        } catch (err) { showToast(err.message, 'error'); }
    });
}

export async function renderPHP(el, siteId) {
    const phpOpts = {
        memory_limit: ['32M', '64M', '128M', '256M', '512M', '768M', '1024M', '2048M'],
        max_execution_time: ['30', '60', '120', '300', '600', '900'],
        max_input_time: ['30', '60', '120', '300', '600', '900'],
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
                        <label class="form-label">Memory Limit</label>
                        <select class="form-select" id="php-memory">${opts('memory_limit', settings.memory_limit || '128M')}</select>
                    </div>
                    <div class="form-group">
                        <label class="form-label">Max Execution Time <span style="font-weight:400;color:var(--text-tertiary)">(seconds)</span></label>
                        <select class="form-select" id="php-exec-time">${opts('max_execution_time', settings.max_execution_time || 30)}</select>
                    </div>
                </div>
                <div class="form-row">
                    <div class="form-group">
                        <label class="form-label">Max Input Time <span style="font-weight:400;color:var(--text-tertiary)">(seconds)</span></label>
                        <select class="form-select" id="php-input-time">${opts('max_input_time', settings.max_input_time || 30)}</select>
                    </div>
                    <div class="form-group">
                        <label class="form-label">Max Input Vars</label>
                        <select class="form-select" id="php-input-vars">${opts('max_input_vars', settings.max_input_vars || 1000)}</select>
                    </div>
                </div>
                <div class="form-row">
                    <div class="form-group">
                        <label class="form-label">Post Max Size</label>
                        <select class="form-select" id="php-post-max">${opts('post_max_size', settings.post_max_size || '16M')}</select>
                    </div>
                    <div class="form-group">
                        <label class="form-label">Upload Max Filesize</label>
                        <select class="form-select" id="php-upload-max">${opts('upload_max_filesize', settings.upload_max_filesize || '16M')}</select>
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

export async function renderVhost(container, site, siteId) {
    showLoading(container);
    try {
        const data = await request(`/api/vhost?site_id=${siteId}`);
        container.innerHTML = `
        <div class="card">
            <div class="card-header">
                <h3 class="card-title">Nginx Vhost Configuration</h3>
            </div>
            <div style="padding: var(--space-4);">
                <div class="warning-banner">
                    <span class="warning-banner-icon">${icons.alertTriangle}</span>
                    <span>This configuration uses <strong>{token}</strong> placeholders for dynamic values. When site settings are saved, only those tokens are updated — your custom nginx directives are preserved. Use <em>Reset to Default</em> to regenerate the full template from scratch.</span>
                </div>
                <div class="form-group">
                    <textarea class="form-textarea mono" id="vhost-editor" style="min-height: 400px; font-size: var(--font-size-xs); line-height: 1.6; tab-size: 4; white-space: pre; overflow-x: auto;">${escapeHtml(data.config)}</textarea>
                </div>
                <div style="display: flex; gap: var(--space-3);">
                    <button class="btn btn-primary" id="save-vhost">Save & Apply</button>
                    <button class="btn btn-secondary" id="reset-vhost">Reset to Default</button>
                </div>
                <details style="margin-top: var(--space-3);">
                    <summary style="cursor: pointer; font-size: var(--font-size-sm); color: var(--text-tertiary);">Available Template Variables</summary>
                    <div style="margin-top: var(--space-2); font-size: var(--font-size-xs); color: var(--text-secondary); font-family: var(--font-mono);">
                        <div style="display: grid; grid-template-columns: auto 1fr; gap: var(--space-1) var(--space-3);">
                            <span>{domain}</span><span style="color:var(--text-tertiary);">Primary domain name</span>
                            <span>{domain_aliases}</span><span style="color:var(--text-tertiary);">Space-separated domain aliases</span>
                            <span>{user}</span><span style="color:var(--text-tertiary);">System user</span>
                            <span>{site_root}</span><span style="color:var(--text-tertiary);">Web root path</span>
                            <span>{php_version}</span><span style="color:var(--text-tertiary);">PHP version (e.g. 8.2)</span>
                            <span>{proxy_url}</span><span style="color:var(--text-tertiary);">Proxy backend URL</span>
                            <span>{ssl_cert}</span><span style="color:var(--text-tertiary);">SSL certificate file path</span>
                            <span>{ssl_key}</span><span style="color:var(--text-tertiary);">SSL key file path</span>
                            <span>{logs_dir}</span><span style="color:var(--text-tertiary);">Log directory path</span>
                            <span>{wordpress_security}</span><span style="color:var(--text-tertiary);">WordPress security location blocks (managed by WP Tools)</span>
                        </div>
                        <div style="margin-top:var(--space-2);color:var(--text-tertiary);">Changes to domain, aliases, PHP version, web root, SSL paths and WordPress security rules are applied automatically when site settings are saved — your custom edits outside these tokens are preserved.</div>
                    </div>
                </details>
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
