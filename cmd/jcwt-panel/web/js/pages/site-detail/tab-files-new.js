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
            <iframe id="vf-iframe" style="width: 100%; height: 75vh; border: 1px solid var(--border-primary); border-radius: var(--radius-md); background: #0f172a;"></iframe>
        </div>
    </div>`;

    const apiUrl = vuefinder.url(siteId);
    const csrfToken = document.querySelector('meta[name="csrf-token"]')?.content || '';

    function initVueFinder() {
        const iframe = document.getElementById('vf-iframe');
        if (!iframe) return;

        iframe.srcdoc = `<!DOCTYPE html>
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
        html, body { margin: 0; padding: 0; background: #0f172a; color: #f8fafc; font-family: system-ui, -apple-system, sans-serif; height: 100%; width: 100%; overflow: hidden; }
        #vuefinder { height: 100vh; width: 100%; box-sizing: border-box; }
    </style>
</head>
<body>
    <div id="vuefinder"></div>
    <script type="module">
        import { createApp, h } from 'vue';
        import VueFinder from 'vuefinder';

        window.addEventListener('error', function(e) {
            const el = document.getElementById('vuefinder');
            if (el) el.innerHTML = '<div style="padding:24px;color:#ef4444;font-family:monospace;font-size:14px;"><strong>VueFinder Error:</strong><br>' + (e.message || e) + '</div>';
        });

        try {
            const Comp = VueFinder.VueFinder || (VueFinder.default && VueFinder.default.VueFinder) || VueFinder.default || VueFinder;
            const app = createApp({
                render() {
                    return h(Comp, {
                        id: 'vf',
                        url: '${apiUrl}',
                        request: {
                            baseUrl: '${apiUrl}',
                            headers: {
                                'X-CSRF-Token': '${csrfToken}'
                            }
                        }
                    });
                }
            });
            if (VueFinder.install) {
                app.use(VueFinder);
            } else if (VueFinder.default && VueFinder.default.install) {
                app.use(VueFinder.default);
            }
            app.mount('#vuefinder');
        } catch (err) {
            const el = document.getElementById('vuefinder');
            if (el) el.innerHTML = '<div style="padding:24px;color:#ef4444;font-family:monospace;font-size:14px;"><strong>Initialization Error:</strong><br>' + (err.stack || err.message) + '</div>';
        }
    </script>
</body>
</html>`;
    }

    initVueFinder();

    document.getElementById('vf-reload')?.addEventListener('click', () => {
        initVueFinder();
        showToast('File Manager reloaded', 'success');
    });
}
