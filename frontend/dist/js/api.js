import { hasWailsBinding } from './utils.js';
import { normalizeFavoriteSource } from './favoriteSources.js';

const FAVORITE_CATEGORIES_KEY = "favoriteCategories";
const TELEGRAM_FAVORITES_KEY = "telegramFavorites";
const YOUTUBE_FAVORITES_KEY = "youtubeFavorites";
const TELEGRAM_FAVORITE_CATEGORIES_KEY = "telegramFavoriteCategories";
const YOUTUBE_FAVORITE_CATEGORIES_KEY = "youtubeFavoriteCategories";

function readJson(key, fallback) {
  try {
    return JSON.parse(localStorage.getItem(key) || JSON.stringify(fallback));
  } catch {
    return fallback;
  }
}

function writeJson(key, value) {
  localStorage.setItem(key, JSON.stringify(value));
}

function normalizeCategoryName(name) {
  return String(name || "").trim();
}

function favoriteCategoriesKey(source) {
  return `${FAVORITE_CATEGORIES_KEY}:${normalizeFavoriteSource(source)}`;
}

function getNextCategoryId(categories) {
  return categories.reduce((max, item) => Math.max(max, Number(item.id) || 0), 0) + 1;
}

export async function callGetRate(base, target) {
  if (hasWailsBinding()) {
    return window.go.backend.App.GetRate(base, target);
  }
  if (base === target) {
    return { base, date: "", to: target, rate: 1, found: true };
  }
  const res = await fetch(
    `https://api.frankfurter.dev/v2/rate/${encodeURIComponent(base)}/${encodeURIComponent(target)}`
  );
  if (res.status === 404) {
    return { base, date: "", to: target, rate: 0, found: false };
  }
  if (!res.ok) throw new Error(`Request failed: ${res.status}`);
  const data = await res.json();
  return {
    base: data.base,
    date: data.date,
    to: data.quote,
    rate: data.rate,
    found: true,
  };
}

export async function callGetAllRates(base) {
  if (hasWailsBinding()) {
    return window.go.backend.App.GetAllRates(base);
  }
  const res = await fetch(
    `https://api.frankfurter.dev/v2/rates?base=${encodeURIComponent(base)}`
  );
  if (!res.ok) throw new Error(`Request failed: ${res.status}`);
  const list = await res.json();
  const rates = {};
  const codes = [];
  const date = list.length > 0 ? list[0].date : "";
  list.forEach((item) => {
    rates[item.quote] = item.rate;
    codes.push(item.quote);
  });
  codes.sort();
  return { base, date, rates, codes };
}

// Weather is intentionally fetched through the Wails backend so the UI does
// not call Open-Meteo directly.
export async function callGetWeather(latitude, longitude) {
  if (!hasWailsBinding() || !window.go.backend.App.GetWeather) {
    throw new Error('Weather is available only through the desktop app backend.');
  }
  return window.go.backend.App.GetWeather(latitude, longitude);
}

export async function callGetStoredLocationWeather() {
  if (!hasWailsBinding() || !window.go.backend.App.GetStoredLocationWeather) {
    throw new Error('Weather is available only through the desktop app backend.');
  }
  return window.go.backend.App.GetStoredLocationWeather();
}

export async function callRefreshStoredLocationWeather() {
  if (!hasWailsBinding() || !window.go.backend.App.RefreshStoredLocationWeather) {
    throw new Error('Weather is available only through the desktop app backend.');
  }
  return window.go.backend.App.RefreshStoredLocationWeather();
}

export async function callGetWeatherForCity(city) {
  if (!hasWailsBinding() || !window.go.backend.App.GetWeatherForCity) {
    throw new Error('Weather is available only through the desktop app backend.');
  }
  return window.go.backend.App.GetWeatherForCity(city);
}

// Wallpapers are desktop-only appearance settings. Image files are imported
// by Go and remain outside the embedded frontend assets.
export async function getWallpaperSettings() {
  if (!hasWailsBinding() || !window.go.backend.App.GetWallpaperSettings) {
    throw new Error('Uploaded wallpapers are available only in the desktop app.');
  }
  return window.go.backend.App.GetWallpaperSettings();
}

export async function importWallpaper() {
  if (!hasWailsBinding() || !window.go.backend.App.ImportWallpaper) {
    throw new Error('Wallpaper upload is available only in the desktop app.');
  }
  return window.go.backend.App.ImportWallpaper();
}

export async function selectWallpaper(selection) {
  if (!hasWailsBinding() || !window.go.backend.App.SelectWallpaper) {
    throw new Error('Wallpaper selection is available only in the desktop app.');
  }
  return window.go.backend.App.SelectWallpaper(selection);
}

export async function deleteUserWallpaper(id) {
  if (!hasWailsBinding() || !window.go.backend.App.DeleteUserWallpaper) {
    throw new Error('Uploaded wallpapers are available only in the desktop app.');
  }
  return window.go.backend.App.DeleteUserWallpaper(id);
}

// Saved applications are intentionally desktop-only. Executable paths enter
// through native Go file pickers, and launches are requested by database ID.
export async function listDesktopApps() {
  if (!hasWailsBinding() || !window.go.backend.App.ListDesktopApps) {
    throw new Error('Application launchers are available only in the desktop app.');
  }
  return window.go.backend.App.ListDesktopApps();
}

