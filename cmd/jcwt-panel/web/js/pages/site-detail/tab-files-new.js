import { vuefinder } from '../../api.js';
import { icons, showToast, escapeHtml } from '../../app.js';

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
        <div id="vf-container" style="min-height: 600px; position: relative;">
            <div class="empty-state" id="vf-loading">
                <div class="loading-spinner" style="margin: 0 auto var(--space-3);"></div>
                <div class="empty-state-title">Loading VueFinder...</div>
                <div class="empty-state-text">Initializing pilot native file manager.</div>
            </div>
        </div>
    </div>`;

    const container = document.getElementById('vf-container');
    if (!container) return;

    // Dynamically load VueFinder stylesheet and JS script if not loaded
    if (!document.getElementById('vuefinder-css')) {
        const link = document.createElement('link');
        link.id = 'vuefinder-css';
        link.rel = 'stylesheet';
        link.href = 'https://unpkg.com/vuefinder@2.4.0/dist/style.css';
        document.head.appendChild(link);
    }

    const apiUrl = vuefinder.url(siteId);

    function initVueFinder() {
        const loading = document.getElementById('vf-loading');
        if (loading) loading.style.display = 'none';

        // Render embedded VueFinder iframe proxying to native endpoint for isolated JS environment
        container.innerHTML = `
            <iframe id="vf-iframe" src="about:blank" style="width: 100%; height: 75vh; border: 1px solid var(--border-primary); border-radius: var(--radius-md); background: var(--bg-primary);"></iframe>
        `;

        const iframe = document.getElementById('vf-iframe');
        if (!iframe) return;

        const doc = iframe.contentWindow.document;
        doc.open();
        doc.write(`
            <!DOCTYPE html>
            <html>
            <head>
                <meta charset="utf-8">
                <meta name="viewport" content="width=device-width, initial-scale=1">
                <title>VueFinder</title>
                <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/vuefinder@2.4.0/dist/style.css">
                <script type="importmap">
                {
                    "imports": {
                        "vue": "https://cdn.jsdelivr.net/npm/vue@3/dist/vue.esm-browser.prod.js",
                        "vuefinder": "https://cdn.jsdelivr.net/npm/vuefinder@2.4.0/+esm"
                    }
                }
                </script>
                <style>
                    body { margin: 0; padding: 0; background: #0f172a; color: #f8fafc; font-family: system-ui, -apple-system, sans-serif; }
                    #vuefinder { height: 100vh; }
                </style>
            </head>
            <body>
                <div id="vuefinder"></div>
                <script type="module">
                    import { createApp } from 'vue';
                    import VueFinder from 'vuefinder';

                    const app = createApp({
                        template: '<vue-finder id="vf" :request="requestConfig" />',
                        data() {
                            return {
                                requestConfig: {
                                    baseUrl: '${apiUrl}',
                                    headers: {
                                        'X-CSRF-Token': '${document.querySelector('meta[name="csrf-token"]')?.content || ''}'
                                    }
                                }
                            };
                        }
                    });
                    app.use(VueFinder);
                    app.mount('#vuefinder');
                </script>
            </body>
            </html>
        `);
        doc.close();
    }

    setTimeout(initVueFinder, 300);

    document.getElementById('vf-reload')?.addEventListener('click', () => {
        initVueFinder();
        showToast('File Manager reloaded', 'success');
    });
}
