import { diskUsage, sites } from '../../api.js';
import { icons, showToast, escapeHtml, formatBytes } from '../../app.js';
import { request } from '../../api.js';

// ---- Logs Viewer ----
export async function renderLogs(container, site, siteId) {
    let activeLog = 'access';
    let logLines = 25;

    async function loadLog() {
        container.innerHTML = '<div class="loading-screen"><div class="loading-spinner"></div></div>';
        try {
            const data = await request(`/api/logs?site_id=${siteId}&type=${activeLog}&lines=${logLines}`);
            container.innerHTML = `
            <div class="card">
                <div class="card-header">
                    <h3 class="card-title">Site Logs</h3>
                    <div style="display: flex; gap: var(--space-2); align-items: center;">
                        <select class="form-select" id="log-type" style="width: auto; min-width: 140px; padding: var(--space-1) var(--space-2); font-size: var(--font-size-xs);">
                            <option value="access" ${activeLog === 'access' ? 'selected' : ''}>Access Log</option>
                            <option value="error" ${activeLog === 'error' ? 'selected' : ''}>Error Log</option>
                            ${(site.site_type === 'php' || site.site_type === 'wordpress') ? `<option value="php-error" ${activeLog === 'php-error' ? 'selected' : ''}>PHP Error Log</option>` : ''}
                        </select>
                        <select class="form-select" id="log-lines" style="width: auto; min-width: 80px; padding: var(--space-1) var(--space-2); font-size: var(--font-size-xs);">
                            ${[25,50,100,200,500].map(n => `<option value="${n}" ${logLines === n ? 'selected' : ''}>${n} lines</option>`).join('')}
                        </select>
                        <button class="btn btn-sm btn-ghost" id="refresh-logs">${icons.refresh} Refresh</button>
                    </div>
                </div>
                <div style="padding: var(--space-3);">
                    ${data.hint ? `<div style="background: var(--status-warning-bg, rgba(234,179,8,0.1)); border: 1px solid var(--status-warning, #eab308); border-radius: var(--radius-md); padding: var(--space-2) var(--space-3); margin-bottom: var(--space-3); font-size: var(--font-size-xs); color: var(--text-secondary);">${escapeHtml(data.hint)}</div>` : ''}
                    <pre class="mono" style="background: var(--bg-tertiary); border: 1px solid var(--border-primary); border-radius: var(--radius-md); padding: var(--space-3); overflow-x: auto; max-height: 500px; overflow-y: auto; font-size: var(--font-size-xs); line-height: 1.5; white-space: pre-wrap; word-break: break-all;">${escapeHtml(data.content || 'No log data available.')}</pre>
                    ${data.log_path ? `<div style="margin-top: var(--space-2); font-size: var(--font-size-xs); color: var(--text-tertiary);">Path: ${escapeHtml(data.log_path)}</div>` : ''}
                </div>
            </div>`;

            document.getElementById('log-type')?.addEventListener('change', (e) => {
                activeLog = e.target.value;
                loadLog();
            });
            document.getElementById('log-lines')?.addEventListener('change', (e) => {
                logLines = parseInt(e.target.value);
                loadLog();
            });
            document.getElementById('refresh-logs')?.addEventListener('click', () => loadLog());
        } catch (err) {
            container.innerHTML = `<div class="empty-state"><div class="empty-state-title">Error: ${escapeHtml(err.message)}</div></div>`;
        }
    }

    await loadLog();
}

