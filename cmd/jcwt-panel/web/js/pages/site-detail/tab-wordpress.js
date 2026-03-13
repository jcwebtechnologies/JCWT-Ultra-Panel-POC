import { wordpress } from '../../api.js';
import { icons, showToast, escapeHtml } from '../../app.js';

// ---- WordPress Tools ----
export async function renderWordPressTools(container, site, siteId) {
    container.innerHTML = '<div class="loading-screen"><div class="loading-spinner"></div></div>';
    try {
        const state = await wordpress.status(siteId);

        function renderToggles() {
            container.innerHTML = `
            <div class="card">
                <div class="card-header">
                    <h3 class="card-title">WordPress Tools</h3>
                </div>
                <div style="padding: var(--space-4);">
                    <div class="settings-row" style="margin-bottom: var(--space-4);">
                        <div class="settings-row-label">
                            Allow XML-RPC
                            <small>Enable the XML-RPC endpoint (xmlrpc.php). Disable to block brute-force attacks.</small>
                        </div>
                        <div>
                            <label class="toggle">
                                <input type="checkbox" id="wp-xmlrpc-toggle" ${state.allow_xmlrpc ? 'checked' : ''}>
                                <span class="toggle-slider"></span>
                            </label>
                        </div>
                    </div>
                    <div class="settings-row" style="margin-bottom: var(--space-4);">
                        <div class="settings-row-label">
                            Disable WP-Cron
                            <small>Replace WP's built-in pseudo-cron with a server cron job (recommended).</small>
                        </div>
                        <div>
                            <label class="toggle">
                                <input type="checkbox" id="wp-wpcron-toggle" ${state.disable_wp_cron ? 'checked' : ''}>
                                <span class="toggle-slider"></span>
                            </label>
                        </div>
                    </div>
                    <div class="settings-row">
                        <div class="settings-row-label">
                            Disable File Editing
                            <small>Prevent theme and plugin editing from the WordPress admin dashboard.</small>
                        </div>
                        <div>
                            <label class="toggle">
                                <input type="checkbox" id="wp-fileedit-toggle" ${state.disable_file_edit ? 'checked' : ''}>
                                <span class="toggle-slider"></span>
                            </label>
                        </div>
                    </div>
                </div>
            </div>`;

            async function bindToggle(id, action, stateKey) {
                document.getElementById(id)?.addEventListener('change', async (e) => {
                    const toggle = e.target;
                    toggle.disabled = true;
                    try {
                        const result = await wordpress.toggle(action, siteId);
                        state[stateKey] = result[stateKey];
                        showToast('WordPress settings updated', 'success');
                    } catch (err) {
                        toggle.checked = !toggle.checked;
                        showToast(err.message, 'error');
                    } finally {
                        toggle.disabled = false;
                    }
                });
            }
            bindToggle('wp-xmlrpc-toggle', 'toggle-xmlrpc', 'allow_xmlrpc');
            bindToggle('wp-wpcron-toggle', 'toggle-wp-cron', 'disable_wp_cron');
            bindToggle('wp-fileedit-toggle', 'toggle-disable-file-edit', 'disable_file_edit');
        }

        renderToggles();
    } catch (err) {
        container.innerHTML = `<div class="empty-state"><div class="empty-state-title">Error: ${escapeHtml(err.message)}</div></div>`;
    }
}