export async function addDesktopApp() {
  if (!hasWailsBinding() || !window.go.backend.App.AddDesktopApp) {
    throw new Error('Application launchers are available only in the desktop app.');
  }
  return window.go.backend.App.AddDesktopApp();
}

export async function launchDesktopApp(id) {
  if (!hasWailsBinding() || !window.go.backend.App.LaunchDesktopApp) {
    throw new Error('Application launchers are available only in the desktop app.');
  }
  return window.go.backend.App.LaunchDesktopApp(Number(id));
}

export async function renameDesktopApp(id, displayName) {
  if (!hasWailsBinding() || !window.go.backend.App.RenameDesktopApp) {
    throw new Error('Application launchers are available only in the desktop app.');
  }
  return window.go.backend.App.RenameDesktopApp(Number(id), String(displayName || ''));
}

export async function importDesktopAppIcon(id) {
  if (!hasWailsBinding() || !window.go.backend.App.ImportDesktopAppIcon) {
    throw new Error('Application icons are available only in the desktop app.');
  }
  return window.go.backend.App.ImportDesktopAppIcon(Number(id));
}

export async function deleteDesktopAppIcon(id) {
  if (!hasWailsBinding() || !window.go.backend.App.DeleteDesktopAppIcon) {
    throw new Error('Application icons are available only in the desktop app.');
  }
  return window.go.backend.App.DeleteDesktopAppIcon(Number(id));
}

export async function relocateDesktopApp(id) {
  if (!hasWailsBinding() || !window.go.backend.App.RelocateDesktopApp) {
    throw new Error('Application launchers are available only in the desktop app.');
  }
  return window.go.backend.App.RelocateDesktopApp(Number(id));
}

export async function deleteDesktopApp(id) {
  if (!hasWailsBinding() || !window.go.backend.App.DeleteDesktopApp) {
    throw new Error('Application launchers are available only in the desktop app.');
  }
  return window.go.backend.App.DeleteDesktopApp(Number(id));
}

// File Explorer is desktop-only. Paths remain relative to backend-approved
// roots so the frontend never receives unrestricted filesystem access.
export async function getFileExplorerPlaces() {
  if (!hasWailsBinding() || !window.go.backend.App.GetFileExplorerPlaces) {
    throw new Error('File Explorer is available only in the desktop app.');
  }
  return window.go.backend.App.GetFileExplorerPlaces();
}

export async function chooseFileExplorerFolder() {
  if (!hasWailsBinding() || !window.go.backend.App.ChooseFileExplorerFolder) {
    throw new Error('Folder selection is available only in the desktop app.');
  }
  return window.go.backend.App.ChooseFileExplorerFolder();
}

export async function listFileExplorerDirectory(rootID, path = '', offset = 0, limit = 250, query = '') {
  if (!hasWailsBinding() || !window.go.backend.App.ListFileExplorerDirectory) {
    throw new Error('File Explorer is available only in the desktop app.');
  }
  return window.go.backend.App.ListFileExplorerDirectory({
    root_id: String(rootID || ''),
    path: String(path || ''),
    query: String(query || ''),
    offset: Number(offset) || 0,
    limit: Number(limit) || 250,
  });
}

export async function openFileExplorerFile(rootID, path) {
  if (!hasWailsBinding() || !window.go.backend.App.OpenFileExplorerFile) {
    throw new Error('Opening files is available only in the desktop app.');
  }
  return window.go.backend.App.OpenFileExplorerFile({
    root_id: String(rootID || ''),
    path: String(path || ''),
  });
}

export async function deleteFileExplorerFile(rootID, path) {
  if (!hasWailsBinding() || !window.go.backend.App.DeleteFileExplorerFile) {
    throw new Error('Deleting files is available only in the desktop app.');
  }
  return window.go.backend.App.DeleteFileExplorerFile({
    root_id: String(rootID || ''),
    path: String(path || ''),
  });
}

// Favorites (Wails + localStorage fallback)
export async function listFavorites() {
  if (hasWailsBinding()) {
    return await window.go.backend.App.ListFavorites();
  }
  return JSON.parse(localStorage.getItem("favorites") || "[]");
}

export async function getFavoritesWithRates() {
  if (hasWailsBinding()) {
    const payload = await window.go.backend.App.GetFavoritesWithRates();
    return payload.favorites;
  }
  return [];
}

export async function addFavorite(key) {
  if (hasWailsBinding()) {
    return await window.go.backend.App.AddFavorite(key);
  }
  const codes = JSON.parse(localStorage.getItem("favorites") || "[]");
  if (codes.includes(key)) {
    return { added: false, exists: true, pair: key };
  }
  codes.push(key);
  codes.sort();
  localStorage.setItem("favorites", JSON.stringify(codes));
  return { added: true, exists: false, pair: key };
}

export async function removeFavorite(key) {
  if (hasWailsBinding()) {
    return await window.go.backend.App.RemoveFavorite(key);
  }
  const codes = JSON.parse(localStorage.getItem("favorites") || "[]")
    .filter((item) => item !== key);
  localStorage.setItem("favorites", JSON.stringify(codes));
}

// Favorite categories
export async function listFavoriteCategories(source = "telegram") {
	const normalizedSource = normalizeFavoriteSource(source);
	if (hasWailsBinding()) {
		if (!window.go.backend.App.ListFavoriteCategories) {
			throw new Error("The desktop backend does not support favorite categories.");
		}
		return await window.go.backend.App.ListFavoriteCategories(normalizedSource);
	}
  return readJson(favoriteCategoriesKey(normalizedSource), []);
}

