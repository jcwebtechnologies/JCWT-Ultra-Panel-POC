// JCWT Ultra Panel — Shared UI State Helpers
import { escapeHtml } from './app.js';

/**
 * Show a centered loading spinner in the container.
 */
export function showLoading(container) {
    container.innerHTML = '<div class="loading-screen"><div class="loading-spinner"></div></div>';
}

/**
 * Show an error empty-state with an optional icon.
 * @param {HTMLElement} container
 * @param {string} title
 * @param {string} [message]
 * @param {string} [iconHtml] - Raw SVG/emoji for the icon slot
 */
export function showError(container, title, message, iconHtml) {
    container.innerHTML = `<div class="empty-state">
        ${iconHtml ? `<div class="empty-state-icon">${iconHtml}</div>` : ''}
        <div class="empty-state-title">${escapeHtml(title)}</div>
        ${message ? `<div class="empty-state-text">${escapeHtml(message)}</div>` : ''}
    </div>`;
}

/**
 * Show an empty-state placeholder (e.g. "No items yet").
 */
export function showEmpty(container, title, message, iconHtml) {
    container.innerHTML = `<div class="empty-state">
        ${iconHtml ? `<div class="empty-state-icon">${iconHtml}</div>` : ''}
        <div class="empty-state-title">${escapeHtml(title)}</div>
        ${message ? `<div class="empty-state-text">${escapeHtml(message)}</div>` : ''}
    </div>`;
}
