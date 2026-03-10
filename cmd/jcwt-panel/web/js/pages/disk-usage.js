// JCWT Ultra Panel — Site Disk Usage Management Page
import { diskUsage } from '../api.js';
import { icons, showToast, showConfirm, escapeHtml } from '../app.js';

export async function render(container) {
    document.getElementById('page-title').textContent = 'Site Disk Usage';
    container.innerHTML = '<div class="loading-screen"><div class="loading-spinner"></div></div>';

    async function load() {
        try {
            const sites = await diskUsage.allSites();
            const list = Array.isArray(sites) ? sites : [];

            container.innerHTML = `
            <div class="page-header">
                <div class="page-header-left">
                    <h2>Site Disk Usage</h2>
                    <p>Disk usage overview for all sites</p>
                </div>
                <button class="btn btn-secondary" id="refresh-all-du">
                    <span class="nav-icon" style="width:16px;height:16px">${icons.refresh}</span> Refresh
                </button>
            </div>

            ${list.length === 0 ? `
                <div class="empty-state">
                    <div class="empty-state-icon"><span class="nav-icon" style="width:48px;height:48px;color:var(--text-tertiary)">${icons.database}</span></div>
                    <div class="empty-state-title">No Sites</div>
                    <div class="empty-state-text">Create a site to see disk usage information.</div>
                </div>
            ` : `
                <div class="card">
                    <div class="table-wrapper">
                        <table class="data-table">
                            <thead>
                                <tr>
                                    <th>Site</th>
                                    <th>System User</th>
                                    <th style="text-align:right;">Total</th>
                                    <th style="text-align:right;">htdocs</th>
                                    <th style="text-align:right;">logs</th>
                                    <th style="text-align:right;">tmp</th>
                                    <th style="text-align:right;">backups</th>
                                    <th style="width:100px;text-align:right;">Actions</th>
                                </tr>
                            </thead>
                            <tbody>
                                ${list.map(s => `
                                <tr>
                                    <td>
                                        <a href="#/sites/${s.id}" style="color:var(--accent-primary);text-decoration:none;font-weight:600;">${escapeHtml(s.domain)}</a>
                                    </td>
                                    <td><code style="font-size:var(--font-size-xs);">${escapeHtml(s.system_user)}</code></td>
                                    <td style="text-align:right;font-weight:600;" class="mono">${escapeHtml(s.total)}</td>
                                    <td style="text-align:right;" class="mono">${escapeHtml(s.htdocs)}</td>
                                    <td style="text-align:right;" class="mono">${escapeHtml(s.logs)}</td>
                                    <td style="text-align:right;" class="mono">${escapeHtml(s.tmp)}</td>
                                    <td style="text-align:right;" class="mono">${escapeHtml(s.backups)}</td>
                                    <td style="text-align:right;">
                                        <button class="btn btn-sm btn-secondary cleanup-tmp-btn" data-id="${s.id}" data-domain="${escapeHtml(s.domain)}" title="Clean up tmp directory">
                                            <span class="nav-icon" style="width:14px;height:14px">${icons.trash}</span> Clean Tmp
                                        </button>
                                    </td>
                                </tr>
                                `).join('')}
                            </tbody>
                        </table>
                    </div>
                </div>
            `}`;

            // Refresh button
            container.querySelector('#refresh-all-du')?.addEventListener('click', () => load());

            // Cleanup tmp buttons
            container.querySelectorAll('.cleanup-tmp-btn').forEach(btn => {
                btn.addEventListener('click', async () => {
                    const siteId = parseInt(btn.dataset.id);
                    const domain = btn.dataset.domain;

                    const confirmed = await showConfirm(
                        'Clean Up Tmp',
                        `Delete all files in the tmp directory for ${domain}? This cannot be undone.`,
                        'Clean Up',
                        'btn-danger'
                    );
                    if (!confirmed) return;

                    btn.disabled = true;
                    btn.innerHTML = '<span class="loading-spinner" style="width:14px;height:14px"></span>';

                    try {
                        await diskUsage.cleanupTmp(siteId);
                        showToast(`Tmp directory cleaned for ${domain}`, 'success');
                        load();
                    } catch (err) {
                        showToast(err.message || 'Cleanup failed', 'error');
                        btn.disabled = false;
                        btn.innerHTML = `<span class="nav-icon" style="width:14px;height:14px">${icons.trash}</span> Clean Tmp`;
                    }
                });
            });

        } catch (err) {
            container.innerHTML = `<div class="empty-state"><div class="empty-state-title">Error: ${escapeHtml(err.message)}</div></div>`;
        }
    }

    await load();
}
