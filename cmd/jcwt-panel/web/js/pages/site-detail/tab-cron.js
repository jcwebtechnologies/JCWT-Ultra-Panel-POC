import { cron } from '../../api.js';
import { icons, showToast, showModal, closeModal, escapeHtml, showConfirm } from '../../app.js';

export async function renderCron(el, siteId) {
    try {
        const jobs = await cron.list(siteId);
        const cronMap = {};
        jobs.forEach(j => { cronMap[j.id] = j; });

        el.innerHTML = `
        <div class="card">
            <div class="card-header">
                <h3 class="card-title">Cron Jobs</h3>
                <button class="btn btn-primary btn-sm" id="add-cron-btn">${icons.plus} Add</button>
            </div>
            ${jobs.length === 0 ? `
                <div class="empty-state p-6">
                    <div class="empty-state-title">No cron jobs</div>
                    <div class="empty-state-text">Add scheduled tasks for this site.</div>
                </div>
            ` : `
                <div class="table-container" style="border: none;">
                    <table class="data-table responsive-cards">
                        <thead><tr><th>Schedule</th><th>Command</th><th>Status</th><th>Actions</th></tr></thead>
                        <tbody>
                            ${jobs.map(j => `
                            <tr>
                                <td data-label="Schedule" class="mono">${escapeHtml(j.schedule)}</td>
                                <td data-label="Command" class="mono truncate" style="max-width: 300px;">${escapeHtml(j.command)}</td>
                                <td data-label="Status"><span class="badge ${j.enabled ? 'badge-success' : 'badge-warning'}">${j.enabled ? 'Active' : 'Paused'}</span></td>
                                <td>
                                    <div class="table-actions">
                                        <button class="btn btn-sm btn-secondary edit-cron" data-id="${j.id}">Edit</button>
                                        <button class="btn btn-sm btn-ghost toggle-cron" data-id="${j.id}">${j.enabled ? 'Pause' : 'Enable'}</button>
                                        <button class="btn btn-sm btn-danger delete-cron" data-id="${j.id}">Delete</button>
                                    </div>
                                </td>
                            </tr>`).join('')}
                        </tbody>
                    </table>
                </div>
            `}
        </div>`;

        document.getElementById('add-cron-btn')?.addEventListener('click', () => {
            showModal('Add Cron Job', `
                <div class="form-group">
                    <label class="form-label">Schedule</label>
                    <input type="text" class="form-input" id="cron-schedule" placeholder="*/5 * * * *">
                    <div class="form-help">Cron expression: min hour day month weekday</div>
                </div>
                <div class="form-group">
                    <label class="form-label">Command</label>
                    <input type="text" class="form-input" id="cron-command" placeholder="/usr/bin/php /home/user/htdocs/cron.php">
                </div>
            `, `
                <button class="btn btn-secondary" onclick="document.getElementById('modal-overlay').remove()">Cancel</button>
                <button class="btn btn-primary" id="submit-cron">Add Job</button>
            `);

            document.getElementById('submit-cron')?.addEventListener('click', async () => {
                try {
                    await cron.create({
                        site_id: parseInt(siteId),
                        schedule: document.getElementById('cron-schedule').value,
                        command: document.getElementById('cron-command').value,
                    });
                    closeModal();
                    showToast('Cron job added!', 'success');
                    renderCron(el, siteId);
                } catch (err) { showToast(err.message, 'error'); }
            });
        });

        el.querySelectorAll('.toggle-cron').forEach(btn => {
            btn.addEventListener('click', async () => {
                const job = cronMap[btn.dataset.id];
                if (!job) return;
                try {
                    await cron.update({
                        id: parseInt(btn.dataset.id),
                        site_id: parseInt(siteId),
                        schedule: job.schedule,
                        command: job.command,
                        enabled: !job.enabled,
                    });
                    showToast('Cron job updated', 'success');
                    renderCron(el, siteId);
                } catch (err) { showToast(err.message, 'error'); }
            });
        });

        el.querySelectorAll('.edit-cron').forEach(btn => {
            btn.addEventListener('click', () => {
                const job = cronMap[btn.dataset.id];
                if (!job) return;
                const cronId = btn.dataset.id;
                const currentSchedule = job.schedule;
                const currentCommand = job.command;
                const isEnabled = !!job.enabled;
                showModal('Edit Cron Job', `
                    <div class="form-group">
                        <label class="form-label">Schedule</label>
                        <input type="text" class="form-input" id="edit-cron-schedule" value="${escapeHtml(currentSchedule)}">
                        <div class="form-help">Cron expression: min hour day month weekday</div>
                    </div>
                    <div class="form-group">
                        <label class="form-label">Command</label>
                        <input type="text" class="form-input" id="edit-cron-command" value="${escapeHtml(currentCommand)}">
                    </div>
                `, `
                    <button class="btn btn-secondary" onclick="document.getElementById('modal-overlay').remove()">Cancel</button>
                    <button class="btn btn-primary" id="save-edit-cron">Save Changes</button>
                `);

                document.getElementById('save-edit-cron')?.addEventListener('click', async () => {
                    const schedule = document.getElementById('edit-cron-schedule').value.trim();
                    const command = document.getElementById('edit-cron-command').value.trim();
                    if (!schedule || !command) { showToast('Schedule and command are required', 'error'); return; }
                    try {
                        await cron.update({
                            id: parseInt(cronId),
                            site_id: parseInt(siteId),
                            schedule,
                            command,
                            enabled: isEnabled,
                        });
                        closeModal();
                        showToast('Cron job updated!', 'success');
                        renderCron(el, siteId);
                    } catch (err) { showToast(err.message, 'error'); }
                });
            });
        });

        el.querySelectorAll('.delete-cron').forEach(btn => {
            btn.addEventListener('click', async () => {
                const job = cronMap[btn.dataset.id];
                const desc = job ? job.command : 'this cron job';
                if (!await showConfirm('Delete Cron Job', `Delete this cron job?\n\n${desc}`, 'Delete', 'btn-danger')) return;
                try {
                    await cron.delete(btn.dataset.id, siteId);
                    showToast('Cron job deleted', 'success');
                    renderCron(el, siteId);
                } catch (err) { showToast(err.message, 'error'); }
            });
        });

    } catch (err) { el.innerHTML = `<p>Error: ${err.message}</p>`; }
}
