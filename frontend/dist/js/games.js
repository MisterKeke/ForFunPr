import { els } from './dom.js';
import {
  addSteamGame,
  deleteSteamGame,
  getSteamGameSettings,
  listSteamCountries,
  listSteamGames,
  refreshSteamGames,
  refreshSteamGamesOnOpen,
  setSteamGameCountry,
} from './api.js';
import { escapeHtml } from './utils.js';
import { showConfirmation } from './ui.js';

const STEAM_GAMES_CHANGED_EVENT = 'steam-games:changed';
const FALLBACK_COUNTRIES = [
  { code: 'TR', name: 'Türkiye' },
  { code: 'US', name: 'United States' },
  { code: 'GB', name: 'United Kingdom' },
  { code: 'DE', name: 'Germany' },
  { code: 'FR', name: 'France' },
  { code: 'NL', name: 'Netherlands' },
  { code: 'PL', name: 'Poland' },
  { code: 'SE', name: 'Sweden' },
  { code: 'UA', name: 'Ukraine' },
  { code: 'JP', name: 'Japan' },
  { code: 'KR', name: 'South Korea' },
  { code: 'CN', name: 'China' },
  { code: 'IN', name: 'India' },
  { code: 'BR', name: 'Brazil' },
  { code: 'MX', name: 'Mexico' },
  { code: 'CA', name: 'Canada' },
  { code: 'AU', name: 'Australia' },
];

let games = [];
let countries = [...FALLBACK_COUNTRIES];
let settings = { country_code: 'TR', updated_at: '' };
let settingsError = '';
let listLoading = false;
let addInFlight = false;
let refreshInFlight = false;
let settingsInFlight = false;
let launchRefreshRequested = false;
let listRequestID = 0;
const deletingGameIDs = new Set();

function firstValue(...values) {
  return values.find((value) => value !== undefined && value !== null);
}

function cleanString(value) {
  return String(value ?? '').trim();
}

function nullableNumber(value) {
  if (value === undefined || value === null || value === '') return null;
  const number = Number(value);
  return Number.isFinite(number) ? number : null;
}

function normalizeCountryCode(value) {
  const code = cleanString(value).toUpperCase();
  return /^[A-Z]{2}$/.test(code) ? code : '';
}

function normalizeCountry(value) {
  const code = normalizeCountryCode(firstValue(value?.code, value?.country_code, value?.countryCode));
  if (!code) return null;
  return {
    code,
    name: cleanString(firstValue(value?.name, value?.display_name, value?.displayName)) || code,
  };
}

function normalizeSettings(value) {
  const source = value?.settings || value || {};
  return {
    country_code: normalizeCountryCode(firstValue(source.country_code, source.countryCode)) || 'TR',
    updated_at: cleanString(firstValue(source.updated_at, source.updatedAt)),
  };
}

function normalizeGame(value) {
  const regularPrice = nullableNumber(firstValue(value?.regular_price_minor, value?.regularPriceMinor));
  const currentPrice = nullableNumber(firstValue(value?.current_price_minor, value?.currentPriceMinor));
  let priceStatus = cleanString(firstValue(value?.price_status, value?.priceStatus)).toLowerCase();
  if (!['priced', 'free', 'unavailable'].includes(priceStatus)) {
    priceStatus = value?.is_free || value?.isFree
      ? 'free'
      : (currentPrice !== null ? 'priced' : 'unavailable');
  }

  return {
    id: Number(firstValue(value?.id, 0)) || 0,
    steam_app_id: Number(firstValue(value?.steam_app_id, value?.steamAppID, value?.steamAppId, 0)) || 0,
    store_url: cleanString(firstValue(value?.store_url, value?.storeURL, value?.storeUrl)),
    name: cleanString(value?.name) || 'Untitled Steam game',
    image_url: cleanString(firstValue(value?.image_url, value?.imageURL, value?.imageUrl)),
    image_source_url: cleanString(firstValue(value?.image_source_url, value?.imageSourceURL, value?.imageSourceUrl)),
    price_status: priceStatus,
    currency: cleanString(value?.currency).toUpperCase(),
    regular_price_minor: regularPrice,
    current_price_minor: currentPrice,
    discount_percent: Math.max(0, Math.min(100, Number(firstValue(value?.discount_percent, value?.discountPercent, 0)) || 0)),
    price_country_code: normalizeCountryCode(firstValue(value?.price_country_code, value?.priceCountryCode)),
    last_checked_at: cleanString(firstValue(value?.last_checked_at, value?.lastCheckedAt)),
    last_attempted_at: cleanString(firstValue(value?.last_attempted_at, value?.lastAttemptedAt)),
    last_refresh_error: cleanString(firstValue(value?.last_refresh_error, value?.lastRefreshError)),
    created_at: cleanString(firstValue(value?.created_at, value?.createdAt)),
    updated_at: cleanString(firstValue(value?.updated_at, value?.updatedAt)),
  };
}

