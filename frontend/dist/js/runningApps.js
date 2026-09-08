import { activateRunningApp, listRunningApps } from './api.js';
import { els } from './dom.js';
import { escapeHtml, hasWailsBinding } from './utils.js';

const POLL_INTERVAL_MS = 2800;
const CAPABILITY_EVENT = 'something:capability-unavailable';

let initialized = false;
let apps = [];
let activeRequest = null;
let pollTimer = null;
let lastRenderFingerprint = null;
let unsupported = false;

function getErrorMessage(error, fallback = 'Running apps could not be loaded.') {
  if (typeof error === 'string' && error.trim()) return error;
  if (error?.message) return error.message;
  return fallback;
}

function isUnsupportedMessage(message) {
  return /supported only on Windows|available only in the Windows desktop app/i.test(String(message || ''));
}

function hasRunningAppsBinding() {
  return Boolean(hasWailsBinding() && window.go.backend.App.ListRunningApps && window.go.backend.App.ActivateRunningApp);
}

function safeIconDataURL(value) {
  const icon = String(value || '');
  return /^data:image\/png;base64,[a-z0-9+/=]+$/i.test(icon) ? icon : '';
}

function normalizeApp(raw) {
  const windowID = String(raw?.window_id ?? raw?.WindowID ?? '');
  const title = String(raw?.title ?? raw?.Title ?? '').trim();
  const processName = String(raw?.process_name ?? raw?.ProcessName ?? '').trim();
  if (!windowID) return null;
  return {
    windowID,
    title,
    label: title || processName || 'Untitled window',
    processName,
    iconDataURL: safeIconDataURL(raw?.icon_data_url ?? raw?.IconDataURL),
    minimized: Boolean(raw?.minimized ?? raw?.Minimized),
    active: Boolean(raw?.active ?? raw?.Active),
  };
}

function normalizeApps(result) {
  return (Array.isArray(result) ? result : [])
    .map(normalizeApp)
    .filter(Boolean);
}

function responseFingerprint(items) {
  return JSON.stringify(items.map((item) => [
    item.windowID,
    item.title,
    item.processName,
    item.iconDataURL,
    item.minimized,
    item.active,
  ]));
}

function appInitial(app) {
  const source = app.title || app.processName || app.label;
  const char = Array.from(String(source || '').trim())[0];
  return char ? char.toUpperCase() : 'A';
}

function processIsRedundant(app) {
  const title = app.title.trim().toLowerCase().replace(/\.exe$/i, '');
  const process = app.processName.trim().toLowerCase().replace(/\.exe$/i, '');
  return !process || title === process;
}

function setBusy(isBusy) {
  els.runningAppsList?.setAttribute('aria-busy', String(isBusy));
  if (els.runningAppsRefresh) {
    els.runningAppsRefresh.disabled = unsupported || isBusy;
  }
}

function setError(message = '') {
  if (!els.runningAppsError) return;
  els.runningAppsError.textContent = message;
  els.runningAppsError.classList.toggle('hidden', !message);
}

function showInitialLoading() {
  els.runningAppsCount.textContent = '';
  els.runningAppsLoading.classList.remove('hidden');
  els.runningAppsEmpty.classList.add('hidden');
  els.runningAppsList.classList.add('hidden');
  setError('');
}

function showInitialError(message) {
  els.runningAppsCount.textContent = '';
  els.runningAppsLoading.classList.add('hidden');
  els.runningAppsEmpty.classList.add('hidden');
  els.runningAppsList.classList.add('hidden');
  setError(message);
}

function showSuccessState() {
  els.runningAppsLoading.classList.add('hidden');
  setError('');
  els.runningAppsCount.textContent = apps.length === 0 ? '' : `${apps.length} open`;
  els.runningAppsEmpty.classList.toggle('hidden', apps.length > 0);
  els.runningAppsList.classList.toggle('hidden', apps.length === 0);
}

function renderApp(app) {
  const secondary = processIsRedundant(app) ? '' : app.processName;
  const status = [
    app.active ? '<span class="running-apps-badge running-apps-badge-active">Active</span>' : '',
    app.minimized ? '<span class="running-apps-badge">Minimized</span>' : '',
  ].filter(Boolean).join('');
  const icon = app.iconDataURL
    ? `<img src="${escapeHtml(app.iconDataURL)}" alt="" />`
    : '';
  return `
    <div class="running-apps-row" role="listitem">
      <button
        class="running-apps-item${app.active ? ' is-active' : ''}${app.minimized ? ' is-minimized' : ''}"
        type="button"
        data-window-id="${escapeHtml(app.windowID)}"
        title="${escapeHtml(app.label)}"
        aria-label="Switch to ${escapeHtml(app.label)}"
      >
        <span class="running-apps-icon${app.iconDataURL ? ' has-image' : ''}" aria-hidden="true">
          ${icon}
          <span class="running-apps-icon-fallback${app.iconDataURL ? ' hidden' : ''}">${escapeHtml(appInitial(app))}</span>
        </span>
        <span class="running-apps-copy">
          <span class="running-apps-title">${escapeHtml(app.label)}</span>
          ${(secondary || status) ? `
            <span class="running-apps-meta">
              ${secondary ? `<span class="running-apps-process">${escapeHtml(secondary)}</span>` : ''}
              ${status}
            </span>
          ` : ''}
        </span>
      </button>
    </div>
  `;
}

