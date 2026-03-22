import { databases, dbUsers } from '../../api.js';
import { icons, showToast, showModal, closeModal, escapeHtml, showConfirm } from '../../app.js';
import { request } from '../../api.js';
import { showLoading } from '../../ui.js';

export async function renderDatabases(container, siteId, site, refreshTabs) {
    showLoading(container);

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
            <div class="empty-state p-6">
                <div class="empty-state-title">No databases</div>
                <div class="empty-state-text">Create a database linked to this site.</div>
            </div>` : `
            <div class="table-responsive">
                <table class="data-table responsive-cards">
                    <thead><tr><th>Database Name</th><th>Users</th><th>Created</th><th>Actions</th></tr></thead>
                    <tbody>
                        ${siteDbs.map(db => {
            const users = (allUsers || []).filter(u => u.database_id === db.id);
            return `<tr>
                                <td data-label="Database"><span class="mono">${escapeHtml(db.db_name)}</span> <span class="copy-name" data-name="${escapeHtml(db.db_name)}" class="copy-btn-inline" title="Copy to clipboard"><span class="nav-icon nav-icon-sm">${icons.copy}</span></span></td>
                                <td data-label="Users">${users.length > 0 ? users.map(u => `<span class="badge badge-info">${escapeHtml(u.username)}</span>`).join(' ') : '<span style="color: var(--text-tertiary);">None</span>'}</td>
                                <td data-label="Created">${db.created_at ? new Date(db.created_at).toLocaleDateString() : 'N/A'}</td>
                                <td>
                                    <button class="btn btn-sm btn-danger delete-site-db" data-id="${db.id}" data-name="${escapeHtml(db.db_name)}">Delete</button>
                                </td>
                            </tr>`;
        }).join('')}
                    </tbody>
                </table>
            </div>`}
        </div>`;

        document.getElementById('add-site-db')?.addEventListener('click', () => {
            showModal('Create Database', `
                <div class="form-group">
                    <label class="form-label">Database Name</label>
                    <div style="display:flex;align-items:stretch;">
                        <span style="background:var(--bg-tertiary);border:1px solid var(--border-primary);border-right:none;padding:var(--space-2) var(--space-3);border-radius:var(--radius-md) 0 0 var(--radius-md);color:var(--text-tertiary);white-space:nowrap;display:flex;align-items:center;" class="mono">${escapeHtml(site.system_user)}_</span>
                        <input type="text" class="form-input mono" id="new-db-name" placeholder="myapp" required pattern="^[a-z][a-z0-9_]*$" maxlength="16" style="border-radius:0 var(--radius-md) var(--radius-md) 0;">
                    </div>
                    <small style="color: var(--text-tertiary); font-size: var(--font-size-xs);">Lowercase letters, numbers, underscore only. Must start with a letter. Max 16 characters.</small>
                </div>
            `, `
                <button class="btn btn-secondary" onclick="document.getElementById('modal-overlay').remove()">Cancel</button>
                <button class="btn btn-primary" id="confirm-create-db">Create</button>
            `);

            document.getElementById('confirm-create-db')?.addEventListener('click', async () => {
                const dbName = document.getElementById('new-db-name').value.trim();
                if (!dbName) { showToast('Name required', 'error'); return; }
                if (!/^[a-z][a-z0-9_]*$/.test(dbName) || dbName.length > 16) {
                    showToast('Invalid name: lowercase letters, numbers, underscore only (start with a letter, max 16 chars)', 'error'); return;
                }
                try {
                    await databases.create({ db_name: dbName, site_id: parseInt(siteId) });
                    closeModal();
                    showToast('Database created!', 'success');
                    renderDatabases(container, siteId, site, refreshTabs);
                } catch (err) { showToast(err.message, 'error'); }
            });
        });

        container.querySelectorAll('.delete-site-db').forEach(btn => {
            btn.addEventListener('click', async () => {
                const id = btn.dataset.id;
                const name = btn.dataset.name;
                const linkedUsers = (allUsers || []).filter(u => String(u.database_id) === String(id));
                let msg = `Delete database "<strong>${escapeHtml(name)}</strong>"? This will also drop it from MariaDB.`;
                if (linkedUsers.length > 0) {
                    msg += `<br><br><strong style="color: var(--danger);">The following database user${linkedUsers.length > 1 ? 's' : ''} will also be deleted:</strong><br>` +
                        linkedUsers.map(u => `• <span class="mono">${escapeHtml(u.username)}</span>`).join('<br>');
                }
                if (!await showConfirm('Delete Database', msg, 'Delete', 'btn-danger', { html: true })) return;
                try {
                    await databases.delete(id);
                    showToast(`Database ${name} deleted`, 'success');
                    renderDatabases(container, siteId, site, refreshTabs);
                } catch (err) { showToast(err.message, 'error'); }
            });
        });

        container.querySelectorAll('.copy-name').forEach(el => {
            el.addEventListener('click', () => {
                navigator.clipboard.writeText(el.dataset.name).then(() => showToast('Copied to clipboard', 'success'));
            });
        });

    } catch (err) {
        container.innerHTML = `<div class="empty-state"><div class="empty-state-title">Error: ${escapeHtml(err.message)}</div></div>`;
    }
}

