import {
  chooseFileExplorerFolder,
  getFileExplorerPlaces,
  listFileExplorerDirectory,
} from './api.js';
import { els } from './dom.js';

const PAGE_SIZE = 250;
const HISTORY_LIMIT = 50;

const state = {
  initialized: false,
  loadPromise: null,
  requestVersion: 0,
  busy: false,
  places: [],
  listing: null,
  entries: [],
  history: [],
  historyIndex: -1,
};

const placeIcons = {
  home: '\u2302',
  'home-secondary': '\u2302',
  desktop: '\u25a3',
  documents: '\u25a4',
  downloads: '\u2193',
  custom: '\u25c7',
};

const modifiedDateFormatter = new Intl.DateTimeFormat(undefined, {
  dateStyle: 'medium',
  timeStyle: 'short',
});

export function initFileExplorer() {
  els.fileExplorerBack?.addEventListener('click', () => void moveThroughHistory(-1));
  els.fileExplorerForward?.addEventListener('click', () => void moveThroughHistory(1));
  els.fileExplorerUp?.addEventListener('click', () => {
    if (state.listing?.can_go_up) {
      void openLocation(state.listing.root_id, state.listing.parent_path, 'push');
    }
  });
  els.fileExplorerRefresh?.addEventListener('click', () => {
    if (state.listing) {
      void openLocation(state.listing.root_id, state.listing.path, 'none');
    }
  });
  els.fileExplorerChoose?.addEventListener('click', () => void chooseFolder());
  els.fileExplorerLoadMore?.addEventListener('click', () => void loadMore());
}

export function loadFileExplorer() {
  if (state.initialized) return Promise.resolve();
  if (state.loadPromise) return state.loadPromise;

  state.loadPromise = initializeExplorer().finally(() => {
    state.loadPromise = null;
  });
  return state.loadPromise;
}

async function initializeExplorer() {
  setBusy(true);
  hideError();
  try {
    state.places = await getFileExplorerPlaces();
    renderPlaces();
    const initialPlace = state.places.find((place) => place.kind === 'home') || state.places[0];
    if (!initialPlace) {
      throw new Error('No folders are available to browse.');
    }
    const opened = await openLocation(initialPlace.root_id, '', 'replace');
    state.initialized = opened;
  } catch (error) {
    showError(error);
  } finally {
    setBusy(false);
  }
}

async function chooseFolder() {
  setBusy(true);
  hideError();
  const version = ++state.requestVersion;
  try {
    const listing = await chooseFileExplorerFolder();
    if (!listing || version !== state.requestVersion) return;
    applyListing(listing, false);
    rememberLocation(listing.root_id, listing.path, 'push');
    state.places = await getFileExplorerPlaces();
    renderPlaces();
    state.initialized = true;
  } catch (error) {
    showError(error);
  } finally {
    if (version === state.requestVersion) setBusy(false);
  }
}

async function openLocation(rootID, path, historyMode) {
  const version = ++state.requestVersion;
  setBusy(true);
  hideError();
  try {
    const listing = await listFileExplorerDirectory(rootID, path, 0, PAGE_SIZE);
    if (version !== state.requestVersion) return false;
    applyListing(listing, false);
    rememberLocation(listing.root_id, listing.path, historyMode);
    state.initialized = true;
    return true;
  } catch (error) {
    if (version === state.requestVersion) showError(error);
    return false;
  } finally {
    if (version === state.requestVersion) setBusy(false);
  }
}

async function loadMore() {
  if (!state.listing?.has_more || state.busy) return;
  const version = ++state.requestVersion;
  setBusy(true);
  hideError();
  try {
    const listing = await listFileExplorerDirectory(
      state.listing.root_id,
      state.listing.path,
      state.listing.next_offset,
      PAGE_SIZE,
    );
    if (version !== state.requestVersion) return;
    applyListing(listing, true);
  } catch (error) {
    if (version === state.requestVersion) showError(error);
  } finally {
    if (version === state.requestVersion) setBusy(false);
  }
}

