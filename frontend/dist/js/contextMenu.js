import { chooseWallpaper, getAvailableWallpapers } from './wallpapers.js';
import { openTodoModal } from './todos.js';

const VIEWPORT_MARGIN = 8;

let contextMenu;
let wallpaperSubmenuHost;
let wallpaperSubmenuTrigger;
let wallpaperSubmenu;

function setSubmenuOpen(open) {
  wallpaperSubmenuHost?.classList.toggle('is-open', open);
  wallpaperSubmenuTrigger?.setAttribute('aria-expanded', String(open));

  if (!open || !contextMenu || !wallpaperSubmenu) return;

  wallpaperSubmenu.classList.remove('opens-left');
  wallpaperSubmenu.style.top = '-7px';
  wallpaperSubmenu.style.maxHeight = `${Math.max(160, window.innerHeight - (VIEWPORT_MARGIN * 2))}px`;

  const menuRect = contextMenu.getBoundingClientRect();
  const submenuWidth = wallpaperSubmenu.offsetWidth;
  const shouldOpenLeft = menuRect.right + submenuWidth + VIEWPORT_MARGIN > window.innerWidth;
  wallpaperSubmenu.classList.toggle('opens-left', shouldOpenLeft);

  const submenuRect = wallpaperSubmenu.getBoundingClientRect();
  if (submenuRect.bottom > window.innerHeight - VIEWPORT_MARGIN) {
    wallpaperSubmenu.style.top = `${window.innerHeight - VIEWPORT_MARGIN - menuRect.top - submenuRect.height}px`;
  }

  const adjustedRect = wallpaperSubmenu.getBoundingClientRect();
  if (adjustedRect.top < VIEWPORT_MARGIN) {
    wallpaperSubmenu.style.top = `${VIEWPORT_MARGIN - menuRect.top}px`;
  }
}

function closeContextMenu() {
  if (!contextMenu || contextMenu.hidden) return;
  setSubmenuOpen(false);
  contextMenu.hidden = true;
}

function createWallpaperOption(wallpaper) {
  const button = document.createElement('button');
  button.className = 'app-context-menu__item app-context-menu__wallpaper-option';
  button.type = 'button';
  button.dataset.wallpaperSelection = wallpaper.key;
  button.setAttribute('role', 'menuitemradio');
  button.setAttribute('aria-checked', String(wallpaper.selected));

  if (wallpaper.selected) button.classList.add('active');

  const preview = document.createElement('span');
  preview.className = 'app-context-menu__wallpaper-preview';
  preview.setAttribute('aria-hidden', 'true');
  if (wallpaper.preview) {
    preview.style.backgroundImage = `url(${JSON.stringify(wallpaper.preview)})`;
  } else {
    preview.classList.add('app-context-menu__wallpaper-preview--original');
  }

  const copy = document.createElement('span');
  copy.className = 'app-context-menu__wallpaper-copy';

  const label = document.createElement('strong');
  label.textContent = wallpaper.label;

  const description = document.createElement('small');
  description.textContent = wallpaper.description;

  const check = document.createElement('span');
  check.className = 'app-context-menu__check';
  check.setAttribute('aria-hidden', 'true');
  check.textContent = wallpaper.selected ? '\u2713' : '';

  copy.append(label, description);
  button.append(preview, copy, check);
  button.addEventListener('click', () => {
    closeContextMenu();
    void chooseWallpaper(wallpaper.key);
  });

  return button;
}

function renderWallpaperSubmenu() {
  const wallpapers = getAvailableWallpapers();
  wallpaperSubmenu.replaceChildren(...wallpapers.map(createWallpaperOption));
}

function positionContextMenu(x, y) {
  contextMenu.style.left = '0px';
  contextMenu.style.top = '0px';

  const { width, height } = contextMenu.getBoundingClientRect();
  const maxLeft = Math.max(VIEWPORT_MARGIN, window.innerWidth - width - VIEWPORT_MARGIN);
  const maxTop = Math.max(VIEWPORT_MARGIN, window.innerHeight - height - VIEWPORT_MARGIN);

  contextMenu.style.left = `${Math.min(Math.max(VIEWPORT_MARGIN, x), maxLeft)}px`;
  contextMenu.style.top = `${Math.min(Math.max(VIEWPORT_MARGIN, y), maxTop)}px`;
}

function openContextMenu(x, y) {
  renderWallpaperSubmenu();
  setSubmenuOpen(false);
  contextMenu.hidden = false;
  positionContextMenu(x, y);
  contextMenu.focus({ preventScroll: true });
}

function moveMenuFocus(event, direction) {
  const currentMenu = event.target.closest('[role="menu"]');
  if (!currentMenu) return;

  const items = Array.from(currentMenu.querySelectorAll(':scope > button:not(:disabled), :scope > .app-context-menu__submenu-host > button:not(:disabled)'));
  if (!items.length) return;

  const currentIndex = items.indexOf(document.activeElement);
  const nextIndex = currentIndex < 0
    ? 0
    : (currentIndex + direction + items.length) % items.length;

  event.preventDefault();
  items[nextIndex].focus();
}

