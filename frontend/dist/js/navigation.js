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
  els.menuButtons.forEach((btn) => {
    btn.classList.toggle("active", btn.dataset.view === viewName);
  });
  els.views.forEach((view) => {
    view.classList.toggle("active", view.id === `view-${viewName}`);
  });
  hideError();

  // Автоматическая загрузка при переключении на Telegram
  if (viewName === 'telegram') {
    loadTelegramFavorites();
    const channel = (els.telegramChannel.value || "durov").trim().toUpperCase();
    if (!els.telegramPosts.querySelector('.telegram-post')) {
      loadTelegramPosts(channel);
    }
  }
}