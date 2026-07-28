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

// File Explorer is desktop-only and intentionally exposes directory metadata
// without any file-reading or file-opening operation.
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

export async function listFileExplorerDirectory(rootID, path = '', offset = 0, limit = 250) {
  if (!hasWailsBinding() || !window.go.backend.App.ListFileExplorerDirectory) {
    throw new Error('File Explorer is available only in the desktop app.');
  }
  return window.go.backend.App.ListFileExplorerDirectory({
    root_id: String(rootID || ''),
    path: String(path || ''),
    offset: Number(offset) || 0,
    limit: Number(limit) || 250,
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

export async function toggleTodoSubtask(todoID, subtaskID) {
	if (hasWailsBinding()) {
		return await window.go.backend.App.ToggleTodoSubtask({
			todo_id: Number(todoID),
			subtask_id: Number(subtaskID),
		});
	}
	return [];
}

export async function toggleTodo(id) {
  if (hasWailsBinding()) {
    return await window.go.backend.App.ToggleTodo({ id: Number(id) });
  }
  return [];
}

export async function deleteTodo(id) {
  if (hasWailsBinding()) {
    return await window.go.backend.App.DeleteTodo({ id: Number(id) });
  }
  return [];
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
