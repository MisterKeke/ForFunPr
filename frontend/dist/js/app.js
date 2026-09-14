import { initNavigation } from './navigation.js';
import { initCurrency } from './currency.js';
import { initFavorites, loadFavorites } from './favorites.js';
import { initTodos, loadTodos } from './todos.js';
import { initTelegram } from './telegram.js';
import { initYoutube } from './youtube.js';
import { initFavoriteCategoryModal } from './favoriteCategories.js';
import { initCalendar } from './calendar.js';
import { initDashboard, loadDashboard } from './dashboard.js';
import { initWeather } from './weather.js';
import { initRunningApps } from './runningApps.js';
import { initWallpapers } from './wallpapers.js';
import { initMCPServer } from './mcp.js';
import { initFileExplorer } from './fileExplorer.js';
import { initContextMenu } from './contextMenu.js';
import { initNotes } from './notes.js';
import { initBookmarks } from './bookmarks.js';
import { initDesktopApps } from './apps.js';
import { initGames, initializeSteamGames } from './games.js';
import { initSetups, loadSetups } from './setups.js';
import { initUtilities } from './utilities.js';
import { initWebsiteSearch } from './websiteSearch.js';
import { initGitWorkspaces } from './gitWorkspaces.js';
import { showError } from './ui.js';

const BACKEND_READY_TIMEOUT_MS = 30000;
const BACKEND_READY_POLL_MS = 100;

function wait(delay) {
  return new Promise((resolve) => window.setTimeout(resolve, delay));
}

async function waitForBackendReady() {
  const app = window.go?.backend?.App;
  if (!app?.GetStartupStatus) return null;

  const deadline = Date.now() + BACKEND_READY_TIMEOUT_MS;
  let lastError = null;
  while (Date.now() < deadline) {
    let status;
    try {
      status = await app.GetStartupStatus();
      lastError = null;
    } catch (error) {
      lastError = error;
      await wait(BACKEND_READY_POLL_MS);
      continue;
    }
    if (status?.ready) return status;
    if (status?.error) throw new Error(status.error);
    await wait(BACKEND_READY_POLL_MS);
  }

  if (lastError) console.warn('The last backend readiness check failed.', lastError);
  throw new Error('The application backend did not become ready. Restart Something and check the local logs.');
}

function disableFeature(selector, warning) {
  document.querySelectorAll(selector).forEach((root) => {
    root.dataset.capabilityUnavailable = 'true';
    root.title = warning || 'This feature is unavailable.';
    if (root.matches('button, input, select, textarea')) root.disabled = true;
    root.querySelectorAll('button, input, select, textarea').forEach((control) => { control.disabled = true; });
  });
}

function disableRunningAppsFeature(selector, warning) {
  const message = warning || 'Running taskbar applications are supported only on Windows.';
  disableFeature(selector, message);
  document.querySelectorAll(selector).forEach((root) => {
    root.dispatchEvent(new CustomEvent('something:capability-unavailable', {
      detail: { name: 'running_apps', warning: message },
    }));
  });
}

function applyStartupCapabilities(status) {
  if (!status) return;
  window.somethingCapabilities = status.capabilities || {};
  const mappings = {
    wallpaper: '#wallpaper-upload, [data-wallpaper-key^="custom:"], [data-wallpaper-delete]',
    clipboard: '#clipboard-toggle, #clipboard-save-settings, #clipboard-list [data-action="copy"]',
    screenshots: '#utility-screenshots',
    file_shell: '.file-explorer-row.file .file-explorer-entry-open, .file-explorer-delete, [data-git-action="folder"]',
    launcher: '[data-app-action="launch"], [data-setup-action="start"], #git-workspace-editor, [data-git-action="editor"]',
    running_apps: '#running-apps-card',
    setup_icons: '#setup-icon-upload',
    desktop_app_icons: '[data-app-action="icon"], [data-app-action="remove-icon"]',
    desktop_api: '#mcp-toggle',
    git_workspaces: '#view-git-workspaces',
  };
  const unavailable = Object.entries(mappings).filter(([name]) => {
    const capability = status.capabilities?.[name];
    return capability && !capability.available;
  });
  const runningAppsUnavailable = unavailable.find(([name]) => name === 'running_apps');
  if (runningAppsUnavailable) {
    const [, selector] = runningAppsUnavailable;
    disableRunningAppsFeature(selector, status.capabilities.running_apps.warning);
  }

  const dynamicUnavailable = unavailable.filter(([name]) => name !== 'running_apps');
  const applyUnavailable = () => dynamicUnavailable.forEach(([name, selector]) => {
    disableFeature(selector, status.capabilities[name].warning);
  });
  applyUnavailable();
  if (dynamicUnavailable.length > 0) {
    new MutationObserver(applyUnavailable).observe(document.body, { childList: true, subtree: true });
  }
  (status.warnings || []).forEach((warning) => console.warn(`Something capability: ${warning}`));
}

async function startApplication() {
  let startupStatus;
  try {
    startupStatus = await waitForBackendReady();
  } catch (error) {
    console.error('Application backend startup failed.', error);
    showError(error?.message || 'The application backend could not be started.');
    return;
  }

  applyStartupCapabilities(startupStatus);

  initWallpapers();
  initNavigation();
  initCalendar();
  initCurrency();
  initFavorites();
  initTodos();
  initFavoriteCategoryModal();
  initTelegram();
  initYoutube();
  initDashboard();
  initWeather();
  initRunningApps();
  initMCPServer();
  initFileExplorer();
  initContextMenu();
  initNotes();
  initBookmarks();
  initDesktopApps();
  initGames();
  initSetups();
  initUtilities();
  initWebsiteSearch();
  initGitWorkspaces();

  // Initial data load
  void loadDashboard();
  void loadFavorites();
  void loadTodos();
  void initializeSteamGames();
  void loadSetups({ refreshApps: true });
}

document.addEventListener('DOMContentLoaded', () => {
  void startApplication();
});
