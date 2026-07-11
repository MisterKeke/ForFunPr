import { els } from './dom.js';
import { escapeHtml, formatTelegramDate, hasWailsBinding } from './utils.js';
import {
  getTodayIncompleteTodos,
  getInitialFavoriteUpdates,
  refreshFavoriteUpdates as refreshFavoriteUpdatesApi,
  getFavoriteUpdateState,
  toggleTodo,
} from './api.js';
import { loadTodos } from './todos.js';
import { PRIORITY_LABELS } from './todoConstants.js';
const renderedFavoriteUpdateKeys = new Set();

let dashboardTodos = [];
let telegramUpdates = [];
let youtubeUpdates = [];
let refreshTimer = null;
let refreshInFlight = false;

function formatDateTime(value) {
  if (!value) return "";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleString(undefined, {
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}

function getTodoTitle(todo) {
  return todo.title;
}

function getTodoDescription(todo) {
  return todo.description;
}

function getFavoriteUpdateKey(update) {
  const source = String(update?.source || "").toLowerCase();
  if (source === "telegram") {
    const username = String(update.username || "").replace(/^@/, "").toLowerCase();
    const postId = String(update.postId || update.post_id || "").trim();
    if (username && postId) return `telegram:${username}:${postId}`;
  }
  if (source === "youtube") {
    const videoId = String(update.videoId || update.video_id || "").trim();
    if (videoId) return `youtube:${videoId}`;
  }
  return "";
}

function setDashboardTaskState({ loading = false, error } = {}) {
  els.dashboardTasksLoading?.classList.toggle("hidden", !loading);
  if (els.dashboardTasksError && error !== undefined) {
    els.dashboardTasksError.classList.toggle("hidden", !error);
    els.dashboardTasksError.textContent = error;
  }
}

function setFavoriteUpdateState({ loading = false, error, refreshedAt = "" } = {}) {
  els.dashboardFavoriteUpdatesLoading?.classList.toggle("hidden", !loading);
  if (els.dashboardFavoriteUpdatesError && error !== undefined) {
    els.dashboardFavoriteUpdatesError.classList.toggle("hidden", !error);
    els.dashboardFavoriteUpdatesError.textContent = error;
  }
  if (els.dashboardFavoriteRefresh) {
    els.dashboardFavoriteRefresh.disabled = loading;
  }
  if (els.dashboardFavoriteRefreshedAt && refreshedAt) {
    els.dashboardFavoriteRefreshedAt.textContent = `Refreshed ${formatDateTime(refreshedAt)}`;
    els.dashboardFavoriteRefreshedAt.classList.remove("hidden");
  }
}

function renderDashboardTasks() {
  if (!els.dashboardTasks) return;

  if (!hasWailsBinding()) {
    els.dashboardTasks.innerHTML = '<div class="dashboard-empty">Run the app to see today\'s tasks.</div>';
    return;
  }

  if (dashboardTodos.length === 0) {
    els.dashboardTasks.innerHTML = '<div class="dashboard-empty">No incomplete tasks due today.</div>';
    return;
  }

  els.dashboardTasks.innerHTML = dashboardTodos
    .map((todo) => {
      const priority = (todo.priority || "medium").toLowerCase();
      const description = getTodoDescription(todo);
      return `
        <div class="dashboard-task" data-id="${escapeHtml(todo.id)}">
          <button class="todo-check dashboard-task-check" type="button" title="Complete task"></button>
          <div class="dashboard-task-body">
            <div class="dashboard-task-title-row">
              <span class="todo-id-badge">#${escapeHtml(todo.id)}</span>
              <span class="dashboard-task-title">${escapeHtml(getTodoTitle(todo))}</span>
            </div>
            ${description ? `<span class="todo-desc">${escapeHtml(description)}</span>` : ""}
          </div>
          <span class="todo-priority-badge priority-${escapeHtml(priority)}">${escapeHtml(PRIORITY_LABELS[priority] || "Medium")}</span>
        </div>
      `;
    })
    .join("");
}

export async function loadDashboardTasks() {
  if (!els.dashboardTasks) return;
  setDashboardTaskState({ loading: true, error: "" });
  try {
    const list = await getTodayIncompleteTodos();
    dashboardTodos = list;
    renderDashboardTasks();
  } catch (err) {
    console.error(err);
    dashboardTodos = [];
    renderDashboardTasks();
    setDashboardTaskState({ error: err.message || String(err) });
  } finally {
    setDashboardTaskState({ loading: false });
  }
}

function addFavoriteUpdates(updates) {
  const additions = { telegram: 0, youtube: 0 };
  (Array.isArray(updates) ? updates : []).forEach((update) => {
    const key = getFavoriteUpdateKey(update);
    if (!key || renderedFavoriteUpdateKeys.has(key)) return;
    renderedFavoriteUpdateKeys.add(key);

    if (key.startsWith("telegram:")) {
      telegramUpdates.push(update);
      additions.telegram += 1;
    } else if (key.startsWith("youtube:")) {
      youtubeUpdates.push(update);
      additions.youtube += 1;
    }
  });
  return additions;
}

function clearFavoriteUpdates() {
  renderedFavoriteUpdateKeys.clear();
  telegramUpdates = [];
  youtubeUpdates = [];
}

function renderTelegramUpdate(update) {
  const username = String(update.username || "").replace(/^@/, "");
  const postId = String(update.postId || update.post_id || "").trim();
  const fallbackUrl = postId.includes("/")
    ? `https://t.me/${postId.replace(/^\/+/, "")}`
    : (username && postId ? `https://t.me/${username}/${postId}` : "");
  const url = update.postUrl || update.post_url || fallbackUrl;
  const images = Array.isArray(update.images) ? update.images : [];
  const preview = update.preview || "";
  const imageHtml = images.length > 0
    ? `<div class="dashboard-update-images">${images.slice(0, 3).map((image) => `<img src="${escapeHtml(image)}" alt="" loading="lazy" onerror="this.classList.add('img-broken')" />`).join("")}</div>`
    : "";
  const title = username ? `@${username}` : "Telegram";
  const content = preview ? escapeHtml(preview) : "New Telegram post";

  return `
    <article class="dashboard-update">
      <div class="dashboard-update-source">${escapeHtml(title)}</div>
      ${url ? `<a class="dashboard-update-title" href="${escapeHtml(url)}" target="_blank" rel="noopener noreferrer">${content}</a>` : `<div class="dashboard-update-title">${content}</div>`}
      ${imageHtml}
      <div class="dashboard-update-meta">
        ${update.publishedAt ? `<span>${escapeHtml(formatTelegramDate(update.publishedAt))}</span>` : ""}
        ${update.views ? `<span>${escapeHtml(update.views)} views</span>` : ""}
      </div>
    </article>
  `;
}

function renderYouTubeUpdate(update) {
  const title = update.title || "New YouTube video";
  const channel = update.channelTitle || update.channel_title || update.channelId || "YouTube";
  const url = update.videoUrl || update.video_url || (update.videoId ? `https://www.youtube.com/watch?v=${update.videoId}` : "");
  const thumb = update.thumbnail
    ? `<div class="dashboard-update-thumb"><img src="${escapeHtml(update.thumbnail)}" alt="" loading="lazy" /></div>`
    : "";

  return `
    <article class="dashboard-update">
      ${thumb}
      <div class="dashboard-update-source">${escapeHtml(channel)}</div>
      ${url ? `<a class="dashboard-update-title" href="${escapeHtml(url)}" target="_blank" rel="noopener noreferrer">${escapeHtml(title)}</a>` : `<div class="dashboard-update-title">${escapeHtml(title)}</div>`}
      <div class="dashboard-update-meta">
        ${update.publishedAt ? `<span>${escapeHtml(formatDateTime(update.publishedAt))}</span>` : ""}
      </div>
    </article>
  `;
}

function renderFavoriteUpdates() {
  if (!els.dashboardFavoriteUpdates) return;

  const hasTelegram = telegramUpdates.length > 0;
  const hasYouTube = youtubeUpdates.length > 0;
  const hasAny = hasTelegram || hasYouTube;

  els.dashboardFavoriteEmpty?.classList.toggle("hidden", hasAny);
  els.dashboardTelegramSection?.classList.toggle("hidden", !hasTelegram);
  els.dashboardYoutubeSection?.classList.toggle("hidden", !hasYouTube);

  if (els.dashboardTelegramUpdates) {
    els.dashboardTelegramUpdates.innerHTML = telegramUpdates.map(renderTelegramUpdate).join("");
  }
  if (els.dashboardYoutubeUpdates) {
    els.dashboardYoutubeUpdates.innerHTML = youtubeUpdates.map(renderYouTubeUpdate).join("");
  }
}

function summarizeFavoriteErrors(errors) {
  if (!Array.isArray(errors) || errors.length === 0) return "";
  const sample = errors.slice(0, 2).map((item) => {
    const source = item.source_id || item.sourceId || item.source || "source";
    return `${source}: ${item.error || "failed"}`;
  });
  const suffix = errors.length > sample.length ? ` and ${errors.length - sample.length} more` : "";
  return `Some favorites could not refresh (${sample.join("; ")}${suffix}).`;
}

async function loadFavoriteUpdates(loadFn) {
  if (!els.dashboardFavoriteUpdates) return;
  setFavoriteUpdateState({ loading: true, error: "" });

  try {
    const result = await loadFn();
    addFavoriteUpdates(result?.updates || []);
    renderFavoriteUpdates();
    setFavoriteUpdateState({
      loading: false,
      error: summarizeFavoriteErrors(result?.errors),
      refreshedAt: result?.scan_started_at || result?.state?.last_refresh_at || "",
    });
  } catch (err) {
    console.error(err);
    renderFavoriteUpdates();
    setFavoriteUpdateState({ loading: false, error: err.message || String(err) });
  }
}

async function loadFavoriteUpdateStatus() {
  try {
    const state = await getFavoriteUpdateState();
    setFavoriteUpdateState({ refreshedAt: state.last_refresh_at || state.current_opened_at || "" });
  } catch (err) {
    console.warn("Could not read favorite update state:", err);
  }
}

export async function loadInitialFavoriteUpdates() {
  await loadFavoriteUpdates(getInitialFavoriteUpdates);
}

export async function refreshFavoriteUpdates() {
  if (refreshInFlight) return;

  refreshInFlight = true;
  try {
    clearFavoriteUpdates();
    renderFavoriteUpdates();
    await loadFavoriteUpdates(refreshFavoriteUpdatesApi);
  } finally {
    refreshInFlight = false;
  }
}

export function initDashboard() {
  els.dashboardFavoriteRefresh?.addEventListener("click", refreshFavoriteUpdates);

  els.dashboardTasks?.addEventListener("click", async (event) => {
    const button = event.target.closest(".dashboard-task-check");
    if (!button || !hasWailsBinding()) return;

    const item = button.closest(".dashboard-task");
    const id = item?.dataset.id;
    if (!id) return;

    button.disabled = true;
    try {
      await toggleTodo(Number(id));
      await Promise.all([loadDashboardTasks(), loadTodos()]);
    } catch (err) {
      console.error(err);
      setDashboardTaskState({ error: err.message || String(err) });
    } finally {
      button.disabled = false;
    }
  });

  if (!refreshTimer) {
    refreshTimer = setInterval(refreshFavoriteUpdates, 10 * 60 * 1000);
  }

  document.addEventListener("todos:changed", loadDashboardTasks);
}

export async function loadDashboard() {
  await loadFavoriteUpdateStatus();
  await Promise.all([loadDashboardTasks(), loadInitialFavoriteUpdates()]);
}
