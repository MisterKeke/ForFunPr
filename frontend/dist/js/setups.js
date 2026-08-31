import { launchDesktopApp, listDesktopApps } from './api.js';
import { els } from './dom.js';
import { escapeHtml } from './utils.js';
import { showConfirmation } from './ui.js';
import {
  backendStartSetup,
  createSetupRecord,
  deleteSetupRecord,
  listSetupRecords,
  updateSetupRecord,
} from './setupStore.js';

const SETUPS_CHANGED_EVENT = 'setups:changed';
const MAXIMUM_ICON_BYTES = 5 * 1024 * 1024;
const ACCEPTED_ICON_TYPES = new Set(['image/jpeg', 'image/png', 'image/webp', 'image/x-icon', 'image/vnd.microsoft.icon']);
const ACCEPTED_ICON_EXTENSIONS = new Set(['ico', 'jpg', 'jpeg', 'png', 'webp']);

let setups = [];
let apps = [];
let busy = false;
let editingSetupID = null;
let selectedAppIDs = new Set();
let existingIconURL = '';
let draftIconDataURL = '';
let iconRemoved = false;
let loadRequest = 0;

function errorMessage(error) {
  return error?.message || String(error || 'The setup action could not be completed.');
}

function normalizeSetup(value) {
  const nestedApps = Array.isArray(value?.apps) ? value.apps : [];
  const appIDs = Array.isArray(value?.app_ids)
    ? value.app_ids
    : nestedApps.map((app) => app?.id);
  return {
    ...value,
    id: Number(value?.id),
    name: String(value?.name || 'Setup'),
    description: String(value?.description || ''),
    app_ids: [...new Set(appIDs.map(Number).filter((id) => Number.isInteger(id) && id > 0))],
    icon_url: String(value?.icon_url || ''),
    icon_data_url: String(value?.icon_data_url || ''),
  };
}

function setupInitial(name) {
  return Array.from(String(name || '').trim())[0]?.toUpperCase() || 'S';
}

function appInitial(name) {
  return Array.from(String(name || '').trim())[0]?.toUpperCase() || 'A';
}

function setupIconURL(setup) {
  return String(setup?.icon_url || setup?.icon_data_url || '');
}

function setupIconMarkup(setup, extraClass = '') {
  const iconURL = setupIconURL(setup);
  const className = `setup-icon${iconURL ? ' has-image' : ''}${extraClass ? ` ${extraClass}` : ''}`;
  const content = iconURL
    ? `<img src="${escapeHtml(iconURL)}" alt="" />`
    : `<span>${escapeHtml(setupInitial(setup?.name))}</span>`;
  return `<span class="${className}" aria-hidden="true">${content}</span>`;
}

function appIconMarkup(app) {
  const name = app?.display_name || 'Application';
  return app?.icon_url
    ? `<img src="${escapeHtml(app.icon_url)}" alt="" />`
    : `<span>${escapeHtml(appInitial(name))}</span>`;
}

function appsByID() {
  return new Map(apps.map((app) => [Number(app.id), app]));
}

function setupAppSummary(setup) {
  const catalog = appsByID();
  const names = setup.app_ids.map((id) => catalog.get(id)?.display_name || 'Removed app');
  return names.join(' · ') || 'No applications selected';
}

function setPageError(message = '') {
  if (!els.setupsError) return;
  els.setupsError.textContent = message;
  els.setupsError.classList.toggle('hidden', !message);
}

function setPageStatus(message = '', tone = 'info') {
  [els.setupsStatus, els.dashboardSetupsStatus].forEach((element) => {
    if (!element) return;
    element.textContent = message;
    element.dataset.tone = tone;
    element.classList.toggle('hidden', !message);
  });
}

function setModalError(message = '') {
  if (!els.setupModalError) return;
  els.setupModalError.textContent = message;
  els.setupModalError.classList.toggle('hidden', !message);
}

