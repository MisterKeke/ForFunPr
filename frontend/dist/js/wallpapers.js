import { els } from './dom.js';
import {
  deleteUserWallpaper,
  getWallpaperSettings,
  importWallpaper,
  selectWallpaper,
} from './api.js';
import { escapeHtml, hasWailsBinding } from './utils.js';

const STORAGE_KEY = 'selectedWallpaper';
const DEFAULT_SELECTION = 'builtin:hu-tao';
const BUILTIN_WALLPAPERS = new Set(['original', 'sandrone', 'hu-tao', 'skirk']);

let selectedWallpaper = DEFAULT_SELECTION;
let userWallpapers = [];

function normalizeCachedSelection(value) {
  const saved = String(value || '').trim();
  if (BUILTIN_WALLPAPERS.has(saved)) return `builtin:${saved}`;
  if (saved.startsWith('builtin:') && BUILTIN_WALLPAPERS.has(saved.slice(8))) return saved;
  if (/^custom:[a-f0-9]{32}$/.test(saved)) return saved;
  return DEFAULT_SELECTION;
}

function readCachedSelection() {
  try {
    return normalizeCachedSelection(localStorage.getItem(STORAGE_KEY));
  } catch {
    return DEFAULT_SELECTION;
  }
}

function cacheSelection(selection) {
  try {
    localStorage.setItem(STORAGE_KEY, selection);
  } catch {
    // SQLite remains authoritative when local browser storage is unavailable.
  }
}

function findUserWallpaper(selection) {
  if (!selection.startsWith('custom:')) return null;
  const id = selection.slice(7);
  return userWallpapers.find((wallpaper) => wallpaper.id === id) || null;
}

function selectionAvailable(selection) {
  if (selection.startsWith('builtin:')) {
    return BUILTIN_WALLPAPERS.has(selection.slice(8));
  }
  return Boolean(findUserWallpaper(selection));
}

function applyWallpaper(selection) {
  const effectiveSelection = selectionAvailable(selection) ? selection : DEFAULT_SELECTION;
  const customWallpaper = findUserWallpaper(effectiveSelection);

  if (customWallpaper) {
    document.body.dataset.wallpaper = 'custom';
    document.body.style.setProperty(
      '--wallpaper-image',
      `url("${customWallpaper.url}")`
    );
  } else {
    document.body.style.removeProperty('--wallpaper-image');
    document.body.dataset.wallpaper = effectiveSelection.slice(8);
  }

  selectedWallpaper = effectiveSelection;
  document.querySelectorAll('[data-wallpaper-key]').forEach((button) => {
    const active = button.dataset.wallpaperKey === effectiveSelection;
    button.classList.toggle('active', active);
    button.setAttribute('aria-pressed', String(active));
  });
}

function showWallpaperError(message = '') {
  if (!els.wallpaperError) return;
  els.wallpaperError.textContent = message;
  els.wallpaperError.classList.toggle('hidden', !message);
}

function setWallpaperBusy(busy) {
  if (els.wallpaperUpload) {
    els.wallpaperUpload.disabled = busy;
    els.wallpaperUpload.textContent = busy ? 'Importing...' : 'Upload wallpaper';
  }
}

function renderUserWallpapers() {
  if (!els.userWallpaperOptions || !els.userWallpaperEmpty) return;

  els.userWallpaperEmpty.classList.toggle('hidden', userWallpapers.length > 0);
  els.userWallpaperOptions.innerHTML = userWallpapers.map((wallpaper) => `
    <article class="user-wallpaper-card">
      <button
        class="wallpaper-btn user-wallpaper-btn"
        type="button"
        data-wallpaper-key="custom:${escapeHtml(wallpaper.id)}"
        aria-pressed="false"
      >
        <span class="wallpaper-swatch" data-wallpaper-preview="${escapeHtml(wallpaper.id)}" aria-hidden="true"></span>
        <span class="wallpaper-label">
          <strong>${escapeHtml(wallpaper.display_name || 'Uploaded wallpaper')}</strong>
          <small>Uploaded image</small>
        </span>
        <span class="wallpaper-check" aria-hidden="true">&#10003;</span>
      </button>
      <button
        class="user-wallpaper-remove"
        type="button"
        data-wallpaper-delete="${escapeHtml(wallpaper.id)}"
        aria-label="Delete ${escapeHtml(wallpaper.display_name || 'uploaded wallpaper')}"
      >Delete</button>
    </article>
  `).join('');

  els.userWallpaperOptions.querySelectorAll('[data-wallpaper-preview]').forEach((preview) => {
    const wallpaper = userWallpapers.find((item) => item.id === preview.dataset.wallpaperPreview);
    if (wallpaper) preview.style.backgroundImage = `url("${wallpaper.url}")`;
  });
  applyWallpaper(selectedWallpaper);
}

