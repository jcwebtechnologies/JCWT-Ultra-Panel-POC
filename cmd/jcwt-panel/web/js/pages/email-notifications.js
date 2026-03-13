// JCWT Ultra Panel — Email Notifications Settings Page
import { emailTemplates } from '../api.js';
import { icons, showToast, showModal, closeModal, escapeHtml } from '../app.js';

const DEFAULT_HEADER_HTML = `<td style="background:#6366f1;padding:24px 32px;border-radius:8px 8px 0 0;text-align:center;"><h1 style="margin:0;color:#ffffff;font-size:22px;font-weight:700;">JCWT Ultra Panel</h1></td>`;
const DEFAULT_FOOTER_HTML = `<td style="background:#f9fafb;padding:20px 32px;border:1px solid #e5e7eb;border-top:none;border-radius:0 0 8px 8px;text-align:center;"><p style="margin:0;color:#9ca3af;font-size:12px;">&copy; ${new Date().getFullYear()} JCWT Ultra Panel &mdash; This is an automated message.</p></td>`;

export async function render(container) {
    document.getElementById('page-title').textContent = 'Email Notifications';
    container.innerHTML = '<div class="loading-screen"><div class="loading-spinner"></div></div>';

    try {
        const [templates, layout] = await Promise.all([
            emailTemplates.list(),
            emailTemplates.getLayout()
        ]);
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
                    <table class="data-table responsive-cards">
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
                                <td data-label="Template">
                                    <div style="font-weight:600;">${escapeHtml(t.name)}</div>
                                    <div style="font-size:var(--font-size-xs);color:var(--text-tertiary);margin-top:2px;">${escapeHtml(t.description)}</div>
                                </td>
                                <td data-label="Subject"><code style="font-size:var(--font-size-xs);">${escapeHtml(t.subject)}</code></td>
                                <td data-label="Status">
                                    <span class="badge ${t.enabled ? 'badge-success' : 'badge-warning'}">${t.enabled ? 'Enabled' : 'Disabled'}</span>
                                </td>
                                <td data-label="Actions" style="text-align:right;">
                                    <button class="btn btn-sm btn-secondary edit-template-btn" data-id="${t.id}" title="Edit template">
                                        <span class="nav-icon nav-icon-xs">${icons.edit}</span>
                                    </button>
                                </td>
                            </tr>
                            `).join('')}
                        </tbody>
                    </table>
                </div>
            </div>
        `}

        <div style="margin-top:var(--space-6);">
            <h3 style="margin-bottom:var(--space-3);">Email Header & Footer</h3>
            <p style="color:var(--text-secondary);margin-bottom:var(--space-4);font-size:var(--font-size-sm);">Custom HTML for the header and footer that wraps all email templates. Leave empty to use the default layout.</p>
            <div class="card" style="padding:var(--space-5);">
                <form id="layout-form">
                    <div class="form-group">
                        <label class="form-label">Header HTML</label>
                        <textarea class="form-input mono" id="layout-header" rows="8" style="font-size:var(--font-size-xs);line-height:1.5;" placeholder="Leave empty for default header (panel name on purple bar)">${escapeHtml(layout.email_header_html || DEFAULT_HEADER_HTML)}</textarea>
                        <small style="color:var(--text-tertiary);font-size:var(--font-size-xs);">Inline-styled HTML that appears above the email body inside the email table layout.</small>
                    </div>
                    <div class="form-group">
                        <label class="form-label">Footer HTML</label>
                        <textarea class="form-input mono" id="layout-footer" rows="6" style="font-size:var(--font-size-xs);line-height:1.5;" placeholder="Leave empty for default footer (© year panel name)">${escapeHtml(layout.email_footer_html || DEFAULT_FOOTER_HTML)}</textarea>
                        <small style="color:var(--text-tertiary);font-size:var(--font-size-xs);">Inline-styled HTML that appears below the email body. Use inline styles only — email clients strip &lt;style&gt; blocks.</small>
                    </div>
                    <div style="display:flex;gap:var(--space-2);align-items:center;">
                        <button type="submit" class="btn btn-primary" id="save-layout-btn">Save Layout</button>
                        <button type="button" class="btn btn-secondary" id="reset-layout-btn">Reset to Default</button>
                    </div>
                </form>
            </div>
        </div>`;

        // Bind edit buttons
        container.querySelectorAll('.edit-template-btn').forEach(btn => {
            btn.addEventListener('click', async () => {
                const id = parseInt(btn.dataset.id);
                await showEditModal(id, () => render(container));
            });
        });

        // Layout form
        container.querySelector('#layout-form')?.addEventListener('submit', async (e) => {
            e.preventDefault();
            const btn = container.querySelector('#save-layout-btn');
            const headerHTML = container.querySelector('#layout-header').value;
            const footerHTML = container.querySelector('#layout-footer').value;
            btn.disabled = true;
            btn.textContent = 'Saving...';
            try {
                await emailTemplates.updateLayout({ email_header_html: headerHTML, email_footer_html: footerHTML });
                showToast('Email layout updated', 'success');
            } catch (err) {
                showToast(err.message || 'Failed to update layout', 'error');
            }
            btn.disabled = false;
            btn.textContent = 'Save Layout';
        });

        // Reset to Default button
        container.querySelector('#reset-layout-btn')?.addEventListener('click', () => {
            container.querySelector('#layout-header').value = DEFAULT_HEADER_HTML;
            container.querySelector('#layout-footer').value = DEFAULT_FOOTER_HTML;
            showToast('Defaults restored — click Save Layout to apply', 'info');
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
                    <small style="color:var(--text-tertiary);font-size:var(--font-size-xs);">HTML content inside the email body. The header and footer configured below are added automatically.</small>
                </div>
                <div class="form-group" style="display:flex;align-items:center;gap:var(--space-2);">
                    <label class="toggle">
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
