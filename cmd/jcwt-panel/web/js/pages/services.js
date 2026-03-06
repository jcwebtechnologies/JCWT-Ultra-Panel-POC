// JCWT Ultra Panel — Services Page
import { request } from '../api.js';
import { showToast, showConfirm, icons } from '../app.js';

let lastRefreshTime = 0;

export async function render(container) {
    document.getElementById('page-title').textContent = 'Services';
    container.innerHTML = `
        <div class="page-header">
            <div>
                <h2>Services</h2>
                <p>Monitor and manage system services</p>
            </div>
            <button class="btn btn-secondary btn-sm" id="refresh-services">
                <span class="nav-icon">${icons.refresh}</span> Refresh
            </button>
        </div>
        <div id="services-list">
            <div class="empty-state"><div class="empty-state-title">Loading services...</div></div>
        </div>`;

    document.getElementById('refresh-services')?.addEventListener('click', () => {
        const now = Date.now();
        if (now - lastRefreshTime < 3000) {
            showToast('Please wait a moment before refreshing again', 'warning');
            return;
        }
        lastRefreshTime = now;
        loadServices(container);
    });
    await loadServices(container);
}

async function loadServices(container) {
    const listEl = document.getElementById('services-list');

    try {
        const services = await request('/api/services');
        if (!services || services.length === 0) {
            listEl.innerHTML = `<div class="empty-state"><div class="empty-state-title">No services found</div></div>`;
            return;
        }

        // Sort: active first
        services.sort((a, b) => (a.active === 'active' ? -1 : 1) - (b.active === 'active' ? -1 : 1));

        listEl.innerHTML = `<div class="services-grid">${services.map(svc => {
            const isActive = svc.active === 'active';
            const statusClass = isActive ? 'status-active' : 'status-inactive';
            const statusText = isActive ? 'Running' : 'Stopped';
            const canRestart = svc.name !== 'jcwt-panel';

            let memoryText = '';
            if (svc.memory && svc.memory !== '[not set]') {
                const bytes = parseInt(svc.memory);
                if (!isNaN(bytes)) {
                    memoryText = bytes > 1048576 ? `${(bytes / 1048576).toFixed(1)} MB` : `${(bytes / 1024).toFixed(0)} KB`;
                }
            }

            return `
            <div class="card service-card">
                <div class="service-header">
                    <div class="service-info">
                        <div class="service-name">${escapeHtml(svc.name)}</div>
                        <div class="service-unit mono" style="font-size: var(--font-size-xs); color: var(--text-tertiary);">${escapeHtml(svc.unit)}</div>
                    </div>
                    <div class="service-status ${statusClass}">
                        <span class="status-dot"></span> ${statusText}
                    </div>
                </div>
                <div class="service-meta">
                    ${memoryText ? `<span class="service-meta-item">Memory: ${memoryText}</span>` : ''}
                    ${svc.uptime ? `<span class="service-meta-item">Since: ${escapeHtml(svc.uptime.split('.')[0] || svc.uptime)}</span>` : ''}
                </div>
                ${canRestart ? `
                <div class="service-actions" style="margin-top: var(--space-3);">
                    <button class="btn btn-sm btn-secondary restart-service" data-service="${escapeHtml(svc.name)}">
                        <span class="nav-icon">${icons.refresh}</span> Restart
                    </button>
                </div>` : `
                <div style="margin-top: var(--space-3); font-size: var(--font-size-xs); color: var(--text-tertiary);">
                    Use <code>systemctl restart jcwt-panel</code> to restart the panel
                </div>`}
            </div>`;
        }).join('')}</div>`;

        // Bind restart buttons
        listEl.querySelectorAll('.restart-service').forEach(btn => {
            btn.addEventListener('click', async () => {
                const name = btn.dataset.service;
                if (!await showConfirm('Restart Service', `Restart ${name}? This may briefly interrupt the service.`, 'Restart', 'btn-primary')) return;
                btn.disabled = true;
                btn.innerHTML = `<span class="nav-icon">${icons.refresh}</span> Restarting...`;
                try {
                    await request('/api/services', {
                        method: 'POST',
                        body: JSON.stringify({ service: name }),
                    });
                    showToast(`${name} restarted successfully`, 'success');
                    setTimeout(() => loadServices(container), 1000);
                } catch (err) {
                    showToast(err.message, 'error');
                    btn.disabled = false;
                    btn.innerHTML = `<span class="nav-icon">${icons.refresh}</span> Restart`;
                }
            });
        });
    } catch (err) {
        listEl.innerHTML = `<div class="empty-state"><div class="empty-state-title">Failed to load services</div><div class="empty-state-text">${escapeHtml(err.message)}</div></div>`;
    }
}

function escapeHtml(str) {
    if (!str) return '';
    const div = document.createElement('div');
    div.textContent = str;
    return div.innerHTML;
}
