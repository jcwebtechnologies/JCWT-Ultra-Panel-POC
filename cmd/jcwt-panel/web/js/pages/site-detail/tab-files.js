import { files } from '../../api.js';
import { icons, showToast, showModal, closeModal, escapeHtml } from '../../app.js';

export async function renderFiles(el, siteId, siteToken) {
    el.innerHTML = `
    <div class="card" style="padding: var(--space-4);">
        <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: var(--space-3); flex-wrap: wrap; gap: var(--space-2);">
            <h3 class="card-title" style="margin: 0;">File Manager</h3>
            <div style="display: flex; gap: var(--space-2); align-items: center;">
                <button class="btn btn-sm btn-secondary" id="btn-compress-zip" title="Compress a folder or file into a .zip or .tar.gz archive">
                    📦 Zip Folder
                </button>
                <button class="btn btn-sm btn-secondary" id="btn-extract-archive" title="Extract a .zip or .tar.gz archive">
                    📂 Extract Archive
                </button>
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

    // Bind Zip / Compress Modal
    document.getElementById('btn-compress-zip')?.addEventListener('click', () => {
        showModal('Zip / Compress Folder', `
            <form id="compress-form" autocomplete="off">
                <div class="form-group">
                    <label class="form-label">Target Folder or File</label>
                    <input type="text" class="form-input mono" id="compress-target" value="htdocs" placeholder="htdocs or htdocs/wp-content" required>
                    <div class="form-help">Relative path inside site home (e.g. htdocs, htdocs/wp-content)</div>
                </div>
                <div class="form-group">
                    <label class="form-label">Output Archive Name</label>
                    <input type="text" class="form-input mono" id="compress-output" value="archive.zip" placeholder="archive.zip or backup.tar.gz" required>
                    <div class="form-help">Supports .zip and .tar.gz</div>
                </div>
            </form>
        `, `
            <button class="btn btn-secondary" onclick="document.querySelector('.modal-backdrop')?.remove()">Cancel</button>
            <button class="btn btn-primary" id="submit-compress">Compress Now</button>
        `);

        document.getElementById('submit-compress')?.addEventListener('click', async () => {
            const target = document.getElementById('compress-target').value.trim();
            const outputName = document.getElementById('compress-output').value.trim();
            if (!target || !outputName) {
                showToast('Target and output filename are required', 'error');
                return;
            }
            const btn = document.getElementById('submit-compress');
            btn.disabled = true;
            btn.innerHTML = '<span class="loading-spinner btn-spinner"></span> Compressing...';
            try {
                const res = await files.compress(siteId, target, outputName);
                showToast(res.message || 'Folder compressed successfully!', 'success');
                closeModal();
                const iframe = document.getElementById('fb-iframe');
                if (iframe) iframe.src = iframe.src;
            } catch (err) {
                showToast(err.message || 'Compression failed', 'error');
                btn.disabled = false;
                btn.textContent = 'Compress Now';
            }
        });
    });

    // Bind Extract Archive Modal
    document.getElementById('btn-extract-archive')?.addEventListener('click', () => {
        showModal('Extract Zip / Tar Archive', `
            <form id="extract-form" autocomplete="off">
                <div class="form-group">
                    <label class="form-label">Archive File Path</label>
                    <input type="text" class="form-input mono" id="extract-archive-path" placeholder="htdocs/_htdocs.zip or backups/archive.tar.gz" required>
                    <div class="form-help">Relative path to archive (.zip, .tar.gz, .tgz, .tar)</div>
                </div>
                <div class="form-group">
                    <label class="form-label">Extract Destination Directory</label>
                    <input type="text" class="form-input mono" id="extract-destination" value="htdocs" placeholder="htdocs" required>
                    <div class="form-help">Destination folder where files will be unpacked</div>
                </div>
            </form>
        `, `
            <button class="btn btn-secondary" onclick="document.querySelector('.modal-backdrop')?.remove()">Cancel</button>
            <button class="btn btn-primary" id="submit-extract">Extract Now</button>
        `);

        document.getElementById('submit-extract')?.addEventListener('click', async () => {
            const archivePath = document.getElementById('extract-archive-path').value.trim();
            const destination = document.getElementById('extract-destination').value.trim();
            if (!archivePath || !destination) {
                showToast('Archive path and destination directory are required', 'error');
                return;
            }
            const btn = document.getElementById('submit-extract');
            btn.disabled = true;
            btn.innerHTML = '<span class="loading-spinner btn-spinner"></span> Extracting...';
            try {
                const res = await files.extract(siteId, archivePath, destination);
                showToast(res.message || 'Archive extracted successfully!', 'success');
                closeModal();
                const iframe = document.getElementById('fb-iframe');
                if (iframe) iframe.src = iframe.src;
            } catch (err) {
                showToast(err.message || 'Extraction failed', 'error');
                btn.disabled = false;
                btn.textContent = 'Extract Now';
            }
        });
    });

    try {
        const data = await files.list(siteId);
        let fbUrl = data.url || `/fb/${siteToken}/`;
        if (!fbUrl.endsWith('/login')) {
            fbUrl = fbUrl.endsWith('/') ? `${fbUrl}login` : `${fbUrl}/login`;
        }

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
