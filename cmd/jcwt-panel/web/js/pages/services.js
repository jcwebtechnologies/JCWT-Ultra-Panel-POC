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
        loadServices(container).then(() => showToast('Services refreshed', 'success'));
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

        // Sort alphabetically by name
        services.sort((a, b) => a.name.localeCompare(b.name));

        listEl.innerHTML = `<div class="services-grid">${services.map(svc => {
            const isActive = svc.active === 'active';
            const statusClass = isActive ? 'status-active' : 'status-inactive';
            const statusText = isActive ? 'Running' : 'Stopped';
            const isPanel = svc.name === 'jcwt-panel';
            const canReload = !isPanel && (svc.name === 'nginx' || svc.name.startsWith('php'));

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
                ${!isPanel ? `
                <div class="service-actions" style="margin-top: var(--space-3); display: flex; gap: var(--space-2); flex-wrap: wrap;">
                    ${!isActive ? `<button class="btn btn-sm btn-primary start-service" data-service="${escapeHtml(svc.name)}">
                        <span class="nav-icon">${icons.play}</span> Start
                    </button>` : `<button class="btn btn-sm btn-danger stop-service" data-service="${escapeHtml(svc.name)}">
                        <span class="nav-icon">${icons.stop}</span> Stop
                    </button>`}
                    ${isActive ? `<button class="btn btn-sm btn-secondary restart-service" data-service="${escapeHtml(svc.name)}">
                        <span class="nav-icon">${icons.refreshCw}</span> Restart
                    </button>` : ''}
                    ${isActive && canReload ? `<button class="btn btn-sm btn-secondary reload-service" data-service="${escapeHtml(svc.name)}">
                        <span class="nav-icon">${icons.refresh}</span> Reload
                    </button>` : ''}
                </div>` : `
                <div style="margin-top: var(--space-3); font-size: var(--font-size-xs); color: var(--text-tertiary);">
                    Use systemctl restart jcwt-panel to restart the panel
                </div>`}
            </div>`;
        }).join('')}</div>`;

        // Bind restart buttons
        listEl.querySelectorAll('.restart-service').forEach(btn => {
            btn.addEventListener('click', async () => {
                const name = btn.dataset.service;
                if (!await showConfirm('Restart Service', `Restart ${name}? This may briefly interrupt the service.`, 'Restart', 'btn-primary')) return;
                btn.disabled = true;
                btn.innerHTML = `<span class="nav-icon">${icons.refreshCw}</span> Restarting...`;
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
                    btn.innerHTML = `<span class="nav-icon">${icons.refreshCw}</span> Restart`;
                }
            });
        });

        // Bind reload buttons
        listEl.querySelectorAll('.reload-service').forEach(btn => {
            btn.addEventListener('click', async () => {
                const name = btn.dataset.service;
                if (!await showConfirm('Reload Service', `Reload ${name}? This gracefully reloads the configuration without dropping active connections.`, 'Reload', 'btn-primary')) return;
                btn.disabled = true;
                btn.innerHTML = `<span class="nav-icon">${icons.refresh}</span> Reloading...`;
                try {
                    await request('/api/services?action=reload', {
                        method: 'POST',
                        body: JSON.stringify({ service: name }),
                    });
                    showToast(`${name} reloaded successfully`, 'success');
                    setTimeout(() => loadServices(container), 1000);
                } catch (err) {
                    showToast(err.message, 'error');
                    btn.disabled = false;
                    btn.innerHTML = `<span class="nav-icon">${icons.refresh}</span> Reload`;
                }
            });
        });

        // Bind stop buttons
        listEl.querySelectorAll('.stop-service').forEach(btn => {
            btn.addEventListener('click', async () => {
                const name = btn.dataset.service;
                if (!await showConfirm('Stop Service', `Stop ${name}? The service will no longer be running until started again.`, 'Stop', 'btn-danger')) return;
                btn.disabled = true;
                btn.innerHTML = `<span class="nav-icon">${icons.stop}</span> Stopping...`;
                try {
                    await request('/api/services?action=stop', {
                        method: 'POST',
                        body: JSON.stringify({ service: name }),
                    });
                    showToast(`${name} stopped`, 'success');
                    setTimeout(() => loadServices(container), 1000);
                } catch (err) {
                    showToast(err.message, 'error');
                    btn.disabled = false;
                    btn.innerHTML = `<span class="nav-icon">${icons.stop}</span> Stop`;
                }
            });
        });

        // Bind start buttons
        listEl.querySelectorAll('.start-service').forEach(btn => {
            btn.addEventListener('click', async () => {
                const name = btn.dataset.service;
                btn.disabled = true;
                btn.innerHTML = `<span class="nav-icon">${icons.play}</span> Starting...`;
                try {
                    await request('/api/services?action=start', {
                        method: 'POST',
                        body: JSON.stringify({ service: name }),
                    });
                    showToast(`${name} started`, 'success');
                    setTimeout(() => loadServices(container), 1000);
                } catch (err) {
                    showToast(err.message, 'error');
                    btn.disabled = false;
                    btn.innerHTML = `<span class="nav-icon">${icons.play}</span> Start`;
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
