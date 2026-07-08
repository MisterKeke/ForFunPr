import { hasWailsBinding } from './utils.js';

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
  return [];
}

export async function addTelegramFavorite(channel) {
  if (hasWailsBinding()) {
    return await window.go.backend.App.AddTelegramFavorite(channel);
  }
}

export async function removeTelegramFavorite(channel) {
  if (hasWailsBinding()) {
    return await window.go.backend.App.RemoveTelegramFavorite(channel);
  }
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
  return [];
}

export async function addYouTubeFavorite(channelID) {
  if (hasWailsBinding()) {
    return await window.go.backend.App.AddYouTubeFavorite(channelID);
  }
}

export async function removeYouTubeFavorite(channelID) {
  if (hasWailsBinding()) {
    return await window.go.backend.App.RemoveYouTubeFavorite(channelID);
  }
}