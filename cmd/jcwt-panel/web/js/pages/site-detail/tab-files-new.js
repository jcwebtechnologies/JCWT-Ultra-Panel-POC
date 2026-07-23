import { vuefinder } from '../../api.js';
import { icons, showToast, escapeHtml } from '../../app.js';

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
                <div class="empty-state-text">Initializing pilot native file manager.</div>
            </div>
        </div>
    </div>`;

    if (!document.getElementById('vuefinder-css')) {
        const link = document.createElement('link');
        link.id = 'vuefinder-css';
        link.rel = 'stylesheet';
        link.href = 'https://cdn.jsdelivr.net/npm/vuefinder@2.4.0/dist/style.css';
        document.head.appendChild(link);
    }

    async function init() {
        const mountArea = document.getElementById('vf-mount-area');
        if (!mountArea) return;

        mountArea.innerHTML = `
            <div class="empty-state" id="vf-loading">
                <div class="loading-spinner" style="margin: 0 auto var(--space-3);"></div>
                <div class="empty-state-title">Loading VueFinder File Manager...</div>
                <div class="empty-state-text">Initializing pilot native file manager.</div>
            </div>`;

        try {
            // Import Vue 3 and VueFinder directly as ES modules (Single file dist targets!)
            const Vue = await import('https://cdn.jsdelivr.net/npm/vue@3/dist/vue.esm-browser.prod.js');
            const VueFinderModule = await import('https://cdn.jsdelivr.net/npm/vuefinder@2.4.0/dist/vuefinder.js');

            const VueFinder = VueFinderModule.default || VueFinderModule;
            const Comp = VueFinder.VueFinder || (VueFinder.default && VueFinder.default.VueFinder) || VueFinder.default || VueFinder;

            mountArea.innerHTML = `<div id="vf-root" style="height:70vh;width:100%;"></div>`;

            const csrfToken = document.querySelector('meta[name="csrf-token"]')?.content || '';

            const app = Vue.createApp({
                render() {
                    return Vue.h(Comp, {
                        id: 'vf',
                        url: apiUrl,
                        request: {
                            baseUrl: apiUrl,
                            headers: {
                                'X-CSRF-Token': csrfToken
                            }
                        }
                    });
                }
            });

            if (VueFinder.install) {
                app.use(VueFinder);
            } else if (VueFinderModule.install) {
                app.use(VueFinderModule);
            }

            app.mount('#vf-root');

        } catch (err) {
            console.error('VueFinder load failed:', err);
            const mountArea = document.getElementById('vf-mount-area');
            if (mountArea) {
                mountArea.innerHTML = `
                    <div class="empty-state p-6">
                        <div class="empty-state-title" style="color: var(--status-error);">VueFinder Load Error</div>
                        <div class="empty-state-text" style="font-family: monospace; text-align: left; background: var(--bg-tertiary); padding: 12px; border-radius: 6px; word-break: break-all; margin: 12px 0;">
                            ${escapeHtml(err.stack || err.message || String(err))}
                        </div>
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
