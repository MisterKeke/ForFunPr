import { initNavigation } from './navigation.js';
import { initCurrency } from './currency.js';
import { initFavorites, loadFavorites } from './favorites.js';
import { initTodos, loadTodos } from './todos.js';
import { initTelegram } from './telegram.js';

document.addEventListener('DOMContentLoaded', () => {
  initNavigation();
  initCurrency();
  initFavorites();
  initTodos();
  initTelegram();

  // Первоначальная загрузка данных
  loadFavorites();
  loadTodos();
});