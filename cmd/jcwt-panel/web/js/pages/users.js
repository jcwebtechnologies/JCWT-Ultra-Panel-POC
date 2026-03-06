// JCWT Ultra Panel — Users Management Page (Admin Only)
import { request } from '../api.js';
import { icons, showToast, showModal, closeModal, escapeHtml, showConfirm } from '../app.js';

export async function render(container) {
    document.getElementById('page-title').textContent = 'User Management';
    container.innerHTML = '<div class="loading-screen"><div class="loading-spinner"></div></div>';

    try {
        const users = await request('/api/users');

        container.innerHTML = `
        <div class="page-header">
            <div class="page-header-left">
                <h2>User Management</h2>
                <p>Manage panel users and their roles</p>
            </div>
            <button class="btn btn-primary" id="add-user-btn">${icons.plus} Add User</button>
        </div>

        <div class="card">
            <div class="card-header">
                <h3 class="card-title">Panel Users</h3>
            </div>
            <div class="table-responsive">
                <table class="data-table">
                    <thead>
                        <tr>
                            <th>Username</th>
                            <th>Email</th>
                            <th>Role</th>
                            <th>Created</th>
                            <th>Actions</th>
                        </tr>
                    </thead>
                    <tbody>
                        ${(users || []).map(u => `
                        <tr>
                            <td><span class="mono" style="font-weight: 600;">${escapeHtml(u.username)}</span></td>
                            <td>${u.email ? escapeHtml(u.email) : '<span style="color: var(--text-tertiary);">—</span>'}</td>
                            <td>
                                <span class="badge ${u.role === 'admin' ? 'badge-primary' : u.role === 'manager' ? 'badge-info' : 'badge-default'}">
                                    ${u.role === 'admin' ? '👑' : u.role === 'manager' ? '🔧' : '👁️'} ${u.role}
                                </span>
                            </td>
                            <td>${u.created_at ? new Date(u.created_at).toLocaleDateString() : 'N/A'}</td>
                            <td style="display: flex; gap: var(--space-1);">
                                <button class="btn btn-sm btn-secondary edit-user" data-id="${u.id}" data-username="${escapeHtml(u.username)}" data-role="${u.role}" data-email="${escapeHtml(u.email || '')}">Edit</button>
                                <button class="btn btn-sm btn-danger delete-user" data-id="${u.id}" data-username="${escapeHtml(u.username)}">Delete</button>
                            </td>
                        </tr>`).join('')}
                    </tbody>
                </table>
            </div>
        </div>

        <div class="card" style="margin-top: var(--space-4);">
            <div class="card-header"><h3 class="card-title">Role Permissions</h3></div>
            <div style="padding: var(--space-4);">
                <table class="data-table">
                    <thead>
                        <tr><th>Permission</th><th>👑 Admin</th><th>🔧 Manager</th><th>👁️ Viewer</th></tr>
                    </thead>
                    <tbody>
                        <tr><td>View Dashboard</td><td>✅</td><td>✅</td><td>✅</td></tr>
                        <tr><td>Manage Sites</td><td>✅</td><td>✅</td><td>❌</td></tr>
                        <tr><td>Manage Databases</td><td>✅</td><td>✅</td><td>❌</td></tr>
                        <tr><td>View Services</td><td>✅</td><td>✅</td><td>✅</td></tr>
                        <tr><td>Manage Services</td><td>✅</td><td>❌</td><td>❌</td></tr>
                        <tr><td>Panel Settings</td><td>✅</td><td>❌</td><td>❌</td></tr>
                        <tr><td>User Management</td><td>✅</td><td>❌</td><td>❌</td></tr>
                    </tbody>
                </table>
            </div>
        </div>`;

        // Add User button
        document.getElementById('add-user-btn')?.addEventListener('click', () => {
            showModal('Add User', `
                <div class="form-group">
                    <label class="form-label">Username <span style="color: var(--status-error);">*</span></label>
                    <input type="text" class="form-input" id="new-username" placeholder="johndoe" required pattern="^[a-zA-Z][a-zA-Z0-9_]{2,30}$" maxlength="31">
                    <small style="color: var(--text-tertiary);">3-31 chars, start with letter, letters/numbers/underscore only</small>
                </div>
                <div class="form-group">
                    <label class="form-label">Email</label>
                    <input type="email" class="form-input" id="new-email" placeholder="john@example.com">
                </div>
                <div class="form-group">
                    <label class="form-label">Password <span style="color: var(--status-error);">*</span></label>
                    <input type="password" class="form-input" id="new-password" placeholder="Min 8 characters" minlength="8">
                </div>
                <div class="form-group">
                    <label class="form-label">Role <span style="color: var(--status-error);">*</span></label>
                    <select class="form-select" id="new-role">
                        <option value="viewer">👁️ Viewer — read-only access</option>
                        <option value="manager">🔧 Manager — manage sites & databases</option>
                        <option value="admin">👑 Admin — full access</option>
                    </select>
                </div>
            `, `
                <button class="btn btn-secondary" onclick="document.getElementById('modal-overlay').remove()">Cancel</button>
                <button class="btn btn-primary" id="confirm-add-user">Create User</button>
            `);

            document.getElementById('confirm-add-user')?.addEventListener('click', async () => {
                const username = document.getElementById('new-username').value.trim();
                const email = document.getElementById('new-email').value.trim();
                const password = document.getElementById('new-password').value;
                const role = document.getElementById('new-role').value;

                if (!username) { showToast('Username is required', 'error'); return; }
                if (!/^[a-zA-Z][a-zA-Z0-9_]{2,30}$/.test(username)) {
                    showToast('Invalid username format', 'error'); return;
                }
                if (!password || password.length < 8) {
                    showToast('Password must be at least 8 characters', 'error'); return;
                }

                try {
                    await request('/api/users', {
                        method: 'POST',
                        body: JSON.stringify({ username, password, role, email }),
                    });
                    closeModal();
                    showToast(`User "${username}" created!`, 'success');
                    render(container);
                } catch (err) { showToast(err.message, 'error'); }
            });
        });

        // Edit user buttons
        container.querySelectorAll('.edit-user').forEach(btn => {
            btn.addEventListener('click', () => {
                const id = parseInt(btn.dataset.id);
                const username = btn.dataset.username;
                const role = btn.dataset.role;
                const email = btn.dataset.email;

                showModal(`Edit User: ${username}`, `
                    <div class="form-group">
                        <label class="form-label">Email</label>
                        <input type="email" class="form-input" id="edit-email" value="${escapeHtml(email)}" placeholder="john@example.com">
                    </div>
                    <div class="form-group">
                        <label class="form-label">Role</label>
                        <select class="form-select" id="edit-role">
                            <option value="viewer" ${role === 'viewer' ? 'selected' : ''}>👁️ Viewer</option>
                            <option value="manager" ${role === 'manager' ? 'selected' : ''}>🔧 Manager</option>
                            <option value="admin" ${role === 'admin' ? 'selected' : ''}>👑 Admin</option>
                        </select>
                    </div>
                    <div class="form-group">
                        <label class="form-label">New Password <small style="color: var(--text-tertiary);">(leave blank to keep current)</small></label>
                        <input type="password" class="form-input" id="edit-password" placeholder="Min 8 characters">
                    </div>
                `, `
                    <button class="btn btn-secondary" onclick="document.getElementById('modal-overlay').remove()">Cancel</button>
                    <button class="btn btn-primary" id="confirm-edit-user">Save Changes</button>
                `);

                document.getElementById('confirm-edit-user')?.addEventListener('click', async () => {
                    const newEmail = document.getElementById('edit-email').value.trim();
                    const newRole = document.getElementById('edit-role').value;
                    const newPassword = document.getElementById('edit-password').value;

                    if (newPassword && newPassword.length < 8) {
                        showToast('Password must be at least 8 characters', 'error'); return;
                    }

                    try {
                        await request('/api/users', {
                            method: 'PUT',
                            body: JSON.stringify({ id, role: newRole, email: newEmail, password: newPassword || undefined }),
                        });
                        closeModal();
                        showToast(`User "${username}" updated!`, 'success');
                        render(container);
                    } catch (err) { showToast(err.message, 'error'); }
                });
            });
        });

        // Delete user buttons
        container.querySelectorAll('.delete-user').forEach(btn => {
            btn.addEventListener('click', async () => {
                const id = btn.dataset.id;
                const username = btn.dataset.username;
                if (!await showConfirm('Delete User', `Delete user "${username}"? This cannot be undone.`, 'Delete', 'btn-danger')) return;

                try {
                    await request(`/api/users?id=${id}`, { method: 'DELETE' });
                    showToast(`User "${username}" deleted`, 'success');
                    render(container);
                } catch (err) { showToast(err.message, 'error'); }
            });
        });

    } catch (err) {
        if (err.message.includes('insufficient permissions')) {
            container.innerHTML = `
            <div class="empty-state">
                <div class="empty-state-title">🔒 Access Denied</div>
                <div class="empty-state-text">Only administrators can manage users.</div>
            </div>`;
        } else {
            container.innerHTML = `<div class="empty-state"><div class="empty-state-title">Error: ${escapeHtml(err.message)}</div></div>`;
        }
    }
}
