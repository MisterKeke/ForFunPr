import { els } from './dom.js';
import { escapeHtml } from './utils.js';
import {
  getChannelVideos,
  youtubeCacheClear,
  listYouTubeFavorites,
  addYouTubeFavorite,
  removeYouTubeFavorite
} from './api.js';
import { showError } from './ui.js';

// ----- Videos -----
export async function loadYouTubeVideos(channel, forceRefresh = false) {
  const loadingEl = els.youtubeLoading;
  const errorEl = els.youtubeError;
  const videosEl = els.youtubeVideos;

  loadingEl.classList.remove("hidden");
  errorEl.classList.add("hidden");
  errorEl.textContent = "";
  videosEl.innerHTML = "";

  try {
    const videos = await getChannelVideos(channel, forceRefresh);
    if (!videos || videos.length === 0) {
      videosEl.innerHTML = '<div class="youtube-video-empty">No videos found in this channel</div>';
      return;
    }
    renderYouTubeVideos(videos);
  } catch (err) {
    errorEl.textContent = err.message || String(err);
    errorEl.classList.remove("hidden");
    videosEl.innerHTML = '';
  } finally {
    loadingEl.classList.add("hidden");
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
    const channels = await listYouTubeFavorites();
    renderYouTubeFavorites(Array.isArray(channels) ? channels : []);
  } catch (err) {
    console.error(err);
  }
}

export function renderYouTubeFavorites(channels) {
  const currentChannel = (els.youtubeChannel.value || "").toLowerCase();
  if (!channels || channels.length === 0) {
    els.youtubeFavoriteList.innerHTML = '<div class="youtube-favorite-empty">No favorite channels yet</div>';
    return;
  }
  els.youtubeFavoriteList.innerHTML = channels
    .map((channel) => `
      <div class="youtube-favorite-chip ${channel.toLowerCase() === currentChannel ? "active" : ""}" data-channel="${escapeHtml(channel)}">
        <span>${escapeHtml(channel)}</span>
        <button class="youtube-favorite-remove" type="button" data-channel="${escapeHtml(channel)}" title="Remove from favorites">x</button>
      </div>
    `)
    .join("");
}

// ----- Init event listeners -----
export function initYoutube() {
  els.youtubeLoad.addEventListener("click", () => {
    const channel = (els.youtubeChannel.value || "durov").trim();
    loadYouTubeVideos(channel, true);
  });

  els.youtubeRefresh.addEventListener("click", () => {
    const channel = (els.youtubeChannel.value || "durov").trim();
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
      await addYouTubeFavorite(channel);
      await loadYouTubeFavorites();
    } catch (err) {
      showError(err.message || String(err));
    }
  });

  els.youtubeFavoriteList.addEventListener("click", async (event) => {
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
      if (channel) {
        els.youtubeChannel.value = channel;
        loadYouTubeVideos(channel, true);
        const channels = Array.from(
          els.youtubeFavoriteList.querySelectorAll(".youtube-favorite-chip")
        ).map((el) => el.dataset.channel);
        renderYouTubeFavorites(channels);
      }
    }
  });
}