// ---- Disk Usage ----
export async function renderDiskUsage(container, site, siteId) {
    container.innerHTML = '<div class="loading-screen"><div class="loading-spinner"></div></div>';

    let sortMode = 'size';
    let refreshCooldown = false;

    async function load() {
        try {
            const data = await diskUsage.siteTree(siteId);
            const tree = data.tree;
            const total = data.total || 'N/A';

            container.innerHTML = `
            <div class="card">
                <div class="card-header">
                    <h3 class="card-title">Disk Usage</h3>
                    <div style="display:flex;gap:var(--space-2);align-items:center;">
                        <div class="btn-group" style="display:flex;border:1px solid var(--border-primary);border-radius:var(--radius-md);overflow:hidden;">
                            <button class="btn btn-sm ${sortMode === 'name' ? 'btn-primary' : 'btn-ghost'}" id="sort-name" style="border-radius:0;border:none;padding:var(--space-1) var(--space-3);">Name</button>
                            <button class="btn btn-sm ${sortMode === 'size' ? 'btn-primary' : 'btn-ghost'}" id="sort-size" style="border-radius:0;border:none;padding:var(--space-1) var(--space-3);">Size</button>
                        </div>
                        <button class="btn btn-sm btn-ghost" id="refresh-du"><span class="nav-icon nav-icon-xs">${icons.refresh}</span></button>
                    </div>
                </div>
                <div style="padding: 0;">
                    <div style="padding:var(--space-3) var(--space-4);background:var(--bg-tertiary);border-bottom:1px solid var(--border-primary);display:flex;justify-content:space-between;align-items:center;">
                        <span style="font-weight:600;">Total</span>
                        <span class="mono" style="font-weight:600;">${escapeHtml(total)}</span>
                    </div>
                    <div id="du-tree" style="font-size:var(--font-size-sm);">
                        ${tree ? renderTreeHTML(tree.children || [], 0, sortMode) : '<div style="padding:var(--space-4);color:var(--text-tertiary);">No data available</div>'}
                    </div>
                </div>
            </div>`;

            container.querySelector('#sort-name')?.addEventListener('click', () => { sortMode = 'name'; load(); });
            container.querySelector('#sort-size')?.addEventListener('click', () => { sortMode = 'size'; load(); });
            container.querySelector('#refresh-du')?.addEventListener('click', () => {
                if (refreshCooldown) { showToast('Please wait before refreshing again', 'warning'); return; }
                refreshCooldown = true;
                showToast('Refreshing disk usage...', 'info');
                load().then(() => setTimeout(() => { refreshCooldown = false; }, 10000));
            });

            // Event delegation for tree expand/collapse
            const duTree = container.querySelector('#du-tree');
            if (duTree) {
                duTree.addEventListener('click', (e) => {
                    const row = e.target.closest('.du-row[data-has-children="true"]');
                    if (!row) return;
                    e.stopPropagation();
                    const node = row.parentElement;
                    if (!node) return;
                    const children = node.querySelector('.du-children');
                    const toggle = row.querySelector('.du-toggle');
                    if (children) {
                        const isHidden = children.style.display === 'none';
                        children.style.display = isHidden ? '' : 'none';
                        if (toggle) toggle.style.transform = isHidden ? 'rotate(90deg)' : '';
                    }
                });
            }

        } catch (err) {
            container.innerHTML = `<div class="card"><div style="padding:var(--space-4);color:var(--status-error);">Failed to load disk usage: ${escapeHtml(err.message)}</div></div>`;
        }
    }

    await load();
}

function renderTreeHTML(children, depth, sortMode) {
    if (!children || children.length === 0) return '';

    const sorted = [...children].sort((a, b) => {
        if (sortMode === 'size') return b.size - a.size;
        return a.name.localeCompare(b.name);
    });

    return sorted.map(node => {
        const hasChildren = node.children && node.children.length > 0;
        const indent = depth * 24;
        const collapsed = depth > 0;
        const chevron = hasChildren
            ? `<span class="du-toggle" style="cursor:pointer;display:inline-flex;width:18px;height:18px;align-items:center;justify-content:center;transition:transform 0.15s;${collapsed ? '' : 'transform:rotate(90deg);'}"><svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"><polyline points="9 18 15 12 9 6"/></svg></span>`
            : `<span style="display:inline-block;width:18px;"></span>`;

        const childrenHTML = hasChildren ? renderTreeHTML(node.children, depth + 1, sortMode) : '';

        return `
            <div class="du-node" data-depth="${depth}">
                <div class="du-row" style="display:flex;align-items:center;padding:var(--space-2) var(--space-4);border-bottom:1px solid var(--border-primary);gap:var(--space-1);cursor:${hasChildren ? 'pointer' : 'default'};" data-has-children="${hasChildren}">
                    <span style="display:inline-block;width:${indent}px;flex-shrink:0;"></span>
                    ${chevron}
                    <span class="nav-icon" style="width:16px;height:16px;flex-shrink:0;color:var(--text-tertiary);">${icons.folder}</span>
                    <span style="flex:1;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;">${escapeHtml(node.name)}</span>
                    <span class="mono" style="flex-shrink:0;color:var(--text-secondary);font-size:var(--font-size-xs);">${escapeHtml(node.size_str)}</span>
                </div>
                <div class="du-children" style="${collapsed ? 'display:none;' : ''}">${childrenHTML}</div>
            </div>`;
    }).join('');
}

