import {
  getWebsiteSearchState,
  addWebsiteSearchTarget,
  deleteWebsiteSearchTarget,
  searchWebsites,
  getWebsiteSearchRun,
} from './api.js';
import { escapeHtml, hasWailsBinding } from './utils.js';

const byID = (id) => document.getElementById(id);

let initialized = false;
let loading = false;
let targets = [];
let recentSearches = [];

function messageOf(error, fallback) {
  return error?.message || String(error || '') || fallback;
}

function setError(id, message = '') {
  const element = byID(id);
  if (!element) return;
  element.textContent = message;
  element.classList.toggle('hidden', !message);
}

function formatDateTime(value) {
  if (!value) return '';
  const normalized = /^\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}$/.test(value)
    ? `${value.replace(' ', 'T')}Z`
    : value;
  const date = new Date(normalized);
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString();
}

function hostnameOf(value) {
  try {
    return new URL(value).hostname;
  } catch {
    return value;
  }
}

function renderTargets() {
  const count = targets.length;
  byID('search-target-count').textContent = `${count} ${count === 1 ? 'page' : 'pages'}`;
  byID('search-submit').disabled = loading || count === 0;
  byID('search-target-list').innerHTML = count ? targets.map((target) => `
    <article class="search-target" data-search-target-id="${Number(target.id)}">
      <div class="search-target-copy">
        <strong>${escapeHtml(hostnameOf(target.url))}</strong>
        <span title="${escapeHtml(target.url)}">${escapeHtml(target.url)}</span>
        ${target.last_checked_at ? `<small>Last checked ${escapeHtml(formatDateTime(target.last_checked_at))}</small>` : '<small>Not checked yet</small>'}
      </div>
      <button class="danger-btn small-btn" type="button" data-action="remove-target" ${loading ? 'disabled' : ''}>Remove</button>
    </article>
  `).join('') : '<div class="search-empty">Add a page URL to begin.</div>';
}

function renderHistory() {
  byID('search-history').innerHTML = recentSearches.length ? recentSearches.map((item) => `
    <button class="search-history-item" type="button" data-search-run-id="${Number(item.id)}">
      <span>
        <strong>${escapeHtml(item.query)}</strong>
        <small>${escapeHtml(formatDateTime(item.created_at))}${item.use_browser_fallback ? ' · browser fallback' : ''}</small>
      </span>
      <span class="search-history-counts">${Number(item.found)} found · ${Number(item.failed)} failed</span>
    </button>
  `).join('') : '<div class="search-empty">Completed searches will appear here.</div>';
}

function resultTitle(result) {
  return result.title || result.hostname || hostnameOf(result.original_url) || 'Web page';
}

function renderSearchResponse(response) {
  const checked = Number(response?.checked || 0);
  const found = Number(response?.found || 0);
  const failed = Number(response?.failed || 0);
  byID('search-summary').textContent = `Checked: ${checked} · Found: ${found} · Failed: ${failed}`;

  const results = Array.isArray(response?.results) ? response.results : [];
  const matches = results.filter((item) => Number(item.match_count) > 0);
  const failures = results.filter((item) => item.error_code);
  const warnings = results.filter((item) => item.fallback_warning && !item.error_code && Number(item.match_count) === 0);
  const sections = [];

  if (matches.length) {
    sections.push(`<div class="search-result-group"><h3>Matching pages</h3>${matches.map((result) => {
      const snippets = Array.isArray(result.snippets) ? result.snippets : [];
      const redirected = result.final_url && result.final_url !== result.original_url;
      return `
        <article class="search-result-item">
          <div class="search-result-heading">
            <div>
              <span class="search-result-domain">${escapeHtml(result.hostname || hostnameOf(result.original_url))}</span>
              <h3>${escapeHtml(resultTitle(result))}</h3>
            </div>
            <span class="search-method-badge">${result.fetch_method === 'browser' ? 'Browser' : 'HTTP'}</span>
          </div>
          <div class="search-result-url" title="${escapeHtml(result.original_url)}">${escapeHtml(result.original_url)}</div>
          ${redirected ? `<div class="search-result-redirect">Resolved to ${escapeHtml(result.final_url)}</div>` : ''}
          <div class="search-match-count">${Number(result.match_count)} ${Number(result.match_count) === 1 ? 'match' : 'matches'} on this page</div>
          <div class="search-snippets">${snippets.map((snippet) => `<blockquote>${escapeHtml(snippet)}</blockquote>`).join('')}</div>
          ${result.fallback_warning ? `<p class="search-result-warning">${escapeHtml(result.fallback_warning)}</p>` : ''}
          <div class="search-result-actions">
            <button class="primary-btn small-btn" type="button" data-action="open-result" data-open-url="${escapeHtml(result.open_url || result.final_url || result.original_url)}">Open</button>
            <button class="secondary-btn small-btn" type="button" data-action="open-result" data-open-url="${escapeHtml(result.original_url)}">Open original</button>
          </div>
        </article>`;
    }).join('')}</div>`);
  } else {
    sections.push('<div class="search-empty search-no-matches">The text was not found on any successfully checked page.</div>');
  }

  if (failures.length) {
    sections.push(`<div class="search-result-group search-failure-group"><h3>Could not check</h3>${failures.map((result) => `
      <article class="search-failure-item">
        <div><strong>${escapeHtml(result.hostname || hostnameOf(result.original_url))}</strong><span>${escapeHtml(result.original_url)}</span></div>
        <p>${escapeHtml(result.error_message || 'The page could not be checked.')}</p>
        ${result.fallback_warning ? `<small>${escapeHtml(result.fallback_warning)}</small>` : ''}
      </article>
    `).join('')}</div>`);
  }

  if (warnings.length) {
    sections.push(`<div class="search-result-group search-warning-group"><h3>Browser fallback notes</h3>${warnings.map((result) => `
      <p><strong>${escapeHtml(result.hostname || hostnameOf(result.original_url))}:</strong> ${escapeHtml(result.fallback_warning)}</p>
    `).join('')}</div>`);
  }

  byID('search-results').innerHTML = sections.join('');
}

