// JCWT Ultra Panel — Dashboard Page (Auto-Refresh)
import { dashboard } from '../api.js';
import { icons, showToast } from '../app.js';

let refreshInterval = null;

export async function render(container) {
    document.getElementById('page-title').textContent = 'Dashboard';

    // Clean up any previous interval
    if (refreshInterval) { clearInterval(refreshInterval); refreshInterval = null; }

    container.innerHTML = '<div class="loading-screen"><div class="loading-spinner"></div></div>';

    try {
        const stats = await dashboard.stats();
        renderDashboard(container, stats);
        bindCopyButtons(container);
        setupAutoRefresh(container);
    } catch (err) {
        container.innerHTML = `<div class="empty-state">
            <div class="empty-state-icon"><span class="nav-icon" style="width:48px;height:48px;color:var(--status-warning)">${icons.alertTriangle}</span></div>
            <div class="empty-state-title">Failed to load dashboard</div>
            <div class="empty-state-text">${err.message}</div>
        </div>`;
    }
}

function renderDashboard(container, stats) {
    const savedInterval = localStorage.getItem('dashboard_refresh') || '0';

    container.innerHTML = `
    <div class="stats-grid">
        <div class="stat-card">
            <div class="stat-icon purple">${icons.sites}</div>
            <div class="stat-info">
                <div class="stat-label">Active Sites</div>
                <div class="stat-value">${stats.total_sites}</div>
                <div class="stat-sub">PHP-powered sites</div>
            </div>
        </div>
        <div class="stat-card">
            <div class="stat-icon green">${icons.database}</div>
            <div class="stat-info">
                <div class="stat-label">Databases</div>
                <div class="stat-value">${stats.total_databases}</div>
                <div class="stat-sub">MariaDB databases</div>
            </div>
        </div>
        <div class="stat-card">
            <div class="stat-icon blue">${icons.clock}</div>
            <div class="stat-info">
                <div class="stat-label">Cron Jobs</div>
                <div class="stat-value">${stats.total_cron_jobs}</div>
                <div class="stat-sub">Scheduled tasks</div>
            </div>
        </div>
        <div class="stat-card">
            <div class="stat-icon orange">${icons.server}</div>
            <div class="stat-info">
                <div class="stat-label">Server Uptime</div>
                <div class="stat-value" style="font-size: var(--font-size-md);">${stats.uptime || 'N/A'}</div>
                <div class="stat-sub">${stats.hostname || 'Server'}</div>
            </div>
        </div>
    </div>

    <div style="display: grid; grid-template-columns: repeat(auto-fit, minmax(380px, 1fr)); gap: var(--space-4);">
        <div class="card" id="resources-card" style="position: relative;">
            <div class="card-header">
                <h3 class="card-title">System Resources</h3>
                <div class="auto-refresh-control">
                    <label style="font-size: var(--font-size-xs); color: var(--text-tertiary); margin-right: var(--space-2);">Auto-refresh:</label>
                    <select class="form-select" id="refresh-selector" style="width: auto; min-width: 80px; padding: var(--space-1) var(--space-2); font-size: var(--font-size-xs);">
                        <option value="0" ${savedInterval === '0' ? 'selected' : ''}>Off</option>
                        <option value="5000" ${savedInterval === '5000' ? 'selected' : ''}>5s</option>
                        <option value="10000" ${savedInterval === '10000' ? 'selected' : ''}>10s</option>
                        <option value="15000" ${savedInterval === '15000' ? 'selected' : ''}>15s</option>
                        <option value="30000" ${savedInterval === '30000' ? 'selected' : ''}>30s</option>
                    </select>
                </div>
            </div>
            <div id="resource-stats">
                ${renderResources(stats)}
            </div>
        </div>

        <div class="card" id="server-info-card" style="position: relative;">
            <div class="card-header">
                <h3 class="card-title">Server Information</h3>
            </div>
            <div class="info-grid" id="server-info-content" style="grid-template-columns: 1fr;">
                ${renderServerInfo(stats)}
            </div>
        </div>
    </div>`;
}

