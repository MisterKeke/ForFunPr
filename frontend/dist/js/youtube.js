import { els } from './dom.js';
import { escapeHtml } from './utils.js';
import {
  getChannelVideos,
  youtubeCacheClear,
  listYouTubeFavoritesWithCategories,
  addYouTubeFavorite,
  removeYouTubeFavorite,
  assignYouTubeFavoriteCategory
} from './api.js';
import {
  loadFavoriteCategories,
  normalizeFavoriteItem,
  openFavoriteCategoryModal,
  renderFavoriteCategoryGroups
} from './favoriteCategories.js';
import { showError } from './ui.js';

let youtubeFavoriteItems = [];
let youtubeFavoriteCategories = [];
let youtubeActiveCategoryId = "all";
let youtubeVideosRequestId = 0;

// ----- Videos -----
export async function loadYouTubeVideos(channel, forceRefresh = false) {
  const requestId = ++youtubeVideosRequestId;
  const loadingEl = els.youtubeLoading;
  const errorEl = els.youtubeError;
  const videosEl = els.youtubeVideos;

  loadingEl.classList.remove("hidden");
  errorEl.classList.add("hidden");
  errorEl.textContent = "";
  videosEl.innerHTML = "";

  try {
    const videos = await getChannelVideos(channel, forceRefresh);
    if (requestId !== youtubeVideosRequestId) return;
    if (!videos || videos.length === 0) {
      videosEl.innerHTML = '<div class="youtube-video-empty">No videos found in this channel</div>';
      return;
    }
    renderYouTubeVideos(videos);
  } catch (err) {
    if (requestId !== youtubeVideosRequestId) return;
    errorEl.textContent = err.message || String(err);
    errorEl.classList.remove("hidden");
    videosEl.innerHTML = '';
  } finally {
    if (requestId === youtubeVideosRequestId) {
      loadingEl.classList.add("hidden");
    }
  }
}

function renderYouTubeVideos(videos) {
  els.youtubeVideos.innerHTML = videos.map(video => {
    const thumb = video.thumbnail ? `<div class="youtube-video-thumb"><img src="${escapeHtml(video.thumbnail)}" alt="${escapeHtml(video.title)}" loading="lazy" style="width:100%;height:100%;object-fit:cover;border-radius:8px;" /></div>` : '';
    const meta = `
      <div class="youtube-video-meta">
        ${video.views ? `<span>👁️ ${escapeHtml(video.views)}</span>` : ''}
        ${video.publishedAt ? `<span>📅 ${escapeHtml(new Date(video.publishedAt).toLocaleString())}</span>` : ''}
        ${video.duration ? `<span>⏱ ${escapeHtml(video.duration)}</span>` : ''}
      </div>
    `;
    const title = video.videoUrl ? `<div class="youtube-video-title"><a href="${escapeHtml(video.videoUrl)}" target="_blank" rel="noopener noreferrer">${escapeHtml(video.title)}</a></div>` : `<div class="youtube-video-title">${escapeHtml(video.title)}</div>`;
    return `
      <div class="youtube-video">
        ${thumb}
        ${title}
        ${meta}
        ${video.description ? `<div class="youtube-video-desc" style="color:var(--text-dim);margin-top:8px;">${escapeHtml(video.description)}</div>` : ''}
      </div>
    `;
  }).join('');
}

// ----- Favorites -----
export async function loadYouTubeFavorites() {
  try {
    const [channels, categories] = await Promise.all([
      listYouTubeFavoritesWithCategories(),
      loadFavoriteCategories("youtube", true),
    ]);
    youtubeFavoriteItems = (Array.isArray(channels) ? channels : [])
      .map((item) => normalizeFavoriteItem(item, "channel_id"))
      .filter((item) => item.sourceId);
    youtubeFavoriteCategories = categories;
    renderYouTubeFavorites(youtubeFavoriteItems);
  } catch (err) {
    console.error(err);
  }
}

export function renderYouTubeFavorites(channels) {
  const currentChannel = (els.youtubeChannel.value || "").toLowerCase();
  const items = (Array.isArray(channels) ? channels : [])
    .map((item) => normalizeFavoriteItem(item, "channel_id"))
    .filter((item) => item.sourceId);
  els.youtubeFavoriteList.innerHTML = renderFavoriteCategoryGroups({
    items,
    categories: youtubeFavoriteCategories,
    activeCategoryId: youtubeActiveCategoryId,
    currentSourceId: currentChannel,
    sourcePrefix: "youtube",
    emptyMessage: "No favorite channels yet",
    formatLabel: (channel, item) => escapeHtml(item?.label || channel),
  });
}

// ----- Init event listeners -----
export function initYoutube() {
  els.youtubeLoad.addEventListener("click", () => {
    const channel = (els.youtubeChannel.value || "T2X2_latest_news").trim();
    loadYouTubeVideos(channel, true);
  });

  els.youtubeRefresh.addEventListener("click", () => {
    const channel = (els.youtubeChannel.value || "T2X2_latest_news").trim();
    youtubeCacheClear();
    loadYouTubeVideos(channel, true);
  });

  els.youtubeChannel.addEventListener("keydown", (event) => {
    if (event.key === "Enter") {
      els.youtubeLoad.click();
    }
  });

  els.youtubeFavoriteAdd.addEventListener("click", async () => {
    const channel = (els.youtubeChannel.value || "").trim();
    if (!channel) {
      showError("Enter a channel before saving to favorites.");
      return;
    }
    try {
      const category = await openFavoriteCategoryModal({
        title: "Add YouTube favorite",
        targetLabel: channel,
        source: "youtube",
      });
      if (!category) return;
      await addYouTubeFavorite(channel);
      try {
        await assignYouTubeFavoriteCategory(channel, category.id);
      } catch (assignErr) {
        console.warn("Favorite saved, but category assignment failed:", assignErr);
      }
      await loadYouTubeFavorites();
    } catch (err) {
      showError(err.message || String(err));
    }
  });

  els.youtubeFavoriteList.addEventListener("click", async (event) => {
    const filterBtn = event.target.closest(".favorite-category-filter");
    if (filterBtn) {
      youtubeActiveCategoryId = filterBtn.dataset.categoryId || "all";
      renderYouTubeFavorites(youtubeFavoriteItems);
      return;
    }

    const removeBtn = event.target.closest(".youtube-favorite-remove");
    if (removeBtn) {
      const channel = removeBtn.dataset.channel;
      if (channel) {
        try {
          await removeYouTubeFavorite(channel);
          await loadYouTubeFavorites();
        } catch (err) {
          showError(err.message || String(err));
        }
      }
      return;
    }

    const chip = event.target.closest(".youtube-favorite-chip");
    if (chip) {
      const channel = chip.dataset.channel;
      const displayChannel = chip.dataset.displayChannel || channel;
      if (channel) {
        els.youtubeChannel.value = displayChannel;
        loadYouTubeVideos(displayChannel, true);
        renderYouTubeFavorites(youtubeFavoriteItems);
      }
    }
  });
}