function renderApps(nextApps) {
  const nextFingerprint = responseFingerprint(nextApps);
  apps = nextApps;
  showSuccessState();
  if (nextFingerprint === lastRenderFingerprint) return;
  els.runningAppsList.innerHTML = apps.map(renderApp).join('');
  lastRenderFingerprint = nextFingerprint;
}

function stopPolling() {
  if (pollTimer) {
    window.clearTimeout(pollTimer);
    pollTimer = null;
  }
}

function schedulePolling() {
  stopPolling();
  if (unsupported || document.visibilityState === 'hidden') return;
  pollTimer = window.setTimeout(() => {
    pollTimer = null;
    void requestRefresh({ silent: true });
  }, POLL_INTERVAL_MS);
}

function showUnsupported(message) {
  unsupported = true;
  stopPolling();
  apps = [];
  lastRenderFingerprint = null;
  els.runningAppsList.innerHTML = '';
  els.runningAppsList.classList.add('hidden');
  els.runningAppsEmpty.classList.add('hidden');
  els.runningAppsLoading.classList.add('hidden');
  els.runningAppsCount.textContent = '';
  setError(message || 'Running taskbar applications are supported only on Windows.');
  setBusy(false);
  els.runningAppsList.querySelectorAll('button').forEach((button) => {
    button.disabled = true;
  });
}

async function loadRunningApps({ manual = false, silent = false } = {}) {
  if (unsupported) return apps;
  if (!hasRunningAppsBinding()) {
    showUnsupported('Running taskbar applications are available only in the Windows desktop app.');
    return apps;
  }
  if (activeRequest) return activeRequest;
  if (!apps.length && !silent) {
    showInitialLoading();
  } else if (manual) {
    setError('');
  }

  setBusy(true);
  activeRequest = (async () => {
    try {
      const result = await listRunningApps();
      if (unsupported) return apps;
      renderApps(normalizeApps(result));
      return apps;
    } catch (error) {
      const message = getErrorMessage(error);
      if (isUnsupportedMessage(message)) {
        showUnsupported(message);
        return apps;
      }
      if (apps.length) {
        els.runningAppsLoading.classList.add('hidden');
        els.runningAppsEmpty.classList.add('hidden');
        els.runningAppsList.classList.remove('hidden');
        setError(message);
      } else {
        showInitialError(message);
      }
      return apps;
    } finally {
      activeRequest = null;
      setBusy(false);
    }
  })();
  return activeRequest;
}

function requestRefresh(options = {}) {
  stopPolling();
  return loadRunningApps(options).finally(schedulePolling);
}

async function activateSelectedApp(button) {
  const windowID = String(button.dataset.windowId || '');
  if (!windowID || unsupported || button.disabled) return;

  button.disabled = true;
  button.dataset.activating = 'true';
  setError('');
  try {
    await activateRunningApp(windowID);
  } catch (error) {
    setError(getErrorMessage(error, 'That window could not be activated.'));
  } finally {
    delete button.dataset.activating;
    button.disabled = unsupported;
  }
}

function handleListClick(event) {
  const button = event.target.closest('.running-apps-item[data-window-id]');
  if (!button || !els.runningAppsList.contains(button)) return;
  void activateSelectedApp(button);
}

function handleIconError(event) {
  const image = event.target;
  if (!(image instanceof HTMLImageElement) || !image.closest('.running-apps-icon')) return;
  const icon = image.closest('.running-apps-icon');
  image.classList.add('hidden');
  icon.classList.remove('has-image');
  icon.querySelector('.running-apps-icon-fallback')?.classList.remove('hidden');
}

function handleVisibilityChange() {
  if (document.visibilityState === 'hidden') {
    stopPolling();
    return;
  }
  void requestRefresh({ silent: apps.length > 0 });
}

function handleCapabilityUnavailable(event) {
  if (event.detail?.name !== 'running_apps') return;
  showUnsupported(event.detail.warning);
}

export function initRunningApps() {
  if (initialized || !els.runningAppsCard) return;
  initialized = true;

  els.runningAppsRefresh?.addEventListener('click', () => {
    void requestRefresh({ manual: true });
  });
  els.runningAppsList?.addEventListener('click', handleListClick);
  els.runningAppsList?.addEventListener('error', handleIconError, true);
  els.runningAppsCard.addEventListener(CAPABILITY_EVENT, handleCapabilityUnavailable);
  document.addEventListener('visibilitychange', handleVisibilityChange);
  window.addEventListener('focus', () => {
    void requestRefresh({ silent: apps.length > 0 });
  });
  window.addEventListener('beforeunload', stopPolling);

  const runningAppsCapability = window.somethingCapabilities?.running_apps;
  if (runningAppsCapability && !runningAppsCapability.available) {
    showUnsupported(runningAppsCapability.warning);
    return;
  }
  void requestRefresh();
}
