import { els } from './dom.js';
import { hideError } from './ui.js';
import { loadTelegramFavorites, loadTelegramPosts } from './telegram.js';

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
  // highlight active menu button
  els.menuButtons.forEach((btn) => {
    btn.classList.toggle("active", btn.dataset.view === viewName);
  });

  // show/hide views
  els.views.forEach((view) => {
    view.classList.toggle("active", view.id === `view-${viewName}`);
  });

  // toggle telegram-mode on the root shell so CSS can stretch .app
  const shell = document.querySelector('.app-shell');
  if (shell) {
    if (viewName === 'telegram') {
      shell.classList.add('telegram-mode');
    } else {
      shell.classList.remove('telegram-mode');
    }
  }

  hideError();

  // Automatic load when switching to Telegram
  if (viewName === 'telegram') {
    loadTelegramFavorites();
    const channel = (els.telegramChannel.value || "durov").trim().toUpperCase();
    if (!els.telegramPosts.querySelector('.telegram-post')) {
      loadTelegramPosts(channel);
    }
  }
}