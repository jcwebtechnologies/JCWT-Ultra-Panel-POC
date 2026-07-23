import { vuefinder } from '../../api.js';
import { showToast, escapeHtml } from '../../app.js';

export async function renderVueFinder(el, siteId, siteToken) {
    const apiUrl = vuefinder.url(siteId);

    el.innerHTML = `
    <div class="card" style="padding: var(--space-4);">
        <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: var(--space-3); flex-wrap: wrap; gap: var(--space-2);">
            <div style="display: flex; align-items: center; gap: var(--space-2);">
                <h3 class="card-title" style="margin: 0;">File Manager (New - VueFinder Pilot)</h3>
                <span class="badge badge-info" style="font-size: var(--font-size-xs);">Native Go REST API</span>
            </div>
            <div style="display: flex; gap: var(--space-2); align-items: center;">
                <button class="btn btn-sm btn-ghost" id="vf-reload">↻ Reload</button>
            </div>
        </div>
        <div id="vf-mount-area" style="min-height: 600px; width: 100%;">
            <div class="empty-state" id="vf-loading">
                <div class="loading-spinner" style="margin: 0 auto var(--space-3);"></div>
                <div class="empty-state-title">Loading VueFinder File Manager...</div>
                <div class="empty-state-text">Initializing native file manager.</div>
            </div>
        </div>
    </div>`;

    if (!document.getElementById('vuefinder-css')) {
        const link = document.createElement('link');
        link.id = 'vuefinder-css';
        link.rel = 'stylesheet';
        link.href = '/css/vendor/vuefinder.css';
        document.head.appendChild(link);
    }

    async function init() {
        const mountArea = document.getElementById('vf-mount-area');
        if (!mountArea) return;

        mountArea.innerHTML = `
            <div class="empty-state" id="vf-loading">
                <div class="loading-spinner" style="margin: 0 auto var(--space-3);"></div>
                <div class="empty-state-title">Loading VueFinder File Manager...</div>
                <div class="empty-state-text">Initializing native file manager.</div>
            </div>`;

        try {
            // Single fully self-contained bundle — zero bare specifier imports, zero CDN calls
            const { createApp, h } = await import('/js/vendor/vuefinder.bundle.js');
            const vfMod = await import('/js/vendor/vuefinder.bundle.js');

            // VueFinder exports: { VueFinder, VueFinderPlugin, RemoteDriver, default, ... }
            const VueFinderComp = vfMod.VueFinder || vfMod.default?.VueFinder || vfMod.default;
            const VueFinderPlugin = vfMod.VueFinderPlugin || vfMod.default?.VueFinderPlugin || vfMod.default;
            const RemoteDriver = vfMod.RemoteDriver || vfMod.default?.RemoteDriver;

            if (!VueFinderComp) throw new Error('VueFinder component not found in bundle exports');

            mountArea.innerHTML = `<div id="vf-root" style="height:72vh;width:100%;"></div>`;

            const csrfToken = document.querySelector('meta[name="csrf-token"]')?.content || '';

            const driver = RemoteDriver
                ? new RemoteDriver({
                    baseURL: apiUrl,
                    headers: { 'X-CSRF-Token': csrfToken },
                })
                : null;

            const app = createApp({
                render() {
                    return h(VueFinderComp, {
                        id: 'vf',
                        ...(driver ? { driver } : { url: apiUrl }),
                        request: {
                            baseUrl: apiUrl,
                            headers: { 'X-CSRF-Token': csrfToken },
                        },
                    });
                }
            });

            app.use(VueFinderPlugin);
            app.mount('#vf-root');

        } catch (err) {
            console.error('VueFinder init failed:', err);
            const mountArea = document.getElementById('vf-mount-area');
            if (mountArea) {
                mountArea.innerHTML = `
                    <div class="empty-state" style="padding: var(--space-4);">
                        <div class="empty-state-title" style="color: var(--status-error);">VueFinder Load Error</div>
                        <pre style="font-size:12px; text-align:left; background: var(--bg-tertiary); padding: 12px; border-radius: 6px; overflow:auto; margin: 12px 0; white-space:pre-wrap;">${escapeHtml(err.stack || err.message || String(err))}</pre>
                        <button class="btn btn-sm btn-primary" id="vf-retry-btn">Retry</button>
                    </div>`;
                document.getElementById('vf-retry-btn')?.addEventListener('click', init);
            }
        }
    }

    init();

    document.getElementById('vf-reload')?.addEventListener('click', () => {
        init();
        showToast('File Manager reloaded', 'success');
    });
}