function normalizeGameList(value) {
  const source = Array.isArray(value) ? value : value?.games;
  return (Array.isArray(source) ? source : [])
    .map(normalizeGame)
    .sort((left, right) => left.name.localeCompare(right.name, undefined, { sensitivity: 'base' }));
}

function normalizedDate(value) {
  const text = cleanString(value);
  if (!text) return null;
  const normalized = /^\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}$/.test(text)
    ? `${text.replace(' ', 'T')}Z`
    : text;
  const date = new Date(normalized);
  return Number.isNaN(date.getTime()) ? null : date;
}

function formatDateTime(value) {
  const date = normalizedDate(value);
  if (!date) return cleanString(value);
  return date.toLocaleString(undefined, {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  });
}

function formatSteamPrice(minor, currency) {
  const amount = nullableNumber(minor);
  const code = cleanString(currency).toUpperCase();
  if (amount === null || !code) return '';
  try {
    return new Intl.NumberFormat(undefined, {
      style: 'currency',
      currency: code,
    }).format(amount / 100);
  } catch {
    return `${(amount / 100).toFixed(2)} ${code}`;
  }
}

function countryName(code) {
  const normalized = normalizeCountryCode(code);
  return countries.find((country) => country.code === normalized)?.name || normalized || 'Unknown country';
}

function countryLabel(code) {
  const normalized = normalizeCountryCode(code);
  if (!normalized) return 'Unknown country';
  return `${countryName(normalized)} (${normalized})`;
}

function setGamesError(message = '') {
  els.gamesError.textContent = message;
  els.gamesError.classList.toggle('hidden', !message);
}

function setAddError(message = '') {
  els.gamesAddError.textContent = message;
  els.gamesAddError.classList.toggle('hidden', !message);
}

function setRefreshStatus(message = '', state = 'info') {
  els.gamesRefreshStatus.textContent = message;
  els.gamesRefreshStatus.dataset.state = state;
  els.gamesRefreshStatus.classList.toggle('hidden', !message);
}

function setListLoading(loading) {
  listLoading = loading;
  els.gamesLoading.classList.toggle('hidden', !loading);
  renderGames();
}

function setAddBusy(busy) {
  addInFlight = busy;
  els.gamesAddSubmit.disabled = busy;
  els.gamesURL.disabled = busy;
  els.gamesAddSubmit.textContent = busy ? 'Adding game…' : 'Add game';
}

function setRefreshBusy(busy) {
  refreshInFlight = busy;
  els.gamesRefresh.disabled = busy;
  els.gamesRefresh.innerHTML = busy
    ? '<span aria-hidden="true">&#8635;</span> Refreshing…'
    : '<span aria-hidden="true">&#8635;</span> Refresh prices';
}

function setSettingsBusy(busy) {
  settingsInFlight = busy;
  els.gamesCountry.disabled = busy;
  renderCountryStatus();
}

