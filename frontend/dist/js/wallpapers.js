const STORAGE_KEY = 'selectedWallpaper';
const DEFAULT_WALLPAPER = 'hu-tao';
const WALLPAPERS = new Set(['original', 'sandrone', 'hu-tao', 'skirk']);

function readSavedWallpaper() {
  try {
    const saved = localStorage.getItem(STORAGE_KEY);
    return WALLPAPERS.has(saved) ? saved : DEFAULT_WALLPAPER;
  } catch {
    return DEFAULT_WALLPAPER;
  }
}

function saveWallpaper(wallpaper) {
  try {
    localStorage.setItem(STORAGE_KEY, wallpaper);
  } catch {
    // The wallpaper still works for the current session if storage is unavailable.
  }
}

function applyWallpaper(wallpaper, buttons) {
  document.body.dataset.wallpaper = wallpaper;

  buttons.forEach((button) => {
    const isActive = button.dataset.wallpaper === wallpaper;
    button.classList.toggle('active', isActive);
    button.setAttribute('aria-pressed', String(isActive));
  });
}

export function initWallpapers() {
  const buttons = document.querySelectorAll('.wallpaper-btn');
  const initialWallpaper = readSavedWallpaper();

  applyWallpaper(initialWallpaper, buttons);

  buttons.forEach((button) => {
    button.addEventListener('click', () => {
      const wallpaper = button.dataset.wallpaper;
      if (!WALLPAPERS.has(wallpaper)) return;

      applyWallpaper(wallpaper, buttons);
      saveWallpaper(wallpaper);
    });
  });
}
