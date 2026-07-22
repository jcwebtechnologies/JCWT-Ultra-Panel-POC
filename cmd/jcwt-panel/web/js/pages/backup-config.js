// JCWT Ultra Panel — Backup Configuration Settings
import { backupMethods, request } from '../api.js';
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
            <button type="button" class="btn btn-sm btn-primary" id="add-backup-method-btn">+ Add Backup Method</button>
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
                listEl.innerHTML = methods.map(m => {
                    let cfgDetail = '';
                    if (m.type === 'sftp') {
                        try {
                            const c = typeof m.config === 'string' ? JSON.parse(m.config) : (m.config || {});
                            if (c.host) cfgDetail = ` • ${escapeHtml(c.host)}:${c.port || 22} (${escapeHtml(c.remote_path || '/backups/jcwt-panel')})`;
                        } catch (e) {}
                    }
                    return `
                    <div class="settings-row" style="padding:var(--space-3);border:1px solid var(--border-primary);border-radius:var(--radius-md);margin-bottom:var(--space-2);">
                        <div class="settings-row-label" style="min-width:auto;">
                            <strong>${escapeHtml(m.name)}</strong>
                            <small>Type: ${escapeHtml(m.type.toUpperCase())}${cfgDetail} ${m.enabled ? '<span class="status-badge status-active" style="display:inline-block;padding:2px 8px;font-size:11px;margin-left:6px;">Active</span>' : '<span class="status-badge status-inactive" style="display:inline-block;padding:2px 8px;font-size:11px;margin-left:6px;">Disabled</span>'}</small>
                        </div>
                        <div style="display:flex;gap:var(--space-2);">
                            <button type="button" class="btn btn-sm btn-secondary" data-edit-method="${m.id}">Edit</button>
                            <button type="button" class="btn btn-sm ${m.enabled ? 'btn-secondary' : 'btn-primary'}" data-toggle-method="${m.id}">${m.enabled ? 'Disable' : 'Enable'}</button>
                            <button type="button" class="btn btn-sm btn-danger" data-delete-method="${m.id}">Delete</button>
                        </div>
                    </div>`;
                }).join('');

                listEl.querySelectorAll('[data-edit-method]').forEach(btn => {
                    btn.addEventListener('click', () => {
                        const mid = parseInt(btn.dataset.editMethod);
                        const method = methods.find(m => m.id === mid);
                        if (method) openBackupMethodModal(method, loadBackupMethods);
                    });
                });

                listEl.querySelectorAll('[data-toggle-method]').forEach(btn => {
                    btn.addEventListener('click', async () => {
                        const mid = parseInt(btn.dataset.toggleMethod);
                        const method = methods.find(m => m.id === mid);
                        if (!method) return;
                        try {
                            await backupMethods.update({ id: mid, name: method.name, type: method.type, config: typeof method.config === 'object' ? JSON.stringify(method.config) : (method.config || '{}'), enabled: !method.enabled });
                            showToast('Backup method status updated', 'success');
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
            openBackupMethodModal(null, loadBackupMethods);
        });

    } catch (err) {
        container.innerHTML = `<div class="empty-state"><div class="empty-state-title">Error: ${err.message}</div></div>`;
    }
}

export function openBackupMethodModal(existingMethod = null, onSaved = () => {}) {
    const isEdit = !!existingMethod;
    let existingCfg = {};
    if (existingMethod && existingMethod.config) {
        try {
            existingCfg = typeof existingMethod.config === 'string' ? JSON.parse(existingMethod.config) : existingMethod.config;
        } catch (e) {}
    }

    const type = existingMethod?.type || 'sftp';
    const name = existingMethod?.name || '';

    const content = `
        <div class="form-group">
            <label class="form-label">Method Name *</label>
            <input type="text" class="form-input" id="bm-name" value="${escapeHtml(name)}" placeholder='e.g. "Remote SFTP Storage"'>
        </div>
        <div class="form-group">
            <label class="form-label">Type</label>
            <select class="form-select" id="bm-type" ${isEdit ? 'disabled' : ''}>
                <option value="sftp" ${type === 'sftp' ? 'selected' : ''}>SFTP (SSH File Transfer Protocol)</option>
                <option value="local" ${type === 'local' ? 'selected' : ''}>Local Server Directory</option>
                <option value="s3" ${type === 's3' ? 'selected' : ''}>AWS S3 / S3-Compatible</option>
                <option value="gdrive" ${type === 'gdrive' ? 'selected' : ''}>Google Drive</option>
                <option value="dropbox" ${type === 'dropbox' ? 'selected' : ''}>Dropbox</option>
            </select>
        </div>

        <div id="bm-sftp-fields" style="display: ${type === 'sftp' ? 'block' : 'none'}; padding-top: var(--space-3); border-top: 1px solid var(--border-primary); margin-top: var(--space-3);">
            <div style="display: grid; grid-template-columns: 2fr 1fr; gap: var(--space-3);">
                <div class="form-group">
                    <label class="form-label">Host / IP Address *</label>
                    <input type="text" class="form-input" id="bm-sftp-host" value="${escapeHtml(existingCfg.host || '')}" placeholder="e.g. sftp.example.com or 192.168.1.100">
                </div>
                <div class="form-group">
                    <label class="form-label">Port *</label>
                    <input type="number" class="form-input" id="bm-sftp-port" value="${existingCfg.port || 22}" placeholder="22">
                </div>
            </div>

            <div class="form-group">
                <label class="form-label">Username *</label>
                <input type="text" class="form-input" id="bm-sftp-user" value="${escapeHtml(existingCfg.username || '')}" placeholder="e.g. backupuser">
            </div>

            <div class="form-group">
                <label class="form-label">Authentication Method</label>
                <select class="form-select" id="bm-sftp-authtype">
                    <option value="password" ${(existingCfg.auth_type || 'password') === 'password' ? 'selected' : ''}>Password</option>
                    <option value="key" ${existingCfg.auth_type === 'key' ? 'selected' : ''}>SSH Private Key</option>
                </select>
            </div>

            <div class="form-group" id="bm-sftp-pass-group" style="display: ${(existingCfg.auth_type || 'password') === 'password' ? 'block' : 'none'};">
                <label class="form-label">SFTP Password</label>
                <input type="password" class="form-input" id="bm-sftp-pass" value="${existingCfg.has_password ? '********' : ''}" placeholder="${existingCfg.has_password ? 'Leave blank to keep existing password' : 'Enter SFTP user password'}">
            </div>

            <div id="bm-sftp-key-group" style="display: ${existingCfg.auth_type === 'key' ? 'block' : 'none'};">
                <div class="form-group">
                    <label class="form-label">SSH Private Key</label>
                    <textarea class="form-textarea mono" id="bm-sftp-key" rows="4" placeholder="${existingCfg.has_private_key ? 'SSH Key stored (Leave blank to keep existing key)' : 'Paste PEM or OpenSSH private key contents'}">${existingCfg.has_private_key ? '[SSH Private Key Stored]' : ''}</textarea>
                </div>
                <div class="form-group">
                    <label class="form-label">Passphrase (Optional)</label>
                    <input type="password" class="form-input" id="bm-sftp-passphrase" value="${escapeHtml(existingCfg.key_passphrase || '')}" placeholder="Passphrase if key is passphrase-protected">
                </div>
            </div>

            <div class="form-group">
                <label class="form-label">Remote Directory Path *</label>
                <input type="text" class="form-input" id="bm-sftp-path" value="${escapeHtml(existingCfg.remote_path || '/backups/jcwt-panel')}" placeholder="/backups/jcwt-panel">
            </div>
        </div>
    `;

    const footer = `
        <div style="display:flex; justify-content:space-between; width:100%; align-items:center;">
            <button type="button" class="btn btn-secondary" id="bm-test-btn" style="display:${type === 'sftp' ? 'inline-block' : 'none'};">⚡ Test Connection</button>
            <div style="display:flex; gap:var(--space-2); margin-left:auto;">
                <button type="button" class="btn btn-secondary" onclick="document.getElementById('modal-overlay').remove()">Cancel</button>
                <button type="button" class="btn btn-primary" id="bm-save-btn">${isEdit ? 'Save Changes' : 'Add Method'}</button>
            </div>
        </div>
    `;

    const modal = showModal(isEdit ? 'Edit Backup Method' : 'Add Backup Method', content, footer);

    const typeSelect = modal.querySelector('#bm-type');
    const authSelect = modal.querySelector('#bm-sftp-authtype');
    const testBtn = modal.querySelector('#bm-test-btn');
    const sftpFields = modal.querySelector('#bm-sftp-fields');
    const passGroup = modal.querySelector('#bm-sftp-pass-group');
    const keyGroup = modal.querySelector('#bm-sftp-key-group');

    typeSelect?.addEventListener('change', (e) => {
        const val = e.target.value;
        if (sftpFields) sftpFields.style.display = val === 'sftp' ? 'block' : 'none';
        if (testBtn) testBtn.style.display = val === 'sftp' ? 'inline-block' : 'none';
    });

    authSelect?.addEventListener('change', (e) => {
        const isKey = e.target.value === 'key';
        if (passGroup) passGroup.style.display = isKey ? 'none' : 'block';
        if (keyGroup) keyGroup.style.display = isKey ? 'block' : 'none';
    });

    function getSFTPConfigFromModal() {
        return {
            host: modal.querySelector('#bm-sftp-host')?.value?.trim(),
            port: parseInt(modal.querySelector('#bm-sftp-port')?.value || '22', 10),
            username: modal.querySelector('#bm-sftp-user')?.value?.trim(),
            auth_type: modal.querySelector('#bm-sftp-authtype')?.value,
            password: modal.querySelector('#bm-sftp-pass')?.value,
            private_key: modal.querySelector('#bm-sftp-key')?.value,
            key_passphrase: modal.querySelector('#bm-sftp-passphrase')?.value,
            remote_path: modal.querySelector('#bm-sftp-path')?.value?.trim() || '/backups/jcwt-panel'
        };
    }

    testBtn?.addEventListener('click', async () => {
        const sftpCfg = getSFTPConfigFromModal();
        if (!sftpCfg.host) { showToast('Host / IP address is required', 'error'); return; }
        if (!sftpCfg.username) { showToast('Username is required', 'error'); return; }

        testBtn.disabled = true;
        const origText = testBtn.innerHTML;
        testBtn.innerHTML = 'Testing...';

        try {
            const res = await request('/api/backup-methods?action=test-connection', {
                method: 'POST',
                body: JSON.stringify({
                    id: existingMethod?.id || 0,
                    type: 'sftp',
                    config: sftpCfg
                })
            });
            showToast(res.message || 'SFTP Connection Successful!', 'success');
        } catch (err) {
            showToast(err.message, 'error');
        } finally {
            testBtn.disabled = false;
            testBtn.innerHTML = origText;
        }
    });

    modal.querySelector('#bm-save-btn')?.addEventListener('click', async () => {
        const mName = modal.querySelector('#bm-name')?.value?.trim();
        const mType = typeSelect?.value;
        if (!mName) { showToast('Method name is required', 'error'); return; }

        let cfgObj = {};
        if (mType === 'sftp') {
            cfgObj = getSFTPConfigFromModal();
            if (!cfgObj.host || !cfgObj.username) {
                showToast('SFTP Host and Username are required', 'error');
                return;
            }
        }

        try {
            if (isEdit) {
                await backupMethods.update({
                    id: existingMethod.id,
                    name: mName,
                    type: mType,
                    config: JSON.stringify(cfgObj),
                    enabled: existingMethod.enabled
                });
                showToast('Backup method updated', 'success');
            } else {
                await backupMethods.create({
                    name: mName,
                    type: mType,
                    config: JSON.stringify(cfgObj)
                });
                showToast('Backup method created', 'success');
            }
            closeModal();
            onSaved();
        } catch (err) {
            showToast(err.message, 'error');
        }
    });
}