function setBusy(nextBusy) {
  busy = nextBusy;
  els.setupsList?.setAttribute('aria-busy', String(nextBusy));
  els.setupsLoading?.classList.toggle('hidden', !nextBusy || setups.length > 0);
  [els.setupsCreate, els.setupsEmptyCreate, els.setupModalSave].forEach((button) => {
    if (button) button.disabled = nextBusy;
  });
  els.setupsList?.querySelectorAll('button').forEach((button) => {
    button.disabled = nextBusy;
  });
  els.dashboardSetups?.querySelectorAll('button').forEach((button) => {
    button.disabled = nextBusy;
  });
}

function renderSetups() {
  if (!els.setupsList || !els.setupsEmpty) return;
  const hasSetups = setups.length > 0;
  els.setupsList.classList.toggle('hidden', !hasSetups);
  els.setupsEmpty.classList.toggle('hidden', hasSetups);

  const catalog = appsByID();
  els.setupsList.innerHTML = setups.map((setup) => {
    const selectedApps = setup.app_ids.map((id) => catalog.get(id)).filter(Boolean);
    const missingCount = setup.app_ids.length - selectedApps.length;
    const appChips = selectedApps.slice(0, 4).map((app) => `
      <span class="setup-app-chip">
        <span class="setup-app-chip-icon${app.icon_url ? ' has-image' : ''}" aria-hidden="true">${appIconMarkup(app)}</span>
        <span>${escapeHtml(app.display_name || 'Application')}</span>
      </span>
    `).join('');
    const extraCount = Math.max(0, selectedApps.length - 4);
    return `
      <article class="setup-card" role="listitem">
        <div class="setup-card-heading">
          ${setupIconMarkup(setup, 'setup-card-icon')}
          <div class="setup-card-copy">
            <h2>${escapeHtml(setup.name)}</h2>
            <p>${escapeHtml(setup.description || setupAppSummary(setup))}</p>
          </div>
          <span class="setup-app-count">${setup.app_ids.length} ${setup.app_ids.length === 1 ? 'app' : 'apps'}</span>
        </div>
        <div class="setup-card-apps" aria-label="Applications in ${escapeHtml(setup.name)}">
          ${appChips}
          ${extraCount ? `<span class="setup-app-more">+${extraCount} more</span>` : ''}
          ${missingCount ? `<span class="setup-app-missing">${missingCount} removed</span>` : ''}
        </div>
        <div class="setup-card-actions">
          <button class="primary-btn setup-start" type="button" data-setup-action="start" data-setup-id="${setup.id}">Start</button>
          <button class="secondary-btn" type="button" data-setup-action="edit" data-setup-id="${setup.id}">Edit</button>
          <button class="secondary-btn setup-delete" type="button" data-setup-action="delete" data-setup-id="${setup.id}">Delete</button>
        </div>
      </article>
    `;
  }).join('');
  renderDashboardSetups();
  setBusy(busy);
}

function renderDashboardSetups() {
  if (!els.dashboardSetups) return;
  if (setups.length === 0) {
    els.dashboardSetups.innerHTML = `
      <div class="dashboard-setup-empty">
        <span>No setups created yet.</span>
        <button class="secondary-btn" type="button" data-setup-action="create">Create one</button>
      </div>
    `;
    return;
  }

  els.dashboardSetups.innerHTML = setups.map((setup) => `
    <article class="dashboard-setup" role="listitem">
      ${setupIconMarkup(setup, 'dashboard-setup-icon')}
      <span class="dashboard-setup-copy">
        <strong>${escapeHtml(setup.name)}</strong>
        <small>${escapeHtml(setupAppSummary(setup))}</small>
      </span>
      <button class="primary-btn" type="button" data-setup-action="start" data-setup-id="${setup.id}">Start</button>
    </article>
  `).join('');
  setBusy(busy);
}

async function refreshApps() {
  const result = await listDesktopApps();
  apps = Array.isArray(result) ? result : [];
}

