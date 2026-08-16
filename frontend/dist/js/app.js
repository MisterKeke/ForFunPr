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

  // Initial data load
  loadDashboard();
  loadFavorites();
  loadTodos();
  void initializeSteamGames();
});