async function moveThroughHistory(direction) {
  const nextIndex = state.historyIndex + direction;
  if (nextIndex < 0 || nextIndex >= state.history.length || state.busy) return;
  const destination = state.history[nextIndex];
  const opened = await openLocation(destination.rootID, destination.path, 'none');
  if (opened) {
    state.historyIndex = nextIndex;
    updateControls();
  }
}

function applyListing(listing, append) {
  state.listing = listing;
  state.entries = append
    ? sortEntries([...state.entries, ...(listing.entries || [])])
    : sortEntries(listing.entries || []);
  renderPlaces();
  renderBreadcrumbs();
  renderEntries();
  renderDirectoryMeta();
  updateControls();
}

function rememberLocation(rootID, path, mode) {
  if (mode === 'none') return;
  const location = { rootID, path };
  if (mode === 'replace') {
    state.history = [location];
    state.historyIndex = 0;
    updateControls();
    return;
  }

  const current = state.history[state.historyIndex];
  if (current?.rootID === rootID && current?.path === path) return;
  state.history = state.history.slice(0, state.historyIndex + 1);
  state.history.push(location);
  if (state.history.length > HISTORY_LIMIT) state.history.shift();
  state.historyIndex = state.history.length - 1;
  updateControls();
}

function renderPlaces() {
  if (!els.fileExplorerPlaceList) return;
  els.fileExplorerPlaceList.replaceChildren();
  state.places.forEach((place) => {
    const button = document.createElement('button');
    button.type = 'button';
    button.className = 'file-explorer-place';
    button.classList.toggle('active', place.root_id === state.listing?.root_id);
    if (place.root_id === state.listing?.root_id) {
      button.setAttribute('aria-current', 'location');
    }

    const icon = document.createElement('span');
    icon.className = 'file-explorer-place-icon';
    icon.setAttribute('aria-hidden', 'true');
    icon.textContent = placeIcons[place.kind] || placeIcons.custom;

    const label = document.createElement('span');
    label.textContent = place.label;
    button.append(icon, label);
    button.addEventListener('click', () => void openLocation(place.root_id, '', 'push'));
    els.fileExplorerPlaceList.append(button);
  });
}

function renderBreadcrumbs() {
  if (!els.fileExplorerBreadcrumbs) return;
  els.fileExplorerBreadcrumbs.replaceChildren();
  const breadcrumbs = state.listing?.breadcrumbs || [];
  breadcrumbs.forEach((breadcrumb, index) => {
    if (index > 0) {
      const separator = document.createElement('span');
      separator.className = 'file-explorer-breadcrumb-separator';
      separator.setAttribute('aria-hidden', 'true');
      separator.textContent = '/';
      els.fileExplorerBreadcrumbs.append(separator);
    }

    const button = document.createElement('button');
    button.type = 'button';
    button.className = 'file-explorer-breadcrumb';
    button.textContent = breadcrumb.label;
    const isCurrent = index === breadcrumbs.length - 1;
    button.classList.toggle('current', isCurrent);
    button.setAttribute('aria-current', isCurrent ? 'page' : 'false');
    button.disabled = isCurrent;
    if (!isCurrent) {
      button.addEventListener('click', () => {
        void openLocation(state.listing.root_id, breadcrumb.path, 'push');
      });
    }
    els.fileExplorerBreadcrumbs.append(button);
  });
}

