import {
  addDesktopApp,
  deleteDesktopAppIcon,
  deleteDesktopApp,
  importDesktopAppIcon,
  launchDesktopApp,
  listDesktopApps,
  relocateDesktopApp,
  renameDesktopApp,
} from './api.js';
import { els } from './dom.js';
import { escapeHtml, hasWailsBinding } from './utils.js';
import { showConfirmation } from './ui.js';

const DESKTOP_APPS_CHANGED_EVENT = 'desktop-apps:changed';

let apps = [];
let busy = false;
let editingApp = null;

function errorMessage(error) {
  return error?.message || String(error || 'The application action could not be completed.');
}

function showAppsError(message = '') {
  if (!els.appsError) return;
  els.appsError.textContent = message;
  els.appsError.classList.toggle('hidden', !message);
}

function setAppsBusy(nextBusy) {
  busy = nextBusy;
  els.appsList?.setAttribute('aria-busy', String(nextBusy));
  els.appsLoading?.classList.toggle('hidden', !nextBusy || apps.length > 0);
  if (els.appsAdd) {
    els.appsAdd.disabled = nextBusy;
    els.appsAdd.textContent = nextBusy ? 'Please wait...' : 'Add app';
  }
  els.appsList?.querySelectorAll('button').forEach((button) => {
    button.disabled = nextBusy || button.dataset.appUnavailable === 'true';
  });
}

function appInitial(name) {
  return Array.from(String(name || '').trim())[0]?.toUpperCase() || 'A';
}

function appIconMarkup(app, name) {
  if (app.icon_url) {
    return `<img src="${escapeHtml(app.icon_url)}" alt="" />`;
  }
  return escapeHtml(appInitial(name));
}

function renderApps() {
  if (!els.appsList || !els.appsEmpty) return;
  els.appsEmpty.classList.toggle('hidden', apps.length > 0);
  els.appsList.classList.toggle('hidden', apps.length === 0);
  els.appsList.innerHTML = apps.map((app) => {
    const available = Boolean(app.available);
    const name = app.display_name || 'Application';
    return `
      <article class="app-launcher-card${available ? '' : ' is-unavailable'}" role="listitem">
        <button
          class="app-launcher-main"
          type="button"
          data-app-action="launch"
          data-app-id="${Number(app.id)}"
          data-app-unavailable="${String(!available)}"
          ${available ? '' : 'disabled'}
          aria-label="Open ${escapeHtml(name)}"
        >
          <span class="app-launcher-icon${app.icon_url ? ' has-image' : ''}" aria-hidden="true">${appIconMarkup(app, name)}</span>
          <span class="app-launcher-copy">
            <strong>${escapeHtml(name)}</strong>
            <small title="${escapeHtml(app.executable_path || '')}">${escapeHtml(app.executable_path || '')}</small>
          </span>
          <span class="app-launcher-state ${available ? 'is-ready' : 'is-missing'}">
            ${available ? 'Open' : 'Not found'}
          </span>
        </button>
        <div class="app-launcher-actions" aria-label="Manage ${escapeHtml(name)}">
          <button class="secondary-btn" type="button" data-app-action="rename" data-app-id="${Number(app.id)}">Rename</button>
          <button class="secondary-btn" type="button" data-app-action="relocate" data-app-id="${Number(app.id)}">${available ? 'Change path' : 'Locate'}</button>
          <button class="secondary-btn" type="button" data-app-action="icon" data-app-id="${Number(app.id)}">${app.icon_url ? 'Change icon' : 'Add icon'}</button>
          ${app.icon_url ? `<button class="secondary-btn" type="button" data-app-action="remove-icon" data-app-id="${Number(app.id)}">Reset icon</button>` : ''}
          <button class="secondary-btn app-launcher-remove" type="button" data-app-action="delete" data-app-id="${Number(app.id)}">Remove</button>
        </div>
      </article>
    `;
  }).join('');
  setAppsBusy(busy);
}

async function refreshApps() {
  const result = await listDesktopApps();
  apps = Array.isArray(result) ? result : [];
  renderApps();
  document.dispatchEvent(new CustomEvent('desktop-apps:changed', {
    detail: { apps: [...apps] },
  }));
}

export async function loadDesktopApps() {
  if (busy) return;
  showAppsError('');
  if (!hasWailsBinding()) {
    showAppsError('Application launchers are available only in the desktop app.');
    return;
  }

  setAppsBusy(true);
  try {
    await refreshApps();
  } catch (error) {
    showAppsError(errorMessage(error));
  } finally {
    setAppsBusy(false);
  }
}

async function runMutation(action) {
  if (busy) return;
  showAppsError('');
  setAppsBusy(true);
  try {
    await action();
    await refreshApps();
  } catch (error) {
    showAppsError(errorMessage(error));
  } finally {
    setAppsBusy(false);
  }
}

async function addApp() {
  await runMutation(() => addDesktopApp());
}

async function openApp(id) {
  if (busy) return;
  showAppsError('');
  setAppsBusy(true);
  try {
    await launchDesktopApp(id);
  } catch (error) {
    showAppsError(errorMessage(error));
    try {
      await refreshApps();
    } catch {
      // The launch error is the useful message; keep it visible.
    }
  } finally {
    setAppsBusy(false);
  }
}

function setAppEditError(message = '') {
  if (!els.appEditError) return;
  els.appEditError.textContent = message;
  els.appEditError.classList.toggle('hidden', !message);
}

