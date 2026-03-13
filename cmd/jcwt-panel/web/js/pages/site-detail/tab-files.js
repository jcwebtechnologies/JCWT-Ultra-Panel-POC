import { files } from '../../api.js';
import { icons, showToast, escapeHtml } from '../../app.js';

export async function renderFiles(el, siteId, siteToken) {
    el.innerHTML = `
    <div class="card" style="padding: var(--space-4);">
        <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: var(--space-3);">
            <h3 class="card-title" style="margin: 0;">File Manager</h3>
            <div>
                <button class="btn btn-sm btn-ghost" id="fb-reload">↻ Reload</button>
            </div>
        </div>
        <div id="fb-container" style="min-height: 500px; display: flex; align-items: center; justify-content: center;">
            <div class="empty-state">
                <div class="loading-spinner" style="margin: 0 auto var(--space-3);"></div>
                <div class="empty-state-title">Starting File Manager...</div>
                <div class="empty-state-text">Please wait while the file manager initializes.</div>
            </div>
        </div>
    </div>`;

    try {
        const data = await files.list(siteId);
        const fbUrl = data.url || `/fb/${siteToken}/`;

        let retries = 0;
        const maxRetries = 5;

        function loadIframe() {
            const container = document.getElementById('fb-container');
            if (!container) return;

            container.innerHTML = `
                <iframe src="${fbUrl}"
                        style="width: 100%; height: 70vh; border: 1px solid var(--border-primary); border-radius: var(--radius-md);"
                        id="fb-iframe"></iframe>`;

            const iframe = document.getElementById('fb-iframe');
            if (!iframe) return;

            const loadTimeout = setTimeout(() => {
                if (retries < maxRetries) {
                    retries++;
                    container.innerHTML = `
                        <div class="empty-state p-4">
                            <div class="loading-spinner" style="margin: 0 auto var(--space-3);"></div>
                            <div class="empty-state-text">File Browser is starting up... (attempt ${retries + 1}/${maxRetries + 1})</div>
                        </div>`;
                    setTimeout(loadIframe, 2000);
                } else {
                    container.innerHTML = `
                        <div class="empty-state p-6">
                            <div class="empty-state-title">File Browser Unavailable</div>
                            <div class="empty-state-text">Could not connect after ${maxRetries + 1} attempts. The file browser binary may not be installed or has failed to start.</div>
                            <button class="btn btn-primary btn-sm" id="fb-manual-retry">Retry</button>
                        </div>`;
                    document.getElementById('fb-manual-retry')?.addEventListener('click', () => {
                        retries = 0;
                        loadIframe();
                    });
                }
            }, 5000);

            iframe.addEventListener('load', () => clearTimeout(loadTimeout));
        }

        setTimeout(loadIframe, 500);

        document.getElementById('fb-reload')?.addEventListener('click', () => {
            const iframe = document.getElementById('fb-iframe');
            if (iframe) iframe.src = iframe.src;
        });
    } catch (err) {
        document.getElementById('fb-container').innerHTML = `
            <div class="empty-state p-6">
                <div class="empty-state-title">File Browser Error</div>
                <div class="empty-state-text">${escapeHtml(err.message)}</div>
                <button class="btn btn-primary btn-sm" onclick="location.reload()">Retry</button>
            </div>`;
    }
}