export async function loadSetups({ refreshApps: shouldRefreshApps = false } = {}) {
  const requestID = ++loadRequest;
  setPageError('');
  setBusy(true);
  try {
    const operations = [listSetupRecords()];
    if (shouldRefreshApps || apps.length === 0) operations.push(listDesktopApps());
    const [setupResult, appResult] = await Promise.all(operations);
    if (requestID !== loadRequest) return;
    setups = (Array.isArray(setupResult) ? setupResult : []).map(normalizeSetup);
    if (appResult !== undefined) apps = Array.isArray(appResult) ? appResult : [];
    renderSetups();
  } catch (error) {
    if (requestID !== loadRequest) return;
    setPageError(errorMessage(error));
    renderSetups();
  } finally {
    if (requestID === loadRequest) setBusy(false);
  }
}

function updateIconPreview() {
  if (!els.setupIconPreviewImage || !els.setupIconPreviewInitial || !els.setupIconPreview) return;
  const iconURL = draftIconDataURL || (iconRemoved ? '' : existingIconURL);
  els.setupIconPreview.classList.toggle('has-image', Boolean(iconURL));
  els.setupIconPreviewImage.classList.toggle('hidden', !iconURL);
  els.setupIconPreviewInitial.classList.toggle('hidden', Boolean(iconURL));
  els.setupIconPreviewImage.src = iconURL || '';
  els.setupIconPreviewInitial.textContent = setupInitial(els.setupName?.value);
  els.setupIconReset?.classList.toggle('hidden', !iconURL);
}

function renderAppOptions() {
  if (!els.setupAppOptions || !els.setupAppOptionsEmpty) return;
  const catalog = appsByID();
  const missingIDs = [...selectedAppIDs].filter((id) => !catalog.has(id));
  els.setupAppOptionsEmpty.classList.toggle('hidden', apps.length > 0 || missingIDs.length > 0);
  els.setupAppOptions.classList.toggle('hidden', apps.length === 0 && missingIDs.length === 0);

  const availableOptions = apps.map((app) => {
    const id = Number(app.id);
    const checked = selectedAppIDs.has(id);
    const name = app.display_name || 'Application';
    return `
      <label class="setup-app-option${checked ? ' is-selected' : ''}">
        <input type="checkbox" value="${id}" data-setup-app-id="${id}" ${checked ? 'checked' : ''} />
        <span class="setup-app-option-icon${app.icon_url ? ' has-image' : ''}" aria-hidden="true">${appIconMarkup(app)}</span>
        <span class="setup-app-option-copy">
          <strong>${escapeHtml(name)}</strong>
          <small>${app.available ? 'Ready to launch' : 'Executable not found'}</small>
        </span>
        <span class="setup-app-option-check" aria-hidden="true">&#10003;</span>
      </label>
    `;
  });
  const missingOptions = missingIDs.map((id) => `
    <label class="setup-app-option is-selected is-missing">
      <input type="checkbox" value="${id}" data-setup-app-id="${id}" checked />
      <span class="setup-app-option-icon" aria-hidden="true">?</span>
      <span class="setup-app-option-copy">
        <strong>Removed application</strong>
        <small>Uncheck it to remove it from this setup</small>
      </span>
      <span class="setup-app-option-check" aria-hidden="true">&#10003;</span>
    </label>
  `);
  els.setupAppOptions.innerHTML = [...availableOptions, ...missingOptions].join('');
  updateSelectionCount();
}

function updateSelectionCount() {
  if (!els.setupAppSelectionCount) return;
  const count = selectedAppIDs.size;
  els.setupAppSelectionCount.textContent = `${count} selected`;
}