export async function createFavoriteCategory(name, source = "telegram") {
  const normalizedName = normalizeCategoryName(name);
  const normalizedSource = normalizeFavoriteSource(source);
  if (!normalizedName) {
    throw new Error("Enter a category name.");
  }

	if (hasWailsBinding()) {
		if (window.go.backend.App.CreateFavoriteCategoryDetailed) {
			const result = await window.go.backend.App.CreateFavoriteCategoryDetailed({
				name: normalizedName, source: normalizedSource, color: "",
			});
			return result.category;
		}
		if (!window.go.backend.App.CreateFavoriteCategory) {
			throw new Error("The desktop backend does not support favorite categories.");
		}
		return await window.go.backend.App.CreateFavoriteCategory(normalizedName, normalizedSource);
  }

  const key = favoriteCategoriesKey(normalizedSource);
  const categories = readJson(key, []);
  const existing = categories.find(
    (item) => String(item.name || "").toLowerCase() === normalizedName.toLowerCase()
  );
  if (existing) return existing;

  const category = {
    id: getNextCategoryId(categories),
    name: normalizedName,
    source: normalizedSource,
    color: "",
    created_at: new Date().toISOString(),
  };
  categories.push(category);
  writeJson(key, categories);
  return category;
}

export async function renameFavoriteCategory(id, name, source = "telegram") {
	const normalizedName = normalizeCategoryName(name);
	const normalizedSource = normalizeFavoriteSource(source);
	if (!Number.isInteger(Number(id)) || Number(id) <= 0) {
		throw new Error("Choose a valid category.");
	}
	if (!normalizedName) {
		throw new Error("Enter a category name.");
	}
	if (hasWailsBinding()) {
		if (window.go.backend.App.UpdateFavoriteCategory) {
			const categories = await listFavoriteCategories(normalizedSource);
			const current = categories.find((item) => Number(item.id) === Number(id));
			const result = await window.go.backend.App.UpdateFavoriteCategory(Number(id), {
				name: normalizedName, source: normalizedSource, color: current?.color || '',
			});
			return result.category;
		}
		if (!window.go.backend.App.RenameFavoriteCategory) {
			throw new Error("The desktop backend does not support category renaming.");
		}
		return await window.go.backend.App.RenameFavoriteCategory(Number(id), normalizedName);
	}

	const key = favoriteCategoriesKey(normalizedSource);
	const categories = readJson(key, []);
	const index = categories.findIndex((item) => String(item.id) === String(id));
	if (index < 0) throw new Error("The requested category does not exist.");
	const duplicate = categories.some((item, itemIndex) => itemIndex !== index &&
		String(item.name || "").trim().toLowerCase() === normalizedName.toLowerCase());
	if (duplicate) throw new Error("A category with that name already exists.");
	categories[index] = { ...categories[index], name: normalizedName };
	writeJson(key, categories);
	return categories[index];
}

// To-Do
export async function getTodos() {
  if (hasWailsBinding()) {
    if (window.go.backend.App.ListTodos) {
      const result = await window.go.backend.App.ListTodos({ limit: 200, sort: 'created_at', direction: 'desc' });
      return result?.items || [];
    }
    return await window.go.backend.App.GetTodos();
  }
  return [];
}

export async function searchTodos(filter) {
  if (hasWailsBinding() && window.go.backend.App.SearchTodos) {
    return await window.go.backend.App.SearchTodos(filter);
  }
  return await getTodos();
}

export async function getTodosByDueDate(dueDate) {
  if (hasWailsBinding() && window.go.backend.App.GetTodosByDueDate) {
    return await window.go.backend.App.GetTodosByDueDate(dueDate);
  }
  if (hasWailsBinding()) {
    const list = await getTodos();
    return list.filter((todo) => todo.due_date === dueDate);
  }
  return [];
}

export async function getTodayIncompleteTodos() {
  if (hasWailsBinding() && window.go.backend.App.GetTodayTodos) {
    return await window.go.backend.App.GetTodayTodos({ include_overdue: true, include_undated: false });
  }
  if (hasWailsBinding() && window.go.backend.App.GetTodayIncompleteTodos) {
    return await window.go.backend.App.GetTodayIncompleteTodos();
  }
  if (hasWailsBinding()) {
    const today = new Date();
    const todayKey = new Date(today.getTime() - today.getTimezoneOffset() * 60000)
      .toISOString()
      .slice(0, 10);
    const list = await getTodos();
    return list.filter((todo) => {
      return !todo.done && todo.due_date === todayKey;
    });
  }
  return [];
}

