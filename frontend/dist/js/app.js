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
      setup_icons: '#setup-icon-upload',
      desktop_app_icons: '[data-app-action="icon"], [data-app-action="remove-icon"]',
      desktop_api: '#mcp-toggle',
    };
    const unavailable = Object.entries(mappings).filter(([name]) => {
      const capability = status?.capabilities?.[name];
      return capability && !capability.available;
    });
    const applyUnavailable = () => unavailable.forEach(([name, selector]) => {
      disableFeature(selector, status.capabilities[name].warning);
    });
    applyUnavailable();
    if (unavailable.length > 0) {
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