function renderCountryOptions() {
  const selected = normalizeCountryCode(els.gamesCountry.value) || settings.country_code;
  if (settings.country_code && !countries.some((country) => country.code === settings.country_code)) {
    countries = [...countries, { code: settings.country_code, name: settings.country_code }];
  }
  els.gamesCountry.innerHTML = countries.map((country) =>
    `<option value="${escapeHtml(country.code)}">${escapeHtml(country.name)} (${escapeHtml(country.code)})</option>`
  ).join('');
  if ([...els.gamesCountry.options].some((option) => option.value === selected)) {
    els.gamesCountry.value = selected;
  } else {
    els.gamesCountry.value = settings.country_code;
  }
  renderCountryStatus();
}

function renderCountryStatus() {
  const selected = normalizeCountryCode(els.gamesCountry.value);
  const stored = settings.country_code;
  const staleCount = games.filter((game) =>
    game.price_country_code && stored && game.price_country_code !== stored
  ).length;

  let message = '';
  let state = 'current';
  if (settingsError) {
    message = settingsError;
    state = 'error';
  } else if (selected && stored && selected !== stored) {
    message = `Save to use ${countryLabel(selected)} for future price checks. Saving does not refresh prices.`;
    state = 'pending';
  } else if (staleCount > 0) {
    message = `${staleCount} cached ${staleCount === 1 ? 'price is' : 'prices are'} from another country. Click Refresh prices to update.`;
    state = 'stale';
  } else {
    message = `New checks use ${countryLabel(stored)}. Changing this setting never refreshes automatically.`;
  }

  els.gamesCountryStatus.textContent = message;
  els.gamesCountryStatus.dataset.state = state;
  els.gamesCountrySave.disabled = settingsInFlight || !selected || selected === stored;
  els.gamesCountrySave.textContent = settingsInFlight ? 'Saving…' : 'Save country';
}

function latestAttemptTime() {
  return games.reduce((latest, game) => {
    const value = game.last_attempted_at || game.last_checked_at;
    const date = normalizedDate(value);
    if (!date) return latest;
    if (!latest || date.getTime() > latest.date.getTime()) return { value, date };
    return latest;
  }, null)?.value || '';
}

function renderOverview() {
  els.gamesSummary.textContent = games.length === 1 ? '1 tracked game' : `${games.length} tracked games`;
  const attemptedAt = latestAttemptTime();
  els.gamesLastRefreshed.textContent = attemptedAt ? `Last price request ${formatDateTime(attemptedAt)}` : '';
}

function gamePriceMarkup(game) {
  if (game.price_status === 'free') {
    return '<span class="game-card-current-price game-card-free">Free</span>';
  }
  if (game.price_status !== 'priced' || game.current_price_minor === null || !game.currency) {
    return `<span class="game-card-unavailable">Price unavailable for ${escapeHtml(countryLabel(game.price_country_code || settings.country_code))}</span>`;
  }

  const current = formatSteamPrice(game.current_price_minor, game.currency);
  const regular = formatSteamPrice(game.regular_price_minor, game.currency);
  const discounted = game.regular_price_minor !== null &&
    game.current_price_minor < game.regular_price_minor;
  return `
    ${discounted && regular ? `<del class="game-card-regular-price">${escapeHtml(regular)}</del>` : ''}
    <span class="game-card-current-price">${escapeHtml(current)}</span>
  `;
}

function effectiveDiscount(game) {
  if (game.price_status !== 'priced' || game.regular_price_minor === null ||
      game.current_price_minor === null || game.current_price_minor >= game.regular_price_minor) {
    return 0;
  }
  if (game.discount_percent > 0) return game.discount_percent;
  if (game.regular_price_minor === 0) return 0;
  return Math.round((1 - (game.current_price_minor / game.regular_price_minor)) * 100);
}

function gameStoreURL(game) {
  const fallback = game.steam_app_id
    ? `https://store.steampowered.com/app/${game.steam_app_id}/`
    : '';
  const candidate = game.store_url || fallback;
  try {
    const parsed = new URL(candidate);
    if ((parsed.protocol === 'https:' || parsed.protocol === 'http:') &&
        parsed.hostname.toLowerCase() === 'store.steampowered.com') {
      return candidate;
    }
  } catch {
    return fallback;
  }
  return fallback;
}