// ---- Resource Usage ----
export async function renderResourceUsage(container, site, siteId) {
    let pollTimer = null;

    function fmtMB(mb) {
        return mb >= 1024 ? (mb / 1024).toFixed(1) + ' GB' : mb.toFixed(1) + ' MB';
    }

    function barWidth(val, max) {
        return Math.min(100, max > 0 ? (val / max) * 100 : 0).toFixed(1);
    }

    async function load() {
        try {
            const data = await sites.resourceUsage(siteId);
            const procs = data.processes || [];
            const totalMB = data.total_mem_mb || 0;
            const totalCPU = data.total_cpu || 0;
            const procCount = data.process_count ?? procs.length;
            const maxMem = procs.reduce((m, p) => Math.max(m, p.mem_mb), 0.01);
            const maxCPU = procs.reduce((m, p) => Math.max(m, p.cpu_pct), 0.01);

            container.innerHTML = `
            <div class="card" style="margin-bottom:var(--space-4);">
                <div class="card-header">
                    <h3 class="card-title">Resource Usage — ${escapeHtml(site.domain)}</h3>
                    <span style="font-size:var(--font-size-xs);color:var(--text-tertiary);">Auto-refreshes every 5s</span>
                </div>
                <div style="padding:var(--space-4);display:grid;grid-template-columns:repeat(3,1fr);gap:var(--space-4);">
                    <div>
                        <div style="font-size:var(--font-size-sm);color:var(--text-secondary);margin-bottom:var(--space-1);">Processes</div>
                        <div style="font-weight:600;font-size:var(--font-size-lg);">${procCount}</div>
                    </div>
                    <div>
                        <div style="font-size:var(--font-size-sm);color:var(--text-secondary);margin-bottom:var(--space-1);">RAM Used</div>
                        <div style="font-weight:600;font-size:var(--font-size-lg);">${fmtMB(totalMB)}</div>
                        <div style="background:var(--bg-tertiary);border-radius:var(--radius-full);overflow:hidden;height:6px;margin-top:var(--space-2);">
                            <div style="height:6px;background:var(--color-primary);border-radius:var(--radius-full);width:${barWidth(totalMB, 512)}%;transition:width .4s;"></div>
                        </div>
                    </div>
                    <div>
                        <div style="font-size:var(--font-size-sm);color:var(--text-secondary);margin-bottom:var(--space-1);">CPU %</div>
                        <div style="font-weight:600;font-size:var(--font-size-lg);">${totalCPU.toFixed(1)}%</div>
                        <div style="background:var(--bg-tertiary);border-radius:var(--radius-full);overflow:hidden;height:6px;margin-top:var(--space-2);">
                            <div style="height:6px;background:#f59e0b;border-radius:var(--radius-full);width:${Math.min(100, totalCPU).toFixed(1)}%;transition:width .4s;"></div>
                        </div>
                    </div>
                </div>
                <div style="padding:0 var(--space-4) var(--space-4);font-size:var(--font-size-xs);color:var(--text-tertiary);line-height:1.6;border-top:1px solid var(--border-primary);padding-top:var(--space-3);margin-top:0;">
                    <strong>RAM</strong>: sum of RSS (Resident Set Size) of all processes running as this site's Linux user — i.e. PHP-FPM workers, WP-CLI etc. Does not include shared memory from nginx or MySQL.<br>
                    <strong>CPU %</strong>: cumulative CPU percentage sampled at request time via <code>ps</code>. A value of 0% means those processes are currently idle. Each CPU core contributes up to 100%, so values above 100% are normal on multi-core servers (e.g. 200% = fully using 2 cores).
                </div>
            </div>
            <div class="card">
                <div class="card-header">
                    <h3 class="card-title">Processes (${procCount})</h3>
                </div>
                ${procs.length === 0
                    ? `<div class="empty-state p-4">No active processes for this site.</div>`
                    : `<div class="table-responsive">
                        <table class="data-table">
                            <thead><tr><th>PID</th><th>Name</th><th>Command</th><th>Memory</th><th style="min-width:120px;">Mem bar</th><th>CPU %</th><th style="min-width:90px;">CPU bar</th></tr></thead>
                            <tbody>${procs.map(p => `
                                <tr>
                                    <td class="mono" style="font-size:var(--font-size-xs);">${escapeHtml(p.pid)}</td>
                                    <td class="mono">${escapeHtml(p.name)}</td>
                                    <td class="mono" style="font-size:var(--font-size-xs);max-width:260px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;" title="${escapeHtml(p.cmd || '')}">${escapeHtml(p.cmd || '—')}</td>
                                    <td>${fmtMB(p.mem_mb)}</td>
                                    <td><div style="background:var(--bg-tertiary);border-radius:var(--radius-full);overflow:hidden;height:8px;"><div style="height:8px;background:var(--color-primary);border-radius:var(--radius-full);width:${barWidth(p.mem_mb, maxMem)}%;"></div></div></td>
                                    <td>${p.cpu_pct.toFixed(1)}%</td>
                                    <td><div style="background:var(--bg-tertiary);border-radius:var(--radius-full);overflow:hidden;height:8px;"><div style="height:8px;background:#f59e0b;border-radius:var(--radius-full);width:${barWidth(p.cpu_pct, maxCPU)}%;"></div></div></td>
                                </tr>`).join('')}
                            </tbody>
                        </table>
                    </div>`}
            </div>`;
        } catch (err) {
            if (pollTimer) clearInterval(pollTimer);
            container.innerHTML = `<div class="empty-state"><div class="empty-state-title">Error: ${escapeHtml(err.message)}</div></div>`;
        }
    }

    container.innerHTML = '<div class="loading-screen"><div class="loading-spinner"></div></div>';
    await load();
    pollTimer = setInterval(load, 5000);

    // Stop polling when container is removed from DOM
    const obs = new MutationObserver(() => {
        if (!document.contains(container)) {
            clearInterval(pollTimer);
            obs.disconnect();
        }
    });
    obs.observe(document.body, { childList: true, subtree: true });
}
