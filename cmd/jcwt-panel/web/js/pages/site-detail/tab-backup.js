import { databases } from '../../api.js';
import { icons, showToast, showModal, closeModal, escapeHtml, formatBytes, showConfirm } from '../../app.js';
import { request } from '../../api.js';
import { showLoading } from '../../ui.js';

export async function renderBackups(container, site, siteId) {
    showLoading(container);
    try {
        const data = await request(`/api/backups?site_id=${siteId}`);
        const backups = data.backups || [];
        const schedule = data.schedule || {};

        container.innerHTML = `
        <div class="card" style="margin-bottom: var(--space-4);">
            <div class="card-header">
                <h3 class="card-title">Backup Schedule</h3>
                <button class="btn btn-primary btn-sm" id="save-backup-schedule">Save Schedule</button>
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
                        <small style="color:var(--text-tertiary);font-size:var(--font-size-xs);">Older backups beyond this count are automatically deleted after each new backup.</small>
                    </div>
                    <div class="form-group">
                        <label class="form-label">Backup Method</label>
                        <select class="form-select" id="backup-method">
                            <option value="local" ${schedule.method === 'local' || !schedule.method ? 'selected' : ''}>Local Storage</option>
                            ${(data.methods || []).map(m => `<option value="${escapeHtml(m.id)}" ${schedule.method === m.id ? 'selected' : ''}>${escapeHtml(m.name)}</option>`).join('')}
                        </select>
                    </div>
                </div>
            </div>
        </div>

        <div class="card">
            <div class="card-header">
                <h3 class="card-title">Backup & Restore</h3>
                <div style="display: flex; gap: var(--space-2);">
                    <button class="btn btn-sm btn-ghost" id="refresh-backups"><span class="nav-icon nav-icon-xs">${icons.refresh}</span> Refresh</button>
                    <button class="btn btn-primary btn-sm" id="create-backup-btn">${icons.plus} Create Backup Now</button>
                </div>
            </div>
            ${backups.length === 0 ? `
                <div class="empty-state p-6">
                    <div class="empty-state-title">No backups yet</div>
                    <div class="empty-state-text">Create a manual backup or configure automated backups above.</div>
                </div>
            ` : `
                <div class="table-responsive">
                    <table class="data-table responsive-cards">
                        <thead><tr><th>Date</th><th>Size</th><th>Type</th><th>Status</th><th>Actions</th></tr></thead>
                        <tbody id="backup-tbody"></tbody>
                    </table>
                </div>
                <div id="backup-pagination" style="display: flex; justify-content: center; align-items: center; gap: var(--space-2); padding: var(--space-3);"></div>
            `}
        </div>`;

        const perPage = 10;
        let currentPage = 1;
        const totalPages = Math.ceil(backups.length / perPage);

        function renderBackupRows() {
            const tbody = document.getElementById('backup-tbody');
            const pag = document.getElementById('backup-pagination');
            if (!tbody) return;
            const start = (currentPage - 1) * perPage;
            const pageItems = backups.slice(start, start + perPage);
            tbody.innerHTML = pageItems.map(b => `
                <tr>
                    <td data-label="Date">${new Date(b.created_at).toLocaleString()}</td>
                    <td data-label="Size">${b.size ? formatBytes(parseInt(b.size)) : (b.status === 'in_progress' ? '...' : 'N/A')}</td>
                    <td data-label="Type"><span class="badge ${b.type === 'auto' ? 'badge-info' : 'badge-primary'}">${b.type}</span></td>
                    <td data-label="Status"><span class="badge ${b.status === 'completed' ? 'badge-success' : b.status === 'in_progress' ? 'badge-warning' : 'badge-danger'}">${b.status === 'in_progress' ? 'In Progress' : b.status}</span></td>
                    <td>
                        <div class="table-actions">
                            ${b.status === 'completed' ? `<button class="btn btn-sm btn-secondary download-backup" data-id="${b.id}" title="Download"><span class="nav-icon nav-icon-xs">${icons.download}</span></button>
                            <button class="btn btn-sm btn-secondary restore-backup" data-id="${b.id}">Restore</button>` : ''}
                            <button class="btn btn-sm btn-danger delete-backup" data-id="${b.id}">Delete</button>
                        </div>
                    </td>
                </tr>`).join('');
            if (pag && totalPages > 1) {
                pag.innerHTML = `
                    <button class="btn btn-sm btn-ghost" id="bk-prev" ${currentPage <= 1 ? 'disabled' : ''}>← Prev</button>
                    <span style="font-size: var(--font-size-xs); color: var(--text-secondary);">Page ${currentPage} of ${totalPages}</span>
                    <button class="btn btn-sm btn-ghost" id="bk-next" ${currentPage >= totalPages ? 'disabled' : ''}>Next →</button>`;
                pag.querySelector('#bk-prev')?.addEventListener('click', () => { currentPage--; renderBackupRows(); bindBackupActions(); });
                pag.querySelector('#bk-next')?.addEventListener('click', () => { currentPage++; renderBackupRows(); bindBackupActions(); });
            }
        }
        renderBackupRows();
        bindBackupActions();

        document.getElementById('save-backup-schedule')?.addEventListener('click', async () => {
            try {
                await request(`/api/backups?site_id=${siteId}`, {
                    method: 'PUT',
                    body: JSON.stringify({
                        frequency: document.getElementById('backup-frequency').value,
                        retention: parseInt(document.getElementById('backup-retention').value),
                        method: document.getElementById('backup-method').value,
                    }),
                });
                showToast('Backup schedule saved!', 'success');
            } catch (err) { showToast(err.message, 'error'); }
        });

        document.getElementById('refresh-backups')?.addEventListener('click', () => renderBackups(container, site, siteId));

        document.getElementById('create-backup-btn')?.addEventListener('click', async () => {
            if (!await showConfirm('Create Backup', `Create a backup of ${escapeHtml(site.domain)} now? This may take a moment for large sites.`, 'Create Backup', 'btn-primary')) return;
            const btn = document.getElementById('create-backup-btn');
            btn.disabled = true;
            btn.innerHTML = '<span class="loading-spinner btn-spinner"></span> Creating...';
            try {
                const result = await request('/api/backups', {
                    method: 'POST',
                    body: JSON.stringify({ site_id: parseInt(siteId) }),
                });
                const backupId = result.id;
                showToast('Backup started in background...', 'info');
                const poll = setInterval(async () => {
                    try {
                        const status = await request('/api/backups?action=status', {
                            method: 'POST',
                            body: JSON.stringify({ backup_id: backupId }),
                        });
                        if (status.status === 'completed') {
                            clearInterval(poll);
                            showToast('Backup completed successfully!', 'success');
                            renderBackups(container, site, siteId);
                        } else if (status.status === 'failed') {
                            clearInterval(poll);
                            showToast('Backup failed', 'error');
                            renderBackups(container, site, siteId);
                        }
                    } catch { clearInterval(poll); }
                }, 3000);
            } catch (err) {
                showToast(err.message, 'error');
                btn.disabled = false;
                btn.innerHTML = `${icons.plus} Create Backup Now`;
            }
        });

        function bindBackupActions() {
        container.querySelectorAll('.download-backup').forEach(btn => {
            btn.addEventListener('click', async () => {
                try {
                    const data = await request('/api/backups?action=download-token', {
                        method: 'POST',
                        body: JSON.stringify({ backup_id: parseInt(btn.dataset.id) }),
                    });
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
                let siteDbs = [];
                try {
                    const allDbs = await databases.list();
                    siteDbs = (allDbs || []).filter(db => String(db.site_id) === String(siteId));
                } catch {}

                const dbToggles = siteDbs.length > 0
                    ? siteDbs.map(db => `
                        <div style="display: flex; align-items: center; justify-content: space-between; padding: var(--space-1) var(--space-2);">
                            <span class="mono" style="font-size: var(--font-size-sm); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; min-width: 0;">${escapeHtml(db.db_name)}</span>
                            <label class="toggle" style="margin:0; flex-shrink:0;"><input type="checkbox" class="restore-db-toggle" data-name="${escapeHtml(db.db_name)}" checked><span class="toggle-slider"></span></label>
                        </div>`).join('')
                    : '';

                showModal('Restore Backup', `
                    <p style="margin-bottom: var(--space-3); color: var(--text-secondary);">Select which components to restore. Current data will be replaced.</p>
                    <div class="settings-row" style="margin-bottom: var(--space-3);">
                        <div class="settings-row-label">Web Files<small>Restore files to web root</small></div>
                        <div><label class="toggle"><input type="checkbox" id="restore-files" checked><span class="toggle-slider"></span></label></div>
                    </div>
                    <div class="settings-row" style="margin-bottom: var(--space-2);">
                        <div class="settings-row-label">Databases<small>Restore database SQL dumps</small></div>
                        <div><label class="toggle"><input type="checkbox" id="restore-dbs" checked><span class="toggle-slider"></span></label></div>
                    </div>
                    ${dbToggles ? `<div id="restore-db-list" style="margin-bottom: var(--space-3); border: 1px solid var(--border-primary); border-radius: var(--radius-md); background: var(--bg-secondary); padding: var(--space-2); max-width: 100%;">${dbToggles}</div>` : '<div id="restore-db-list"></div>'}
                    <div class="settings-row" style="margin-bottom: var(--space-3);">
                        <div class="settings-row-label">Cron Jobs<small>Restore scheduled tasks</small></div>
                        <div><label class="toggle"><input type="checkbox" id="restore-cron" checked><span class="toggle-slider"></span></label></div>
                    </div>
                    <p style="color: var(--text-tertiary); font-size: var(--font-size-xs); margin-top: var(--space-2);">Components not included in the backup will be skipped automatically.</p>
                `, `
                    <button class="btn btn-secondary" onclick="document.getElementById('modal-overlay').remove()">Cancel</button>
                    <button class="btn btn-danger" id="confirm-restore">Restore</button>
                `);

                document.getElementById('restore-dbs')?.addEventListener('change', (e) => {
                    const dbList = document.getElementById('restore-db-list');
                    if (dbList) dbList.style.display = e.target.checked ? '' : 'none';
                });

                document.getElementById('confirm-restore')?.addEventListener('click', async () => {
                    const restoreFiles = document.getElementById('restore-files').checked;
                    const restoreDBs = document.getElementById('restore-dbs').checked;
                    const restoreCron = document.getElementById('restore-cron').checked;
                    if (!restoreFiles && !restoreDBs && !restoreCron) {
                        showToast('Select at least one component to restore', 'error');
                        return;
                    }
                    const restoreDBNames = [];
                    if (restoreDBs) {
                        document.querySelectorAll('.restore-db-toggle').forEach(cb => {
                            if (cb.checked) restoreDBNames.push(cb.dataset.name);
                        });
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
                                restore_db_names: restoreDBNames,
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
        }
    } catch (err) {
        container.innerHTML = `<div class="empty-state"><div class="empty-state-title">Error: ${escapeHtml(err.message)}</div></div>`;
    }
}
