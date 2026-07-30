import { els } from './dom.js';
import {
  listBookmarks,
  getBookmark,
  createBookmark,
  updateBookmark,
  setBookmarkRead,
  deleteBookmark,
  listBookmarkTags,
} from './api.js';
import { escapeHtml, hasWailsBinding } from './utils.js';

const BOOKMARKS_CHANGED_EVENT = 'bookmarks:changed';
const BOOKMARK_PAGE_SIZE = 50;

let bookmarks = [];
let total = 0;
let editingBookmark = null;
let requestSequence = 0;
let searchTimer = null;

function setBookmarkError(message = '') {
  els.bookmarksError.textContent = message;
  els.bookmarksError.classList.toggle('hidden', !message);
}

function setBookmarkModalError(message = '') {
  els.bookmarkModalError.textContent = message;
  els.bookmarkModalError.classList.toggle('hidden', !message);
}

function formatBookmarkTime(value) {
  if (!value) return '';
  const normalized = /^\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}$/.test(value)
    ? `${value.replace(' ', 'T')}Z`
    : value;
  const date = new Date(normalized);
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString();
}

function bookmarkHostname(value) {
  try {
    return new URL(value).hostname;
  } catch {
    return value;
  }
}

function renderBookmarks() {
  els.bookmarksSummary.textContent = total === 1 ? '1 bookmark' : `${total} bookmarks`;
  els.bookmarksLoadMore.classList.toggle('hidden', bookmarks.length >= total);
  if (bookmarks.length === 0) {
    els.bookmarksList.innerHTML = '<div class="bookmarks-empty">No bookmarks match these filters.</div>';
    return;
  }
  els.bookmarksList.innerHTML = bookmarks.map((bookmark) => {
    const tags = Array.isArray(bookmark.tags) ? bookmark.tags : [];
    return `
      <article class="bookmark-item ${bookmark.read ? 'read' : 'unread'}" data-bookmark-id="${escapeHtml(bookmark.id)}">
        <div class="bookmark-main">
          <div class="bookmark-title-row">
            <span class="bookmark-state">${bookmark.read ? 'Read' : 'Unread'}</span>
            <h2>${escapeHtml(bookmark.title)}</h2>
          </div>
          <div class="bookmark-domain">${escapeHtml(bookmarkHostname(bookmark.url))}</div>
          <div class="bookmark-url" title="${escapeHtml(bookmark.url)}">${escapeHtml(bookmark.url)}</div>
          ${bookmark.description ? `<p>${escapeHtml(bookmark.description)}</p>` : ''}
          ${tags.length ? `<div class="bookmark-tags">${tags.map((tag) => `<span>${escapeHtml(tag)}</span>`).join('')}</div>` : ''}
          <div class="bookmark-meta">Saved ${escapeHtml(formatBookmarkTime(bookmark.created_at))}</div>
        </div>
        <div class="bookmark-actions">
          <button class="primary-btn small-btn" type="button" data-action="open">Open</button>
          <button class="secondary-btn small-btn" type="button" data-action="read">${bookmark.read ? 'Mark unread' : 'Mark read'}</button>
          <button class="secondary-btn small-btn" type="button" data-action="edit">Edit</button>
          <button class="danger-btn small-btn" type="button" data-action="delete">Delete</button>
        </div>
      </article>
    `;
  }).join('');
}

function currentBookmarkFilter(offset = 0) {
  const tag = els.bookmarksTagFilter.value;
  return {
    query: els.bookmarksSearch.value.trim(),
    status: els.bookmarksStatus.value,
    tags: tag ? [tag] : [],
    limit: BOOKMARK_PAGE_SIZE,
    offset,
  };
}

