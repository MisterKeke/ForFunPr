import { hasWailsBinding } from './utils.js';

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

function normalizeFavoriteSource(source) {
  return source === "youtube" ? "youtube" : "telegram";
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

// Favorites (Wails + localStorage fallback)
export async function listFavorites() {
  if (hasWailsBinding()) {
    return await window.go.backend.App.ListFavorites();
  }
  return JSON.parse(localStorage.getItem("favorites") || "[]");
}

export async function getFavoritesWithRates() {
  if (hasWailsBinding()) {
    const payloadStr = await window.go.backend.App.GetFavoriteswithRates();
    const payload = JSON.parse(payloadStr || "{}");
    return payload.favorites || [];
  }
  return [];
}

export async function addFavorite(key) {
  if (hasWailsBinding()) {
    return await window.go.backend.App.AddFavorite(key);
  }
  const codes = JSON.parse(localStorage.getItem("favorites") || "[]");
  if (codes.includes(key)) {
    return { Added: false, Exists: true, Pair: key };
  }
  codes.push(key);
  codes.sort();
  localStorage.setItem("favorites", JSON.stringify(codes));
  return { Added: true, Exists: false, Pair: key };
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
  if (hasWailsBinding() && window.go.backend.App.ListFavoriteCategories) {
    try {
      return await window.go.backend.App.ListFavoriteCategories(normalizedSource);
    } catch (err) {
      console.warn("Falling back without favorite categories:", err);
      return [];
    }
  }
  return readJson(favoriteCategoriesKey(normalizedSource), []);
}

export async function createFavoriteCategory(name, source = "telegram") {
  const normalizedName = normalizeCategoryName(name);
  const normalizedSource = normalizeFavoriteSource(source);
  if (!normalizedName) {
    throw new Error("Enter a category name.");
  }

  if (hasWailsBinding() && window.go.backend.App.CreateFavoriteCategory) {
    try {
      return await window.go.backend.App.CreateFavoriteCategory(normalizedName, normalizedSource);
    } catch (err) {
      console.warn("Falling back to local favorite category:", err);
    }
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

// To-Do
export async function getTodos() {
  if (hasWailsBinding()) {
    return await window.go.backend.App.GetTodos();
  }
  return [];
}

export async function createTodo(text, description, priority, dueDate) {
  if (hasWailsBinding()) {
    return await window.go.backend.App.CreateTodo(text, description, priority, dueDate);
  }
  return [];
}

export async function updateTodo(id, text, description, priority, dueDate) {
  if (hasWailsBinding()) {
    return await window.go.backend.App.UpdateTodo(Number(id), text, description, priority, dueDate || "");
  }
  return [];
}

export async function toggleTodo(id) {
  if (hasWailsBinding()) {
    return await window.go.backend.App.ToggleTodo(Number(id));
  }
  return [];
}

export async function deleteTodo(id) {
  if (hasWailsBinding()) {
    return await window.go.backend.App.DeleteTodo(Number(id));
  }
  return [];
}

// Telegram
export async function getChannelPosts(channel, forceRefresh = false) {
  if (hasWailsBinding()) {
    if (forceRefresh) {
      await window.go.backend.App.TelegramCacheClear?.();
    }
    return await window.go.backend.App.GetChannelPosts(channel);
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

export async function telegramCacheClear() {
  if (hasWailsBinding()) {
    return await window.go.backend.App.TelegramCacheClear?.();
  }
}

// Telegram Favorites
export async function listTelegramFavorites() {
  if (hasWailsBinding()) {
    return await window.go.backend.App.ListTelegramFavorites();
  }
  return readJson(TELEGRAM_FAVORITES_KEY, []);
}

export async function listTelegramFavoritesWithCategories() {
  if (hasWailsBinding() && window.go.backend.App.ListTelegramFavoritesWithCategories) {
    try {
      return await window.go.backend.App.ListTelegramFavoritesWithCategories();
    } catch (err) {
      console.warn("Falling back to Telegram favorites without categories:", err);
    }
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
  if (hasWailsBinding() && window.go.backend.App.AssignTelegramFavoriteCategory) {
    return await window.go.backend.App.AssignTelegramFavoriteCategory(username, Number(categoryID));
  }

  const normalized = String(username || "").trim().replace(/^@/, "").toLowerCase();
  if (!normalized) return;
  const categoryMap = readJson(TELEGRAM_FAVORITE_CATEGORIES_KEY, {});
  categoryMap[normalized] = Number(categoryID);
  writeJson(TELEGRAM_FAVORITE_CATEGORIES_KEY, categoryMap);
}

// YouTube
export async function getChannelVideos(channel, forceRefresh = false) {
  if (hasWailsBinding()) {
    if (forceRefresh) {
      await window.go.backend.App.YouTubeCacheClear?.();
    }
    return await window.go.backend.App.GetChannelVideos(channel);
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

export async function youtubeCacheClear() {
  if (hasWailsBinding()) {
    return await window.go.backend.App.YouTubeCacheClear?.();
  }
}

export async function listYouTubeFavorites() {
  if (hasWailsBinding()) {
    return await window.go.backend.App.ListYouTubeFavorites();
  }
  return readJson(YOUTUBE_FAVORITES_KEY, []);
}

export async function listYouTubeFavoritesWithCategories() {
  if (hasWailsBinding() && window.go.backend.App.ListYouTubeFavoritesWithCategories) {
    try {
      return await window.go.backend.App.ListYouTubeFavoritesWithCategories();
    } catch (err) {
      console.warn("Falling back to YouTube favorites without categories:", err);
    }
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
  if (hasWailsBinding() && window.go.backend.App.AssignYouTubeFavoriteCategory) {
    return await window.go.backend.App.AssignYouTubeFavoriteCategory(channelID, Number(categoryID));
  }

  const normalized = String(channelID || "").trim();
  if (!normalized) return;
  const categoryMap = readJson(YOUTUBE_FAVORITE_CATEGORIES_KEY, {});
  categoryMap[normalized] = Number(categoryID);
  writeJson(YOUTUBE_FAVORITE_CATEGORIES_KEY, categoryMap);
}
