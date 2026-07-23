import { getCsrfToken } from '../../api.js';
import { showToast, escapeHtml } from '../../app.js';

// Vue resolved via importmap in index.html → /js/vendor/vue.esm-browser.prod.js
import { createApp, h } from 'vue';

let vfModuleCache = null;
async function loadVueFinder() {
    if (!vfModuleCache) {
        vfModuleCache = await import('/js/vendor/vuefinder.bundle.js');
    }
    return vfModuleCache;
}

export async function renderVueFinder(el, siteId, siteToken) {
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
        <div id="vf-mount-area" style="min-height: 600px; width: 100%;"></div>
    </div>`;

    if (!document.getElementById('vuefinder-css')) {
        const link = document.createElement('link');
        link.id = 'vuefinder-css';
        link.rel = 'stylesheet';
        link.href = '/css/vendor/vuefinder.css';
        document.head.appendChild(link);
    }

    let vfApp = null;

    async function init() {
        const mountArea = document.getElementById('vf-mount-area');
        if (!mountArea) return;

        // Unmount any previous Vue instance
        if (vfApp) {
            try { vfApp.unmount(); } catch (_) {}
            vfApp = null;
        }

        mountArea.innerHTML = `
            <div class="empty-state" style="padding: var(--space-8);">
                <div class="loading-spinner" style="margin: 0 auto var(--space-3);"></div>
                <div class="empty-state-title">Loading File Manager...</div>
            </div>`;

        try {
            const vfMod = await loadVueFinder();

            // VueFinder 4.x named exports
            const VueFinderComp   = vfMod.VueFinder;
            const VueFinderPlugin = vfMod.VueFinderPlugin;
            const RemoteDriver    = vfMod.RemoteDriver;

            if (!VueFinderComp)   throw new Error('VueFinder component not found in bundle');
            if (!VueFinderPlugin) throw new Error('VueFinderPlugin not found in bundle');
            if (!RemoteDriver)    throw new Error('RemoteDriver not found in bundle');

            // Base URL is clean — site_id passed via X-Site-Id header so VueFinder 4.x
            // correctly constructs: POST /api/vuefinder/{action}  (not ?site_id=6/action)
            const driver = new RemoteDriver({
                baseURL: '/api/vuefinder',
                headers: {
                    'X-CSRF-Token': getCsrfToken(),
                    'X-Site-Id':    String(siteId),
                },
            });

            mountArea.innerHTML = '<div id="vf-root" style="height:72vh;width:100%;"></div>';

            vfApp = createApp({
                render() {
                    return h(VueFinderComp, { id: 'vf', driver });
                }
            });
            vfApp.use(VueFinderPlugin);
            vfApp.mount('#vf-root');

        } catch (err) {
            console.error('VueFinder init failed:', err);
            const area = document.getElementById('vf-mount-area');
            if (area) {
                area.innerHTML = `
                    <div class="empty-state" style="padding: var(--space-4);">
                        <div class="empty-state-title" style="color: var(--status-error);">VueFinder Load Error</div>
                        <pre style="font-size:12px; text-align:left; background: var(--bg-tertiary); padding: 12px;
                                    border-radius: 6px; overflow:auto; margin: 12px 0; white-space:pre-wrap;">${escapeHtml(err.stack || err.message || String(err))}</pre>
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