export async function getThisWeekIncompleteTodos() {
  if (hasWailsBinding() && window.go.backend.App.GetThisWeekTodos) {
    return await window.go.backend.App.GetThisWeekTodos({ include_overdue: false, include_undated: false });
  }
  if (hasWailsBinding() && window.go.backend.App.GetThisWeekIncompleteTodos) {
    return await window.go.backend.App.GetThisWeekIncompleteTodos();
  }
  if (hasWailsBinding()) {
    const now = new Date();
    const tomorrow = new Date(now.getFullYear(), now.getMonth(), now.getDate() + 1);
    const weekday = now.getDay() || 7;
    const sunday = new Date(now.getFullYear(), now.getMonth(), now.getDate() + 7 - weekday);

    if (tomorrow > sunday) return [];

    const localDateKey = (date) => new Date(date.getTime() - date.getTimezoneOffset() * 60000)
      .toISOString()
      .slice(0, 10);
    const startKey = localDateKey(tomorrow);
    const endKey = localDateKey(sunday);
    const list = await getTodos();

    const priorityOrder = { high: 0, medium: 1, low: 2 };
    return list
      .filter((todo) => !todo.done && todo.due_date >= startKey && todo.due_date <= endKey)
      .sort((left, right) => {
        const dateOrder = left.due_date.localeCompare(right.due_date);
        if (dateOrder !== 0) return dateOrder;
        return (priorityOrder[left.priority] ?? 3) - (priorityOrder[right.priority] ?? 3);
      });
  }
  return [];
}

export async function createTodo(request) {
  if (hasWailsBinding()) {
	return await window.go.backend.App.CreateTodo(request);
  }
  return [];
}

export async function updateTodo(request) {
  if (hasWailsBinding()) {
	return await window.go.backend.App.UpdateTodo(request);
  }
  return [];
}

export async function toggleTodoSubtask(todoID, subtaskID, expectedRevision = null) {
	if (hasWailsBinding()) {
		return await window.go.backend.App.ToggleTodoSubtask({
			todo_id: Number(todoID),
			subtask_id: Number(subtaskID),
			expected_revision: expectedRevision == null ? null : Number(expectedRevision),
		});
	}
	return [];
}

export async function toggleTodo(id, expectedRevision = null) {
  if (hasWailsBinding()) {
    return await window.go.backend.App.ToggleTodo({ id: Number(id), expected_revision: expectedRevision == null ? null : Number(expectedRevision) });
  }
  return [];
}

export async function deleteTodo(id, expectedRevision = null) {
  if (hasWailsBinding()) {
    return await window.go.backend.App.DeleteTodo({ id: Number(id), expected_revision: expectedRevision == null ? null : Number(expectedRevision) });
  }
  return [];
}

export async function getTodoDatePreferences() {
  return requireOrganizerBinding('GetTodoDatePreferences')();
}

export async function setTodoDatePreferences(weekStart) {
  return requireOrganizerBinding('SetTodoDatePreferences')({
    week_start: Number(weekStart), time_zone: '',
  });
}

// Dashboard favorite updates
export async function getInitialFavoriteUpdates() {
  if (hasWailsBinding() && window.go.backend.App.GetInitialFavoriteUpdates) {
    return await window.go.backend.App.GetInitialFavoriteUpdates();
  }
  return { scan_started_at: "", updates: [], new_updates: [], errors: [], state: await getFavoriteUpdateState() };
}

export async function refreshFavoriteUpdates() {
  if (hasWailsBinding() && window.go.backend.App.RefreshFavoriteUpdates) {
    return await window.go.backend.App.RefreshFavoriteUpdates();
  }
  return { scan_started_at: "", updates: [], new_updates: [], errors: [], state: await getFavoriteUpdateState() };
}

export async function getFavoriteUpdateState() {
  if (hasWailsBinding() && window.go.backend.App.GetFavoriteUpdateState) {
    return await window.go.backend.App.GetFavoriteUpdateState();
  }
  return {
    previous_opened_at: "",
    current_opened_at: "",
    previous_refresh_at: "",
    last_refresh_at: "",
    update_windows: {
      new_while_closed: { published_after: "", published_until: "" },
      new_while_open: { published_after: "", published_until: "" },
    },
  };
}

// Telegram
export async function getChannelPosts(channel, forceRefresh = false) {
	if (hasWailsBinding()) {
		return forceRefresh
			? await window.go.backend.App.RefreshChannelPosts(channel)
			: await window.go.backend.App.GetChannelPosts(channel);
  }
  // Fallback
  console.warn('Using fallback – please use Wails backend for production');
  return [
    {
      text: "Test post from " + channel,
      images: [],
      date: new Date().toISOString(),
      views: "123",
      postId: "test1"
    }
  ];
}

// Telegram Favorites
export async function listTelegramFavorites() {
  if (hasWailsBinding()) {
    return await window.go.backend.App.ListTelegramFavorites();
  }
  return readJson(TELEGRAM_FAVORITES_KEY, []);
}

export async function listTelegramFavoritesWithCategories() {
	if (hasWailsBinding()) {
		if (!window.go.backend.App.ListTelegramFavoritesWithCategories) {
			throw new Error("The desktop backend does not support categorized Telegram favorites.");
		}
		return await window.go.backend.App.ListTelegramFavoritesWithCategories();
  }

  const favorites = await listTelegramFavorites();
  const categoryMap = readJson(TELEGRAM_FAVORITE_CATEGORIES_KEY, {});
  return favorites.map((username) => ({
    username,
    category_id: categoryMap[username] || null,
  }));
}

export async function addTelegramFavorite(channel) {
  if (hasWailsBinding()) {
    return await window.go.backend.App.AddTelegramFavorite(channel);
  }
  const normalized = String(channel || "").trim().replace(/^@/, "").toLowerCase();
  if (!normalized) return [];
  const favorites = readJson(TELEGRAM_FAVORITES_KEY, []);
  if (!favorites.includes(normalized)) {
    favorites.push(normalized);
    writeJson(TELEGRAM_FAVORITES_KEY, favorites);
  }
  return favorites;
}