function renderServerInfo(stats) {
    return `
                ${(stats.ipv4_addresses && stats.ipv4_addresses.length > 0) ? `
                <div class="info-item">
                    <span class="info-label">IPv4 Address${stats.ipv4_addresses.length > 1 ? 'es' : ''}</span>
                    <span class="info-value">${stats.ipv4_addresses.map(ip => `<span class="ip-copy" data-ip="${ip}" style="cursor:pointer; padding: 2px 8px; background: var(--bg-tertiary); border-radius: var(--radius-sm); font-size: var(--font-size-sm); display: inline-flex; align-items: center; gap: 4px;" title="Click to copy">${ip} <span class="nav-icon" style="width:12px;height:12px;opacity:0.6">${icons.copy}</span></span>`).join(' ')}</span>
                </div>` : ''}
                ${(stats.ipv6_addresses && stats.ipv6_addresses.length > 0) ? `
                <div class="info-item">
                    <span class="info-label">IPv6 Address${stats.ipv6_addresses.length > 1 ? 'es' : ''}</span>
                    <span class="info-value" style="flex-wrap: wrap;">${stats.ipv6_addresses.map(ip => `<span class="ip-copy" data-ip="${ip}" style="cursor:pointer; padding: 2px 8px; background: var(--bg-tertiary); border-radius: var(--radius-sm); font-size: var(--font-size-xs); word-break: break-all; display: inline-flex; align-items: center; gap: 4px;" title="Click to copy">${ip} <span class="nav-icon" style="width:12px;height:12px;opacity:0.6">${icons.copy}</span></span>`).join(' ')}</span>
                </div>` : ''}
                <div class="info-item">
                    <span class="info-label">Architecture</span>
                    <span class="info-value"><span class="badge badge-primary">${stats.arch || 'arm64'}</span></span>
                </div>
                <div class="info-item">
                    <span class="info-label">PHP Versions</span>
                    <span class="info-value">${(stats.php_versions || []).map(v => `<span class="badge badge-info">PHP ${v}</span>`).join(' ')}</span>
                </div>
                <div class="info-item">
                    <span class="info-label">Server Time</span>
                    <span class="info-value mono">${stats.server_time ? new Date(stats.server_time).toLocaleString() : 'N/A'}</span>
                </div>
                <div class="info-item">
                    <span class="info-label">Hostname</span>
                    <span class="info-value mono">${stats.hostname || 'N/A'}</span>
                </div>`;
}

function renderResources(stats) {
    return `
    <div style="display: flex; flex-direction: column; gap: var(--space-4);">
        <div>
            <div style="display: flex; justify-content: space-between; margin-bottom: var(--space-2);">
                <span style="font-size: var(--font-size-sm); color: var(--text-secondary);">Memory</span>
                <span style="font-size: var(--font-size-sm); font-weight: 600;">${stats.memory_used_mb || 0} / ${stats.memory_total_mb || 0} MB</span>
            </div>
            <div class="progress-bar">
                <div class="progress-fill" style="width: ${Math.round(stats.memory_used_pct || 0)}%"></div>
            </div>
        </div>
        <div>
            <div style="display: flex; justify-content: space-between; margin-bottom: var(--space-2);">
                <span style="font-size: var(--font-size-sm); color: var(--text-secondary);">Disk Usage</span>
                <span style="font-size: var(--font-size-sm); font-weight: 600;">${stats.disk_used_gb || 0} GB / ${stats.disk_total_gb || 0} GB (${stats.disk_used_pct || 0}%)</span>
            </div>
            <div class="progress-bar">
                <div class="progress-fill" style="width: ${stats.disk_used_pct || 0}%"></div>
            </div>
        </div>
    </div>`;
}

function setupAutoRefresh(container) {
    const selector = document.getElementById('refresh-selector');
    if (!selector) return;

    selector.addEventListener('change', (e) => {
        const ms = parseInt(e.target.value);
        localStorage.setItem('dashboard_refresh', String(ms));
        if (refreshInterval) { clearInterval(refreshInterval); refreshInterval = null; }
        if (ms > 0) {
            refreshInterval = setInterval(() => refreshResources(), ms);
        }
    });

    // Auto-start if previously configured
    const saved = parseInt(localStorage.getItem('dashboard_refresh') || '0');
    if (saved > 0) {
        refreshInterval = setInterval(() => refreshResources(), saved);
    }
}

function showCardOverlay(cardId) {
    const card = document.getElementById(cardId);
    if (!card) return;
    // Remove existing overlay if present
    card.querySelector('.refresh-overlay')?.remove();
    const overlay = document.createElement('div');
    overlay.className = 'refresh-overlay';
    overlay.innerHTML = '<div class="refresh-spinner"></div>';
    card.appendChild(overlay);
}

function hideCardOverlay(cardId) {
    const card = document.getElementById(cardId);
    if (!card) return;
    card.querySelector('.refresh-overlay')?.remove();
}

async function refreshResources() {
    try {
        showCardOverlay('resources-card');
        showCardOverlay('server-info-card');

        const stats = await dashboard.stats();

        const el = document.getElementById('resource-stats');
        if (el) {
            el.innerHTML = renderResources(stats);
        } else {
            // Page navigated away, clean up
            if (refreshInterval) { clearInterval(refreshInterval); refreshInterval = null; }
            return;
        }

        // Also update server info
        const infoEl = document.getElementById('server-info-content');
        if (infoEl) {
            infoEl.innerHTML = renderServerInfo(stats);
            bindCopyButtons(document.getElementById('server-info-card'));
        }

        hideCardOverlay('resources-card');
        hideCardOverlay('server-info-card');
    } catch (err) {
        hideCardOverlay('resources-card');
        hideCardOverlay('server-info-card');
        // Silent fail on auto-refresh
    }
}

function bindCopyButtons(container) {
    container.querySelectorAll('.ip-copy').forEach(el => {
        el.addEventListener('click', () => {
            const ip = el.dataset.ip;
            navigator.clipboard.writeText(ip).then(() => {
                showToast(`Copied: ${ip}`, 'success');
            }).catch(() => {
                // Fallback
                const ta = document.createElement('textarea');
                ta.value = ip;
                document.body.appendChild(ta);
                ta.select();
                document.execCommand('copy');
                document.body.removeChild(ta);
                showToast(`Copied: ${ip}`, 'success');
            });
        });
    });
}
