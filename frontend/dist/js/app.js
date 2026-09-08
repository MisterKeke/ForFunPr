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

async function applyStartupCapabilities() {
  if (!window.go?.backend?.App?.GetStartupStatus) return;
  try {
    const status = await window.go.backend.App.GetStartupStatus();
    window.somethingCapabilities = status?.capabilities || {};
    const mappings = {
      wallpaper: '#wallpaper-upload, [data-wallpaper-key^="custom:"], [data-wallpaper-delete]',
      clipboard: '#clipboard-toggle, #clipboard-save-settings, #clipboard-list [data-action="copy"]',
      screenshots: '#utility-screenshots',
      file_shell: '.file-explorer-row.file .file-explorer-entry-open, .file-explorer-delete',
      launcher: '[data-app-action="launch"], [data-setup-action="start"]',
      running_apps: '#running-apps-card',
      setup_icons: '#setup-icon-upload',
      desktop_app_icons: '[data-app-action="icon"], [data-app-action="remove-icon"]',
      desktop_api: '#mcp-toggle',
    };
    const unavailable = Object.entries(mappings).filter(([name]) => {
      const capability = status?.capabilities?.[name];
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
    (status?.warnings || []).forEach((warning) => console.warn(`Something capability: ${warning}`));
  } catch (error) {
    console.warn('Startup capabilities could not be loaded.', error);
  }
}

document.addEventListener('DOMContentLoaded', () => {
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

  void applyStartupCapabilities();

  // Initial data load
  loadDashboard();
  loadFavorites();
  loadTodos();
  void initializeSteamGames();
  void loadSetups({ refreshApps: true });
});
