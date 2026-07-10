import { els } from './dom.js';
import { hideError } from './ui.js';
import { loadTelegramFavorites, loadTelegramPosts } from './telegram.js';
import { loadYouTubeFavorites, loadYouTubeVideos } from './youtube.js';

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
  els.menuButtons.forEach((btn) => {
    btn.classList.toggle("active", btn.dataset.view === viewName);
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
      shell.classList.remove('main-mode', 'youtube-mode');
    } else if (viewName === 'Youtube' || viewName === 'youtube') {
      shell.classList.add('youtube-mode');
      shell.classList.remove('main-mode', 'telegram-mode');
    } else if (viewName === 'main') {
      shell.classList.add('main-mode');
      shell.classList.remove('telegram-mode', 'youtube-mode');
    } else {
      shell.classList.remove('main-mode', 'telegram-mode', 'youtube-mode');
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
}