function applySettings(settings) {
  userWallpapers = Array.isArray(settings?.user_wallpapers) ? settings.user_wallpapers : [];
  selectedWallpaper = normalizeCachedSelection(settings?.selected || DEFAULT_SELECTION);
  renderUserWallpapers();
  applyWallpaper(selectedWallpaper);
  cacheSelection(selectedWallpaper);
}

async function chooseWallpaper(selection) {
  if (!selectionAvailable(selection)) return;

  const previousSelection = selectedWallpaper;
  showWallpaperError('');
  applyWallpaper(selection);
  cacheSelection(selection);

  if (!hasWailsBinding()) return;
  try {
    const settings = await selectWallpaper(selection);
    applySettings(settings);
  } catch (error) {
    applyWallpaper(previousSelection);
    cacheSelection(previousSelection);
    showWallpaperError(error?.message || String(error));
  }
}

async function loadWallpaperSettings() {
  if (!hasWailsBinding()) return;

  try {
    let settings = await getWallpaperSettings();
    userWallpapers = Array.isArray(settings?.user_wallpapers) ? settings.user_wallpapers : [];

    // Migrate the previous built-in/local custom choice once when the SQLite
    // setting has not yet been created.
    const cachedSelection = readCachedSelection();
    if (!settings?.selection_saved) {
      const migrationSelection = selectionAvailable(cachedSelection)
        ? cachedSelection
        : DEFAULT_SELECTION;
      settings = await selectWallpaper(migrationSelection);
    }
    applySettings(settings);
  } catch (error) {
    showWallpaperError(error?.message || String(error));
  }
}

async function uploadWallpaper() {
  showWallpaperError('');
  setWallpaperBusy(true);
  try {
    const wallpaper = await importWallpaper();
    if (!wallpaper) return;
    const settings = await getWallpaperSettings();
    applySettings(settings);
  } catch (error) {
    showWallpaperError(error?.message || String(error));
  } finally {
    setWallpaperBusy(false);
  }
}

async function removeWallpaper(id) {
  const wallpaper = userWallpapers.find((item) => item.id === id);
  if (!wallpaper) return;
  if (!window.confirm(`Delete "${wallpaper.display_name}" from this device?`)) return;

  showWallpaperError('');
  try {
    const settings = await deleteUserWallpaper(id);
    applySettings(settings);
  } catch (error) {
    showWallpaperError(error?.message || String(error));
  }
}

export function initWallpapers() {
  document.querySelectorAll('.wallpaper-btn[data-wallpaper]').forEach((button) => {
    button.dataset.wallpaperKey = `builtin:${button.dataset.wallpaper}`;
    button.addEventListener('click', () => {
      void chooseWallpaper(button.dataset.wallpaperKey);
    });
  });

  selectedWallpaper = readCachedSelection();
  applyWallpaper(selectedWallpaper);

  if (els.wallpaperUpload) {
    if (hasWailsBinding()) {
      els.wallpaperUpload.addEventListener('click', () => void uploadWallpaper());
    } else {
      els.wallpaperUpload.disabled = true;
      els.wallpaperUpload.title = 'Run the desktop app to upload wallpapers.';
    }
  }

  els.userWallpaperOptions?.addEventListener('click', (event) => {
    const deleteButton = event.target.closest('[data-wallpaper-delete]');
    if (deleteButton) {
      void removeWallpaper(deleteButton.dataset.wallpaperDelete);
      return;
    }
    const wallpaperButton = event.target.closest('[data-wallpaper-key]');
    if (wallpaperButton) void chooseWallpaper(wallpaperButton.dataset.wallpaperKey);
  });

  void loadWallpaperSettings();
}
