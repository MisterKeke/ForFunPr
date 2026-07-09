import { initNavigation } from './navigation.js';
import { initCurrency } from './currency.js';
import { initFavorites, loadFavorites } from './favorites.js';
import { initTodos, loadTodos } from './todos.js';
import { initTelegram } from './telegram.js';
import { initYoutube } from './youtube.js';
import { initFavoriteCategoryModal } from './favoriteCategories.js';

document.addEventListener('DOMContentLoaded', () => {
  initNavigation();
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