export async function removeTelegramFavorite(channel) {
  if (hasWailsBinding()) {
    return await window.go.backend.App.RemoveTelegramFavorite(channel);
  }
  const normalized = String(channel || "").trim().replace(/^@/, "").toLowerCase();
  const favorites = readJson(TELEGRAM_FAVORITES_KEY, []).filter((item) => item !== normalized);
  const categoryMap = readJson(TELEGRAM_FAVORITE_CATEGORIES_KEY, {});
  delete categoryMap[normalized];
  writeJson(TELEGRAM_FAVORITES_KEY, favorites);
  writeJson(TELEGRAM_FAVORITE_CATEGORIES_KEY, categoryMap);
  return favorites;
}

export async function assignTelegramFavoriteCategory(username, categoryID) {
	if (hasWailsBinding()) {
		if (!window.go.backend.App.AssignTelegramFavoriteCategory) {
			throw new Error("The desktop backend does not support Telegram favorite categories.");
		}
		return await window.go.backend.App.AssignTelegramFavoriteCategory(username, Number(categoryID));
  }

  const normalized = String(username || "").trim().replace(/^@/, "").toLowerCase();
  if (!normalized) return;
  const categoryMap = readJson(TELEGRAM_FAVORITE_CATEGORIES_KEY, {});
  categoryMap[normalized] = Number(categoryID);
  writeJson(TELEGRAM_FAVORITE_CATEGORIES_KEY, categoryMap);
}

// MCP server lifecycle
export async function getMCPServerStatus() {
  if (!hasWailsBinding() || !window.go.backend.App.GetMCPServerStatus) {
    throw new Error("MCP server control is available only in the desktop app.");
  }

  return await window.go.backend.App.GetMCPServerStatus();
}

export async function setMCPServerEnabled(enabled) {
  if (!hasWailsBinding() || !window.go.backend.App.SetMCPServerEnabled) {
    throw new Error("MCP server control is available only in the desktop app.");
  }

  return await window.go.backend.App.SetMCPServerEnabled(Boolean(enabled));
}

// YouTube
export async function getChannelVideos(channel, forceRefresh = false) {
	if (hasWailsBinding()) {
		return forceRefresh
			? await window.go.backend.App.RefreshChannelVideos(channel)
			: await window.go.backend.App.GetChannelVideos(channel);
  }
  // Fallback
  console.warn('Using fallback – please use Wails backend for production');
  return [
    {
      title: "Test video from " + channel,
      description: "This is a local fallback test video.",
      thumbnail: "",
      publishedAt: new Date().toISOString(),
      channelId: channel,
      channelTitle: channel,
      videoUrl: "#",
      views: "123",
      duration: "0:30",
      videoId: "test1"
    }
  ];
}

export async function listYouTubeFavorites() {
  if (hasWailsBinding()) {
    return await window.go.backend.App.ListYouTubeFavorites();
  }
  return readJson(YOUTUBE_FAVORITES_KEY, []);
}

export async function listYouTubeFavoritesWithCategories() {
	if (hasWailsBinding()) {
		if (!window.go.backend.App.ListYouTubeFavoritesWithCategories) {
			throw new Error("The desktop backend does not support categorized YouTube favorites.");
		}
		return await window.go.backend.App.ListYouTubeFavoritesWithCategories();
  }

  const favorites = await listYouTubeFavorites();
  const categoryMap = readJson(YOUTUBE_FAVORITE_CATEGORIES_KEY, {});
  return favorites.map((channelID) => ({
    channel_id: channelID,
    category_id: categoryMap[channelID] || null,
  }));
}

export async function addYouTubeFavorite(channelID) {
  if (hasWailsBinding()) {
    return await window.go.backend.App.AddYouTubeFavorite(channelID);
  }
  const normalized = String(channelID || "").trim();
  if (!normalized) return [];
  const favorites = readJson(YOUTUBE_FAVORITES_KEY, []);
  if (!favorites.includes(normalized)) {
    favorites.push(normalized);
    writeJson(YOUTUBE_FAVORITES_KEY, favorites);
  }
  return favorites;
}

export async function removeYouTubeFavorite(channelID) {
  if (hasWailsBinding()) {
    return await window.go.backend.App.RemoveYouTubeFavorite(channelID);
  }
  const normalized = String(channelID || "").trim();
  const favorites = readJson(YOUTUBE_FAVORITES_KEY, []).filter((item) => item !== normalized);
  const categoryMap = readJson(YOUTUBE_FAVORITE_CATEGORIES_KEY, {});
  delete categoryMap[normalized];
  writeJson(YOUTUBE_FAVORITES_KEY, favorites);
  writeJson(YOUTUBE_FAVORITE_CATEGORIES_KEY, categoryMap);
  return favorites;
}

export async function assignYouTubeFavoriteCategory(channelID, categoryID) {
	if (hasWailsBinding()) {
		if (!window.go.backend.App.AssignYouTubeFavoriteCategory) {
			throw new Error("The desktop backend does not support YouTube favorite categories.");
		}
		return await window.go.backend.App.AssignYouTubeFavoriteCategory(channelID, Number(categoryID));
  }

  const normalized = String(channelID || "").trim();
  if (!normalized) return;
  const categoryMap = readJson(YOUTUBE_FAVORITE_CATEGORIES_KEY, {});
  categoryMap[normalized] = Number(categoryID);
  writeJson(YOUTUBE_FAVORITE_CATEGORIES_KEY, categoryMap);
}