async function openSetupModal(setup = null) {
  if (!els.setupModal || busy) return;
  editingSetupID = setup ? Number(setup.id) : null;
  selectedAppIDs = new Set(setup?.app_ids || []);
  existingIconURL = setupIconURL(setup);
  draftIconDataURL = '';
  iconRemoved = false;
  setModalError('');

  els.setupModalHeading.textContent = setup ? 'Edit setup' : 'Create setup';
  els.setupModalSave.textContent = setup ? 'Save changes' : 'Save setup';
  els.setupName.value = setup?.name || '';
  els.setupDescription.value = setup?.description || '';
  els.setupIconFile.value = '';
  els.setupModal.classList.remove('hidden');
  els.setupModal.setAttribute('aria-hidden', 'false');
  updateIconPreview();
  renderAppOptions();
  els.setupName.focus();

  els.setupAppOptionsLoading?.classList.remove('hidden');
  try {
    await refreshApps();
    renderAppOptions();
  } catch (error) {
    setModalError(errorMessage(error));
  } finally {
    els.setupAppOptionsLoading?.classList.add('hidden');
  }
}

function closeSetupModal() {
  if (!els.setupModal || busy) return;
  els.setupModal.classList.add('hidden');
  els.setupModal.setAttribute('aria-hidden', 'true');
  editingSetupID = null;
  selectedAppIDs = new Set();
  existingIconURL = '';
  draftIconDataURL = '';
  iconRemoved = false;
  setModalError('');
}

function readFileAsDataURL(file) {
  return new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = () => resolve(String(reader.result || ''));
    reader.onerror = () => reject(reader.error || new Error('Could not read the selected icon.'));
    reader.readAsDataURL(file);
  });
}

function validIconFile(file) {
  const extension = String(file?.name || '').split('.').pop()?.toLowerCase() || '';
  return ACCEPTED_ICON_TYPES.has(String(file?.type || '').toLowerCase()) || ACCEPTED_ICON_EXTENSIONS.has(extension);
}

async function selectIcon(file) {
  if (!file) return;
  setModalError('');
  if (!validIconFile(file)) {
    setModalError('Choose an ICO, JPEG, PNG, or WebP image.');
    return;
  }
  if (file.size <= 0 || file.size > MAXIMUM_ICON_BYTES) {
    setModalError('Setup icons must be between 1 byte and 5 MB.');
    return;
  }
  try {
    draftIconDataURL = await readFileAsDataURL(file);
    iconRemoved = false;
    updateIconPreview();
  } catch (error) {
    setModalError(errorMessage(error));
  }
}

function resetIcon() {
  draftIconDataURL = '';
  iconRemoved = true;
  if (els.setupIconFile) els.setupIconFile.value = '';
  updateIconPreview();
}

async function saveSetup() {
  if (busy) return;
  const wasEditing = Boolean(editingSetupID);
  const name = String(els.setupName?.value || '').trim();
  const description = String(els.setupDescription?.value || '').trim();
  if (!name) {
    setModalError('Enter a setup name.');
    els.setupName?.focus();
    return;
  }
  if (selectedAppIDs.size === 0) {
    setModalError('Choose at least one application.');
    els.setupAppOptions?.scrollIntoView({ block: 'nearest' });
    return;
  }

  setModalError('');
  setBusy(true);
  try {
    const request = {
      id: editingSetupID || 0,
      name,
      description,
      app_ids: [...selectedAppIDs],
      icon_data_url: draftIconDataURL,
      remove_icon: iconRemoved,
    };
    if (editingSetupID) {
      await updateSetupRecord(request);
    } else {
      await createSetupRecord(request);
    }
    setBusy(false);
    closeSetupModal();
    setPageStatus(wasEditing ? 'Setup updated.' : 'Setup created.', 'success');
    await loadSetups();
  } catch (error) {
    setModalError(errorMessage(error));
    setBusy(false);
  }
}

async function removeSetup(setup) {
  const confirmed = await showConfirmation({
    title: 'Delete setup?',
    message: `“${setup.name}” will be removed. Its applications will remain in Apps.`,
    confirmLabel: 'Delete setup',
  });
  if (!confirmed) return;

  setPageError('');
  setBusy(true);
  try {
    await deleteSetupRecord(setup.id);
    setPageStatus(`${setup.name} was deleted.`, 'success');
    await loadSetups();
  } catch (error) {
    setPageError(errorMessage(error));
  } finally {
    setBusy(false);
  }
}

