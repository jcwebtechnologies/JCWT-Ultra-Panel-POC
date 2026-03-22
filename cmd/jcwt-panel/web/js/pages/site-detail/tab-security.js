import { sites, sshKeys } from '../../api.js';
import { icons, showToast, showModal, closeModal, escapeHtml, showConfirm } from '../../app.js';
import { request } from '../../api.js';
import { showLoading } from '../../ui.js';

export async function renderSecurity(container, site, siteId, refreshTabs) {
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

    document.getElementById('basic-auth-toggle')?.addEventListener('change', (e) => {
        document.getElementById('auth-users-section').style.display = e.target.checked ? '' : 'none';
    });

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
        const noUsers = list.querySelector('p');
        if (noUsers) noUsers.remove();
        list.appendChild(row);
        row.querySelector('.remove-auth-user').addEventListener('click', () => row.remove());
        bindToggle(row.querySelector('.toggle-pwd-btn'));
    });

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

    container.querySelectorAll('.remove-auth-user').forEach(btn => {
        btn.addEventListener('click', () => btn.closest('.auth-user-row').remove());
    });

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
            const updatedSite = await sites.get(siteId);
            Object.assign(site, updatedSite);
        } catch (err) { showToast(err.message, 'error'); }
    });
}

export async function renderSSHAccess(container, site, siteId) {
    showLoading(container);

    try {
        const [status, keys] = await Promise.all([
            sshKeys.status(siteId),
            sshKeys.list(siteId),
        ]);

        const sshEnabled = status.ssh_enabled;
        const sysUser = status.system_user;

        function renderKeyTable(keyList) {
            if (!keyList || keyList.length === 0) {
                return '<p style="color: var(--text-tertiary); font-size: var(--font-size-sm);">No SSH keys yet. Generate or upload a key pair to get started.</p>';
            }
            return `
            <div class="table-container">
                <table class="data-table responsive-cards has-actions">
                    <thead>
                        <tr>
                            <th>Name</th>
                            <th>Type</th>
                            <th>Fingerprint</th>
                            <th>Authorized</th>
                            <th style="text-align:right;">Actions</th>
                        </tr>
                    </thead>
                    <tbody>
                        ${keyList.map(k => `
                        <tr>
                            <td data-label="Name"><strong>${escapeHtml(k.name)}</strong></td>
                            <td data-label="Type"><span class="mono" style="font-size:var(--font-size-xs);">${escapeHtml(k.key_type.toUpperCase())} ${k.bits}</span></td>
                            <td data-label="Fingerprint"><span class="mono" style="font-size:var(--font-size-xs); word-break:break-all;">${escapeHtml(k.fingerprint).substring(0, 47)}</span></td>
                            <td data-label="Authorized">${k.authorized ? '<span style="color:var(--status-success);font-weight:600;">Yes</span>' : '<span style="color:var(--text-tertiary);">No</span>'}</td>
                            <td data-label="Actions" style="text-align:right;">
                                <div class="table-actions" style="display:flex;gap:var(--space-1);justify-content:flex-end;flex-wrap:wrap;">
                                    <button class="btn btn-sm btn-secondary ssh-view-pub" data-id="${k.id}" data-name="${escapeHtml(k.name)}" title="View Public Key"><span class="nav-icon nav-icon-xs">${icons.eye}</span></button>
                                    ${k.has_private_key ? `<button class="btn btn-sm btn-secondary ssh-view-priv" data-id="${k.id}" data-name="${escapeHtml(k.name)}" title="View Private Key"><span class="nav-icon nav-icon-xs">${icons.key}</span></button>` : ''}
                                    ${k.authorized
                                        ? `<button class="btn btn-sm btn-secondary ssh-deauth" data-id="${k.id}" data-name="${escapeHtml(k.name)}" title="Deauthorize"><span class="nav-icon nav-icon-xs">${icons.lock}</span></button>`
                                        : `<button class="btn btn-sm btn-primary ssh-authorize" data-id="${k.id}" data-name="${escapeHtml(k.name)}" title="Authorize"><span class="nav-icon nav-icon-xs">${icons.shield}</span></button>`
                                    }
                                    <button class="btn btn-sm btn-danger ssh-delete" data-id="${k.id}" data-name="${escapeHtml(k.name)}" title="Delete"><span class="nav-icon nav-icon-xs">${icons.trash}</span></button>
                                </div>
                            </td>
                        </tr>`).join('')}
                    </tbody>
                </table>
            </div>`;
        }

        container.innerHTML = `
        <div class="card" style="margin-bottom: var(--space-4);">
            <div class="card-header">
                <h3 class="card-title">SSH Access</h3>
            </div>
            <div style="padding: var(--space-4);">
                <p style="color: var(--text-secondary); margin-bottom: var(--space-4); font-size: var(--font-size-sm);">
                    Enable SSH access for the system user <strong class="mono">${escapeHtml(sysUser)}</strong>. Only key-based authentication is allowed — password login is disabled at the server level.
                </p>
                <div class="settings-row" style="margin-bottom: 0;">
                    <div class="settings-row-label">Enable SSH<small>Allow SSH login for this site's system user</small></div>
                    <div>
                        <label class="toggle">
                            <input type="checkbox" id="ssh-toggle" ${sshEnabled ? 'checked' : ''}>
                            <span class="toggle-slider"></span>
                        </label>
                    </div>
                </div>
            </div>
        </div>

        <div class="card" style="margin-bottom: var(--space-4);">
            <div class="card-header">
                <h3 class="card-title">SSH Keys</h3>
                <div style="display:flex;gap:var(--space-2);">
                    <button class="btn btn-sm btn-primary" id="ssh-generate-btn"><span class="nav-icon nav-icon-xs">${icons.plus}</span> Generate Key Pair</button>
                    <button class="btn btn-sm btn-secondary" id="ssh-upload-btn"><span class="nav-icon nav-icon-xs">${icons.upload}</span> Upload Key</button>
                </div>
            </div>
            <div style="padding: var(--space-4);" id="ssh-keys-container">
                ${renderKeyTable(keys)}
            </div>
        </div>`;

        document.getElementById('ssh-toggle')?.addEventListener('change', async (e) => {
            const toggle = e.target;
            toggle.disabled = true;
            try {
                await sshKeys.toggle(siteId, toggle.checked);
                showToast(toggle.checked ? 'SSH access enabled' : 'SSH access disabled', 'success');
            } catch (err) {
                toggle.checked = !toggle.checked;
                showToast(err.message, 'error');
            } finally {
                toggle.disabled = false;
            }
        });

        document.getElementById('ssh-generate-btn')?.addEventListener('click', () => {
            showModal('Generate SSH Key Pair', `
                <div class="form-group">
                    <label class="form-label">Key Name</label>
                    <input type="text" class="form-input" id="gen-key-name" placeholder="e.g. deploy-key">
                </div>
                <div class="form-row">
                    <div class="form-group">
                        <label class="form-label">Key Type</label>
                        <select class="form-select" id="gen-key-type">
                            <option value="rsa">RSA</option>
                            <option value="ed25519">Ed25519</option>
                        </select>
                    </div>
                    <div class="form-group" id="gen-bits-group">
                        <label class="form-label">Key Size (bits)</label>
                        <select class="form-select" id="gen-key-bits">
                            <option value="2048" selected>2048</option>
                            <option value="4096">4096</option>
                        </select>
                    </div>
                </div>
                <div class="form-group" style="margin-top: var(--space-2);">
                    <div style="display: flex; align-items: center; gap: var(--space-3);">
                        <label class="toggle">
                            <input type="checkbox" id="gen-passphrase-toggle">
                            <span class="toggle-slider"></span>
                        </label>
                        <span class="form-label" style="margin: 0;">Set Passphrase</span>
                    </div>
                </div>
                <div class="form-group" id="gen-passphrase-group" style="display:none;">
                    <label class="form-label">Passphrase</label>
                    <input type="password" class="form-input" id="gen-passphrase" placeholder="Enter passphrase for the private key">
                </div>
            `, `
                <button class="btn btn-secondary" onclick="document.getElementById('modal-overlay').remove()">Cancel</button>
                <button class="btn btn-primary" id="gen-key-submit">Generate</button>
            `);
            document.getElementById('gen-key-type')?.addEventListener('change', (e) => {
                const bitsGroup = document.getElementById('gen-bits-group');
                if (e.target.value === 'ed25519') {
                    bitsGroup.style.display = 'none';
                } else {
                    bitsGroup.style.display = '';
                    document.getElementById('gen-key-bits').innerHTML = '<option value="2048" selected>2048</option><option value="4096">4096</option>';
                }
            });
            document.getElementById('gen-passphrase-toggle')?.addEventListener('change', (e) => {
                document.getElementById('gen-passphrase-group').style.display = e.target.checked ? '' : 'none';
                if (!e.target.checked) document.getElementById('gen-passphrase').value = '';
            });
            document.getElementById('gen-key-submit')?.addEventListener('click', async () => {
                const name = document.getElementById('gen-key-name').value.trim();
                if (!name) { showToast('Key name is required', 'error'); return; }
                const keyType = document.getElementById('gen-key-type').value;
                const btn = document.getElementById('gen-key-submit');
                btn.disabled = true;
                btn.innerHTML = '<span class="loading-spinner btn-spinner"></span> Generating...';
                try {
                    const data = {
                        site_id: parseInt(siteId),
                        name,
                        key_type: keyType,
                        bits: keyType === 'ed25519' ? 256 : parseInt(document.getElementById('gen-key-bits').value),
                    };
                    const passphrase = document.getElementById('gen-passphrase')?.value || '';
                    if (passphrase) data.passphrase = passphrase;
                    await sshKeys.generate(data);
                    closeModal();
                    showToast('Key pair generated', 'success');
                    renderSSHAccess(container, site, siteId);
                } catch (err) {
                    btn.disabled = false;
                    btn.textContent = 'Generate';
                    showToast(err.message, 'error');
                }
            });
        });

        document.getElementById('ssh-upload-btn')?.addEventListener('click', () => {
            showModal('Upload SSH Key', `
                <div class="form-group">
                    <label class="form-label">Key Name</label>
                    <input type="text" class="form-input" id="upl-key-name" placeholder="e.g. my-laptop">
                </div>
                <div class="form-row">
                    <div class="form-group">
                        <label class="form-label">Key Type</label>
                        <select class="form-select" id="upl-key-type">
                            <option value="rsa">RSA</option>
                            <option value="ed25519">Ed25519</option>
                        </select>
                    </div>
                    <div class="form-group" id="upl-bits-group">
                        <label class="form-label">Key Size (bits)</label>
                        <select class="form-select" id="upl-key-bits">
                            <option value="2048" selected>2048</option>
                            <option value="4096">4096</option>
                        </select>
                    </div>
                </div>
                <div class="form-group">
                    <label class="form-label">Public Key <span style="color:var(--status-error);">*</span></label>
                    <textarea class="form-textarea mono" id="upl-pub-key" rows="4" placeholder="ssh-rsa AAAA... or paste contents of .pub file" style="font-size:var(--font-size-xs);"></textarea>
                    <label class="btn btn-sm btn-secondary" style="margin-top: var(--space-2); cursor: pointer;">
                        <span class="nav-icon nav-icon-xs">${icons.upload}</span> Upload .pub file
                        <input type="file" id="upl-pub-file" accept=".pub,.pem,.txt" style="display:none;">
                    </label>
                </div>
                <div class="form-group">
                    <label class="form-label">Private Key <span style="color:var(--text-tertiary);">(optional)</span></label>
                    <textarea class="form-textarea mono" id="upl-priv-key" rows="4" placeholder="-----BEGIN OPENSSH PRIVATE KEY----- or paste contents" style="font-size:var(--font-size-xs);"></textarea>
                    <label class="btn btn-sm btn-secondary" style="margin-top: var(--space-2); cursor: pointer;">
                        <span class="nav-icon nav-icon-xs">${icons.upload}</span> Upload private key file
                        <input type="file" id="upl-priv-file" accept=".pem,.key,.txt,*" style="display:none;">
                    </label>
                </div>
            `, `
                <button class="btn btn-secondary" onclick="document.getElementById('modal-overlay').remove()">Cancel</button>
                <button class="btn btn-primary" id="upl-key-submit">Upload</button>
            `);
            document.getElementById('upl-key-type')?.addEventListener('change', (e) => {
                const bitsGroup = document.getElementById('upl-bits-group');
                if (e.target.value === 'ed25519') {
                    bitsGroup.style.display = 'none';
                } else {
                    bitsGroup.style.display = '';
                    document.getElementById('upl-key-bits').innerHTML = '<option value="2048" selected>2048</option><option value="4096">4096</option>';
                }
            });
            document.getElementById('upl-pub-file')?.addEventListener('change', (e) => {
                const file = e.target.files[0];
                if (file) {
                    const reader = new FileReader();
                    reader.onload = () => { document.getElementById('upl-pub-key').value = reader.result; };
                    reader.readAsText(file);
                }
            });
            document.getElementById('upl-priv-file')?.addEventListener('change', (e) => {
                const file = e.target.files[0];
                if (file) {
                    const reader = new FileReader();
                    reader.onload = () => { document.getElementById('upl-priv-key').value = reader.result; };
                    reader.readAsText(file);
                }
            });
            document.getElementById('upl-key-submit')?.addEventListener('click', async () => {
                const name = document.getElementById('upl-key-name').value.trim();
                const pubKey = document.getElementById('upl-pub-key').value.trim();
                if (!name) { showToast('Key name is required', 'error'); return; }
                if (!pubKey) { showToast('Public key is required', 'error'); return; }
                const btn = document.getElementById('upl-key-submit');
                btn.disabled = true;
                btn.textContent = 'Uploading...';
                try {
                    await sshKeys.upload({
                        site_id: parseInt(siteId),
                        name,
                        key_type: document.getElementById('upl-key-type').value,
                        bits: parseInt(document.getElementById('upl-key-bits').value),
                        public_key: pubKey,
                        private_key: document.getElementById('upl-priv-key').value.trim(),
                    });
                    closeModal();
                    showToast('Key uploaded', 'success');
                    renderSSHAccess(container, site, siteId);
                } catch (err) {
                    btn.disabled = false;
                    btn.textContent = 'Upload';
                    showToast(err.message, 'error');
                }
            });
        });

        function bindKeyActions() {
            container.querySelectorAll('.ssh-view-pub').forEach(btn => {
                btn.addEventListener('click', async () => {
                    try {
                        const data = await sshKeys.viewKey(btn.dataset.id, 'public');
                        showModal('Public Key — ' + escapeHtml(btn.dataset.name), `
                            <textarea class="form-textarea mono" readonly style="min-height:120px;font-size:var(--font-size-xs);word-break:break-all;">${escapeHtml(data.content)}</textarea>
                        `, `
                            <button class="btn btn-secondary" onclick="document.getElementById('modal-overlay').remove()">Close</button>
                            <button class="btn btn-primary ssh-copy-key">Copy</button>
                            <a class="btn btn-secondary ssh-dl-key" download="${escapeHtml(data.name)}.pub">Download</a>
                        `);
                        const blob = new Blob([data.content], { type: 'text/plain' });
                        document.querySelector('.ssh-dl-key').href = URL.createObjectURL(blob);
                        document.querySelector('.ssh-copy-key')?.addEventListener('click', () => {
                            navigator.clipboard.writeText(data.content).then(() => showToast('Copied', 'success'));
                        });
                    } catch (err) { showToast(err.message, 'error'); }
                });
            });

            container.querySelectorAll('.ssh-view-priv').forEach(btn => {
                btn.addEventListener('click', async () => {
                    try {
                        const data = await sshKeys.viewKey(btn.dataset.id, 'private');
                        showModal('Private Key — ' + escapeHtml(btn.dataset.name), `
                            <div class="warning-banner">
                                <span class="warning-banner-icon">${icons.alertTriangle}</span>
                                <span>Keep this private key secure. Never share it publicly.</span>
                            </div>
                            <textarea class="form-textarea mono" readonly wrap="off" style="min-height:180px;font-size:var(--font-size-xs);white-space:pre;overflow-x:auto;">${escapeHtml(data.content)}</textarea>
                        `, `
                            <button class="btn btn-secondary" onclick="document.getElementById('modal-overlay').remove()">Close</button>
                            <button class="btn btn-primary ssh-copy-key">Copy</button>
                            <a class="btn btn-secondary ssh-dl-key" download="${escapeHtml(data.name)}.pem">Download</a>
                        `);
                        // Normalize to Unix line endings (\n only) so the downloaded file
                        // is not corrupted if copied/opened in Windows editors
                        const keyContent = data.content.replace(/\r\n/g, '\n').replace(/\r/g, '\n');
                        const blob = new Blob([keyContent], { type: 'application/octet-stream' });
                        document.querySelector('.ssh-dl-key').href = URL.createObjectURL(blob);
                        document.querySelector('.ssh-copy-key')?.addEventListener('click', () => {
                            navigator.clipboard.writeText(keyContent).then(() => showToast('Copied', 'success'));
                        });
                    } catch (err) { showToast(err.message, 'error'); }
                });
            });

            container.querySelectorAll('.ssh-authorize').forEach(btn => {
                btn.addEventListener('click', () => {
                    showModal('Authorize Public Key', `
                        <div class="warning-banner">
                            <span class="warning-banner-icon">${icons.alertTriangle}</span>
                            <span>This will add the public key "<strong>${escapeHtml(btn.dataset.name)}</strong>" to the user's <code>~/.ssh/authorized_keys</code> file. Anyone with the corresponding private key will be able to SSH into this server as <strong>${escapeHtml(sysUser)}</strong>.</span>
                        </div>
                    `, `
                        <button class="btn btn-secondary" onclick="document.getElementById('modal-overlay').remove()">Cancel</button>
                        <button class="btn btn-primary" id="confirm-auth-key">Authorize</button>
                    `);
                    document.getElementById('confirm-auth-key')?.addEventListener('click', async () => {
                        const confirmBtn = document.getElementById('confirm-auth-key');
                        confirmBtn.disabled = true;
                        confirmBtn.textContent = 'Authorizing...';
                        try {
                            await sshKeys.authorize({ id: parseInt(btn.dataset.id), site_id: parseInt(siteId), authorized: true });
                            closeModal();
                            showToast('Key authorized', 'success');
                            renderSSHAccess(container, site, siteId);
                        } catch (err) {
                            confirmBtn.disabled = false;
                            confirmBtn.textContent = 'Authorize';
                            showToast(err.message, 'error');
                        }
                    });
                });
            });

            container.querySelectorAll('.ssh-deauth').forEach(btn => {
                btn.addEventListener('click', async () => {
                    const ok = await showConfirm('Deauthorize Key', `Remove "${btn.dataset.name}" from authorized_keys? This will revoke SSH access for this key.`);
                    if (!ok) return;
                    try {
                        await sshKeys.authorize({ id: parseInt(btn.dataset.id), site_id: parseInt(siteId), authorized: false });
                        showToast('Key deauthorized', 'success');
                        renderSSHAccess(container, site, siteId);
                    } catch (err) { showToast(err.message, 'error'); }
                });
            });

            container.querySelectorAll('.ssh-delete').forEach(btn => {
                btn.addEventListener('click', async () => {
                    const ok = await showConfirm('Delete SSH Key', `Delete key "${btn.dataset.name}"? If this key is authorized, it will be removed from authorized_keys as well.`);
                    if (!ok) return;
                    try {
                        await sshKeys.delete(btn.dataset.id, siteId);
                        showToast('Key deleted', 'success');
                        renderSSHAccess(container, site, siteId);
                    } catch (err) { showToast(err.message, 'error'); }
                });
            });
        }
        bindKeyActions();

    } catch (err) {
        container.innerHTML = `<div class="empty-state"><div class="empty-state-title">Error: ${escapeHtml(err.message)}</div></div>`;
    }
}
