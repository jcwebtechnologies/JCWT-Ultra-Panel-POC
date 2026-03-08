// JCWT Ultra Panel — Firewall Management Page
import { firewall } from '../api.js';
import { icons, showToast, escapeHtml, showConfirm } from '../app.js';

export async function render(container) {
    document.getElementById('page-title').textContent = 'Firewall';
    container.innerHTML = '<div class="loading-screen"><div class="loading-spinner"></div></div>';

    try {
        const data = await firewall.list();
        const rules = data.rules || [];
        const status = data.status || 'unknown';

        container.innerHTML = `
        <div class="page-header">
            <div class="page-header-left">
                <h2>Firewall Rules</h2>
                <p>Manage UFW firewall rules for your server</p>
            </div>
            <div class="page-header-actions" style="gap:var(--space-3);">
                <button class="btn ${status === 'active' ? 'btn-danger' : 'btn-success'}" id="toggle-fw-btn">
                    ${status === 'active' ? 'Disable Firewall' : 'Enable Firewall'}
                </button>
                <button class="btn btn-primary" id="add-rule-btn">
                    <span class="nav-icon">${icons.plus}</span> Add Rule
                </button>
            </div>
        </div>

        <div class="fw-status" style="margin-bottom:var(--space-4);padding:var(--space-3) var(--space-4);border-radius:var(--radius-md);background:var(--bg-secondary);display:flex;align-items:center;gap:var(--space-3);">
            <span class="status-badge ${status === 'active' ? 'status-running' : 'status-stopped'}">${status}</span>
            <span style="color:var(--text-secondary);font-size:var(--font-size-sm);">UFW Firewall Status</span>
        </div>

        <div class="card">
            ${rules.length === 0 ? `
                <div class="empty-state" style="padding:var(--space-8) 0;">
                    <div class="empty-state-icon"><span class="nav-icon" style="width:32px;height:32px;color:var(--text-tertiary)">${icons.shield}</span></div>
                    <div class="empty-state-title">No Custom Rules</div>
                    <div class="empty-state-text">SSH (22), HTTP (80), HTTPS (443) and Panel port are allowed by default.</div>
                </div>
            ` : `
                <div class="table-container">
                    <table class="data-table">
                        <thead>
                            <tr>
                                <th>Direction</th>
                                <th>Action</th>
                                <th>Protocol</th>
                                <th>Port</th>
                                <th>Source</th>
                                <th>Comment</th>
                                <th>Status</th>
                                <th>Actions</th>
                            </tr>
                        </thead>
                        <tbody>
                            ${rules.map(r => `
                                <tr>
                                    <td><span class="badge">${escapeHtml(r.direction)}</span></td>
                                    <td><span class="status-badge ${r.action === 'allow' ? 'status-running' : 'status-stopped'}">${escapeHtml(r.action)}</span></td>
                                    <td>${escapeHtml(r.protocol)}</td>
                                    <td><code>${escapeHtml(r.port)}</code></td>
                                    <td>${r.source ? escapeHtml(r.source) : '<span style="color:var(--text-tertiary)">any</span>'}</td>
                                    <td style="color:var(--text-secondary);font-size:var(--font-size-sm);">${escapeHtml(r.comment || '')}</td>
                                    <td>
                                        <label class="toggle" style="margin:0;">
                                            <input type="checkbox" ${r.enabled ? 'checked' : ''} data-toggle-rule="${r.id}">
                                            <span class="toggle-slider"></span>
                                        </label>
                                    </td>
                                    <td>
                                        <button class="btn btn-sm btn-danger" data-delete-rule="${r.id}" title="Delete">
                                            <span class="nav-icon" style="width:14px;height:14px;">${icons.trash}</span>
                                        </button>
                                    </td>
                                </tr>
                            `).join('')}
                        </tbody>
                    </table>
                </div>
            `}
        </div>`;

        // Toggle firewall
        document.getElementById('toggle-fw-btn')?.addEventListener('click', async () => {
            const enable = status !== 'active';
            const confirmed = await showConfirm(
                enable ? 'Enable Firewall' : 'Disable Firewall',
                enable ? 'Enable the firewall? Make sure SSH (port 22) is allowed before proceeding.' : 'Disable the firewall? All traffic will be allowed.',
                enable ? 'Enable' : 'Disable',
                enable ? 'btn-success' : 'btn-danger'
            );
            if (!confirmed) return;
            try {
                await firewall.toggle(enable);
                showToast(`Firewall ${enable ? 'enabled' : 'disabled'}`, 'success');
                render(container);
            } catch (err) { showToast(err.message, 'error'); }
        });

        // Add rule
        document.getElementById('add-rule-btn')?.addEventListener('click', () => {
            showAddRuleForm(container);
        });

        // Toggle individual rules
        container.querySelectorAll('[data-toggle-rule]').forEach(cb => {
            cb.addEventListener('change', async () => {
                const id = parseInt(cb.dataset.toggleRule);
                const rule = rules.find(r => r.id === id);
                if (!rule) return;
                try {
                    await firewall.update({
                        id, direction: rule.direction, action: rule.action,
                        protocol: rule.protocol, port: rule.port,
                        source: rule.source, comment: rule.comment,
                        enabled: cb.checked,
                    });
                    showToast('Rule updated', 'success');
                } catch (err) {
                    showToast(err.message, 'error');
                    cb.checked = !cb.checked;
                }
            });
        });

        // Delete rules
        container.querySelectorAll('[data-delete-rule]').forEach(btn => {
            btn.addEventListener('click', async () => {
                const confirmed = await showConfirm('Delete Firewall Rule', 'Are you sure you want to delete this rule? This action cannot be undone.', 'Delete', 'btn-danger');
                if (!confirmed) return;
                try {
                    await firewall.delete(parseInt(btn.dataset.deleteRule));
                    showToast('Rule deleted', 'success');
                    render(container);
                } catch (err) { showToast(err.message, 'error'); }
            });
        });

    } catch (err) {
        container.innerHTML = `<div class="empty-state"><div class="empty-state-title">Error: ${escapeHtml(err.message)}</div></div>`;
    }
}