// Notes and bookmarks are local-first desktop resources. Unlike old browser
// development fallbacks, these calls never pretend a mutation succeeded when
// the Wails backend is unavailable.
function requireOrganizerBinding(name) {
  if (!hasWailsBinding() || !window.go.backend.App[name]) {
    throw new Error('This feature is available only through the desktop app backend.');
  }
  return (...args) => window.go.backend.App[name](...args);
}

export async function listNotes(filter = {}) {
  return requireOrganizerBinding('ListNotes')({
    query: String(filter.query || ''),
    archive_status: String(filter.archive_status || 'active'),
    pinned: typeof filter.pinned === 'boolean' ? filter.pinned : null,
    limit: Number(filter.limit || 50),
    offset: Number(filter.offset || 0),
  });
}

export async function getNote(id) {
  return requireOrganizerBinding('GetNote')(Number(id));
}

export async function createNote(request) {
  return requireOrganizerBinding('CreateNote')(request);
}

export async function updateNote(request) {
  return requireOrganizerBinding('UpdateNote')(request);
}

export async function setNotePinned(id, value, expectedRevision) {
  return requireOrganizerBinding('SetNotePinned')({
    id: Number(id), value: Boolean(value), expected_revision: Number(expectedRevision),
  });
}

export async function setNoteArchived(id, value, expectedRevision) {
  return requireOrganizerBinding('SetNoteArchived')({
    id: Number(id), value: Boolean(value), expected_revision: Number(expectedRevision),
  });
}

export async function deleteNote(id) {
  return requireOrganizerBinding('DeleteNote')(Number(id));
}

export async function updateFavoriteCategory(id, request) {
	if (!hasWailsBinding() || !window.go.backend.App.UpdateFavoriteCategory) {
		return renameFavoriteCategory(id, request.name, request.source);
	}
	return await window.go.backend.App.UpdateFavoriteCategory(Number(id), request);
}

export async function deleteFavoriteCategory(id, mode = 'unassign', targetCategoryID = null) {
	if (!hasWailsBinding() || !window.go.backend.App.DeleteFavoriteCategory) {
		throw new Error('The desktop backend does not support category deletion.');
	}
	return await window.go.backend.App.DeleteFavoriteCategory({
		id: Number(id), mode: String(mode),
		target_category_id: targetCategoryID == null ? null : Number(targetCategoryID),
	});
}

export async function reorderFavoriteCategories(source, categoryIDs) {
	if (!hasWailsBinding() || !window.go.backend.App.ReorderFavoriteCategories) {
		throw new Error('The desktop backend does not support category reordering.');
	}
	return await window.go.backend.App.ReorderFavoriteCategories({
		source: normalizeFavoriteSource(source), category_ids: categoryIDs.map(Number),
	});
}

export async function listNoteTopics(filter = {}) {
  if (hasWailsBinding() && window.go.backend.App.ListNoteTopicsPage) {
    return requireOrganizerBinding('ListNoteTopicsPage')({
      limit: Number(filter.limit || 50), offset: Number(filter.offset || 0),
    });
  }
  const items = await requireOrganizerBinding('ListNoteTopics')();
  return { items: Array.isArray(items) ? items : [], total: items?.length || 0, limit: 50, offset: 0 };
}

export async function getNoteTopicBoard(topicID) {
  return requireOrganizerBinding('GetNoteTopicBoard')(Number(topicID));
}

export async function createNoteTopic(title) {
  return requireOrganizerBinding('CreateNoteTopic')({ title: String(title || '') });
}

export async function renameNoteTopic(id, title, expectedRevision = null) {
  return requireOrganizerBinding('RenameNoteTopic')({
    id: Number(id), title: String(title || ''), expected_revision: expectedRevision,
  });
}

export async function deleteNoteTopic(topicID, expectedRevision = null) {
  if (hasWailsBinding() && window.go.backend.App.DeleteNoteTopicWithRevision) {
    return requireOrganizerBinding('DeleteNoteTopicWithRevision')({ id: Number(topicID), expected_revision: expectedRevision });
  }
  await requireOrganizerBinding('DeleteNoteTopic')(Number(topicID));
  return { changed: true, topic_id: Number(topicID) };
}

export async function addNoteTopicBlock(request) {
  return requireOrganizerBinding('AddNoteTopicBlock')({
    topic_id: Number(request.topic_id),
    note_id: Number(request.note_id),
    position_x: Number(request.position_x),
    position_y: Number(request.position_y),
    expected_revision: request.expected_revision ?? null,
  });
}

export async function updateNoteTopicBlockPosition(request) {
  return requireOrganizerBinding('UpdateNoteTopicBlockPosition')({
    block_id: Number(request.block_id),
    position_x: Number(request.position_x),
    position_y: Number(request.position_y),
    expected_revision: request.expected_revision ?? null,
  });
}

export async function updateNoteTopicBlockPositions(request) {
  return requireOrganizerBinding('UpdateNoteTopicBlockPositions')({
    topic_id: Number(request.topic_id),
    positions: request.positions.map((item) => ({
      block_id: Number(item.block_id), position_x: Number(item.position_x), position_y: Number(item.position_y),
    })),
    expected_revision: request.expected_revision ?? null,
  });
}

