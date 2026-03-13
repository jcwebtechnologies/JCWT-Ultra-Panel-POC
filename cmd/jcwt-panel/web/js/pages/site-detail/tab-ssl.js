import { ssl } from '../../api.js';
import { icons, showToast, showModal, closeModal, escapeHtml, showConfirm } from '../../app.js';
import { request } from '../../api.js';

export function renderSSL(el, site, siteId) {
    el.innerHTML = '<div class="loading-screen"><div class="loading-spinner"></div></div>';

    request(`/api/ssl-certs?site_id=${siteId}`).then(data => {
        const certs = data.certificates || [];
        const activeCert = certs.find(c => c.active);
        const hasSelfSigned = certs.some(c => c.type === 'self-signed');

        el.innerHTML = `
        <div class="card" style="margin-bottom: var(--space-4);">
            <div class="card-header">
                <h3 class="card-title">SSL Certificates</h3>
            </div>
            <div style="padding: var(--space-4);">
                <div class="info-item" style="margin-bottom: var(--space-4);">
                    <span class="info-label">Active Certificate</span>
                    <span class="info-value"><span class="badge ${site.ssl_type === 'none' ? 'badge-warning' : 'badge-success'}">${site.ssl_type === 'none' ? 'None' : site.ssl_type}</span></span>
                </div>
                ${activeCert && activeCert.cert_path ? `<div class="info-item" style="margin-bottom: var(--space-4);"><span class="info-label">Certificate Path</span><span class="info-value mono" style="font-size: var(--font-size-xs);">${escapeHtml(activeCert.cert_path)}</span></div>` : ''}
                ${activeCert && activeCert.common_name ? `<div class="info-item" style="margin-bottom: var(--space-4);"><span class="info-label">Common Name</span><span class="info-value mono" style="font-size: var(--font-size-xs);">${escapeHtml(activeCert.common_name)}</span></div>` : ''}
                ${activeCert && activeCert.san && activeCert.san.length ? `<div class="info-item" style="margin-bottom: var(--space-4);"><span class="info-label">Subject Alt Names</span><span class="info-value mono" style="font-size: var(--font-size-xs);">${escapeHtml(activeCert.san.join(', '))}</span></div>` : ''}
                ${activeCert && activeCert.not_after ? `<div class="info-item" style="margin-bottom: var(--space-4);"><span class="info-label">Expires</span><span class="info-value">${new Date(activeCert.not_after).toLocaleDateString()}${(() => { const d = Math.ceil((new Date(activeCert.not_after) - Date.now()) / 86400000); return d < 0 ? ' <span class="badge badge-danger">Expired</span>' : d < 30 ? ` <span class="badge badge-warning">${d}d left</span>` : ` <span class="badge badge-success">${d}d left</span>`; })()}</span></div>` : ''}

                <div style="display: flex; gap: var(--space-3); flex-wrap: wrap; margin-bottom: var(--space-4);">
                    ${!hasSelfSigned ? `<button class="btn btn-primary" id="ssl-self-signed">${icons.lock} Generate Self-Signed</button>` : ''}
                    <button class="btn btn-secondary" id="ssl-custom">${icons.upload} Upload Certificate</button>
                    <button class="btn btn-success" id="ssl-letsencrypt"><span class="nav-icon nav-icon-sm">${icons.shield}</span> Issue Let's Encrypt</button>
                </div>
            </div>
        </div>

        ${certs.length > 0 ? `
        <div class="card">
            <div class="card-header">
                <h3 class="card-title">Uploaded Certificates</h3>
            </div>
            <div class="table-responsive">
                <table class="data-table responsive-cards">
                    <thead><tr><th>Type</th><th>Label / CN</th><th>SAN</th><th>Expires</th><th>Status</th><th>Actions</th></tr></thead>
                    <tbody>
                        ${certs.map(c => {
                            const san = (c.san || []).join(', ') || '—';
                            const expiry = c.not_after ? new Date(c.not_after) : null;
                            const daysLeft = expiry ? Math.ceil((expiry - Date.now()) / 86400000) : null;
                            const expiryBadge = daysLeft !== null
                                ? (daysLeft < 0 ? '<span class="badge badge-danger">Expired</span>'
                                    : daysLeft < 30 ? `<span class="badge badge-warning">${daysLeft}d left</span>`
                                    : `<span class="badge badge-success">${daysLeft}d left</span>`)
                                : 'N/A';
                            const expiryDate = expiry ? expiry.toLocaleDateString() : '';
                            return `
                        <tr>
                            <td data-label="Type"><span class="badge ${c.type === 'self-signed' ? 'badge-warning' : c.type === 'letsencrypt' ? 'badge-success' : 'badge-info'}">${escapeHtml(c.type)}</span></td>
                            <td data-label="Label / CN">${escapeHtml(c.label || c.type)}${c.common_name ? `<br><small class="mono" style="color:var(--text-tertiary)">${escapeHtml(c.common_name)}</small>` : ''}</td>
                            <td data-label="SAN"><small class="mono" style="word-break:break-all;color:var(--text-secondary)">${escapeHtml(san)}</small></td>
                            <td data-label="Expires">${expiryBadge}${expiryDate ? `<br><small style="color:var(--text-tertiary)">${expiryDate}</small>` : ''}</td>
                            <td data-label="Status">${c.active ? '<span class="badge badge-success">Active</span>' : '<span class="badge" style="background:var(--bg-tertiary);color:var(--text-tertiary)">Inactive</span>'}</td>
                            <td>
                                <div class="table-actions">
                                    ${!c.active ? `<button class="btn btn-sm btn-primary activate-cert" data-id="${c.id}">Activate</button>` : ''}
                                    ${!c.active ? `<button class="btn btn-sm btn-danger delete-cert" data-id="${c.id}">Delete</button>` : ''}
                                </div>
                            </td>
                        </tr>`;
                        }).join('')}
                    </tbody>
                </table>
            </div>
        </div>` : ''}`;

        // Generate self-signed
        document.getElementById('ssl-self-signed')?.addEventListener('click', async () => {
            const btn = document.getElementById('ssl-self-signed');
            btn.disabled = true;
            btn.innerHTML = '<span class="loading-spinner btn-spinner"></span> Generating...';
            try {
                await ssl.selfSigned(siteId);
                showToast('Self-signed certificate generated!', 'success');
                const mod = await import('../site-detail.js');
                mod.render(document.getElementById('page-content'), site.token, 'ssl');
            } catch (err) {
                btn.disabled = false;
                btn.innerHTML = `${icons.lock} Generate Self-Signed`;
                showToast(err.message, 'error');
            }
        });

        // Upload custom cert
        document.getElementById('ssl-custom')?.addEventListener('click', () => {
            showModal('Upload SSL Certificate', `
                <form id="upload-cert-form">
                    <div class="form-group">
                        <label class="form-label">Label (optional)</label>
                        <input type="text" class="form-input" id="cert-label" placeholder="e.g. Let's Encrypt 2025">
                    </div>
                    <div class="form-group">
                        <label class="form-label">Certificate (.pem, .crt)</label>
                        <input type="file" class="form-input" id="cert-file" accept=".pem,.crt" required>
                    </div>
                    <div class="form-group">
                        <label class="form-label">Private Key (.pem, .key)</label>
                        <input type="file" class="form-input" id="key-file" accept=".pem,.key" required>
                    </div>
                </form>
            `, `
                <button class="btn btn-secondary" onclick="document.getElementById('modal-overlay').remove()">Cancel</button>
                <button class="btn btn-primary" id="submit-cert">Upload</button>
            `);

            document.getElementById('submit-cert')?.addEventListener('click', async () => {
                const certInput = document.getElementById('cert-file');
                const keyInput = document.getElementById('key-file');
                if (!certInput.files[0] || !keyInput.files[0]) { showToast('Both files required', 'error'); return; }
                const formData = new FormData();
                formData.append('certificate', certInput.files[0]);
                formData.append('private_key', keyInput.files[0]);
                formData.append('label', document.getElementById('cert-label')?.value || '');
                try {
                    await ssl.custom(siteId, formData);
                    closeModal();
                    showToast('Certificate uploaded & activated!', 'success');
                    const mod = await import('../site-detail.js');
                    mod.render(document.getElementById('page-content'), site.token, 'ssl');
                } catch (err) { showToast(err.message, 'error'); }
            });
        });

        // Let's Encrypt
        document.getElementById('ssl-letsencrypt')?.addEventListener('click', () => {
            const mainDomain = site.domain;
            const aliases = (site.aliases || '').split(/\s+/).filter(Boolean);
            const allDomains = [mainDomain, ...aliases];

            const domainToggles = allDomains.map((d, i) => `
                <div style="display: flex; align-items: center; justify-content: space-between; padding: var(--space-2) 0;">
                    <span style="font-size: var(--font-size-sm);">${escapeHtml(d)}</span>
                    <label class="toggle">
                        <input type="checkbox" class="le-domain" value="${escapeHtml(d)}" ${i === 0 ? 'checked' : ''}>
                        <span class="toggle-slider"></span>
                    </label>
                </div>
            `).join('');

            const content = `
                <p style="margin-bottom: var(--space-4); font-size: var(--font-size-sm); color: var(--text-secondary);">
                    Select which domains to include in the certificate. All selected domains must point to this server's IP via DNS (A/AAAA record).
                </p>
                <div class="form-group">
                    <label class="form-label">Domains</label>
                    ${domainToggles}
                </div>
            `;
            const footer = `
                <button class="btn btn-secondary" onclick="document.getElementById('modal-overlay').remove()">Cancel</button>
                <button class="btn btn-success" id="le-issue-btn"><span class="nav-icon nav-icon-sm">${icons.shield}</span> Issue Certificate</button>
            `;
            const modal = showModal("New Let's Encrypt SSL Certificate", content, footer, { persistent: true });

            modal.querySelector('#le-issue-btn')?.addEventListener('click', async () => {
                const checked = [...modal.querySelectorAll('.le-domain:checked')].map(cb => cb.value);
                if (checked.length === 0) { showToast('Select at least one domain', 'error'); return; }
                const btn = modal.querySelector('#le-issue-btn');
                btn.disabled = true;
                btn.innerHTML = `<span class="loading-spinner btn-spinner"></span> Issuing...`;
                try {
                    await ssl.letsEncrypt(siteId, checked);
                    closeModal();
                    showToast("Let's Encrypt certificate issued & activated!", 'success');
                    const mod = await import('../site-detail.js');
                    mod.render(document.getElementById('page-content'), site.token, 'ssl');
                } catch (err) {
                    showToast(err.message || "Let's Encrypt failed", 'error');
                    btn.disabled = false;
                    btn.innerHTML = `<span class="nav-icon nav-icon-sm">${icons.shield}</span> Issue Certificate`;
                }
            });
        });

        // Activate cert
        el.querySelectorAll('.activate-cert').forEach(btn => {
            btn.addEventListener('click', async () => {
                try {
                    await request(`/api/ssl-certs?action=activate`, {
                        method: 'POST',
                        body: JSON.stringify({ cert_id: parseInt(btn.dataset.id), site_id: parseInt(siteId) }),
                    });
                    showToast('Certificate activated!', 'success');
                    const mod = await import('../site-detail.js');
                    mod.render(document.getElementById('page-content'), site.token, 'ssl');
                } catch (err) { showToast(err.message, 'error'); }
            });
        });

        // Delete cert
        el.querySelectorAll('.delete-cert').forEach(btn => {
            btn.addEventListener('click', async () => {
                if (!await showConfirm('Delete Certificate', 'Delete this certificate permanently?', 'Delete', 'btn-danger')) return;
                try {
                    await request(`/api/ssl-certs?id=${btn.dataset.id}&site_id=${siteId}`, { method: 'DELETE' });
                    showToast('Certificate deleted', 'success');
                    const mod = await import('../site-detail.js');
                    mod.render(document.getElementById('page-content'), site.token, 'ssl');
                } catch (err) { showToast(err.message, 'error'); }
            });
        });
    }).catch(err => {
        el.innerHTML = `
        <div class="card">
            <h3 class="card-title" style="margin-bottom: var(--space-4);">SSL Certificate</h3>
            <div class="info-item" style="margin-bottom: var(--space-4);">
                <span class="info-label">Current Status</span>
                <span class="info-value"><span class="badge ${site.ssl_type === 'none' ? 'badge-warning' : 'badge-success'}">${site.ssl_type}</span></span>
            </div>
            ${site.ssl_cert_path ? `<div class="info-item" style="margin-bottom: var(--space-4);"><span class="info-label">Certificate Path</span><span class="info-value mono">${escapeHtml(site.ssl_cert_path)}</span></div>` : ''}
            <div style="display: flex; gap: var(--space-3); flex-wrap: wrap;">
                <button class="btn btn-primary" id="ssl-self-signed">${icons.lock} Generate Self-Signed</button>
                <button class="btn btn-secondary" id="ssl-custom">${icons.upload} Upload Custom Certificate</button>
            </div>
        </div>`;

        document.getElementById('ssl-self-signed')?.addEventListener('click', async () => {
            const btn = document.getElementById('ssl-self-signed');
            btn.disabled = true;
            btn.innerHTML = '<span class="loading-spinner btn-spinner"></span> Generating...';
            try {
                await ssl.selfSigned(siteId);
                showToast('Self-signed certificate generated!', 'success');
                const mod = await import('../site-detail.js');
                mod.render(document.getElementById('page-content'), site.token, 'ssl');
            } catch (err) {
                btn.disabled = false;
                btn.innerHTML = `${icons.lock} Generate Self-Signed`;
                showToast(err.message, 'error');
            }
        });

        document.getElementById('ssl-custom')?.addEventListener('click', () => {
            showModal('Upload SSL Certificate', `
                <form id="upload-cert-form">
                    <div class="form-group">
                        <label class="form-label">Certificate (.pem, .crt)</label>
                        <input type="file" class="form-input" id="cert-file" accept=".pem,.crt" required>
                    </div>
                    <div class="form-group">
                        <label class="form-label">Private Key (.pem, .key)</label>
                        <input type="file" class="form-input" id="key-file" accept=".pem,.key" required>
                    </div>
                </form>
            `, `
                <button class="btn btn-secondary" onclick="document.getElementById('modal-overlay').remove()">Cancel</button>
                <button class="btn btn-primary" id="submit-cert">Upload</button>
            `);

            document.getElementById('submit-cert')?.addEventListener('click', async () => {
                const certInput = document.getElementById('cert-file');
                const keyInput = document.getElementById('key-file');
                if (!certInput.files[0] || !keyInput.files[0]) { showToast('Both files required', 'error'); return; }
                const formData = new FormData();
                formData.append('certificate', certInput.files[0]);
                formData.append('private_key', keyInput.files[0]);
                try {
                    await ssl.custom(siteId, formData);
                    closeModal();
                    showToast('Certificate uploaded!', 'success');
                    const mod = await import('../site-detail.js');
                    mod.render(document.getElementById('page-content'), site.token, 'ssl');
                } catch (err) { showToast(err.message, 'error'); }
            });
        });
    });
}
