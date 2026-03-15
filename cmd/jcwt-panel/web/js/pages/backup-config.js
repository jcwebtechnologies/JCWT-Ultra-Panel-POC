// JCWT Ultra Panel — Backup Configuration Settings
import { backupMethods } from '../api.js';
import { icons, showToast, escapeHtml, showConfirm, showModal, closeModal } from '../app.js';
import { showLoading } from '../ui.js';

export async function render(container) {
    showLoading(container);

    try {
        container.innerHTML = `
        <div class="page-header">
            <div class="page-header-left">
                <h2>Backup Configuration</h2>
                <p>Configure backup storage methods for site backups</p>
            </div>
        </div>

        <div class="card" style="margin-bottom: var(--space-6);">
            <h3 class="settings-section-title"><span class="nav-icon section-icon">${icons.database}</span> Backup Methods</h3>
            <p style="color: var(--text-tertiary); font-size: var(--font-size-sm); margin-bottom: var(--space-4);">Configure backup storage methods available for site backups.</p>
            <div id="backup-methods-list" style="margin-bottom: var(--space-4);"></div>
            <button type="button" class="btn btn-sm btn-secondary" id="add-backup-method-btn">+ Add Backup Method</button>
        </div>`;

        async function loadBackupMethods() {
            const listEl = document.getElementById('backup-methods-list');
            if (!listEl) return;
            try {
                const data = await backupMethods.list();
                const methods = Array.isArray(data) ? data : (data?.methods || []);
                if (methods.length === 0) {
                    listEl.innerHTML = '<div style="color:var(--text-tertiary);font-size:var(--font-size-sm);">No backup methods configured. A default "Local" method will be used for site backups.</div>';
                    return;
                }
                listEl.innerHTML = methods.map(m => `
                    <div class="settings-row" style="padding:var(--space-3);border:1px solid var(--border-primary);border-radius:var(--radius-md);margin-bottom:var(--space-2);">
                        <div class="settings-row-label" style="min-width:auto;">
                            <strong>${escapeHtml(m.name)}</strong>
                            <small>Type: ${escapeHtml(m.type)} ${m.enabled ? '(Active)' : '(Disabled)'}</small>
                        </div>
                        <div style="display:flex;gap:var(--space-2);">
                            <button type="button" class="btn btn-sm ${m.enabled ? 'btn-secondary' : 'btn-primary'}" data-toggle-method="${m.id}">${m.enabled ? 'Disable' : 'Enable'}</button>
                            <button type="button" class="btn btn-sm btn-danger" data-delete-method="${m.id}">Delete</button>
                        </div>
                    </div>
                `).join('');

                listEl.querySelectorAll('[data-toggle-method]').forEach(btn => {
                    btn.addEventListener('click', async () => {
                        const mid = parseInt(btn.dataset.toggleMethod);
                        const method = methods.find(m => m.id === mid);
                        if (!method) return;
                        try {
                            await backupMethods.update({ id: mid, name: method.name, type: method.type, config: method.config || '{}', enabled: !method.enabled });
                            showToast('Backup method updated', 'success');
                            loadBackupMethods();
                        } catch (err) { showToast(err.message, 'error'); }
                    });
                });

                listEl.querySelectorAll('[data-delete-method]').forEach(btn => {
                    btn.addEventListener('click', async () => {
                        const confirmed = await showConfirm('Delete Backup Method', 'Are you sure you want to delete this backup method? This cannot be undone.');
                        if (!confirmed) return;
                        try {
                            await backupMethods.delete(parseInt(btn.dataset.deleteMethod));
                            showToast('Backup method deleted', 'success');
                            loadBackupMethods();
                        } catch (err) { showToast(err.message, 'error'); }
                    });
                });
            } catch (err) {
                listEl.innerHTML = '<div style="color:var(--status-error);font-size:var(--font-size-sm);">Failed to load backup methods</div>';
            }
        }
        loadBackupMethods();

        document.getElementById('add-backup-method-btn')?.addEventListener('click', () => {
            const content = `
                <div class="form-group">
                    <label class="form-label">Method Name</label>
                    <input type="text" class="form-input" id="bm-name" placeholder='e.g. "Local Backups"'>
                </div>
                <div class="form-group">
                    <label class="form-label">Type</label>
                    <select class="form-select" id="bm-type">
                        <option value="local">Local</option>
                        <option value="s3">S3</option>
                        <option value="sftp">SFTP</option>
                        <option value="gdrive">Google Drive</option>
                        <option value="dropbox">Dropbox</option>
                    </select>
                </div>
            `;
            const footer = `
                <button class="btn btn-secondary" onclick="document.getElementById('modal-overlay').remove()">Cancel</button>
                <button class="btn btn-primary" id="bm-save-btn">Add Method</button>
            `;
            const modal = showModal('Add Backup Method', content, footer);
            modal.querySelector('#bm-save-btn')?.addEventListener('click', async () => {
                const name = modal.querySelector('#bm-name')?.value?.trim();
                const type = modal.querySelector('#bm-type')?.value;
                if (!name) { showToast('Enter a method name', 'error'); return; }
                try {
                    await backupMethods.create({ name, type, config: '{}' });
                    closeModal();
                    showToast('Backup method added', 'success');
                    loadBackupMethods();
                } catch (err) { showToast(err.message, 'error'); }
            });
        });

    } catch (err) {
        container.innerHTML = `<div class="empty-state"><div class="empty-state-title">Error: ${err.message}</div></div>`;
    }
}