export async function deleteNoteTopicBlock(blockID, expectedRevision = null) {
  if (hasWailsBinding() && window.go.backend.App.DeleteNoteTopicBlockWithRevision) {
    return requireOrganizerBinding('DeleteNoteTopicBlockWithRevision')({ id: Number(blockID), expected_revision: expectedRevision });
  }
  await requireOrganizerBinding('DeleteNoteTopicBlock')(Number(blockID));
  return { changed: true };
}

export async function createNoteTopicConnection(request) {
  return requireOrganizerBinding('CreateNoteTopicConnection')({
    topic_id: Number(request.topic_id),
    from_block_id: Number(request.from_block_id),
    to_block_id: Number(request.to_block_id),
    relation_type: String(request.relation_type || 'leads_to'),
    expected_revision: request.expected_revision ?? null,
  });
}

export async function deleteNoteTopicConnection(connectionID, expectedRevision = null) {
  if (hasWailsBinding() && window.go.backend.App.DeleteNoteTopicConnectionWithRevision) {
    return requireOrganizerBinding('DeleteNoteTopicConnectionWithRevision')({ id: Number(connectionID), expected_revision: expectedRevision });
  }
  await requireOrganizerBinding('DeleteNoteTopicConnection')(Number(connectionID));
  return { changed: true };
}

export async function searchNoteTopicPicker(topicID, query = '', limit = 50, offset = 0) {
  return requireOrganizerBinding('SearchNoteTopicPicker')({
    topic_id: Number(topicID), query: String(query || ''), limit: Number(limit), offset: Number(offset),
  });
}

export async function listNoteTodos(noteID) {
  return requireOrganizerBinding('ListNoteTodos')(Number(noteID));
}

export async function listTodoNotes(todoID) {
  return requireOrganizerBinding('ListTodoNotes')(Number(todoID));
}

export async function linkNoteTodo(noteID, todoID) {
  if (hasWailsBinding() && window.go.backend.App.LinkNoteTodoWithStatus) {
    return requireOrganizerBinding('LinkNoteTodoWithStatus')({
      note_id: Number(noteID), todo_id: Number(todoID),
    });
  }
  await requireOrganizerBinding('LinkNoteTodo')({
    note_id: Number(noteID), todo_id: Number(todoID),
  });
  return { changed: true, note_id: Number(noteID), todo_id: Number(todoID) };
}

export async function unlinkNoteTodo(noteID, todoID) {
  if (hasWailsBinding() && window.go.backend.App.UnlinkNoteTodoWithStatus) {
    return requireOrganizerBinding('UnlinkNoteTodoWithStatus')({
      note_id: Number(noteID), todo_id: Number(todoID),
    });
  }
  await requireOrganizerBinding('UnlinkNoteTodo')({
    note_id: Number(noteID), todo_id: Number(todoID),
  });
  return { changed: true, note_id: Number(noteID), todo_id: Number(todoID) };
}

export async function listBookmarks(filter = {}) {
  return requireOrganizerBinding('ListBookmarks')({
    query: String(filter.query || ''),
    status: String(filter.status || 'all'),
    tags: Array.isArray(filter.tags) ? filter.tags : [],
    limit: Number(filter.limit || 50),
    offset: Number(filter.offset || 0),
  });
}

export async function getBookmark(id) {
  return requireOrganizerBinding('GetBookmark')(Number(id));
}

export async function createBookmark(request) {
  return requireOrganizerBinding('CreateBookmark')(request);
}

export async function updateBookmark(request) {
  return requireOrganizerBinding('UpdateBookmark')(request);
}

export async function setBookmarkRead(id, read, expectedRevision) {
  return requireOrganizerBinding('SetBookmarkRead')({
    id: Number(id), read: Boolean(read), expected_revision: Number(expectedRevision),
  });
}

export async function deleteBookmark(id) {
  return requireOrganizerBinding('DeleteBookmark')(Number(id));
}

export async function listBookmarkTags() {
  return requireOrganizerBinding('ListBookmarkTags')();
}

// The Steam tracker is local-first: listing and settings calls read the local
// cache, while only Add and the two explicit refresh methods contact Steam.
export async function listSteamGames() {
  return requireOrganizerBinding('ListSteamGames')();
}

export async function addSteamGame(storeURL) {
  return requireOrganizerBinding('AddSteamGame')(String(storeURL || '').trim());
}

export async function deleteSteamGame(id) {
  return requireOrganizerBinding('DeleteSteamGame')(Number(id));
}

export async function getSteamGameSettings() {
  return requireOrganizerBinding('GetSteamGameSettings')();
}

export async function setSteamGameCountry(countryCode) {
  return requireOrganizerBinding('SetSteamGameCountry')(
    String(countryCode || '').trim().toUpperCase()
  );
}

export async function listSteamCountries() {
  return requireOrganizerBinding('ListSteamCountries')();
}

export async function refreshSteamGamesOnOpen() {
  return requireOrganizerBinding('RefreshSteamGamesOnOpen')();
}

export async function refreshSteamGames() {
  return requireOrganizerBinding('RefreshSteamGames')();
}

// Utilities are intentionally desktop-only for now. They are not routed
// through the loopback REST API, CLI, or MCP server.
export async function getClipboardState() {
  return requireOrganizerBinding('GetClipboardState')();
}

