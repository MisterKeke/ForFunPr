import { els } from './dom.js';
import { normalizeCode, escapeHtml, formatTelegramDate } from './utils.js';
import {
  getChannelPosts,
  telegramCacheClear,
  listTelegramFavorites,
  addTelegramFavorite,
  removeTelegramFavorite
} from './api.js';
import { showError } from './ui.js';

// ----- Posts -----
export async function loadTelegramPosts(channel, forceRefresh = false) {
  const loadingEl = els.telegramLoading;
  const errorEl = els.telegramError;
  const postsEl = els.telegramPosts;

  loadingEl.classList.remove("hidden");
  errorEl.classList.add("hidden");
  errorEl.textContent = "";
  postsEl.innerHTML = "";

  try {
    const posts = await getChannelPosts(channel, forceRefresh);
    if (!posts || posts.length === 0) {
      postsEl.innerHTML = '<div class="telegram-post-empty">No posts found in this channel</div>';
      return;
    }
    renderTelegramPosts(posts);
  } catch (err) {
    errorEl.textContent = err.message || String(err);
    errorEl.classList.remove("hidden");
    postsEl.innerHTML = '';
  } finally {
    loadingEl.classList.add("hidden");
  }
}

function renderTelegramPosts(posts) {
  els.telegramPosts.innerHTML = posts.map(post => {
    const imagesHtml = post.images && post.images.length > 0
      ? `<div class="telegram-post-images">${post.images.map(img => 
         `<img src="${escapeHtml(img)}" alt="Post image" loading="lazy" />`
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
    const channels = await listTelegramFavorites();
    renderTelegramFavorites(Array.isArray(channels) ? channels : []);
  } catch (err) {
    console.error(err);
  }
}

export function renderTelegramFavorites(channels) {
  const currentChannel = normalizeCode(els.telegramChannel.value).toLowerCase();
  if (!channels || channels.length === 0) {
    els.telegramFavoriteList.innerHTML = '<div class="telegram-favorite-empty">No favorite channels yet</div>';
    return;
  }
  els.telegramFavoriteList.innerHTML = channels
    .map((channel) => `
      <div class="telegram-favorite-chip ${channel === currentChannel ? "active" : ""}" data-channel="${escapeHtml(channel)}">
        <span>@${escapeHtml(channel)}</span>
        <button class="telegram-favorite-remove" type="button" data-channel="${escapeHtml(channel)}" title="Remove from favorites">x</button>
      </div>
    `)
    .join("");
}

// ----- Init event listeners -----
export function initTelegram() {
  // Load posts
  els.telegramLoad.addEventListener("click", () => {
    const channel = normalizeCode(els.telegramChannel.value) || "durov";
    loadTelegramPosts(channel, true);
  });

  els.telegramRefresh.addEventListener("click", () => {
    const channel = normalizeCode(els.telegramChannel.value) || "durov";
    telegramCacheClear();  // без await, fire-and-forget
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
      await addTelegramFavorite(channel);
      await loadTelegramFavorites();
    } catch (err) {
      showError(err.message || String(err));
    }
  });

  // Favorite list click: remove or select
  els.telegramFavoriteList.addEventListener("click", async (event) => {
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
        // re-render favorites to highlight active
        const channels = Array.from(
          els.telegramFavoriteList.querySelectorAll(".telegram-favorite-chip")
        ).map((el) => el.dataset.channel);
        renderTelegramFavorites(channels);
      }
    }
  });
}