export async function loadBookmarks({ append = false } = {}) {
  const request = ++requestSequence;
  const offset = append ? bookmarks.length : 0;
  if (!append) setBookmarkError();
  if (els.bookmarksTagFilter.options.length <= 1) {
    void loadBookmarkTagOptions();
  }
  try {
    const result = await listBookmarks(currentBookmarkFilter(offset));
    if (request !== requestSequence) return;
    const items = Array.isArray(result?.bookmarks) ? result.bookmarks : [];
    bookmarks = append ? bookmarks.concat(items) : items;
    total = Number(result?.total || 0);
    renderBookmarks();
  } catch (error) {
    if (request === requestSequence) {
      setBookmarkError(error?.message || 'Bookmarks could not be loaded.');
    }
  }
}

async function loadBookmarkTagOptions() {
  try {
    const selected = els.bookmarksTagFilter.value;
    const tags = await listBookmarkTags();
    els.bookmarksTagFilter.innerHTML = '<option value="">All tags</option>' +
      (Array.isArray(tags) ? tags : []).map((tag) =>
        `<option value="${escapeHtml(tag)}">${escapeHtml(tag)}</option>`
      ).join('');
    if ([...els.bookmarksTagFilter.options].some((option) => option.value === selected)) {
      els.bookmarksTagFilter.value = selected;
    }
  } catch (error) {
    setBookmarkError(error?.message || 'Bookmark tags could not be loaded.');
  }
}

function parseBookmarkTags(value) {
  const seen = new Set();
  return String(value || '').split(',').map((tag) => tag.trim()).filter((tag) => {
    const key = tag.toLowerCase();
    if (!tag || seen.has(key)) return false;
    seen.add(key);
    return true;
  });
}

function openBookmarkModal(bookmark = null) {
  editingBookmark = bookmark;
  els.bookmarkModalHeading.textContent = bookmark ? 'Edit bookmark' : 'Add bookmark';
  els.bookmarkModalURL.value = bookmark?.url || '';
  els.bookmarkModalTitle.value = bookmark?.title || '';
  els.bookmarkModalDescription.value = bookmark?.description || '';
  els.bookmarkModalTags.value = Array.isArray(bookmark?.tags) ? bookmark.tags.join(', ') : '';
  setBookmarkModalError();
  els.bookmarkModal.classList.remove('hidden');
  els.bookmarkModal.setAttribute('aria-hidden', 'false');
  els.bookmarkModalURL.focus();
}

function closeBookmarkModal() {
  editingBookmark = null;
  els.bookmarkModal.classList.add('hidden');
  els.bookmarkModal.setAttribute('aria-hidden', 'true');
  setBookmarkModalError();
}

async function saveBookmarkModal() {
  const request = {
    url: els.bookmarkModalURL.value.trim(),
    title: els.bookmarkModalTitle.value.trim(),
    description: els.bookmarkModalDescription.value.trim(),
    tags: parseBookmarkTags(els.bookmarkModalTags.value),
  };
  if (!request.url) {
    setBookmarkModalError('Enter an HTTP or HTTPS URL.');
    els.bookmarkModalURL.focus();
    return;
  }
  if (!request.title) {
    setBookmarkModalError('Enter a bookmark title.');
    els.bookmarkModalTitle.focus();
    return;
  }
  els.bookmarkModalSave.disabled = true;
  try {
    if (editingBookmark) {
      await updateBookmark({
        id: editingBookmark.id,
        ...request,
        expected_revision: editingBookmark.revision,
      });
    } else {
      await createBookmark(request);
    }
    closeBookmarkModal();
    await Promise.all([loadBookmarks(), loadBookmarkTagOptions()]);
  } catch (error) {
    setBookmarkModalError(error?.message || 'Bookmark could not be saved.');
  } finally {
    els.bookmarkModalSave.disabled = false;
  }
}