function renderEntries() {
  if (!els.fileExplorerEntries) return;
  els.fileExplorerEntries.replaceChildren();
  state.entries.forEach((entry) => {
    const row = document.createElement(entry.is_directory ? 'button' : 'div');
    if (entry.is_directory) row.type = 'button';
    row.className = `file-explorer-row ${entry.is_directory ? 'directory' : 'file'}`;
    if (!entry.is_directory) row.setAttribute('aria-disabled', 'true');

    const nameCell = document.createElement('span');
    nameCell.className = 'file-explorer-name';
    const icon = document.createElement('span');
    icon.className = `file-explorer-entry-icon ${entry.is_directory ? 'folder' : entry.is_symbolic_link ? 'link' : 'document'}`;
    icon.setAttribute('aria-hidden', 'true');
    const name = document.createElement('span');
    name.className = 'file-explorer-entry-name';
    name.textContent = entry.name;
    nameCell.append(icon, name);

    row.append(
      nameCell,
      explorerCell(entry.is_directory ? '\u2014' : formatBytes(entry.size), 'file-explorer-size'),
      explorerCell(entry.type || 'File', 'file-explorer-type'),
      explorerCell(formatModifiedAt(entry.modified_at), 'file-explorer-modified'),
    );
    if (entry.is_directory) {
      row.title = `Open ${entry.name}`;
      row.addEventListener('click', () => {
        void openLocation(state.listing.root_id, entry.path, 'push');
      });
    }
    els.fileExplorerEntries.append(row);
  });
}

function explorerCell(text, className) {
  const cell = document.createElement('span');
  cell.className = className;
  cell.textContent = text;
  return cell;
}

function renderDirectoryMeta() {
  const count = state.entries.length;
  if (els.fileExplorerSummary) {
    els.fileExplorerSummary.textContent = `${count.toLocaleString()} ${count === 1 ? 'item' : 'items'}`;
  }
  els.fileExplorerEmpty?.classList.toggle('hidden', count !== 0 || state.busy);
  els.fileExplorerLoadMore?.classList.toggle('hidden', !state.listing?.has_more);
  if (els.fileExplorerLimit) {
    const truncated = Boolean(state.listing?.truncated);
    els.fileExplorerLimit.classList.toggle('hidden', !truncated);
    els.fileExplorerLimit.textContent = truncated
      ? `Showing the first ${Number(state.listing.maximum_items || count).toLocaleString()} items.`
      : '';
  }
}

function sortEntries(entries) {
  return entries.slice().sort((left, right) => {
    if (left.is_directory !== right.is_directory) return left.is_directory ? -1 : 1;
    return String(left.name).localeCompare(String(right.name), undefined, {
      numeric: true,
      sensitivity: 'base',
    });
  });
}

function formatBytes(value) {
  const bytes = Number(value) || 0;
  if (bytes < 1024) return `${bytes} B`;
  const units = ['KB', 'MB', 'GB', 'TB'];
  let size = bytes;
  let unitIndex = -1;
  do {
    size /= 1024;
    unitIndex += 1;
  } while (size >= 1024 && unitIndex < units.length - 1);
  return `${size < 10 ? size.toFixed(1) : size.toFixed(0)} ${units[unitIndex]}`;
}

function formatModifiedAt(value) {
  if (!value) return '\u2014';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return '\u2014';
  return modifiedDateFormatter.format(date);
}

function setBusy(busy) {
  state.busy = busy;
  els.fileExplorerLoading?.classList.toggle('hidden', !busy);
  updateControls();
  if (!busy) renderDirectoryMeta();
}

function updateControls() {
  if (els.fileExplorerBack) {
    els.fileExplorerBack.disabled = state.busy || state.historyIndex <= 0;
  }
  if (els.fileExplorerForward) {
    els.fileExplorerForward.disabled = state.busy || state.historyIndex >= state.history.length - 1;
  }
  if (els.fileExplorerUp) {
    els.fileExplorerUp.disabled = state.busy || !state.listing?.can_go_up;
  }
  if (els.fileExplorerRefresh) {
    els.fileExplorerRefresh.disabled = state.busy || !state.listing;
  }
  if (els.fileExplorerChoose) els.fileExplorerChoose.disabled = state.busy;
  if (els.fileExplorerLoadMore) {
    els.fileExplorerLoadMore.disabled = state.busy;
  }
}

function hideError() {
  if (!els.fileExplorerError) return;
  els.fileExplorerError.textContent = '';
  els.fileExplorerError.classList.add('hidden');
}

function showError(error) {
  if (!els.fileExplorerError) return;
  els.fileExplorerError.textContent = error?.message || String(error || 'The folder could not be loaded.');
  els.fileExplorerError.classList.remove('hidden');
}
