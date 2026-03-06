// JCWT Ultra Panel — Databases Page
import { databases, dbUsers, sites } from '../api.js';
import { icons, showToast, showModal, closeModal, escapeHtml, showConfirm } from '../app.js';

export async function render(container) {
    document.getElementById('page-title').textContent = 'Databases';
    container.innerHTML = '<div class="loading-screen"><div class="loading-spinner"></div></div>';

    let activeTab = 'databases';

    async function renderContent() {
        try {
            const [dbList, userList, siteList] = await Promise.all([
                databases.list(), dbUsers.list(), sites.list()
            ]);

            container.innerHTML = `
            <div class="page-header">
                <div class="page-header-left">
                    <h2>Database Management</h2>
                    <p>Manage MariaDB databases and users</p>
                </div>
            </div>

            <div class="tabs">
                <button class="tab ${activeTab === 'databases' ? 'active' : ''}" data-tab="databases">Databases (${dbList.length})</button>
                <button class="tab ${activeTab === 'users' ? 'active' : ''}" data-tab="users">Users (${userList.length})</button>
            </div>

            <div id="db-tab-content"></div>`;

            container.querySelectorAll('.tab').forEach(tab => {
                tab.addEventListener('click', () => {
                    activeTab = tab.dataset.tab;
                    renderContent();
                });
            });

            const tabContent = document.getElementById('db-tab-content');

            if (activeTab === 'databases') {
                tabContent.innerHTML = `
                <div style="margin-bottom: var(--space-4);">
                    <button class="btn btn-primary" id="add-db-btn">${icons.plus} Create Database</button>
                </div>
                ${dbList.length === 0 ? `
                    <div class="empty-state">
                        <div class="empty-state-icon">🗄️</div>
                        <div class="empty-state-title">No databases</div>
                        <div class="empty-state-text">Create your first MariaDB database.</div>
                    </div>
                ` : `
                    <div class="table-container">
                        <table class="data-table">
                            <thead><tr><th>Database Name</th><th>Linked Site</th><th>Created</th><th>Actions</th></tr></thead>
                            <tbody>
                                ${dbList.map(d => `
                                <tr>
                                    <td class="mono"><strong>${escapeHtml(d.db_name)}</strong></td>
                                    <td>${d.site_domain ? escapeHtml(d.site_domain) : '<span style="color:var(--text-tertiary)">—</span>'}</td>
                                    <td style="color:var(--text-tertiary);font-size:var(--font-size-xs);">${new Date(d.created_at).toLocaleDateString()}</td>
                                    <td><button class="btn btn-sm btn-danger del-db" data-id="${d.id}" data-name="${escapeHtml(d.db_name)}">Delete</button></td>
                                </tr>`).join('')}
                            </tbody>
                        </table>
                    </div>
                `}`;

                document.getElementById('add-db-btn')?.addEventListener('click', () => {
                    const siteOpts = [`<option value="">— Select a site —</option>`].concat(
                        siteList.map(s => `<option value="${s.id}">${escapeHtml(s.domain)}</option>`)
                    ).join('');

                    showModal('Create Database', `
                        <div class="form-group">
                            <label class="form-label">Database Name</label>
                            <input type="text" class="form-input" id="db-name" placeholder="myapp_db" required pattern="^[a-zA-Z][a-zA-Z0-9_]*$" maxlength="64">
                            <small style="color: var(--text-tertiary); font-size: var(--font-size-xs);">Letters, numbers, underscore only. Must start with a letter.</small>
                        </div>
                        <div class="form-group">
                            <label class="form-label">Link to Site <span style="color: var(--status-error);">*</span></label>
                            <select class="form-select" id="db-site" required>${siteOpts}</select>
                            <small style="color: var(--text-tertiary); font-size: var(--font-size-xs);">Databases must be linked to a site.</small>
                        </div>
                    `, `
                        <button class="btn btn-secondary" onclick="document.getElementById('modal-overlay').remove()">Cancel</button>
                        <button class="btn btn-primary" id="submit-db">Create</button>
                    `);

                    document.getElementById('submit-db')?.addEventListener('click', async () => {
                        const dbName = document.getElementById('db-name').value.trim();
                        const siteId = document.getElementById('db-site').value;
                        if (!dbName) { showToast('Name required', 'error'); return; }
                        if (!/^[a-zA-Z][a-zA-Z0-9_]*$/.test(dbName)) {
                            showToast('Invalid name: use letters, numbers, underscore only (start with a letter)', 'error');
                            return;
                        }
                        if (dbName.length > 64) { showToast('Name too long (max 64 characters)', 'error'); return; }
                        if (!siteId) { showToast('Please select a site to link this database to', 'error'); return; }
                        try {
                            await databases.create({ db_name: dbName, site_id: parseInt(siteId) });
                            closeModal(); showToast('Database created!', 'success');
                            renderContent();
                        } catch (err) { showToast(err.message, 'error'); }
                    });
                });

                tabContent.querySelectorAll('.del-db').forEach(btn => {
                    btn.addEventListener('click', async () => {
                        if (!await showConfirm('Delete Database', `Delete database "${btn.dataset.name}"? This is irreversible and will drop the database from MariaDB.`, 'Delete', 'btn-danger')) return;
                        try {
                            await databases.delete(btn.dataset.id);
                            showToast('Database deleted', 'success');
                            renderContent();
                        } catch (err) { showToast(err.message, 'error'); }
                    });
                });

            } else {
                // Users tab
                tabContent.innerHTML = `
                <div style="margin-bottom: var(--space-4);">
                    <button class="btn btn-primary" id="add-dbuser-btn">${icons.plus} Create User</button>
                </div>
                ${userList.length === 0 ? `
                    <div class="empty-state">
                        <div class="empty-state-icon">🔑</div>
                        <div class="empty-state-title">No database users</div>
                        <div class="empty-state-text">Create a user and assign them to a database.</div>
                    </div>
                ` : `
                    <div class="table-container">
                        <table class="data-table">
                            <thead><tr><th>Username</th><th>Database</th><th>Created</th><th>Actions</th></tr></thead>
                            <tbody>
                                ${userList.map(u => `
                                <tr>
                                    <td class="mono"><strong>${escapeHtml(u.username)}</strong></td>
                                    <td class="mono">${escapeHtml(u.db_name)}</td>
                                    <td style="color:var(--text-tertiary);font-size:var(--font-size-xs);">${new Date(u.created_at).toLocaleDateString()}</td>
                                    <td>
                                        <div class="table-actions">
                                            <button class="btn btn-sm btn-secondary chg-pwd" data-user="${escapeHtml(u.username)}">Password</button>
                                            <button class="btn btn-sm btn-danger del-user" data-id="${u.id}" data-user="${escapeHtml(u.username)}">Delete</button>
                                        </div>
                                    </td>
                                </tr>`).join('')}
                            </tbody>
                        </table>
                    </div>
                `}`;

                document.getElementById('add-dbuser-btn')?.addEventListener('click', () => {
                    const dbOpts = dbList.map(d => `<option value="${d.id}">${escapeHtml(d.db_name)}</option>`).join('');
                    if (!dbOpts) { showToast('Create a database first', 'warning'); return; }

                    showModal('Create Database User', `
                        <div class="form-group">
                            <label class="form-label">Username</label>
                            <input type="text" class="form-input" id="dbuser-name" required pattern="^[a-zA-Z][a-zA-Z0-9_]*$" maxlength="32">
                            <small style="color: var(--text-tertiary); font-size: var(--font-size-xs);">Letters, numbers, underscore only. Must start with a letter.</small>
                        </div>
                        <div class="form-group">
                            <label class="form-label">Password</label>
                            <input type="password" class="form-input" id="dbuser-pwd" required minlength="8">
                            <small style="color: var(--text-tertiary); font-size: var(--font-size-xs);">Minimum 8 characters</small>
                        </div>
                        <div class="form-group">
                            <label class="form-label">Grant Access To</label>
                            <select class="form-select" id="dbuser-db">${dbOpts}</select>
                        </div>
                    `, `
                        <button class="btn btn-secondary" onclick="document.getElementById('modal-overlay').remove()">Cancel</button>
                        <button class="btn btn-primary" id="submit-dbuser">Create</button>
                    `);

                    document.getElementById('submit-dbuser')?.addEventListener('click', async () => {
                        const username = document.getElementById('dbuser-name').value.trim();
                        const password = document.getElementById('dbuser-pwd').value;
                        if (!username || !password) { showToast('Username and password are required', 'error'); return; }
                        if (!/^[a-zA-Z][a-zA-Z0-9_]*$/.test(username)) {
                            showToast('Invalid username: use letters, numbers, underscore only (start with a letter)', 'error');
                            return;
                        }
                        if (username.length > 32) { showToast('Username too long (max 32 characters)', 'error'); return; }
                        if (password.length < 8) { showToast('Password must be at least 8 characters', 'error'); return; }
                        try {
                            await dbUsers.create({
                                username: username,
                                password: password,
                                database_id: parseInt(document.getElementById('dbuser-db').value),
                            });
                            closeModal(); showToast('User created!', 'success');
                            renderContent();
                        } catch (err) { showToast(err.message, 'error'); }
                    });
                });

                tabContent.querySelectorAll('.chg-pwd').forEach(btn => {
                    btn.addEventListener('click', () => {
                        showModal('Change Password', `
                            <div class="form-group">
                                <label class="form-label">New Password for <strong>${btn.dataset.user}</strong></label>
                                <input type="password" class="form-input" id="new-pwd" required>
                            </div>
                        `, `
                            <button class="btn btn-secondary" onclick="document.getElementById('modal-overlay').remove()">Cancel</button>
                            <button class="btn btn-primary" id="submit-pwd">Change</button>
                        `);

                        document.getElementById('submit-pwd')?.addEventListener('click', async () => {
                            const newPwd = document.getElementById('new-pwd').value;
                            if (!newPwd || newPwd.length < 8) {
                                showToast('Password must be at least 8 characters', 'error');
                                return;
                            }
                            try {
                                await dbUsers.changePassword({
                                    username: btn.dataset.user,
                                    new_password: newPwd,
                                });
                                closeModal(); showToast('Password changed!', 'success');
                            } catch (err) { showToast(err.message, 'error'); }
                        });
                    });
                });

                tabContent.querySelectorAll('.del-user').forEach(btn => {
                    btn.addEventListener('click', async () => {
                        if (!await showConfirm('Delete User', `Delete database user "${btn.dataset.user}"? This cannot be undone.`, 'Delete', 'btn-danger')) return;
                        try {
                            await dbUsers.delete(btn.dataset.id);
                            showToast('User deleted', 'success');
                            renderContent();
                        } catch (err) { showToast(err.message, 'error'); }
                    });
                });
            }

        } catch (err) {
            container.innerHTML = `<div class="empty-state"><div class="empty-state-title">Error: ${err.message}</div></div>`;
        }
    }

    renderContent();
}