export async function renderDBUsers(container, siteId, site, refreshTabs) {
    showLoading(container);

    const privilegeLabels = {
        'readonly': 'Read Only',
        'readwrite': 'Read / Write',
        'full': 'Full'
    };
    const privilegeBadge = {
        'readonly': 'badge-warning',
        'readwrite': 'badge-info',
        'full': 'badge-success'
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
                <button class="btn btn-primary btn-sm" id="add-db-user">${icons.plus} Create User</button>
            </div>
            ${siteUsers.length === 0 ? `
            <div class="empty-state p-6">
                <div class="empty-state-title">No database users</div>
                <div class="empty-state-text">${siteDbs.length === 0 ? 'Create a database first, then add users.' : 'Create a user with specific privileges for a database.'}</div>
            </div>` : `
            <div class="table-responsive">
                <table class="data-table responsive-cards">
                    <thead><tr><th>Username</th><th>Database</th><th>Privileges</th><th>Created</th><th>Actions</th></tr></thead>
                    <tbody>
                        ${siteUsers.map(u => `<tr>
                                <td data-label="Username">${escapeHtml(u.username)} <span class="copy-name" data-name="${escapeHtml(u.username)}" class="copy-btn-inline" title="Copy to clipboard"><span class="nav-icon nav-icon-sm">${icons.copy}</span></span></td>
                                <td data-label="Database"><span class="badge badge-info">${escapeHtml(u.db_name)}</span> <span class="copy-name" data-name="${escapeHtml(u.db_name)}" class="copy-btn-inline" title="Copy to clipboard"><span class="nav-icon nav-icon-sm">${icons.copy}</span></span></td>
                                <td data-label="Privileges">
                                    <select class="form-select form-select-sm change-privilege" data-id="${u.id}" data-username="${escapeHtml(u.username)}" style="width: auto; padding: var(--space-1) var(--space-2); font-size: var(--font-size-xs);">
                                        ${Object.entries(privilegeLabels).map(([val, label]) => `<option value="${val}" ${u.privilege_level === val ? 'selected' : ''}>${label}</option>`).join('')}
                                    </select>
                                </td>
                                <td data-label="Created">${u.created_at ? new Date(u.created_at).toLocaleDateString() : 'N/A'}</td>
                                <td style="display: flex; gap: var(--space-1);">
                                    <button class="btn btn-sm btn-secondary change-pwd" data-username="${escapeHtml(u.username)}" title="Change Password">${icons.key} Password</button>
                                    <button class="btn btn-sm btn-secondary open-pma-user" data-id="${u.database_id}" data-user-id="${u.id}" data-name="${escapeHtml(u.db_name)}" title="Open phpMyAdmin">⛁ phpMyAdmin</button>
                                    <button class="btn btn-sm btn-danger delete-db-user" data-id="${u.id}" data-username="${escapeHtml(u.username)}">Delete</button>
                                </td>
                            </tr>`).join('')}
                    </tbody>
                </table>
            </div>`}
        </div>`;

        document.getElementById('add-db-user')?.addEventListener('click', () => {
            if (siteDbs.length === 0) { showToast('Create a database first', 'warning'); return; }
            const dbOpts = siteDbs.map(db => `<option value="${db.id}">${escapeHtml(db.db_name)}</option>`).join('');
            showModal('Create Database User', `
                <div class="form-group">
                    <label class="form-label">Username</label>
                    <div style="display:flex;align-items:stretch;">
                        <span style="background:var(--bg-tertiary);border:1px solid var(--border-primary);border-right:none;padding:var(--space-2) var(--space-3);border-radius:var(--radius-md) 0 0 var(--radius-md);color:var(--text-tertiary);white-space:nowrap;display:flex;align-items:center;" class="mono">${escapeHtml(site.system_user)}_</span>
                        <input type="text" class="form-input mono" id="new-dbuser-name" placeholder="myuser" required pattern="^[a-z][a-z0-9_]*$" maxlength="15" autocomplete="off" style="border-radius:0 var(--radius-md) var(--radius-md) 0;">
                    </div>
                    <small style="color: var(--text-tertiary); font-size: var(--font-size-xs);">Lowercase letters, numbers, underscore only. Must start with a letter. Max 15 characters.</small>
                </div>
                <div class="form-group">
                    <label class="form-label">Password</label>
                    <input type="password" class="form-input" id="new-dbuser-pass" placeholder="Min 8 characters" required minlength="8" autocomplete="new-password">
                </div>
                <div class="form-group">
                    <label class="form-label">Database</label>
                    <select class="form-select" id="new-dbuser-db">${dbOpts}</select>
                </div>
                <div class="form-group">
                    <label class="form-label">Privilege Level</label>
                    <select class="form-select" id="new-dbuser-priv">
                        <option value="readonly">Read Only — SELECT only</option>
                        <option value="readwrite" selected>Read / Write — SELECT, INSERT, UPDATE, DELETE</option>
                        <option value="full">Full — All DML + DDL operations</option>
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
                if (!/^[a-z][a-z0-9_]*$/.test(username) || username.length > 15) {
                    showToast('Invalid: lowercase letters, numbers, underscore only (start with a letter, max 15 chars)', 'error'); return;
                }
                if (password.length < 8) { showToast('Password must be at least 8 characters', 'error'); return; }
                try {
                    await dbUsers.create({ username, password, database_id: databaseId, privilege_level: privilegeLevel });
                    closeModal();
                    showToast('Database user created!', 'success');
                    renderDBUsers(container, siteId, site, refreshTabs);
                } catch (err) { showToast(err.message, 'error'); }
            });
        });

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

        container.querySelectorAll('.open-pma-user').forEach(btn => {
            btn.addEventListener('click', async () => {
                const dbId = btn.dataset.id;
                const dbUserId = btn.dataset.userId;
                const dbName = btn.dataset.name;
                btn.disabled = true;
                btn.textContent = '⏳ Opening...';
                try {
                    const data = await request('/api/pma', {
                        method: 'POST',
                        body: JSON.stringify({ database_id: parseInt(dbId), db_user_id: parseInt(dbUserId) })
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

        container.querySelectorAll('.copy-name').forEach(el => {
            el.addEventListener('click', () => {
                navigator.clipboard.writeText(el.dataset.name).then(() => showToast('Copied to clipboard', 'success'));
            });
        });
    } catch (err) {
        container.innerHTML = `<div class="empty-state"><div class="empty-state-title">Error: ${escapeHtml(err.message)}</div></div>`;
    }
}

export async function renderPhpMyAdmin(container, siteId) {
    showLoading(container);
    try {
        const [allDbs, allUsers] = await Promise.all([databases.list(), dbUsers.list()]);
        const siteDbs = (allDbs || []).filter(db => String(db.site_id) === String(siteId));
        const siteUsers = (allUsers || []).filter(u => String(u.site_id) === String(siteId));

        if (siteDbs.length === 0) {
            container.innerHTML = `<div class="card"><div class="empty-state p-6"><div class="empty-state-title">No databases</div><div class="empty-state-text">Create a database first to use phpMyAdmin.</div></div></div>`;
            return;
        }

        if (siteUsers.length === 0) {
            container.innerHTML = `<div class="card"><div class="empty-state p-6"><div class="empty-state-title">No database users</div><div class="empty-state-text">Create a database user first to open phpMyAdmin.</div></div></div>`;
            return;
        }

        container.innerHTML = `
        <div class="card">
            <div class="card-header">
                <h3 class="card-title">phpMyAdmin</h3>
            </div>
            <div style="padding: var(--space-4);">
                <p style="color: var(--text-secondary); margin-bottom: var(--space-4);">Select a database and user to open phpMyAdmin with matching privileges.</p>
                <div class="form-row">
                    <div class="form-group">
                        <label class="form-label">Database</label>
                        <select class="form-select" id="pma-database">
                            ${siteDbs.map(db => `<option value="${db.id}" data-name="${escapeHtml(db.db_name)}">${escapeHtml(db.db_name)}</option>`).join('')}
                        </select>
                    </div>
                    <div class="form-group">
                        <label class="form-label">Database User</label>
                        <select class="form-select" id="pma-user">
                            <option value="" disabled selected>— Select a user —</option>
                            ${siteUsers.map(u => `<option value="${u.id}" data-db="${u.database_id}" data-level="${escapeHtml(u.privilege_level)}">${escapeHtml(u.username)} (${escapeHtml(u.privilege_level)})</option>`).join('')}
                        </select>
                    </div>
                </div>
                <button class="btn btn-primary" id="pma-open-btn"><span class="nav-icon nav-icon-sm">${icons.pma}</span> Open phpMyAdmin</button>
            </div>
        </div>`;

        const dbSelect = document.getElementById('pma-database');
        const userSelect = document.getElementById('pma-user');
        function filterUsers() {
            const dbId = dbSelect.value;
            let hasVisible = false;
            Array.from(userSelect.options).forEach(opt => {
                if (!opt.value) return;
                const show = opt.dataset.db === dbId;
                opt.style.display = show ? '' : 'none';
                if (!show && opt.selected) opt.selected = false;
                if (show) hasVisible = true;
            });
            if (!userSelect.value || userSelect.selectedOptions[0]?.style.display === 'none') {
                userSelect.value = '';
            }
        }
        dbSelect.addEventListener('change', filterUsers);
        filterUsers();

        document.getElementById('pma-open-btn')?.addEventListener('click', async () => {
            const dbId = parseInt(dbSelect.value);
            const dbName = dbSelect.selectedOptions[0]?.dataset.name || '';
            if (!userSelect.value) {
                showToast('Please select a database user first', 'error');
                return;
            }
            const userId = parseInt(userSelect.value);
            const btn = document.getElementById('pma-open-btn');
            btn.disabled = true;
            btn.innerHTML = '<div class="loading-spinner" style="width:14px;height:14px;border-width:2px;display:inline-block;vertical-align:middle;margin-right:4px;"></div> Opening...';
            try {
                const body = { database_id: dbId, db_user_id: userId };
                const data = await request('/api/pma', {
                    method: 'POST',
                    body: JSON.stringify(body),
                });
                if (data.url) {
                    window.open(data.url, '_blank');
                    showToast(`phpMyAdmin opened for ${escapeHtml(dbName)}`, 'success');
                }
            } catch (err) {
                showToast(`phpMyAdmin error: ${err.message}`, 'error');
            } finally {
                btn.disabled = false;
                btn.innerHTML = `<span class="nav-icon nav-icon-sm">${icons.pma}</span> Open phpMyAdmin`;
            }
        });
    } catch (err) {
        container.innerHTML = `<div class="empty-state"><div class="empty-state-title">Error: ${escapeHtml(err.message)}</div></div>`;
    }
}