function setLoading(value) {
  loading = value;
  byID('search-submit').disabled = value || targets.length === 0;
  byID('search-url-add').disabled = value;
  byID('search-query').disabled = value;
  byID('search-browser-fallback').disabled = value;
  byID('search-progress').textContent = value
    ? (byID('search-browser-fallback').checked
      ? 'Checking pages. Browser fallbacks may take a little longer…'
      : 'Checking pages…')
    : '';
  renderTargets();
}

async function addTarget(event) {
  event.preventDefault();
  setError('search-target-error');
  const input = byID('search-url');
  const value = input.value.trim();
  if (!value) {
    setError('search-target-error', 'Enter an HTTP or HTTPS page URL.');
    input.focus();
    return;
  }
  byID('search-url-add').disabled = true;
  try {
    await addWebsiteSearchTarget(value);
    input.value = '';
    await loadWebsiteSearchState();
    input.focus();
  } catch (error) {
    setError('search-target-error', messageOf(error, 'The page URL could not be saved.'));
  } finally {
    byID('search-url-add').disabled = loading;
  }
}

async function removeTarget(id) {
  setError('search-target-error');
  try {
    await deleteWebsiteSearchTarget(id);
    await loadWebsiteSearchState();
  } catch (error) {
    setError('search-target-error', messageOf(error, 'The page URL could not be removed.'));
  }
}

async function runSearch(event) {
  event.preventDefault();
  setError('search-error');
  const query = byID('search-query').value.trim();
  if (!query) {
    setError('search-error', 'Enter a word or phrase to search for.');
    byID('search-query').focus();
    return;
  }
  if (!targets.length) {
    setError('search-error', 'Add at least one page URL before searching.');
    byID('search-url').focus();
    return;
  }
  setLoading(true);
  try {
    const response = await searchWebsites(query, byID('search-browser-fallback').checked);
    renderSearchResponse(response);
    await loadWebsiteSearchState({ preserveErrors: true });
  } catch (error) {
    setError('search-error', messageOf(error, 'Website Search could not be completed.'));
  } finally {
    setLoading(false);
  }
}

async function loadHistoryRun(id) {
  setError('search-error');
  try {
    const response = await getWebsiteSearchRun(id);
    byID('search-query').value = response.query || '';
    byID('search-browser-fallback').checked = Boolean(response.use_browser_fallback);
    renderSearchResponse(response);
    byID('search-results-card').scrollIntoView({ behavior: 'smooth', block: 'start' });
  } catch (error) {
    setError('search-error', messageOf(error, 'The saved search could not be loaded.'));
  }
}

function openResult(value) {
  let parsed;
  try {
    parsed = new URL(value);
  } catch {
    setError('search-error', 'This result contains an invalid URL.');
    return;
  }
  if (parsed.protocol !== 'http:' && parsed.protocol !== 'https:') {
    setError('search-error', 'Only HTTP and HTTPS results can be opened.');
    return;
  }
  if (window.runtime?.BrowserOpenURL) {
    window.runtime.BrowserOpenURL(value);
  } else {
    window.open(value, '_blank', 'noopener,noreferrer');
  }
}

export function initWebsiteSearch() {
  if (initialized) return;
  initialized = true;
  byID('search-url-form').addEventListener('submit', addTarget);
  byID('search-query-form').addEventListener('submit', runSearch);
  byID('search-target-list').addEventListener('click', (event) => {
    const button = event.target.closest('[data-action="remove-target"]');
    const item = button?.closest('[data-search-target-id]');
    if (item) void removeTarget(Number(item.dataset.searchTargetId));
  });
  byID('search-results').addEventListener('click', (event) => {
    const button = event.target.closest('[data-action="open-result"]');
    if (button?.dataset.openUrl) openResult(button.dataset.openUrl);
  });
  byID('search-history').addEventListener('click', (event) => {
    const button = event.target.closest('[data-search-run-id]');
    if (button) void loadHistoryRun(Number(button.dataset.searchRunId));
  });
}

export async function loadWebsiteSearchState({ preserveErrors = false } = {}) {
  if (!initialized) initWebsiteSearch();
  if (!preserveErrors) {
    setError('search-target-error');
    setError('search-error');
  }
  if (!hasWailsBinding()) {
    setError('search-error', 'Website Search is available only through the desktop app backend.');
    return;
  }
  try {
    const state = await getWebsiteSearchState();
    targets = Array.isArray(state?.targets) ? state.targets : [];
    recentSearches = Array.isArray(state?.recent_searches) ? state.recent_searches : [];
    renderTargets();
    renderHistory();
  } catch (error) {
    setError('search-error', messageOf(error, 'Website Search data could not be loaded.'));
  }
}