function launchResultMessage(setup, result) {
  const failures = Array.isArray(result?.failures) ? result.failures : [];
  const launched = Number(result?.launched ?? result?.launched_count ?? 0);
  if (failures.length === 0) return `${setup.name} started — ${launched} ${launched === 1 ? 'application' : 'applications'} launched.`;
  return `${setup.name}: ${launched} launched, ${failures.length} failed.`;
}

async function startSetup(setup) {
  if (busy) return;
  setPageError('');
  setPageStatus(`Starting ${setup.name}…`, 'info');
  setBusy(true);
  try {
    let result;
    const backendRequest = backendStartSetup(setup.id);
    if (backendRequest) {
      result = await backendRequest;
    } else {
      const failures = [];
      let launched = 0;
      for (const appID of setup.app_ids) {
        try {
          await launchDesktopApp(appID);
          launched += 1;
        } catch (error) {
          failures.push({ app_id: appID, error: errorMessage(error) });
        }
      }
      result = { launched, failures };
    }
    const hasFailures = Array.isArray(result?.failures) && result.failures.length > 0;
    setPageStatus(launchResultMessage(setup, result), hasFailures ? 'warning' : 'success');
  } catch (error) {
    const message = errorMessage(error);
    setPageError(message);
    setPageStatus(message, 'error');
  } finally {
    setBusy(false);
  }
}

function openView(name) {
  document.querySelector(`.menu-btn[data-view="${name}"]`)?.click();
}

function handleSetupAction(event) {
  const button = event.target.closest('[data-setup-action]');
  if (!button || busy) return;
  const action = button.dataset.setupAction;
  if (action === 'create') {
    void openSetupModal();
    return;
  }
  const id = Number(button.dataset.setupId);
  const setup = setups.find((item) => item.id === id);
  if (!setup) return;
  if (action === 'start') void startSetup(setup);
  if (action === 'edit') void openSetupModal(setup);
  if (action === 'delete') void removeSetup(setup);
}

export function initSetups() {
  if (window.runtime?.EventsOn) {
    window.runtime.EventsOn(SETUPS_CHANGED_EVENT, () => {
      void loadSetups({ refreshApps: true });
    });
  }
  els.setupsCreate?.addEventListener('click', () => void openSetupModal());
  els.setupsEmptyCreate?.addEventListener('click', () => void openSetupModal());
  els.setupsList?.addEventListener('click', handleSetupAction);
  els.dashboardSetups?.addEventListener('click', handleSetupAction);
  els.dashboardSetupsManage?.addEventListener('click', () => openView('setups'));
  els.setupModalClose?.addEventListener('click', closeSetupModal);
  els.setupModalCancel?.addEventListener('click', closeSetupModal);
  els.setupModalBackdrop?.addEventListener('click', closeSetupModal);
  els.setupModalSave?.addEventListener('click', () => void saveSetup());
  els.setupOpenApps?.addEventListener('click', () => {
    closeSetupModal();
    openView('apps');
  });
  els.setupIconUpload?.addEventListener('click', () => els.setupIconFile?.click());
  els.setupIconReset?.addEventListener('click', resetIcon);
  els.setupIconFile?.addEventListener('change', () => void selectIcon(els.setupIconFile.files?.[0]));
  els.setupName?.addEventListener('input', updateIconPreview);
  els.setupAppOptions?.addEventListener('change', (event) => {
    const checkbox = event.target.closest('[data-setup-app-id]');
    if (!checkbox) return;
    const id = Number(checkbox.dataset.setupAppId);
    if (checkbox.checked) selectedAppIDs.add(id);
    else selectedAppIDs.delete(id);
    renderAppOptions();
  });

  document.addEventListener('desktop-apps:changed', (event) => {
    apps = Array.isArray(event.detail?.apps) ? event.detail.apps : [];
    renderSetups();
    if (!els.setupModal?.classList.contains('hidden')) renderAppOptions();
  });
  document.addEventListener('keydown', (event) => {
    if (event.key === 'Escape' && !els.setupModal?.classList.contains('hidden')) closeSetupModal();
  });
}