function gameCardMarkup(game) {
  const stale = Boolean(
    settings.country_code && game.price_country_code &&
    settings.country_code !== game.price_country_code
  );
  const deleting = deletingGameIDs.has(game.id);
  const discount = effectiveDiscount(game);
  const imageURL = game.image_url || game.image_source_url;
  const lastChecked = game.last_checked_at
    ? `Checked ${formatDateTime(game.last_checked_at)}`
    : 'Not checked successfully yet';

  return `
    <article class="game-card${stale ? ' is-stale' : ''}${game.last_refresh_error ? ' has-error' : ''}" role="listitem" data-game-id="${escapeHtml(game.id)}">
      <div class="game-card-art">
        <span class="game-card-art-fallback" aria-hidden="true">&#127918;</span>
        ${imageURL ? `<img class="game-card-image" src="${escapeHtml(imageURL)}" alt="${escapeHtml(game.name)} artwork" loading="lazy" />` : ''}
        ${discount > 0 ? `<span class="game-card-discount">-${escapeHtml(discount)}%</span>` : ''}
      </div>
      <div class="game-card-body">
        <div class="game-card-heading">
          <h2 title="${escapeHtml(game.name)}">${escapeHtml(game.name)}</h2>
          <span class="game-card-appid">Steam App ${escapeHtml(game.steam_app_id || 'unknown')}</span>
        </div>
        <div class="game-card-price">${gamePriceMarkup(game)}</div>
        <div class="game-card-meta">
          ${game.price_country_code ? `<span>${escapeHtml(countryLabel(game.price_country_code))}</span>` : ''}
          ${game.currency ? `<span>${escapeHtml(game.currency)}</span>` : ''}
          <span>${escapeHtml(lastChecked)}</span>
        </div>
        ${stale ? `<div class="game-card-stale">Cached for ${escapeHtml(countryLabel(game.price_country_code))}. Refresh to check ${escapeHtml(countryLabel(settings.country_code))}.</div>` : ''}
        ${game.last_refresh_error ? `<div class="game-card-warning" role="status">${escapeHtml(game.last_refresh_error)} The previous cached data was kept.</div>` : ''}
        <div class="game-card-actions">
          <button class="secondary-btn" type="button" data-action="open" ${gameStoreURL(game) ? '' : 'disabled'}>Open store</button>
          <button class="danger-btn" type="button" data-action="delete" ${deleting ? 'disabled' : ''}>${deleting ? 'Deleting…' : 'Delete'}</button>
        </div>
      </div>
    </article>
  `;
}

function bindImageFallbacks() {
  els.gamesList.querySelectorAll('.game-card-image').forEach((image) => {
    image.addEventListener('error', () => {
      image.closest('.game-card-art')?.classList.add('has-image-error');
    }, { once: true });
  });
}

function renderGames() {
  renderOverview();
  renderCountryStatus();
  els.gamesEmpty.classList.toggle('hidden', games.length > 0 || listLoading);
  els.gamesList.classList.toggle('hidden', games.length === 0);
  els.gamesList.innerHTML = games.map(gameCardMarkup).join('');
  bindImageFallbacks();
}

async function loadCountryOptions() {
  try {
    const result = await listSteamCountries();
    const source = Array.isArray(result) ? result : result?.countries;
    const normalized = (Array.isArray(source) ? source : []).map(normalizeCountry).filter(Boolean);
    if (normalized.length > 0) {
      countries = normalized.sort((left, right) => left.name.localeCompare(right.name));
    }
  } catch (error) {
    console.warn('Steam country options are not available yet:', error);
  }
  renderCountryOptions();
}

async function loadSettings() {
  try {
    settings = normalizeSettings(await getSteamGameSettings());
    settingsError = '';
  } catch (error) {
    settingsError = error?.message || 'Steam country settings could not be loaded.';
  }
  renderCountryOptions();
  renderGames();
}