function showAddRuleForm(container) {
    const existing = document.getElementById('add-rule-card');
    if (existing) { existing.remove(); return; }

    const card = document.createElement('div');
    card.id = 'add-rule-card';
    card.className = 'card';
    card.style.marginBottom = 'var(--space-4)';
    card.innerHTML = `
        <h3 style="margin-bottom:var(--space-4);">Add Firewall Rule</h3>
        <div style="display:grid;grid-template-columns:repeat(auto-fill,minmax(180px,1fr));gap:var(--space-3);margin-bottom:var(--space-4);">
            <div>
                <label class="form-label">Direction</label>
                <select class="form-select" id="rule-direction">
                    <option value="in">Incoming</option>
                    <option value="out">Outgoing</option>
                </select>
            </div>
            <div>
                <label class="form-label">Action</label>
                <select class="form-select" id="rule-action">
                    <option value="allow">Allow</option>
                    <option value="deny">Deny</option>
                    <option value="reject">Reject</option>
                </select>
            </div>
            <div>
                <label class="form-label">Protocol</label>
                <select class="form-select" id="rule-protocol">
                    <option value="tcp">TCP</option>
                    <option value="udp">UDP</option>
                    <option value="any">Any</option>
                </select>
            </div>
            <div>
                <label class="form-label">Port</label>
                <input type="text" class="form-input" id="rule-port" placeholder="e.g. 8080 or 3000:3100" inputmode="numeric" pattern="[0-9:\\-]+">
            </div>
            <div>
                <label class="form-label">Source (optional)</label>
                <input type="text" class="form-input" id="rule-source" placeholder="e.g. 192.168.1.0/24">
            </div>
            <div>
                <label class="form-label">Comment</label>
                <input type="text" class="form-input" id="rule-comment" placeholder="Description">
            </div>
        </div>
        <div style="display:flex;gap:var(--space-2);justify-content:flex-end;">
            <button class="btn btn-secondary" id="cancel-rule-btn">Cancel</button>
            <button class="btn btn-primary" id="save-rule-btn">Add Rule</button>
        </div>
    `;

    // Insert after the status bar
    const statusBar = container.querySelector('.fw-status');
    if (statusBar) {
        statusBar.after(card);
    } else {
        container.querySelector('.card')?.before(card);
    }

    document.getElementById('cancel-rule-btn')?.addEventListener('click', () => card.remove());
    document.getElementById('save-rule-btn')?.addEventListener('click', async () => {
        const port = document.getElementById('rule-port').value.trim();
        if (!port) {
            showToast('Port is required', 'error');
            return;
        }
        // Validate port: must be a number (1-65535) or a range like 3000:3100
        if (!/^\d+([:\-]\d+)?$/.test(port)) {
            showToast('Port must be a number (e.g. 8080) or range (e.g. 3000:3100)', 'error');
            return;
        }
        const parts = port.split(/[:\-]/);
        for (const p of parts) {
            const n = parseInt(p, 10);
            if (n < 1 || n > 65535) {
                showToast('Port must be between 1 and 65535', 'error');
                return;
            }
        }
        try {
            await firewall.create({
                direction: document.getElementById('rule-direction').value,
                action: document.getElementById('rule-action').value,
                protocol: document.getElementById('rule-protocol').value,
                port,
                source: document.getElementById('rule-source').value.trim(),
                comment: document.getElementById('rule-comment').value.trim(),
            });
            showToast('Firewall rule added', 'success');
            render(container);
        } catch (err) { showToast(err.message, 'error'); }
    });
}
