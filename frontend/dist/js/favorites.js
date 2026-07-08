import { els } from './dom.js';
import { normalizeCode, escapeHtml } from './utils.js';
import {
  listFavorites,
  getFavoritesWithRates,
  addFavorite,
  removeFavorite
} from './api.js';
import { showError, showSuccess, showWarning, showFavoriteError } from './ui.js';

// Helpers
function favoriteKey(base, target) {
  return `${normalizeCode(base)}:${normalizeCode(target)}`;
}

function splitFavoriteKey(key) {
  const parts = String(key || "").split(":");
  return { base: normalizeCode(parts[0]), target: normalizeCode(parts[1]) };
}

export async function loadFavorites() {
  try {
    const codes = await listFavorites();
    const rates = await getFavoritesWithRates();
    renderFavorites(codes, rates);
  } catch (err) {
    showError(err.message || String(err));
  }
}

export function renderFavorites(keys, rates = []) {
  if (!keys || keys.length === 0) {
    els.favoriteList.innerHTML = '<div class="favorite-empty">No favorites yet</div>';
    return;
  }
  els.favoriteList.innerHTML = keys
    .map((key) => {
      const pair = splitFavoriteKey(key);
      const rateItem = rates.find((item) => item.code === key);
      const rateText = rateItem && rateItem.found
        ? `1 ${pair.base} = ${Number(rateItem.rate).toFixed(4)} ${pair.target}`
        : "";
      return `
        <div class="favorite-item">
          <button class="favorite-chip" type="button" data-code="${escapeHtml(key)}">
            <span>${escapeHtml(pair.base)} to ${escapeHtml(pair.target)}</span>
            ${rateText ? `<small>${escapeHtml(rateText)}</small>` : ""}
          </button>
          <button class="favorite-action remove" type="button" data-code="${escapeHtml(key)}" title="Remove">x</button>
        </div>
      `;
    })
    .join("");
}

export function initFavorites() {
  els.favoriteAdd.addEventListener("click", async () => {
    const base = normalizeCode(els.singleBase.value);
    const target = normalizeCode(els.singleTarget.value);
    if (!base || !target) {
      showError("Enter both currency codes before saving.");
      return;
    }
    if (base === target) {
      showError("Cannot save a favorite for the same currency.");
      return;
    }
    const key = favoriteKey(base, target);
    try {
      const result = await addFavorite(key);
      const errorMessage = result.Error || result.error;
      const exists = result.Exists ?? result.exists;
      const added = result.Added ?? result.added;
      const pair = result.Pair || result.pair || key;

      if (errorMessage) {
        showFavoriteError(errorMessage);
        return;
      }
      await loadFavorites();
      if (exists) {
        showWarning(`"${pair}" is already in your favorites!`);
      } else if (added) {
        showSuccess(`"${pair}" added to favorites!`);
      }
    } catch (err) {
      console.error('Error:', err);
      showFavoriteError(err.message || String(err));
    }
  });

  els.favoriteRefresh.addEventListener("click", loadFavorites);

  els.favoriteList.addEventListener("click", async (event) => {
    const btn = event.target.closest("button");
    if (!btn) return;
    const code = btn.dataset.code;
    if (!code) return;

    if (btn.classList.contains("remove")) {
      try {
        await removeFavorite(code);
        await loadFavorites();
      } catch (err) {
        showError(err.message || String(err));
      }
      return;
    }

    // Click on chip: fill inputs
    const pair = splitFavoriteKey(code);
    els.singleBase.value = pair.base;
    els.singleTarget.value = pair.target;
    els.allBase.value = pair.base;
  });
}