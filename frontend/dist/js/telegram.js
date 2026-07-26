import { els } from './dom.js';
import { normalizeCode, escapeHtml, formatTelegramDate } from './utils.js';
import {
  getChannelPosts,
  listTelegramFavoritesWithCategories,
  addTelegramFavorite,
  removeTelegramFavorite,
  assignTelegramFavoriteCategory
} from './api.js';
import {
  loadFavoriteCategories,
  normalizeFavoriteItem,
  openFavoriteCategoryModal,
  renderFavoriteCategoryGroups
} from './favoriteCategories.js';
import { showError } from './ui.js';

let telegramFavoriteItems = [];
let telegramFavoriteCategories = [];
let telegramActiveCategoryId = "all";
let telegramPostsRequestId = 0;

// ----- Posts -----
export async function loadTelegramPosts(channel, forceRefresh = false) {
  const requestId = ++telegramPostsRequestId;
  const loadingEl = els.telegramLoading;
  const errorEl = els.telegramError;
  const postsEl = els.telegramPosts;

  loadingEl.classList.remove("hidden");
  errorEl.classList.add("hidden");
  errorEl.textContent = "";
  postsEl.innerHTML = "";

  try {
    const posts = await getChannelPosts(channel, forceRefresh);
    if (requestId !== telegramPostsRequestId) return;
    if (!posts || posts.length === 0) {
      postsEl.innerHTML = '<div class="telegram-post-empty">No posts found in this channel</div>';
      return;
    }
    renderTelegramPosts(posts);
  } catch (err) {
    if (requestId !== telegramPostsRequestId) return;
    errorEl.textContent = err.message || String(err);
    errorEl.classList.remove("hidden");
    postsEl.innerHTML = '';
  } finally {
    if (requestId === telegramPostsRequestId) {
      loadingEl.classList.add("hidden");
    }
  }
}

function renderTelegramPosts(posts) {
  els.telegramPosts.innerHTML = posts.map(post => {
    const images = [...new Set((post.images || []).filter(Boolean))];
    const isSingleImage = images.length === 1;
    const imagesHtml = images.length > 0
      ? `<div class="telegram-post-images${isSingleImage ? ' single-image' : ''}">${images.map(img =>
         `<img src="${escapeHtml(img)}" alt="Post image" loading="lazy" onerror="this.classList.add('img-broken')" />`
        ).join('')}</div>`
      : '';
    const metaHtml = `
      <div class="telegram-post-meta">
        ${post.date ? `<span>📅 ${escapeHtml(formatTelegramDate(post.date))}</span>` : ''}
        ${post.views ? `<span>👁️ ${escapeHtml(post.views)}</span>` : ''}
      </div>
    `;
    return `
      <div class="telegram-post">
        ${post.text ? `<div class="telegram-post-text">${escapeHtml(post.text)}</div>` : ''}
        ${imagesHtml}
        ${metaHtml}
      </div>
    `;
  }).join('');
}

// ----- Favorites -----
export async function loadTelegramFavorites() {
  try {
    const [channels, categories] = await Promise.all([
      listTelegramFavoritesWithCategories(),
      loadFavoriteCategories("telegram", true),
    ]);
    telegramFavoriteItems = (Array.isArray(channels) ? channels : [])
      .map((item) => normalizeFavoriteItem(item, "username"))
      .filter((item) => item.sourceId);
    telegramFavoriteCategories = categories;
    renderTelegramFavorites(telegramFavoriteItems);
  } catch (err) {
    console.error(err);
  }
}

export function renderTelegramFavorites(channels) {
  const currentChannel = normalizeCode(els.telegramChannel.value).toLowerCase();
  const items = (Array.isArray(channels) ? channels : [])
    .map((item) => normalizeFavoriteItem(item, "username"))
    .filter((item) => item.sourceId);
  els.telegramFavoriteList.innerHTML = renderFavoriteCategoryGroups({
    items,
    categories: telegramFavoriteCategories,
    activeCategoryId: telegramActiveCategoryId,
    currentSourceId: currentChannel,
    sourcePrefix: "telegram",
    emptyMessage: "No favorite channels yet",
    formatLabel: (channel) => `@${escapeHtml(channel)}`,
  });
}

// ----- Init event listeners -----
export function initTelegram() {
  // Load posts
  els.telegramLoad.addEventListener("click", () => {
    const channel = normalizeCode(els.telegramChannel.value) || "toporlive";
    loadTelegramPosts(channel, true);
  });

  els.telegramRefresh.addEventListener("click", () => {
    const channel = normalizeCode(els.telegramChannel.value) || "toporlive";
    loadTelegramPosts(channel, true);
  });

  els.telegramChannel.addEventListener("keydown", (event) => {
    if (event.key === "Enter") {
      els.telegramLoad.click();
    }
  });

  // Add favorite channel
  els.telegramFavoriteAdd.addEventListener("click", async () => {
    const channel = normalizeCode(els.telegramChannel.value).toLowerCase();
    if (!channel) {
      showError("Enter a channel username before saving to favorites.");
      return;
    }
    try {
      const category = await openFavoriteCategoryModal({
        title: "Add Telegram favorite",
        targetLabel: `@${channel}`,
        source: "telegram",
      });
      if (!category) return;
      await addTelegramFavorite(channel);
      try {
        await assignTelegramFavoriteCategory(channel, category.id);
      } catch (assignErr) {
        console.error("Favorite saved, but category assignment failed:", assignErr);
        showError(`Favorite saved, but category assignment failed: ${assignErr.message || String(assignErr)}`);
      }
      await loadTelegramFavorites();
    } catch (err) {
      showError(err.message || String(err));
    }
  });

  // Favorite list click: remove or select
  els.telegramFavoriteList.addEventListener("click", async (event) => {
    const filterBtn = event.target.closest(".favorite-category-filter");
    if (filterBtn) {
      telegramActiveCategoryId = filterBtn.dataset.categoryId || "all";
      renderTelegramFavorites(telegramFavoriteItems);
      return;
    }

    const removeBtn = event.target.closest(".telegram-favorite-remove");
    if (removeBtn) {
      const channel = removeBtn.dataset.channel;
      if (channel) {
        try {
          await removeTelegramFavorite(channel);
          await loadTelegramFavorites();
        } catch (err) {
          showError(err.message || String(err));
        }
      }
      return;
    }

    const chip = event.target.closest(".telegram-favorite-chip");
    if (chip) {
      const channel = chip.dataset.channel;
      if (channel) {
        els.telegramChannel.value = channel;
        loadTelegramPosts(channel, true);
        renderTelegramFavorites(telegramFavoriteItems);
      }
    }
  });
}
