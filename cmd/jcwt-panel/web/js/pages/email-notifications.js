// JCWT Ultra Panel — Email Notifications Settings Page
import { emailTemplates } from '../api.js';
import { icons, showToast, showModal, closeModal, escapeHtml } from '../app.js';

export async function render(container) {
    document.getElementById('page-title').textContent = 'Email Notifications';
    container.innerHTML = '<div class="loading-screen"><div class="loading-spinner"></div></div>';

    try {
        const templates = await emailTemplates.list();
        const list = Array.isArray(templates) ? templates : [];

        container.innerHTML = `
        <div class="page-header">
            <div class="page-header-left">
                <h2>Email Notifications</h2>
                <p>Manage email templates sent for system events</p>
            </div>
        </div>

        ${list.length === 0 ? `
            <div class="empty-state">
                <div class="empty-state-icon"><span class="nav-icon" style="width:48px;height:48px;color:var(--text-tertiary)">${icons.mail}</span></div>
                <div class="empty-state-title">No Email Templates</div>
                <div class="empty-state-text">Email notification templates will appear here once configured.</div>
            </div>
        ` : `
            <div class="card">
                <div class="table-wrapper">
                    <table class="data-table">
                        <thead>
                            <tr>
                                <th>Template</th>
                                <th>Subject</th>
                                <th>Status</th>
                                <th style="width:100px;text-align:right;">Actions</th>
                            </tr>
                        </thead>
                        <tbody>
                            ${list.map(t => `
                            <tr>
                                <td>
                                    <div style="font-weight:600;">${escapeHtml(t.name)}</div>
                                    <div style="font-size:var(--font-size-xs);color:var(--text-tertiary);margin-top:2px;">${escapeHtml(t.description)}</div>
                                </td>
                                <td><code style="font-size:var(--font-size-xs);">${escapeHtml(t.subject)}</code></td>
                                <td>
                                    <span class="badge ${t.enabled ? 'badge-success' : 'badge-warning'}">${t.enabled ? 'Enabled' : 'Disabled'}</span>
                                </td>
                                <td style="text-align:right;">
                                    <button class="btn btn-sm btn-secondary edit-template-btn" data-id="${t.id}" title="Edit template">
                                        <span class="nav-icon" style="width:14px;height:14px">${icons.edit}</span>
                                    </button>
                                </td>
                            </tr>
                            `).join('')}
                        </tbody>
                    </table>
                </div>
            </div>
        `}`;

        // Bind edit buttons
        container.querySelectorAll('.edit-template-btn').forEach(btn => {
            btn.addEventListener('click', async () => {
                const id = parseInt(btn.dataset.id);
                await showEditModal(id, () => render(container));
            });
        });

    } catch (err) {
        container.innerHTML = `<div class="empty-state"><div class="empty-state-title">Error: ${escapeHtml(err.message)}</div></div>`;
    }
}

async function showEditModal(templateId, onSave) {
    try {
        const tmpl = await emailTemplates.get(templateId);

        const content = `
            <form id="edit-template-form">
                <div class="form-group">
                    <label class="form-label">Template</label>
                    <input type="text" class="form-input" value="${escapeHtml(tmpl.name)}" disabled style="color:var(--text-tertiary);">
                </div>
                <div class="form-group">
                    <label class="form-label">Subject</label>
                    <input type="text" class="form-input" id="et-subject" value="${escapeHtml(tmpl.subject)}">
                    <small style="color:var(--text-tertiary);font-size:var(--font-size-xs);">Use template variables: <code>{{.Domain}}</code>, <code>{{.SystemUser}}</code>, <code>{{.WPAdminUser}}</code>, etc.</small>
                </div>
                <div class="form-group">
                    <label class="form-label">Body Content (HTML)</label>
                    <textarea class="form-input mono" id="et-body" rows="14" style="font-size:var(--font-size-xs);line-height:1.5;">${escapeHtml(tmpl.body_content)}</textarea>
                    <small style="color:var(--text-tertiary);font-size:var(--font-size-xs);">HTML content inside the email body. A common header and footer are added automatically.</small>
                </div>
                <div class="form-group" style="display:flex;align-items:center;gap:var(--space-2);">
                    <label class="toggle-switch">
                        <input type="checkbox" id="et-enabled" ${tmpl.enabled ? 'checked' : ''}>
                        <span class="toggle-slider"></span>
                    </label>
                    <span style="font-size:var(--font-size-sm);">Enabled</span>
                </div>
            </form>
        `;
        const footer = `
            <button class="btn btn-secondary" onclick="document.getElementById('modal-overlay').remove()">Cancel</button>
            <button class="btn btn-primary" id="et-save-btn">Save Template</button>
        `;
        const modal = showModal('Edit Email Template', content, footer);

        modal.querySelector('#et-save-btn')?.addEventListener('click', async () => {
            const subject = modal.querySelector('#et-subject').value.trim();
            const bodyContent = modal.querySelector('#et-body').value;
            const enabled = modal.querySelector('#et-enabled').checked;

            if (!subject) {
                showToast('Subject is required', 'error');
                return;
            }

            const btn = modal.querySelector('#et-save-btn');
            btn.disabled = true;
            btn.textContent = 'Saving...';

            try {
                await emailTemplates.update({
                    id: templateId,
                    subject,
                    body_content: bodyContent,
                    enabled
                });
                closeModal();
                showToast('Template updated', 'success');
                onSave();
            } catch (err) {
                showToast(err.message || 'Failed to update template', 'error');
                btn.disabled = false;
                btn.textContent = 'Save Template';
            }
        });
    } catch (err) {
        showToast('Failed to load template: ' + err.message, 'error');
    }
}
