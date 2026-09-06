import { els } from './dom.js';
import { hideError } from './ui.js';
import { loadTelegramFavorites, loadTelegramPosts } from './telegram.js';
import { loadYouTubeFavorites, loadYouTubeVideos } from './youtube.js';
import { loadUserLocationWeather } from './weather.js';
import { refreshMCPServerStatus } from './mcp.js';
import { loadFileExplorer } from './fileExplorer.js';
import { loadNotes, flushNoteSave } from './notes.js';
import { loadBookmarks } from './bookmarks.js';
import { loadDesktopApps } from './apps.js';
import { loadSteamGames } from './games.js';
import { loadSetups } from './setups.js';
import { loadUtilities } from './utilities.js';
import { loadWebsiteSearchState } from './websiteSearch.js';

export function initNavigation() {
  // Menu buttons for switching views
  els.menuButtons.forEach((btn) => {
    btn.addEventListener("click", () => {
      switchView(btn.dataset.view);
    });
  });

  // Tab buttons inside views (currency tabs)
  els.tabs.forEach((btn) => {
    btn.addEventListener("click", () => {
      els.tabs.forEach((item) => item.classList.remove("active"));
      els.panels.forEach((panel) => panel.classList.remove("active"));
      btn.classList.add("active");
      document.getElementById(`tab-${btn.dataset.tab}`).classList.add("active");
      hideError();
    });
  });
}

export function switchView(viewName) {
  if (viewName !== 'notes' && document.getElementById('view-notes')?.classList.contains('active')) {
    void flushNoteSave();
  }
  els.menuButtons.forEach((btn) => {
    const isActive = btn.dataset.view === viewName;
    btn.classList.toggle("active", isActive);
    if (isActive) {
      btn.setAttribute('aria-current', 'page');
    } else {
      btn.removeAttribute('aria-current');
    }
  });
  els.views.forEach((view) => {
    view.classList.toggle("active", view.id === `view-${viewName}`);
  });
  hideError();

  // Toggle layout mode for views that need extra horizontal room.
  const shell = document.querySelector('.app-shell');
  if (shell) {
    if (viewName === 'telegram') {
      shell.classList.add('telegram-mode');
      shell.classList.remove('main-mode', 'youtube-mode', 'file-explorer-mode', 'utilities-mode');
    } else if (viewName === 'Youtube' || viewName === 'youtube') {
      shell.classList.add('youtube-mode');
      shell.classList.remove('main-mode', 'telegram-mode', 'file-explorer-mode', 'utilities-mode');
    } else if (viewName === 'file-explorer') {
      shell.classList.add('file-explorer-mode');
      shell.classList.remove('main-mode', 'telegram-mode', 'youtube-mode', 'utilities-mode');
    } else if (viewName === 'main') {
      shell.classList.add('main-mode');
      shell.classList.remove('telegram-mode', 'youtube-mode', 'file-explorer-mode', 'utilities-mode');
    } else if (viewName === 'utilities') {
      shell.classList.remove('main-mode', 'telegram-mode', 'youtube-mode', 'file-explorer-mode', 'utilities-mode');
    } else {
      shell.classList.remove('main-mode', 'telegram-mode', 'youtube-mode', 'file-explorer-mode', 'utilities-mode');
    }
  }

  // Automatic load when switching to Telegram
  if (viewName === 'telegram') {
    loadTelegramFavorites();
    const channel = (els.telegramChannel.value || "toporlive").trim().toUpperCase();
    if (!els.telegramPosts.querySelector('.telegram-post')) {
      loadTelegramPosts(channel);
    }
  }

  // Automatic load when switching to Youtube
  if (viewName === 'Youtube' || viewName === 'youtube') {
    // lazy-imported functions are fine; they should be imported at top of this file
    loadYouTubeFavorites();
    const channel = (els.youtubeChannel.value || "T2X2_latest_news").trim();
    if (!els.youtubeVideos.querySelector('.youtube-video')) {
      loadYouTubeVideos(channel);
    }
  }

  if (viewName === 'weather') {
    loadUserLocationWeather();
  }

  if (viewName === 'file-explorer') {
    void loadFileExplorer();
  }

  if (viewName === 'notes') {
    void loadNotes();
  }

  if (viewName === 'bookmarks') {
    void loadBookmarks();
  }

  // This deliberately reads only the SQLite cache. Price checks happen once
  // at app launch or when the user clicks the Games refresh button.
  if (viewName === 'games') {
    void loadSteamGames();
  }

  if (viewName === 'apps') {
    void loadDesktopApps();
  }

  if (viewName === 'setups') {
    void loadSetups({ refreshApps: true });
  }

  if (viewName === 'settings') {
    void refreshMCPServerStatus();
  }

  if (viewName === 'utilities') {
    void loadUtilities();
  }

  if (viewName === 'search') {
    void loadWebsiteSearchState();
  }
}