export async function loadSteamGames() {
  const request = ++listRequestID;
  setGamesError();
  setListLoading(true);
  try {
    const result = await listSteamGames();
    if (request !== listRequestID) return;
    games = normalizeGameList(result);
    renderGames();
  } catch (error) {
    if (request === listRequestID) {
      setGamesError(error?.message || 'Tracked Steam games could not be loaded.');
    }
  } finally {
    if (request === listRequestID) setListLoading(false);
  }
}

function validateSteamStoreURL(value) {
  const raw = cleanString(value);
  if (!raw) return 'Paste a Steam Store game URL.';
  try {
    const parsed = new URL(raw);
    if (parsed.protocol !== 'http:' && parsed.protocol !== 'https:') {
      return 'The Steam URL must use HTTP or HTTPS.';
    }
    if (parsed.hostname.toLowerCase() !== 'store.steampowered.com') {
      return 'Enter a URL from store.steampowered.com.';
    }
    if (!/^\/app\/\d+(?:\/|$)/i.test(parsed.pathname)) {
      return 'The URL must point to a Steam /app/ game page.';
    }
  } catch {
    return 'Enter a valid Steam Store URL.';
  }
  return '';
}

async function addGame() {
  if (addInFlight) return;
  const storeURL = els.gamesURL.value.trim();
  const validationError = validateSteamStoreURL(storeURL);
  if (validationError) {
    setAddError(validationError);
    els.gamesURL.focus();
    return;
  }

  setAddError();
  setAddBusy(true);
  try {
    const created = await addSteamGame(storeURL);
    els.gamesURL.value = '';
    await loadSteamGames();
    const name = cleanString(created?.name);
    setRefreshStatus(name ? `${name} was added to the tracker.` : 'The game was added to the tracker.', 'success');
  } catch (error) {
    setAddError(error?.message || 'The Steam game could not be added.');
  } finally {
    setAddBusy(false);
  }
}

async function saveCountry() {
  if (settingsInFlight) return;
  const countryCode = normalizeCountryCode(els.gamesCountry.value);
  if (!countryCode || countryCode === settings.country_code) return;

  settingsError = '';
  setSettingsBusy(true);
  try {
    const result = await setSteamGameCountry(countryCode);
    settings = normalizeSettings(result || { country_code: countryCode });
    setRefreshStatus(
      `${countryLabel(settings.country_code)} was saved. Existing prices remain cached until you click Refresh prices.`,
      'success'
    );
    renderGames();
  } catch (error) {
    settingsError = error?.message || 'The Steam store country could not be saved.';
  } finally {
    setSettingsBusy(false);
  }
}

function normalizeRefreshResult(value) {
  if (Array.isArray(value)) {
    return { games: normalizeGameList(value), attempted: value.length, updated: value.length, failed: 0, errors: [] };
  }
  const result = value || {};
  const resultGames = Array.isArray(result.games) ? normalizeGameList(result.games) : null;
  const errors = Array.isArray(result.errors) ? result.errors : [];
  const attempted = nullableNumber(result.attempted);
  const failed = nullableNumber(result.failed) ?? errors.length;
  const updated = nullableNumber(result.updated) ?? Math.max(0, (attempted ?? 0) - failed);
  return {
    games: resultGames,
    attempted: attempted ?? (updated + failed),
    updated,
    failed,
    errors,
    started_at: cleanString(firstValue(result.started_at, result.startedAt)),
    completed_at: cleanString(firstValue(result.completed_at, result.completedAt)),
  };
}

async function applyRefreshResult(value) {
  const result = normalizeRefreshResult(value);
  if (result.games) {
    ++listRequestID;
    games = result.games;
    renderGames();
  } else {
    await loadSteamGames();
  }

  if (result.completed_at || result.started_at) {
    els.gamesLastRefreshed.textContent = `Last price request ${formatDateTime(result.completed_at || result.started_at)}`;
  }

  const attempted = result.attempted || (result.updated + result.failed);
  if (attempted === 0) {
    setRefreshStatus('There are no tracked games to refresh.', 'info');
  } else if (result.failed > 0) {
    setRefreshStatus(
      `${result.updated} ${result.updated === 1 ? 'game' : 'games'} refreshed; ${result.failed} could not be updated. Previous cached data was kept.`,
      'warning'
    );
  } else {
    setRefreshStatus(
      `${result.updated || attempted} ${result.updated === 1 || attempted === 1 ? 'game' : 'games'} refreshed for ${countryLabel(settings.country_code)}.`,
      'success'
    );
  }
}