function openAppEditModal(id) {
  const app = apps.find((item) => Number(item.id) === id);
  if (!app) return;
  editingApp = app;
  const name = app.display_name || 'Application';
  els.appEditName.value = name;
  els.appEditCurrentName.textContent = name;
  els.appEditCurrentPath.textContent = app.executable_path || '';
  els.appEditCurrentPath.title = app.executable_path || '';
  els.appEditIconInitial.textContent = appInitial(name);
  if (app.icon_url) {
    els.appEditIconImage.src = app.icon_url;
    els.appEditIconImage.classList.remove('hidden');
    els.appEditIconInitial.classList.add('hidden');
  } else {
    els.appEditIconImage.removeAttribute('src');
    els.appEditIconImage.classList.add('hidden');
    els.appEditIconInitial.classList.remove('hidden');
  }
  setAppEditError();
  els.appEditModal.classList.remove('hidden');
  els.appEditModal.setAttribute('aria-hidden', 'false');
  els.appEditName.focus();
  els.appEditName.select();
}

function closeAppEditModal() {
  if (busy) return;
  editingApp = null;
  els.appEditModal.classList.add('hidden');
  els.appEditModal.setAttribute('aria-hidden', 'true');
  setAppEditError();
}

async function saveAppEditModal() {
  if (!editingApp || busy) return;
  const name = els.appEditName.value.trim();
  if (!name) {
    setAppEditError('Enter an application name.');
    els.appEditName.focus();
    return;
  }

  setAppEditError();
  setAppsBusy(true);
  els.appEditModalSave.disabled = true;
  els.appEditModalCancel.disabled = true;
  els.appEditModalClose.disabled = true;
  try {
    await renameDesktopApp(editingApp.id, name);
    await refreshApps();
    editingApp = null;
    els.appEditModal.classList.add('hidden');
    els.appEditModal.setAttribute('aria-hidden', 'true');
  } catch (error) {
    setAppEditError(errorMessage(error));
  } finally {
    setAppsBusy(false);
    els.appEditModalSave.disabled = false;
    els.appEditModalCancel.disabled = false;
    els.appEditModalClose.disabled = false;
  }
}

async function relocateApp(id) {
  await runMutation(() => relocateDesktopApp(id));
}

async function changeAppIcon(id) {
  await runMutation(() => importDesktopAppIcon(id));
}

async function removeAppIcon(id) {
  const app = apps.find((item) => Number(item.id) === id);
  if (!app?.icon_url) return;
  const confirmed = await showConfirmation({
    title: 'Reset application icon?',
    message: `The custom icon for “${app.display_name}” will be removed.`,
    confirmLabel: 'Reset icon',
  });
  if (!confirmed) return;
  await runMutation(() => deleteDesktopAppIcon(id));
}

async function removeApp(id) {
  const app = apps.find((item) => Number(item.id) === id);
  if (!app) return;
  const confirmed = await showConfirmation({
    title: 'Remove application?',
    message: `“${app.display_name}” will be removed from Something. The application and its files will stay on your device.`,
    confirmLabel: 'Remove application',
  });
  if (!confirmed) return;
  await runMutation(() => deleteDesktopApp(id));
}

function handleAppsClick(event) {
  const button = event.target.closest('[data-app-action][data-app-id]');
  if (!button || !els.appsList?.contains(button) || busy) return;
  const id = Number(button.dataset.appId);
  if (!Number.isInteger(id) || id <= 0) return;

  switch (button.dataset.appAction) {
    case 'launch':
      void openApp(id);
      break;
    case 'rename':
      openAppEditModal(id);
      break;
    case 'relocate':
      void relocateApp(id);
      break;
    case 'icon':
      void changeAppIcon(id);
      break;
    case 'remove-icon':
      void removeAppIcon(id);
      break;
    case 'delete':
      void removeApp(id);
      break;
  }
}

export function initDesktopApps() {
  if (hasWailsBinding() && window.runtime?.EventsOn) {
    window.runtime.EventsOn(DESKTOP_APPS_CHANGED_EVENT, () => {
      void loadDesktopApps();
    });
  }
  els.appsAdd?.addEventListener('click', () => void addApp());
  els.appsList?.addEventListener('click', handleAppsClick);
  els.appEditModalClose?.addEventListener('click', closeAppEditModal);
  els.appEditModalCancel?.addEventListener('click', closeAppEditModal);
  els.appEditModalBackdrop?.addEventListener('click', closeAppEditModal);
  els.appEditModalSave?.addEventListener('click', () => void saveAppEditModal());
  els.appEditName?.addEventListener('keydown', (event) => {
    if (event.key === 'Enter') {
      event.preventDefault();
      void saveAppEditModal();
    }
  });
  els.appEditName?.addEventListener('input', () => {
    if (!editingApp) return;
    const previewName = els.appEditName.value.trim() || 'Application';
    els.appEditCurrentName.textContent = previewName;
    if (!editingApp.icon_url) {
      els.appEditIconInitial.textContent = appInitial(previewName);
    }
  });
  els.appEditIconImage?.addEventListener('error', () => {
    els.appEditIconImage.classList.add('hidden');
    els.appEditIconInitial.classList.remove('hidden');
  });
  document.addEventListener('keydown', (event) => {
    if (event.key === 'Escape' && !els.appEditModal?.classList.contains('hidden')) {
      closeAppEditModal();
    }
  });
}