// ---- WordPress Updates ----
export async function renderWordPressUpdates(container, site, siteId) {
    container.innerHTML = '<div class="loading-screen"><div class="loading-spinner"></div></div>';

    async function load() {
        try {
            const data = await wordpress.checkUpdates(siteId);
            container.innerHTML = `
            <div class="card">
                <div class="card-header">
                    <h3 class="card-title">WordPress Updates</h3>
                    <button class="btn btn-sm btn-ghost" id="wp-refresh-updates"><span class="nav-icon nav-icon-xs">${icons.refresh}</span> Check Again</button>
                </div>
                <div style="padding: var(--space-4);">
                    <div style="display:flex;flex-direction:column;gap:var(--space-4);">

                        <div id="wp-section-core" class="bordered-section-row">
                            <div>
                                <div style="font-weight:600;margin-bottom:var(--space-1);">WordPress Core</div>
                                <div style="font-size:var(--font-size-sm);color:var(--text-secondary);">
                                    ${data.core_update_available
                                        ? '<span class="badge badge-warning">Update available</span>'
                                        : '<span class="badge badge-success">Up to date</span>'}
                                </div>
                            </div>
                            ${data.core_update_available ? `<button class="btn btn-primary btn-sm wp-update-btn" data-action="core-update">Update Core</button>` : ''}
                        </div>

                        <div id="wp-section-plugins" class="bordered-section-row">
                            <div>
                                <div style="font-weight:600;margin-bottom:var(--space-1);">Plugins</div>
                                <div style="font-size:var(--font-size-sm);color:var(--text-secondary);">
                                    ${data.plugins_with_updates > 0
                                        ? `<span class="badge badge-warning">${data.plugins_with_updates} plugin${data.plugins_with_updates > 1 ? 's' : ''} need updating</span>`
                                        : '<span class="badge badge-success">All up to date</span>'}
                                </div>
                            </div>
                            ${data.plugins_with_updates > 0 ? `<button class="btn btn-primary btn-sm wp-update-btn" data-action="plugin-update">Update All Plugins</button>` : ''}
                        </div>

                        <div id="wp-section-themes" class="bordered-section-row">
                            <div>
                                <div style="font-weight:600;margin-bottom:var(--space-1);">Themes</div>
                                <div style="font-size:var(--font-size-sm);color:var(--text-secondary);">
                                    ${data.themes_with_updates > 0
                                        ? `<span class="badge badge-warning">${data.themes_with_updates} theme${data.themes_with_updates > 1 ? 's' : ''} need updating</span>`
                                        : '<span class="badge badge-success">All up to date</span>'}
                                </div>
                            </div>
                            ${data.themes_with_updates > 0 ? `<button class="btn btn-primary btn-sm wp-update-btn" data-action="theme-update">Update All Themes</button>` : ''}
                        </div>

                    </div>
                </div>
                <div id="wp-update-output" style="display:none;padding:0 var(--space-4) var(--space-4);">
                    <pre class="mono" style="background:var(--bg-tertiary);border:1px solid var(--border-primary);border-radius:var(--radius-md);padding:var(--space-3);font-size:var(--font-size-xs);overflow-x:auto;white-space:pre-wrap;word-break:break-all;max-height:200px;overflow-y:auto;"></pre>
                </div>
            </div>`;

            document.getElementById('wp-refresh-updates')?.addEventListener('click', load);

            container.querySelectorAll('.wp-update-btn').forEach(btn => {
                btn.addEventListener('click', async () => {
                    const action = btn.dataset.action;
                    const origText = btn.textContent;
                    btn.disabled = true;
                    btn.innerHTML = '<span class="loading-spinner btn-spinner"></span> Updating...';
                    const outputBox = document.getElementById('wp-update-output');
                    try {
                        let result;
                        if (action === 'core-update') result = await wordpress.coreUpdate(siteId);
                        else if (action === 'plugin-update') result = await wordpress.pluginUpdate(siteId);
                        else result = await wordpress.themeUpdate(siteId);

                        showToast('Update completed!', 'success');
                        if (outputBox) {
                            outputBox.style.display = '';
                            const pre = outputBox.querySelector('pre');
                            if (pre && result.output) pre.textContent = result.output;
                            if (!outputBox.querySelector('.wp-reload-btn')) {
                                const reloadBtn = document.createElement('button');
                                reloadBtn.className = 'btn btn-sm btn-ghost wp-reload-btn';
                                reloadBtn.style.cssText = 'margin-top:var(--space-2);display:block;';
                                reloadBtn.textContent = 'Check for More Updates';
                                reloadBtn.addEventListener('click', load);
                                outputBox.appendChild(reloadBtn);
                            }
                        }
                        // Update the section row: badge → up to date, remove update button
                        const sectionId = action === 'core-update' ? 'wp-section-core'
                            : action === 'plugin-update' ? 'wp-section-plugins' : 'wp-section-themes';
                        const sectionRow = document.getElementById(sectionId);
                        if (sectionRow) {
                            const badgeEl = sectionRow.querySelector('.badge');
                            if (badgeEl) {
                                badgeEl.className = 'badge badge-success';
                                badgeEl.textContent = action === 'core-update' ? 'Up to date' : 'All up to date';
                            }
                            btn.remove();
                        } else {
                            btn.disabled = false;
                            btn.textContent = origText;
                        }
                    } catch (err) {
                        btn.disabled = false;
                        btn.textContent = origText;
                        showToast(err.message, 'error');
                    }
                });
            });

        } catch (err) {
            container.innerHTML = `<div class="empty-state"><div class="empty-state-title">Error: ${escapeHtml(err.message)}</div></div>`;
        }
    }

    await load();
}