export async function refreshSteamGamesAtLaunch() {
  if (launchRefreshRequested) return;
  launchRefreshRequested = true;
  setRefreshBusy(true);
  setRefreshStatus('Checking saved Steam prices for this app launch…', 'info');
  try {
    await applyRefreshResult(await refreshSteamGamesOnOpen());
  } catch (error) {
    setRefreshStatus(
      `${error?.message || 'The automatic Steam price check failed.'} Cached prices were kept.`,
      'error'
    );
  } finally {
    setRefreshBusy(false);
  }
}

async function manualRefresh() {
  if (refreshInFlight) return;
  setRefreshBusy(true);
  setRefreshStatus(`Refreshing prices for ${countryLabel(settings.country_code)}…`, 'info');
  try {
    await applyRefreshResult(await refreshSteamGames());
  } catch (error) {
    setRefreshStatus(
      `${error?.message || 'Steam prices could not be refreshed.'} Cached prices were kept.`,
      'error'
    );
  } finally {
    setRefreshBusy(false);
  }
}

function openGame(game) {
  const storeURL = gameStoreURL(game);
  if (!storeURL) {
    setGamesError('This tracked game does not contain a valid Steam Store URL.');
    return;
  }
  if (window.runtime?.BrowserOpenURL) {
    window.runtime.BrowserOpenURL(storeURL);
  } else {
    window.open(storeURL, '_blank', 'noopener,noreferrer');
  }
}

async function removeGame(game) {
  if (!game?.id || deletingGameIDs.has(game.id)) return;
  const confirmed = await showConfirmation({
    title: 'Delete tracked game?',
    message: `“${game.name}” will be removed from the Steam price tracker.`,
    confirmLabel: 'Delete game',
  });
  if (!confirmed) return;

  deletingGameIDs.add(game.id);
  renderGames();
  try {
    await deleteSteamGame(game.id);
    await loadSteamGames();
    setRefreshStatus(`${game.name} was removed from the tracker.`, 'success');
  } catch (error) {
    setGamesError(error?.message || 'The tracked game could not be deleted.');
  } finally {
    deletingGameIDs.delete(game.id);
    renderGames();
  }
}

function gameFromEventTarget(target) {
  const card = target.closest('[data-game-id]');
  if (!card) return null;
  const id = Number(card.dataset.gameId);
  return games.find((game) => game.id === id) || null;
}

export function initGames() {
  if (window.runtime?.EventsOn) {
    window.runtime.EventsOn(STEAM_GAMES_CHANGED_EVENT, () => {
      void Promise.allSettled([loadSettings(), loadSteamGames()]);
    });
  }
  renderCountryOptions();
  renderGames();

  els.gamesAddForm.addEventListener('submit', (event) => {
    event.preventDefault();
    void addGame();
  });

  els.gamesCountryForm.addEventListener('submit', (event) => {
    event.preventDefault();
    void saveCountry();
  });

  els.gamesCountry.addEventListener('change', renderCountryStatus);
  els.gamesRefresh.addEventListener('click', () => void manualRefresh());

  els.gamesList.addEventListener('click', (event) => {
    const button = event.target.closest('[data-action]');
    if (!button) return;
    const game = gameFromEventTarget(button);
    if (!game) return;
    if (button.dataset.action === 'open') openGame(game);
    if (button.dataset.action === 'delete') void removeGame(game);
  });
}

export async function initializeSteamGames() {
  await Promise.allSettled([loadCountryOptions(), loadSettings()]);
  await loadSteamGames();
  await refreshSteamGamesAtLaunch();
}