async function openExternalBookmark(bookmark) {
  let parsed;
  try {
    parsed = new URL(bookmark.url);
  } catch {
    setBookmarkError('This bookmark contains an invalid URL.');
    return;
  }
  if (parsed.protocol !== 'http:' && parsed.protocol !== 'https:') {
    setBookmarkError('Only HTTP and HTTPS bookmarks can be opened.');
    return;
  }
  if (window.runtime?.BrowserOpenURL) {
    window.runtime.BrowserOpenURL(bookmark.url);
  } else {
    window.open(bookmark.url, '_blank', 'noopener,noreferrer');
  }
  if (!bookmark.read) {
    try {
      await setBookmarkRead(bookmark.id, true, bookmark.revision);
      await loadBookmarks();
    } catch (error) {
      setBookmarkError(error?.message || 'The bookmark opened, but its read state could not be saved.');
    }
  }
}

async function changeBookmarkRead(bookmark) {
  try {
    await setBookmarkRead(bookmark.id, !bookmark.read, bookmark.revision);
    await loadBookmarks();
  } catch (error) {
    setBookmarkError(error?.message || 'Bookmark read state could not be changed.');
  }
}

async function removeBookmark(bookmark) {
  if (!window.confirm(`Permanently delete “${bookmark.title}”?`)) return;
  try {
    await deleteBookmark(bookmark.id);
    await Promise.all([loadBookmarks(), loadBookmarkTagOptions()]);
  } catch (error) {
    setBookmarkError(error?.message || 'Bookmark could not be deleted.');
  }
}

function bookmarkByElement(target) {
  const item = target.closest('[data-bookmark-id]');
  if (!item) return null;
  return bookmarks.find((bookmark) => bookmark.id === Number(item.dataset.bookmarkId)) || null;
}

function listenForBookmarkChanges() {
  if (!hasWailsBinding() || !window.runtime?.EventsOn) return;
  window.runtime.EventsOn(BOOKMARKS_CHANGED_EVENT, () => {
    if (!els.bookmarkModal.classList.contains('hidden')) {
      setBookmarkModalError('Bookmarks changed through another interface. Close and reopen this editor to load the latest version.');
      void Promise.all([loadBookmarks(), loadBookmarkTagOptions()]);
      return;
    }
    void Promise.all([loadBookmarks(), loadBookmarkTagOptions()]);
  });
}

export function initBookmarks() {
  els.bookmarksNew.addEventListener('click', () => openBookmarkModal());
  els.bookmarkModalClose.addEventListener('click', closeBookmarkModal);
  els.bookmarkModalCancel.addEventListener('click', closeBookmarkModal);
  els.bookmarkModalBackdrop.addEventListener('click', closeBookmarkModal);
  els.bookmarkModalSave.addEventListener('click', () => void saveBookmarkModal());
  els.bookmarksStatus.addEventListener('change', () => void loadBookmarks());
  els.bookmarksTagFilter.addEventListener('change', () => void loadBookmarks());
  els.bookmarksSearch.addEventListener('input', () => {
    clearTimeout(searchTimer);
    searchTimer = setTimeout(() => void loadBookmarks(), 250);
  });
  els.bookmarksLoadMore.addEventListener('click', () => void loadBookmarks({ append: true }));
  els.bookmarksList.addEventListener('click', (event) => {
    const action = event.target.closest('[data-action]')?.dataset.action;
    const bookmark = bookmarkByElement(event.target);
    if (!action || !bookmark) return;
    if (action === 'open') void openExternalBookmark(bookmark);
    if (action === 'read') void changeBookmarkRead(bookmark);
    if (action === 'edit') {
      void getBookmark(bookmark.id).then(openBookmarkModal).catch((error) => {
        setBookmarkError(error?.message || 'Bookmark could not be loaded.');
      });
    }
    if (action === 'delete') void removeBookmark(bookmark);
  });
  document.addEventListener('keydown', (event) => {
    if (event.key === 'Escape' && !els.bookmarkModal.classList.contains('hidden')) {
      closeBookmarkModal();
    }
  });
  listenForBookmarkChanges();
  void loadBookmarkTagOptions();
}