export async function updateClipboardSettings(settings) {
  return requireOrganizerBinding('UpdateClipboardSettings')(settings);
}

export async function listClipboardItems(filter = {}) {
  return requireOrganizerBinding('ListClipboardItems')({
    query: String(filter.query || ''), kind: String(filter.kind || ''),
    pinned_only: Boolean(filter.pinned_only), limit: Number(filter.limit || 100), offset: Number(filter.offset || 0),
  });
}

export async function setClipboardItemPinned(id, pinned) {
  return requireOrganizerBinding('SetClipboardItemPinned')(Number(id), Boolean(pinned));
}

export async function restoreClipboardItem(id) {
  return requireOrganizerBinding('RestoreClipboardItem')(Number(id));
}

export async function deleteClipboardItem(id) {
  return requireOrganizerBinding('DeleteClipboardItem')(Number(id));
}

export async function clearClipboardHistory(keepPinned = true) {
  return requireOrganizerBinding('ClearClipboardHistory')(Boolean(keepPinned));
}

export async function evaluateCalculatorExpression(expression, previousResult = 0) {
  return requireOrganizerBinding('EvaluateCalculatorExpression')({ expression, previous_result: Number(previousResult) || 0 });
}

export async function listCalculatorUnits() {
  return requireOrganizerBinding('ListCalculatorUnits')();
}

export async function convertCalculatorUnit(value, from, to) {
  return requireOrganizerBinding('ConvertCalculatorUnit')({ value: Number(value), from, to });
}

export async function calculateDate(request) {
  return requireOrganizerBinding('CalculateDate')(request);
}

export async function listCalculatorHistory(limit = 100) {
  return requireOrganizerBinding('ListCalculatorHistory')(Number(limit));
}

export async function deleteCalculatorHistoryItem(id) {
  return requireOrganizerBinding('DeleteCalculatorHistoryItem')(Number(id));
}

export async function clearCalculatorHistory() {
  return requireOrganizerBinding('ClearCalculatorHistory')();
}

export async function getScreenshotCapabilities() {
  return requireOrganizerBinding('GetScreenshotCapabilities')();
}

export async function captureScreenshot(mode) {
  return requireOrganizerBinding('CaptureScreenshot')({
    mode: String(mode),
    interactive: String(mode) === 'region',
  });
}

export async function listScreenshots(filter = {}) {
  return requireOrganizerBinding('ListScreenshots')({ query: String(filter.query || ''), limit: Number(filter.limit || 60), offset: Number(filter.offset || 0) });
}

export async function renameScreenshot(id, title) {
  return requireOrganizerBinding('RenameScreenshot')(String(id), String(title || ''));
}

export async function saveScreenshotEdit(id, dataURL) {
  return requireOrganizerBinding('SaveScreenshotEdit')({ id: String(id), data_url: String(dataURL) });
}

export async function revertScreenshotEdit(id) {
  return requireOrganizerBinding('RevertScreenshotEdit')(String(id));
}

export async function runScreenshotOCR(id) {
  return requireOrganizerBinding('RunScreenshotOCR')(String(id));
}

export async function cancelScreenshotOCR(id) {
  return requireOrganizerBinding('CancelScreenshotOCR')(String(id));
}

export async function exportScreenshot(id) {
  return requireOrganizerBinding('ExportScreenshot')(String(id));
}

export async function deleteScreenshot(id) {
  return requireOrganizerBinding('DeleteScreenshot')(String(id));
}

export async function listTimeZones() {
  return requireOrganizerBinding('ListTimeZones')();
}

export async function listWorldClocks() {
  return requireOrganizerBinding('ListWorldClocks')();
}

export async function createWorldClock(request) {
  return requireOrganizerBinding('CreateWorldClock')(request);
}

export async function updateWorldClock(request) {
  return requireOrganizerBinding('UpdateWorldClock')(request);
}

export async function deleteWorldClock(id) {
  return requireOrganizerBinding('DeleteWorldClock')(Number(id));
}

export async function reorderWorldClocks(ids) {
  return requireOrganizerBinding('ReorderWorldClocks')({ ids: ids.map(Number) });
}

export async function convertWorldTime(request) {
  return requireOrganizerBinding('ConvertWorldTime')(request);
}

export async function getWebsiteSearchState() {
  return requireOrganizerBinding('GetWebsiteSearchState')();
}

export async function addWebsiteSearchTarget(url) {
  return requireOrganizerBinding('AddWebsiteSearchTarget')(String(url || ''));
}

export async function deleteWebsiteSearchTarget(id) {
  return requireOrganizerBinding('DeleteWebsiteSearchTarget')(Number(id));
}

export async function searchWebsites(query, useBrowserFallback = false) {
  return requireOrganizerBinding('SearchWebsites')({
    query: String(query || ''),
    use_browser_fallback: Boolean(useBrowserFallback),
  });
}

export async function getWebsiteSearchRun(id) {
  return requireOrganizerBinding('GetWebsiteSearchRun')(Number(id));
}

export async function clearWebsiteSearchHistory() {
  return requireOrganizerBinding('ClearWebsiteSearchHistory')();
}

export async function openWebsiteSearchResult(url) {
  return requireOrganizerBinding('OpenWebsiteSearchResult')(String(url || ''));
}
