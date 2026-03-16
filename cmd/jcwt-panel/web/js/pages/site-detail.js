// JCWT Ultra Panel - Site Detail Page (thin coordinator)
import { sites, phpVersions, databases } from '../api.js';
import { icons, showToast, showModal, closeModal, escapeHtml, showConfirm } from '../app.js';
import { ROUTES, siteHref } from '../routes.js';
import { showLoading } from '../ui.js';
import { sslBadge } from '../css-classes.js';

// Tab modules
import { renderOverview, renderPHP, renderVhost } from './site-detail/tab-overview.js';
import { renderSSL } from './site-detail/tab-ssl.js';
import { renderCron } from './site-detail/tab-cron.js';
import { renderFiles } from './site-detail/tab-files.js';
import { renderDatabases, renderDBUsers, renderPhpMyAdmin } from './site-detail/tab-database.js';
import { renderSecurity, renderSSHAccess } from './site-detail/tab-security.js';
import { renderBackups } from './site-detail/tab-backup.js';
import { renderLogs, renderDiskUsage, renderResourceUsage } from './site-detail/tab-monitoring.js';
import { renderWordPressTools, renderWordPressUpdates } from './site-detail/tab-wordpress.js';

export async function render(container, siteToken, section) {
    showLoading(container);

    if (!siteToken) { container.innerHTML = '<p>No site selected</p>'; return; }

    try {
        const [site, versions] = await Promise.all([
            sites.getByToken(siteToken),
            phpVersions.list()
        ]);

        const siteId = site.id;
        let activeSection = section || null;
        let _prevSection = null;

        function renderPage() {
            // Don't stop file browser on section change -- let idle reaper reclaim it.
            // The instance stays alive so it can be reused when the user returns to files.
            _prevSection = activeSection;

            container.innerHTML = `
            <div class="page-header" style="margin-bottom: var(--space-5);">
                <div class="page-header-left">
                    <h2 style="display: flex; align-items: center; gap: var(--space-2);">
                        <a href="http://${escapeHtml(site.domain)}" target="_blank" rel="noopener" style="color: inherit; text-decoration: none;" title="Open site in new tab">${escapeHtml(site.domain)} <span style="font-size: var(--font-size-xs); opacity: 0.4;">↗</span></a>
                    </h2>
                    <div class="site-detail-meta">
                        <span style="display: flex; align-items: center; gap: var(--space-1);"><span class="nav-icon" style="width:14px;height:14px;opacity:0.5;">${icons.key}</span> ${escapeHtml(site.system_user)}</span>
                        <span style="display: flex; align-items: center; gap: var(--space-1); min-width: 0;"><span class="nav-icon" style="width:14px;height:14px;opacity:0.5;flex-shrink:0;">${icons.folder}</span> <span style="overflow:hidden;text-overflow:ellipsis;white-space:nowrap;">${escapeHtml(site.web_root)}</span></span>
                    </div>
                </div>
                <div style="display:flex;align-items:center;gap:var(--space-2);">
                    ${site.site_type === 'wordpress' ? `<a href="https://${escapeHtml(site.domain)}/wp-admin" target="_blank" rel="noopener" class="btn btn-sm btn-primary" style="display:inline-flex;align-items:center;gap:var(--space-1);"><span class="nav-icon nav-icon-xs">${icons.key}</span> WP Admin</a>` : ''}
                    <a href="#${ROUTES.SITES}" class="btn btn-secondary" style="display: inline-flex; align-items: center; gap: var(--space-2);"><span class="nav-icon nav-icon-sm">${icons.sites}</span> All Sites</a>
                </div>
            </div>

            ${activeSection ? `<div class="back-nav"><a href="${siteHref(siteToken)}" class="btn btn-sm btn-primary back-nav-btn" title="Back to Site Overview"><span class="nav-icon nav-icon-sm">${icons.chevronLeft}</span> Site Overview</a></div>` : `
            <div class="site-info-strip">
                <div class="site-info-strip-item">
                    <span class="site-info-strip-label">Type</span>
                    <span class="site-info-strip-value"><span class="badge badge-info">${escapeHtml(site.site_type)}</span></span>
                </div>
                <div class="site-info-strip-item">
                    <span class="site-info-strip-label">SSL</span>
                    <span class="site-info-strip-value"><span class="${sslBadge(site.ssl_type)}">${escapeHtml(site.ssl_type)}</span></span>
                </div>
                ${(site.site_type === 'php' || site.site_type === 'wordpress') ? `
                <div class="site-info-strip-item">
                    <span class="site-info-strip-label">PHP</span>
                    <span class="site-info-strip-value mono">${escapeHtml(site.php_version || 'N/A')}</span>
                </div>` : ''}
                <div class="site-info-strip-item">
                    <span class="site-info-strip-label">Disk</span>
                    <span class="site-info-strip-value" id="strip-disk-usage"><span style="color:var(--text-tertiary);font-size:var(--font-size-xs);">...</span></span>
                </div>
                <div class="site-info-strip-item">
                    <span class="site-info-strip-label">Created</span>
                    <span class="site-info-strip-value">${new Date(site.created_at).toLocaleDateString()}</span>
                </div>
                ${site.site_type === 'wordpress' ? `
                <div class="site-info-strip-item">
                    <a href="https://${escapeHtml(site.domain)}/wp-admin" target="_blank" rel="noopener" class="btn btn-sm btn-secondary" style="font-size:var(--font-size-xs);padding:2px 8px;white-space:nowrap;align-self:center;">WP Admin ↗</a>
                </div>` : ''}
            </div>

            <div class="site-search-bar">
                <span class="search-icon"><span class="nav-icon" style="width:16px;height:16px;">${icons.search}</span></span>
                <input type="text" id="site-feature-search" class="form-input" placeholder="Search features — ssl, php, files, backup, wordpress...">
            </div>

            <div class="site-cards-section">
                <div class="site-cards-section-title">Configuration</div>
                <div class="site-cards-grid">
                    <div class="site-card" data-section="overview">
                        <div class="site-card-icon blue"><span class="nav-icon nav-icon-lg">${icons.settings}</span></div>
                        <div class="site-card-title">Site Settings</div>
                    </div>
                    ${(site.site_type === 'php' || site.site_type === 'wordpress') ? `
                    <div class="site-card" data-section="php">
                        <div class="site-card-icon purple"><span class="nav-icon nav-icon-lg">${icons.code}</span></div>
                        <div class="site-card-title">PHP Settings</div>
                    </div>` : ''}
                    <div class="site-card" data-section="vhost">
                        <div class="site-card-icon"><span class="nav-icon nav-icon-lg">${icons.file}</span></div>
                        <div class="site-card-title">Vhost Editor</div>
                    </div>
                </div>
            </div>

            <div class="site-cards-section">
                <div class="site-cards-section-title">Content</div>
                <div class="site-cards-grid">
                    <div class="site-card" data-section="files">
                        <div class="site-card-icon green"><span class="nav-icon nav-icon-lg">${icons.folder}</span></div>
                        <div class="site-card-title">File Manager</div>
                    </div>
                    <div class="site-card" data-section="cron">
                        <div class="site-card-icon orange"><span class="nav-icon nav-icon-lg">${icons.clock}</span></div>
                        <div class="site-card-title">Cron Jobs</div>
                    </div>
                    <div class="site-card" data-section="backups">
                        <div class="site-card-icon purple"><span class="nav-icon nav-icon-lg">${icons.download}</span></div>
                        <div class="site-card-title">Backup & Restore</div>
                    </div>
                </div>
            </div>

            <div class="site-cards-section">
                <div class="site-cards-section-title">Database</div>
                <div class="site-cards-grid">
                    <div class="site-card" data-section="databases">
                        <div class="site-card-icon blue"><span class="nav-icon nav-icon-lg">${icons.database}</span></div>
                        <div class="site-card-title">Databases</div>
                    </div>
                    <div class="site-card" data-section="dbusers">
                        <div class="site-card-icon purple"><span class="nav-icon nav-icon-lg">${icons.users}</span></div>
                        <div class="site-card-title">Database Users</div>
                    </div>
                    <div class="site-card" data-section="phpmyadmin">
                        <div class="site-card-icon orange"><span class="nav-icon nav-icon-lg">${icons.pma}</span></div>
                        <div class="site-card-title">phpMyAdmin</div>
                    </div>
                </div>
            </div>

            ${site.site_type === 'wordpress' ? `
            <div class="site-cards-section">
                <div class="site-cards-section-title">WordPress</div>
                <div class="site-cards-grid">
                    <div class="site-card" data-section="wptools">
                        <div class="site-card-icon blue"><span class="nav-icon nav-icon-lg">${icons.settings}</span></div>
                        <div class="site-card-title">WP Tools</div>
                    </div>
                    <div class="site-card" data-section="wpupdates">
                        <div class="site-card-icon green"><span class="nav-icon nav-icon-lg">${icons.download}</span></div>
                        <div class="site-card-title">WP Updates</div>
                    </div>
                </div>
            </div>` : ''}

            <div class="site-cards-section">
                <div class="site-cards-section-title">Security & SSL</div>
                <div class="site-cards-grid">
                    <div class="site-card" data-section="ssl">
                        <div class="site-card-icon green"><span class="nav-icon nav-icon-lg">${icons.lock}</span></div>
                        <div class="site-card-title">SSL Certificates</div>
                    </div>
                    <div class="site-card" data-section="security">
                        <div class="site-card-icon red"><span class="nav-icon nav-icon-lg">${icons.shield}</span></div>
                        <div class="site-card-title">Security</div>
                    </div>
                    <div class="site-card" data-section="ssh">
                        <div class="site-card-icon purple"><span class="nav-icon nav-icon-lg">${icons.key}</span></div>
                        <div class="site-card-title">SSH Access</div>
                    </div>
                </div>
            </div>

            <div class="site-cards-section">
                <div class="site-cards-section-title">Monitoring</div>
                <div class="site-cards-grid">
                    <div class="site-card" data-section="disk-usage">
                        <div class="site-card-icon blue"><span class="nav-icon nav-icon-lg">${icons.database}</span></div>
                        <div class="site-card-title">Disk Usage</div>
                    </div>
                    <div class="site-card" data-section="resource-usage">
                        <div class="site-card-icon purple"><span class="nav-icon nav-icon-lg">${icons.dashboard || icons.settings}</span></div>
                        <div class="site-card-title">Resource Usage</div>
                    </div>
                    <div class="site-card" data-section="logs">
                        <div class="site-card-icon orange"><span class="nav-icon nav-icon-lg">${icons.search}</span></div>
                        <div class="site-card-title">Logs</div>
                    </div>
                </div>
            </div>

            <div class="site-cards-section" style="margin-top: var(--space-5);">
                <div class="site-cards-section-title" style="color: var(--status-error);">Danger Zone</div>
                <div style="border: 1px solid var(--status-error); border-radius: var(--radius-lg); padding: var(--space-4); background: rgba(239,68,68,0.04);">
                    <div style="display: flex; justify-content: space-between; align-items: center; flex-wrap: wrap; gap: var(--space-3);">
                        <div>
                            <div style="font-weight: 600; margin-bottom: var(--space-1);">Delete Site</div>
                            <div style="font-size: var(--font-size-sm); color: var(--text-secondary);">Permanently delete this site, all files, databases, backups and system user. This cannot be undone.</div>
                        </div>
                        <button class="btn btn-danger" id="danger-delete-site"><span class="nav-icon nav-icon-sm">${icons.trash}</span> Delete this site</button>
                    </div>
                </div>
            </div>
            `}

            <div id="section-content"></div>`;

            // Danger Zone: Delete Site button
            document.getElementById('danger-delete-site')?.addEventListener('click', async () => {
                const deleteProtected = site.delete_protection === 1 || site.delete_protection === true;
                if (deleteProtected) {
                    showToast('Deletion protection is enabled. Disable it in the Security tab first.', 'error');
                    return;
                }
                let siteDbs = [];
                try {
                    const allDbs = await databases.list();
                    siteDbs = (allDbs || []).filter(db => String(db.site_id) === String(siteId));
                } catch {}
                const dbList = siteDbs.length > 0
                    ? `<div style="margin-top: var(--space-3); padding: var(--space-2) var(--space-3); background: var(--bg-secondary); border: 1px solid var(--border-primary); border-radius: var(--radius-md);"><div style="font-weight: 600; margin-bottom: var(--space-1); font-size: var(--font-size-sm);">Databases that will also be deleted:</div>${siteDbs.map(db => `<div style="font-size: var(--font-size-xs); color: var(--text-secondary);">&bull; <span class="mono">${escapeHtml(db.db_name)}</span></div>`).join('')}</div>`
                    : '';
                showModal('Delete Site', `
                    <div class="danger-banner">
                        <span class="danger-banner-icon">${icons.alertTriangle}</span>
                        <span><strong>This action is irreversible.</strong> The following will be permanently deleted: nginx config, PHP-FPM pool, system user &amp; home directory, all site files, all databases and database users, SSL certificates, cron jobs, backups, and logrotate config.</span>
                    </div>
                    ${dbList}
                    <div style="margin-top: var(--space-3);">
                        <label class="form-label">Type <strong>${escapeHtml(site.domain)}</strong> to confirm:</label>
                        <input type="text" class="form-input" id="confirm-domain-input" placeholder="${escapeHtml(site.domain)}" autocomplete="off">
                    </div>
                `, `
                    <button class="btn btn-secondary" id="cancel-delete-site">Cancel</button>
                    <button class="btn btn-danger" id="confirm-delete-site">Delete Site</button>
                `, { persistent: true });
                // Enable delete button only when domain matches
                const confirmInput = document.getElementById('confirm-domain-input');
                const confirmBtn = document.getElementById('confirm-delete-site');
                confirmInput?.addEventListener('input', () => {
                    confirmBtn.disabled = confirmInput.value.trim() !== site.domain;
                    if (confirmInput.value.trim() === site.domain) {
                        confirmInput.style.borderColor = '';
                    }
                });
                document.getElementById('cancel-delete-site')?.addEventListener('click', () => closeModal());
                confirmBtn?.addEventListener('click', async () => {
                    if (confirmInput.value.trim() !== site.domain) {
                        confirmInput.style.borderColor = 'var(--status-error)';
                        confirmInput.focus();
                        showToast(confirmInput.value.trim() === '' ? 'Please type the domain name to confirm deletion' : 'Domain name does not match', 'error');
                        return;
                    }
                    confirmBtn.disabled = true;
                    confirmBtn.innerHTML = '<span class="loading-spinner btn-spinner"></span> Deleting...';
                    document.getElementById('cancel-delete-site').disabled = true;
                    confirmInput.disabled = true;
                    try {
                        await sites.delete(siteId);
                        closeModal();
                        showToast('Site deleted successfully', 'success');
                        window.location.hash = `#${ROUTES.SITES}`;
                    } catch (err) {
                        confirmBtn.disabled = false;
                        confirmBtn.textContent = 'Delete Site';
                        document.getElementById('cancel-delete-site').disabled = false;
                        confirmInput.disabled = false;
                        showToast(err.message, 'error');
                    }
                });
            });

            // Bind card clicks -- navigate to URL-based sections
            container.querySelectorAll('.site-card').forEach(card => {
                card.addEventListener('click', () => {
                    const sec = card.dataset.section;
                    window.location.hash = siteHref(siteToken, sec);
                });
            });

            // Feature search — filter sections and cards in real-time
            const searchInput = container.querySelector('#site-feature-search');
            searchInput?.addEventListener('input', (e) => {
                const q = e.target.value.toLowerCase().trim();
                container.querySelectorAll('.site-cards-section').forEach(section => {
                    const sectionTitle = (section.querySelector('.site-cards-section-title')?.textContent || '').toLowerCase();
                    const titleMatch = !q || sectionTitle.includes(q);
                    let hasVisible = false;
                    section.querySelectorAll('.site-card').forEach(card => {
                        const cardTitle = (card.querySelector('.site-card-title')?.textContent || '').toLowerCase();
                        const show = !q || cardTitle.includes(q) || titleMatch;
                        card.style.display = show ? '' : 'none';
                        if (show) hasVisible = true;
                    });
                    // Keep section visible if it matched by title or has matching cards;
                    // always show when query is empty
                    section.style.display = (!q || hasVisible) ? '' : 'none';
                });
            });

            // Deferred disk usage load for info strip
            sites.diskUsage(siteId).then(data => {
                const el = document.getElementById('strip-disk-usage');
                if (el) el.innerHTML = `<span class="badge badge-info">${escapeHtml(data.size)}</span>`;
            }).catch(() => {
                const el = document.getElementById('strip-disk-usage');
                if (el) el.textContent = 'N/A';
            });

            // Render active section content
            const sectionContent = document.getElementById('section-content');
            if (activeSection && sectionContent) {
                switch (activeSection) {
                    case 'overview': renderOverview(sectionContent, site, versions, siteId); break;
                    case 'php': renderPHP(sectionContent, siteId, versions); break;
                    case 'databases': renderDatabases(sectionContent, siteId, site, renderPage); break;
                    case 'dbusers': renderDBUsers(sectionContent, siteId, site, renderPage); break;
                    case 'ssl': renderSSL(sectionContent, site, siteId); break;
                    case 'cron': renderCron(sectionContent, siteId); break;
                    case 'security': renderSecurity(sectionContent, site, siteId, renderPage); break;
                    case 'files': renderFiles(sectionContent, siteId, siteToken); break;
                    case 'vhost': renderVhost(sectionContent, site, siteId); break;
                    case 'backups': renderBackups(sectionContent, site, siteId); break;
                    case 'phpmyadmin': renderPhpMyAdmin(sectionContent, siteId); break;
                    case 'logs': renderLogs(sectionContent, site, siteId); break;
                    case 'disk-usage': renderDiskUsage(sectionContent, site, siteId); break;
                    case 'ssh': renderSSHAccess(sectionContent, site, siteId); break;
                    case 'resource-usage': renderResourceUsage(sectionContent, site, siteId); break;
                    case 'wptools': renderWordPressTools(sectionContent, site, siteId); break;
                    case 'wpupdates': renderWordPressUpdates(sectionContent, site, siteId); break;
                }
            }
        }
        renderPage();

    } catch (err) {
        container.innerHTML = `<div class="empty-state"><div class="empty-state-title">Error: ${err.message}</div></div>`;
    }
}