function createContextMenu() {
  contextMenu = document.createElement('div');
  contextMenu.id = 'app-context-menu';
  contextMenu.className = 'app-context-menu';
  contextMenu.setAttribute('role', 'menu');
  contextMenu.setAttribute('aria-label', 'Application menu');
  contextMenu.tabIndex = -1;
  contextMenu.hidden = true;

  const createTaskButton = document.createElement('button');
  createTaskButton.className = 'app-context-menu__item';
  createTaskButton.type = 'button';
  createTaskButton.setAttribute('role', 'menuitem');

  const taskIcon = document.createElement('span');
  taskIcon.className = 'app-context-menu__task-icon';
  taskIcon.setAttribute('aria-hidden', 'true');
  taskIcon.textContent = '+';

  const taskLabel = document.createElement('span');
  taskLabel.className = 'app-context-menu__item-label';
  taskLabel.textContent = 'Create task';

  createTaskButton.append(taskIcon, taskLabel);
  createTaskButton.addEventListener('click', () => {
    closeContextMenu();
    openTodoModal();
  });

  wallpaperSubmenuHost = document.createElement('div');
  wallpaperSubmenuHost.className = 'app-context-menu__submenu-host';

  wallpaperSubmenuTrigger = document.createElement('button');
  wallpaperSubmenuTrigger.className = 'app-context-menu__item app-context-menu__submenu-trigger';
  wallpaperSubmenuTrigger.type = 'button';
  wallpaperSubmenuTrigger.setAttribute('role', 'menuitem');
  wallpaperSubmenuTrigger.setAttribute('aria-haspopup', 'menu');
  wallpaperSubmenuTrigger.setAttribute('aria-expanded', 'false');

  const icon = document.createElement('span');
  icon.className = 'app-context-menu__wallpaper-icon';
  icon.setAttribute('aria-hidden', 'true');

  const label = document.createElement('span');
  label.className = 'app-context-menu__item-label';
  label.textContent = 'Change wallpaper';

  const arrow = document.createElement('span');
  arrow.className = 'app-context-menu__arrow';
  arrow.setAttribute('aria-hidden', 'true');
  arrow.textContent = '\u203a';

  wallpaperSubmenuTrigger.append(icon, label, arrow);

  wallpaperSubmenu = document.createElement('div');
  wallpaperSubmenu.className = 'app-context-menu__submenu';
  wallpaperSubmenu.setAttribute('role', 'menu');
  wallpaperSubmenu.setAttribute('aria-label', 'Choose wallpaper');

  wallpaperSubmenuHost.append(wallpaperSubmenuTrigger, wallpaperSubmenu);
  contextMenu.append(createTaskButton, wallpaperSubmenuHost);
  document.body.append(contextMenu);

  wallpaperSubmenuTrigger.addEventListener('click', () => {
    setSubmenuOpen(true);
    wallpaperSubmenu.querySelector('button')?.focus();
  });
  wallpaperSubmenuHost.addEventListener('pointerenter', () => setSubmenuOpen(true));
  wallpaperSubmenuHost.addEventListener('pointerleave', () => {
    if (!wallpaperSubmenuHost.contains(document.activeElement)) setSubmenuOpen(false);
  });
  wallpaperSubmenuHost.addEventListener('focusout', (event) => {
    if (!wallpaperSubmenuHost.contains(event.relatedTarget)) setSubmenuOpen(false);
  });
}

export function initContextMenu() {
  if (document.getElementById('app-context-menu')) return;
  createContextMenu();

  document.addEventListener('contextmenu', (event) => {
    event.preventDefault();
    if (contextMenu.contains(event.target)) return;
    openContextMenu(event.clientX, event.clientY);
  }, true);

  document.addEventListener('pointerdown', (event) => {
    if (!contextMenu.contains(event.target)) closeContextMenu();
  }, true);

  document.addEventListener('keydown', (event) => {
    if (contextMenu.hidden) return;

    if (event.key === 'Escape' || event.key === 'Tab') {
      closeContextMenu();
      return;
    }

    if (event.key === 'ArrowRight' && document.activeElement === wallpaperSubmenuTrigger) {
      event.preventDefault();
      setSubmenuOpen(true);
      wallpaperSubmenu.querySelector('button')?.focus();
      return;
    }

    if (event.key === 'ArrowLeft' && wallpaperSubmenu.contains(document.activeElement)) {
      event.preventDefault();
      setSubmenuOpen(false);
      wallpaperSubmenuTrigger.focus();
      return;
    }

    if (event.key === 'ArrowDown') moveMenuFocus(event, 1);
    if (event.key === 'ArrowUp') moveMenuFocus(event, -1);
  });

  window.addEventListener('blur', closeContextMenu);
  window.addEventListener('resize', closeContextMenu);
  document.addEventListener('scroll', (event) => {
    if (!contextMenu.contains(event.target)) closeContextMenu();
  }, true);
}
