import { initNavigation } from './navigation.js';
import { initCurrency } from './currency.js';
import { initFavorites, loadFavorites } from './favorites.js';
import { initTodos, loadTodos } from './todos.js';
import { initTelegram } from './telegram.js';
import { initYoutube } from './youtube.js';
import { initFavoriteCategoryModal } from './favoriteCategories.js';
import { initCalendar } from './calendar.js';

document.addEventListener('DOMContentLoaded', () => {
  initNavigation();
  initCalendar();
  initCurrency();
  initFavorites();
  initTodos();
  initFavoriteCategoryModal();
  initTelegram();
  initYoutube();

  // Первоначальная загрузка данных
  loadFavorites();
  loadTodos();
});
