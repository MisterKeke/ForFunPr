export function hasWailsBinding() {
  return Boolean(window.go && window.go.backend && window.go.backend.App);
}

export function normalizeCode(value) {
  return String(value || "").trim().toUpperCase();
}

export function escapeHtml(str) {
  return String(str)
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;");
}

export function formatTelegramDate(dateStr) {
  try {
    const date = new Date(dateStr);
    return date.toLocaleString();
  } catch {
    return dateStr;
  }
}