import {
  getClipboardState, updateClipboardSettings, listClipboardItems, setClipboardItemPinned,
  restoreClipboardItem, deleteClipboardItem, clearClipboardHistory,
  evaluateCalculatorExpression, listCalculatorUnits, convertCalculatorUnit, calculateDate,
  listCalculatorHistory, deleteCalculatorHistoryItem, clearCalculatorHistory,
  getScreenshotCapabilities, captureScreenshot, listScreenshots, renameScreenshot,
  saveScreenshotEdit, revertScreenshotEdit, runScreenshotOCR, cancelScreenshotOCR, exportScreenshot, deleteScreenshot,
  listTimeZones, listWorldClocks, createWorldClock, updateWorldClock, deleteWorldClock, reorderWorldClocks,
  convertWorldTime,
} from './api.js';
import { escapeHtml, hasWailsBinding } from './utils.js';
import { showConfirmation } from './ui.js';

const byID = (id) => document.getElementById(id);
let utilitiesInitialized = false;
let activeUtility = localStorage.getItem('activeUtility') || 'clipboard';
let clipboardState = null;
let clipboardSearchTimer = null;
let calculatorAnswer = 0;
let calculatorResultText = '';
let calculatorUnits = [];
let screenshots = [];
let screenshotCapabilities = { capture_supported: false, ocr_supported: false };
let activeScreenshot = null;
let editorTool = 'crop';
let editorImage = null;
let editorHistory = [];
let editorInteraction = null;
let timeZones = [];
let worldClocks = [];
let worldClockTicker = null;
let editingWorldClockID = 0;

function setError(id, message = '') {
  const element = byID(id);
  if (!element) return;
  element.textContent = message;
  element.classList.toggle('hidden', !message);
}

function messageOf(error, fallback) {
  return error?.message || String(error || '') || fallback;
}

function debounce(callback, delay = 220) {
  let timer = null;
  return (...args) => {
    clearTimeout(timer);
    timer = setTimeout(() => callback(...args), delay);
  };
}

export function initUtilities() {
  if (utilitiesInitialized) return;
  utilitiesInitialized = true;
  document.querySelectorAll('.utility-tab').forEach((button) => {
    button.addEventListener('click', () => selectUtility(button.dataset.utilityTab));
  });
  initClipboard();
  initCalculator();
  initScreenshots();
  initWorldClock();
  selectUtility(activeUtility, false);
}

export async function loadUtilities() {
  if (!utilitiesInitialized) initUtilities();
  await loadActiveUtility();
}

function selectUtility(name, load = true) {
  activeUtility = ['clipboard', 'calculator', 'screenshots', 'world-clock'].includes(name) ? name : 'clipboard';
  localStorage.setItem('activeUtility', activeUtility);
  document.querySelectorAll('.utility-tab').forEach((button) => {
    const selected = button.dataset.utilityTab === activeUtility;
    button.classList.toggle('active', selected);
    button.setAttribute('aria-selected', String(selected));
  });
  document.querySelectorAll('.utility-panel').forEach((panel) => panel.classList.toggle('active', panel.id === `utility-${activeUtility}`));
  if (load) void loadActiveUtility();
}

async function loadActiveUtility() {
  if (!hasWailsBinding()) {
    const errorID = activeUtility === 'world-clock' ? 'world-clock-error' : activeUtility === 'screenshots' ? 'screenshot-error' : `${activeUtility}-error`;
    setError(errorID, 'Utilities are available in the desktop app.');
    return;
  }
  if (activeUtility === 'clipboard') await loadClipboard();
  if (activeUtility === 'calculator') await loadCalculator();
  if (activeUtility === 'screenshots') await loadScreenshots();
  if (activeUtility === 'world-clock') await loadWorldClock();
}

function initClipboard() {
  byID('clipboard-toggle').addEventListener('click', async () => {
    if (!clipboardState) return;
    await saveClipboardSettings(!clipboardState.settings.collection_enabled);
  });
  byID('clipboard-save-settings').addEventListener('click', () => saveClipboardSettings(clipboardState?.settings?.collection_enabled));
  const refresh = debounce(() => loadClipboardItems());
  byID('clipboard-search').addEventListener('input', refresh);
  byID('clipboard-kind').addEventListener('change', () => loadClipboardItems());
  byID('clipboard-pinned-only').addEventListener('change', () => loadClipboardItems());
  byID('clipboard-clear').addEventListener('click', async () => {
    if (!await showConfirmation({ title: 'Clear clipboard history?', message: 'All unpinned clipboard entries will be permanently removed.', confirmLabel: 'Clear history' })) return;
    try { await clearClipboardHistory(true); await loadClipboardItems(); } catch (error) { setError('clipboard-error', messageOf(error, 'Clipboard history could not be cleared.')); }
  });
  byID('clipboard-list').addEventListener('click', handleClipboardAction);
  if (hasWailsBinding() && window.runtime?.EventsOn) {
    window.runtime.EventsOn('clipboard:changed', () => {
      clearTimeout(clipboardSearchTimer);
      clipboardSearchTimer = setTimeout(() => { if (activeUtility === 'clipboard') void loadClipboardItems(); }, 120);
    });
  }
}

async function loadClipboard() {
  setError('clipboard-error');
  try {
    clipboardState = await getClipboardState();
    const settings = clipboardState.settings;
    byID('clipboard-toggle').disabled = !clipboardState.supported;
    byID('clipboard-toggle').setAttribute('aria-checked', String(settings.collection_enabled));
    byID('clipboard-toggle').textContent = settings.collection_enabled ? 'Pause collection' : 'Enable collection';
    byID('clipboard-retention').value = String(settings.retention_days);
    byID('clipboard-maximum-items').value = String(settings.maximum_items);
    byID('clipboard-status').textContent = !clipboardState.supported
      ? 'Clipboard history is currently available only on Windows.'
      : settings.collection_enabled && clipboardState.running
        ? 'Collection is active. Copied text stays on this device.'
        : 'Collection is paused. Existing history remains available.';
    await loadClipboardItems();
  } catch (error) {
    setError('clipboard-error', messageOf(error, 'Clipboard history could not be loaded.'));
  }
}

async function saveClipboardSettings(enabled) {
  if (!clipboardState) return;
  setError('clipboard-error');
  try {
    clipboardState = await updateClipboardSettings({
      collection_enabled: Boolean(enabled),
      retention_days: Number(byID('clipboard-retention').value),
      maximum_items: Number(byID('clipboard-maximum-items').value),
      maximum_text_bytes: Number(clipboardState.settings.maximum_text_bytes || 262144),
    });
    await loadClipboard();
  } catch (error) { setError('clipboard-error', messageOf(error, 'Clipboard settings could not be saved.')); }
}

async function loadClipboardItems() {
  try {
    const result = await listClipboardItems({
      query: byID('clipboard-search').value,
      kind: byID('clipboard-kind').value,
      pinned_only: byID('clipboard-pinned-only').checked,
      limit: 200,
    });
    byID('clipboard-summary').textContent = `${result.total} ${result.total === 1 ? 'entry' : 'entries'}`;
    byID('clipboard-list').innerHTML = result.items.length ? result.items.map((item) => `
      <article class="clipboard-item ${item.pinned ? 'pinned' : ''}" data-clipboard-id="${item.id}">
        <pre class="clipboard-item-content">${escapeHtml(item.content)}</pre>
        <span class="clipboard-item-meta">${escapeHtml(item.kind.toUpperCase())} · ${formatBytes(item.byte_size)} · ${formatDateTime(item.last_copied_at)}${item.copy_count > 1 ? ` · copied ${item.copy_count}×` : ''}</span>
        <div class="clipboard-item-actions">
          <button class="secondary-btn" type="button" data-action="copy">Copy</button>
          <button class="secondary-btn" type="button" data-action="pin">${item.pinned ? 'Unpin' : 'Pin'}</button>
          <button class="danger-btn" type="button" data-action="delete">Delete</button>
        </div>
      </article>`).join('') : '<div class="utility-empty">No clipboard entries match these filters.</div>';
  } catch (error) { setError('clipboard-error', messageOf(error, 'Clipboard history could not be loaded.')); }
}

async function handleClipboardAction(event) {
  const content = event.target.closest('.clipboard-item-content');
  if (content && !event.target.closest('button')) {
    content.classList.toggle('expanded');
    return;
  }
  const button = event.target.closest('button[data-action]');
  const item = button?.closest('[data-clipboard-id]');
  if (!button || !item) return;
  const id = Number(item.dataset.clipboardId);
  button.disabled = true;
  try {
    if (button.dataset.action === 'copy') {
      await restoreClipboardItem(id);
      byID('clipboard-status').textContent = 'Copied back to the clipboard.';
    } else if (button.dataset.action === 'pin') {
      await setClipboardItemPinned(id, button.textContent === 'Pin');
      await loadClipboardItems();
    } else if (button.dataset.action === 'delete') {
      await deleteClipboardItem(id);
      await loadClipboardItems();
    }
  } catch (error) { setError('clipboard-error', messageOf(error, 'Clipboard action failed.')); }
  finally { button.disabled = false; }
}

function initCalculator() {
  document.querySelectorAll('.calculator-mode').forEach((button) => button.addEventListener('click', () => selectCalculatorMode(button.dataset.calculatorMode)));
  byID('calculator-expression-form').addEventListener('submit', handleExpressionCalculation);
  byID('calculator-unit-form').addEventListener('submit', handleUnitConversion);
  byID('calculator-date-form').addEventListener('submit', handleDateCalculation);
  byID('calculator-unit-from').addEventListener('change', syncUnitTargets);
  byID('calculator-unit-swap').addEventListener('click', () => {
    const from = byID('calculator-unit-from'); const to = byID('calculator-unit-to');
    const old = from.value; from.value = to.value; syncUnitTargets(to.value); to.value = old;
  });
  byID('calculator-date-operation').addEventListener('change', syncDateFields);
  byID('calculator-copy-result').addEventListener('click', () => copyText(calculatorResultText));
  byID('calculator-clear-history').addEventListener('click', async () => {
    if (!await showConfirmation({ title: 'Clear calculation history?', message: 'Every saved calculation will be permanently removed.', confirmLabel: 'Clear history' })) return;
    try { await clearCalculatorHistory(); await loadCalculatorHistory(); } catch (error) { setError('calculator-error', messageOf(error, 'History could not be cleared.')); }
  });
  byID('calculator-history').addEventListener('click', handleCalculatorHistoryAction);
  const today = localDateInput(new Date());
  byID('calculator-date-start').value = today;
  byID('calculator-date-end').value = today;
  syncDateFields();
}

async function loadCalculator() {
  setError('calculator-error');
  try {
    if (!calculatorUnits.length) {
      calculatorUnits = await listCalculatorUnits();
      populateUnitSelect(byID('calculator-unit-from'), calculatorUnits);
      byID('calculator-unit-from').value = 'km';
      syncUnitTargets('mi');
    }
    await loadCalculatorHistory();
  } catch (error) { setError('calculator-error', messageOf(error, 'Calculator could not be loaded.')); }
}

function selectCalculatorMode(mode) {
  document.querySelectorAll('.calculator-mode').forEach((button) => button.classList.toggle('active', button.dataset.calculatorMode === mode));
  document.querySelectorAll('.calculator-form').forEach((form) => form.classList.toggle('active', form.id === `calculator-${mode}-form`));
  setError('calculator-error');
}

async function handleExpressionCalculation(event) {
  event.preventDefault(); setError('calculator-error');
  try {
    const result = await evaluateCalculatorExpression(byID('calculator-expression').value, calculatorAnswer);
    calculatorAnswer = result.result; showCalculatorResult(result.result_text, result.approximate); await loadCalculatorHistory();
  } catch (error) { setError('calculator-error', messageOf(error, 'Expression could not be evaluated.')); }
}

async function handleUnitConversion(event) {
  event.preventDefault(); setError('calculator-error');
  try {
    const result = await convertCalculatorUnit(byID('calculator-unit-value').value, byID('calculator-unit-from').value, byID('calculator-unit-to').value);
    calculatorAnswer = result.result; showCalculatorResult(result.result_text, result.approximate); await loadCalculatorHistory();
  } catch (error) { setError('calculator-error', messageOf(error, 'Units could not be converted.')); }
}

async function handleDateCalculation(event) {
  event.preventDefault(); setError('calculator-error');
  try {
    const result = await calculateDate({
      operation: byID('calculator-date-operation').value,
      start_date: byID('calculator-date-start').value,
      end_date: byID('calculator-date-end').value,
      amount: Number(byID('calculator-date-amount').value),
      unit: byID('calculator-date-unit').value,
    });
    showCalculatorResult(result.result_text, false); await loadCalculatorHistory();
  } catch (error) { setError('calculator-error', messageOf(error, 'Date could not be calculated.')); }
}

function showCalculatorResult(text, approximate) {
  calculatorResultText = text;
  byID('calculator-result').querySelector('strong').textContent = `${approximate ? '≈ ' : ''}${text}`;
  byID('calculator-copy-result').classList.remove('hidden');
}

async function loadCalculatorHistory() {
  const history = await listCalculatorHistory(100);
  byID('calculator-history').innerHTML = history.length ? history.map((item) => `
    <div class="calculator-history-item" data-history-id="${item.id}" data-mode="${escapeHtml(item.mode)}" data-input="${escapeHtml(item.input_text)}" data-result="${escapeHtml(item.result_text)}" data-approximate="${item.approximate ? 'true' : 'false'}">
      <small>${escapeHtml(item.input_text)}</small><strong>${item.approximate ? '≈ ' : ''}${escapeHtml(item.result_text)}</strong>
      <button class="secondary-btn calculator-history-delete" type="button" data-action="delete" aria-label="Delete">×</button>
    </div>`).join('') : '<div class="utility-empty">No calculations yet.</div>';
}

async function handleCalculatorHistoryAction(event) {
  const item = event.target.closest('[data-history-id]'); if (!item) return;
  if (event.target.closest('[data-action="delete"]')) {
    try { await deleteCalculatorHistoryItem(item.dataset.historyId); await loadCalculatorHistory(); } catch (error) { setError('calculator-error', messageOf(error, 'History item could not be deleted.')); }
    return;
  }
  calculatorResultText = item.dataset.result;
  showCalculatorResult(item.dataset.result, item.dataset.approximate === 'true');
  if (item.dataset.mode === 'expression') { selectCalculatorMode('expression'); byID('calculator-expression').value = item.dataset.input; }
}

function populateUnitSelect(select, units) {
  let dimension = '';
  let html = '';
  units.forEach((unit) => {
    if (unit.dimension !== dimension) {
      if (dimension) html += '</optgroup>';
      html += `<optgroup label="${escapeHtml(capitalize(unit.dimension))}">`;
      dimension = unit.dimension;
    }
    html += `<option value="${escapeHtml(unit.id)}">${escapeHtml(unit.name)} (${escapeHtml(unit.symbol)})</option>`;
  });
  select.innerHTML = html + (dimension ? '</optgroup>' : '');
}

function syncUnitTargets(preferred = '') {
  const source = calculatorUnits.find((unit) => unit.id === byID('calculator-unit-from').value);
  if (!source) return;
  const targets = calculatorUnits.filter((unit) => unit.dimension === source.dimension);
  byID('calculator-unit-to').innerHTML = targets.map((unit) => `<option value="${escapeHtml(unit.id)}">${escapeHtml(unit.name)} (${escapeHtml(unit.symbol)})</option>`).join('');
  if (targets.some((unit) => unit.id === preferred)) byID('calculator-unit-to').value = preferred;
  else if (targets.length > 1) byID('calculator-unit-to').selectedIndex = 1;
}

function syncDateFields() {
  const difference = byID('calculator-date-operation').value === 'difference';
  byID('calculator-date-end-field').classList.toggle('hidden', !difference);
  byID('calculator-date-amount-field').classList.toggle('hidden', difference);
  byID('calculator-date-unit-field').classList.toggle('hidden', difference);
}

function initScreenshots() {
  byID('screenshot-capture-screen').addEventListener('click', () => handleCapture('screen'));
  byID('screenshot-capture-window').addEventListener('click', () => handleCapture('window'));
  byID('screenshot-capture-region').addEventListener('click', () => handleCapture('region'));
  byID('screenshot-refresh').addEventListener('click', () => loadScreenshots());
  byID('screenshot-search').addEventListener('input', debounce(() => loadScreenshotList()));
  byID('screenshot-grid').addEventListener('click', handleScreenshotAction);
  byID('screenshot-grid').addEventListener('change', handleScreenshotRename);
  if (window.runtime?.EventsOn) {
    window.runtime.EventsOn('screenshots:ocr-status', () => void loadScreenshotList());
  }
  byID('screenshot-editor-close').addEventListener('click', closeScreenshotEditor);
  byID('screenshot-editor-cancel').addEventListener('click', closeScreenshotEditor);
  document.querySelector('#screenshot-editor-modal .modal-backdrop').addEventListener('click', closeScreenshotEditor);
  document.querySelectorAll('.screenshot-tool').forEach((button) => button.addEventListener('click', () => selectEditorTool(button.dataset.screenshotTool)));
  byID('screenshot-editor-undo').addEventListener('click', undoEditor);
  byID('screenshot-editor-reset').addEventListener('click', resetEditor);
  byID('screenshot-editor-save').addEventListener('click', saveEditor);
  document.addEventListener('keydown', (event) => {
    if (event.key === 'Escape' && !byID('screenshot-editor-modal').classList.contains('hidden')) {
      event.preventDefault(); closeScreenshotEditor();
    }
    if (event.ctrlKey && event.shiftKey && event.key.toLowerCase() === 's' && document.getElementById('view-utilities').classList.contains('active')) {
      event.preventDefault(); void handleCapture('region');
    }
  });
  const canvas = byID('screenshot-editor-canvas');
  canvas.addEventListener('pointerdown', editorPointerDown);
  canvas.addEventListener('pointermove', editorPointerMove);
  canvas.addEventListener('pointerup', editorPointerUp);
  canvas.addEventListener('pointercancel', editorPointerUp);
}

async function loadScreenshots() {
  setError('screenshot-error');
  try {
    screenshotCapabilities = await getScreenshotCapabilities();
    ['screenshot-capture-screen', 'screenshot-capture-window', 'screenshot-capture-region'].forEach((id) => { byID(id).disabled = !screenshotCapabilities.capture_supported; });
    byID('screenshot-status').textContent = screenshotCapabilities.capture_supported
      ? `Screen capture is ready. Use Ctrl+Shift+S here for a region capture.${screenshotCapabilities.ocr_supported ? ' Local Windows OCR is available.' : ' OCR is unavailable on this platform.'}`
      : 'Screen capture is currently available only on Windows.';
    await loadScreenshotList();
  } catch (error) { setError('screenshot-error', messageOf(error, 'Screenshot library could not be loaded.')); }
}

async function loadScreenshotList() {
  try {
    const result = await listScreenshots({ query: byID('screenshot-search').value, limit: 100 });
    screenshots = result.screenshots;
    byID('screenshot-summary').textContent = `${result.total} ${result.total === 1 ? 'screenshot' : 'screenshots'}`;
    byID('screenshot-grid').innerHTML = screenshots.length ? screenshots.map((item) => `
      <article class="screenshot-card" data-screenshot-id="${escapeHtml(item.id)}">
        <img class="screenshot-card-image" src="${escapeHtml(item.thumbnail_url)}?v=${encodeURIComponent(item.updated_at)}" alt="${escapeHtml(item.title || 'Screenshot')}" data-action="edit" />
        <div class="screenshot-card-body">
          <input class="screenshot-card-title" type="text" maxlength="200" value="${escapeHtml(item.title)}" placeholder="Untitled screenshot" aria-label="Screenshot title" />
          <p class="screenshot-card-meta">${item.width}×${item.height} · ${formatBytes(item.byte_size)} · ${formatDateTime(item.captured_at)}</p>
          <p class="screenshot-card-meta">OCR: ${escapeHtml(item.ocr_status || 'not_started')}${item.ocr_started_at ? ` · started ${escapeHtml(formatDateTime(item.ocr_started_at))}` : ''}</p>
          ${item.ocr_failure_message ? `<p class="utility-error">${escapeHtml(item.ocr_failure_message)}</p>` : ''}
          ${item.ocr_text ? `<pre class="screenshot-ocr-preview">${escapeHtml(item.ocr_text)}</pre>` : ''}
          <div class="screenshot-card-actions">
            <button class="secondary-btn" type="button" data-action="edit">Edit</button>
            ${item.has_edit ? '<button class="secondary-btn" type="button" data-action="revert">Revert to original</button>' : ''}
            ${item.ocr_status === 'queued' || item.ocr_status === 'processing'
              ? '<button class="secondary-btn" type="button" data-action="cancel-ocr">Cancel OCR</button>'
              : `<button class="secondary-btn" type="button" data-action="ocr" ${screenshotCapabilities.ocr_supported ? '' : 'disabled title="Local OCR is unavailable on this platform."'}>${item.ocr_status === 'complete' ? 'OCR again' : 'Extract text'}</button>`}
            <button class="secondary-btn" type="button" data-action="copy">Copy</button>
            <button class="secondary-btn" type="button" data-action="export">Export</button>
            <button class="danger-btn" type="button" data-action="delete">Delete</button>
          </div>
        </div>
      </article>`).join('') : '<div class="utility-empty">No screenshots yet. Capture the screen, a window, or a region.</div>';
  } catch (error) { setError('screenshot-error', messageOf(error, 'Screenshots could not be loaded.')); }
}

async function handleCapture(mode) {
  if (!screenshotCapabilities.capture_supported) return;
  setError('screenshot-error');
  const captureButtons = ['screenshot-capture-screen', 'screenshot-capture-window', 'screenshot-capture-region'].map(byID);
  captureButtons.forEach((button) => { button.disabled = true; });
  byID('screenshot-status').textContent = mode === 'region' ? 'Drag on the desktop to select a region. Press Escape to cancel.' : 'Capturing…';
  try {
    const result = await captureScreenshot(mode);
    if (result?.cancelled) {
      byID('screenshot-status').textContent = 'Region capture cancelled.';
      return;
    }
    const item = result?.screenshot;
    if (!item) throw new Error('The capture did not return a screenshot.');
    await loadScreenshotList();
    byID('screenshot-status').textContent = 'Screenshot saved locally.';
  } catch (error) { setError('screenshot-error', messageOf(error, 'Screen could not be captured.')); }
  finally { captureButtons.forEach((button) => { button.disabled = !screenshotCapabilities.capture_supported; }); }
}

async function handleScreenshotAction(event) {
  const button = event.target.closest('[data-action]');
  const card = button?.closest('[data-screenshot-id]');
  if (!button || !card) return;
  const item = screenshots.find((value) => value.id === card.dataset.screenshotId);
  if (!item) return;
  button.disabled = true;
  try {
    if (button.dataset.action === 'edit') await openScreenshotEditor(item, 'draw');
    if (button.dataset.action === 'ocr') { byID('screenshot-status').textContent = 'Text extraction queued…'; await runScreenshotOCR(item.id); await loadScreenshotList(); }
    if (button.dataset.action === 'cancel-ocr') {
      const cancelled = await cancelScreenshotOCR(item.id);
      await loadScreenshotList();
      byID('screenshot-status').textContent = cancelled?.ocr_failure_code === 'cancelled'
        ? 'Text extraction cancelled.'
        : 'Text extraction had already finished.';
    }
    if (button.dataset.action === 'revert') {
      if (await showConfirmation({ title: 'Revert screenshot?', message: 'Discard the current edit and restore the original capture?', confirmLabel: 'Revert to original' })) {
        await revertScreenshotEdit(item.id); await loadScreenshotList(); byID('screenshot-status').textContent = 'Original screenshot restored.';
      }
    }
    if (button.dataset.action === 'copy') { await copyImage(item.image_url); byID('screenshot-status').textContent = 'Screenshot copied to the clipboard.'; }
    if (button.dataset.action === 'export') { const path = await exportScreenshot(item.id); if (path) byID('screenshot-status').textContent = `Exported to ${path}`; }
    if (button.dataset.action === 'delete') {
      if (await showConfirmation({ title: 'Delete screenshot?', message: 'The original, edited image, recognized text, and thumbnail will be permanently removed.', confirmLabel: 'Delete screenshot' })) { await deleteScreenshot(item.id); await loadScreenshotList(); }
    }
  } catch (error) { setError('screenshot-error', messageOf(error, 'Screenshot action failed.')); }
  finally { button.disabled = false; }
}

async function handleScreenshotRename(event) {
  if (!event.target.matches('.screenshot-card-title')) return;
  const card = event.target.closest('[data-screenshot-id]');
  try { await renameScreenshot(card.dataset.screenshotId, event.target.value); await loadScreenshotList(); }
  catch (error) { setError('screenshot-error', messageOf(error, 'Screenshot could not be renamed.')); }
}

async function openScreenshotEditor(item, initialTool = 'draw') {
  activeScreenshot = item; editorHistory = []; editorInteraction = null;
  const image = new Image();
  image.decoding = 'async';
  image.src = `${item.image_url}?v=${encodeURIComponent(item.updated_at)}&editor=1`;
  await image.decode();
  editorImage = image;
  const canvas = byID('screenshot-editor-canvas');
  canvas.width = image.naturalWidth; canvas.height = image.naturalHeight;
  canvas.getContext('2d').drawImage(image, 0, 0);
  byID('screenshot-editor-heading').textContent = item.title || 'Edit capture';
  byID('screenshot-editor-modal').classList.remove('hidden');
  byID('screenshot-editor-modal').setAttribute('aria-hidden', 'false');
  setError('screenshot-editor-error'); selectEditorTool(initialTool);
}

function closeScreenshotEditor() {
  byID('screenshot-editor-modal').classList.add('hidden');
  byID('screenshot-editor-modal').setAttribute('aria-hidden', 'true');
  activeScreenshot = null; editorImage = null; editorHistory = []; editorInteraction = null;
}

function selectEditorTool(tool) {
  editorTool = tool;
  document.querySelectorAll('.screenshot-tool').forEach((button) => button.classList.toggle('active', button.dataset.screenshotTool === tool));
  const descriptions = { crop: 'Drag to choose the area to keep, then save.', draw: 'Drag to draw a red freehand line.', rectangle: 'Drag to draw a red rectangle.', arrow: 'Drag from the arrow tail to its point.', text: 'Click where the text should begin.', highlight: 'Drag to add a translucent yellow highlight.', blur: 'Drag over an area to blur it.' };
  byID('screenshot-editor-help').textContent = descriptions[tool] || '';
}

function canvasPoint(event) {
  const canvas = byID('screenshot-editor-canvas'); const rect = canvas.getBoundingClientRect();
  return { x: Math.max(0, Math.min(canvas.width, (event.clientX - rect.left) * canvas.width / rect.width)), y: Math.max(0, Math.min(canvas.height, (event.clientY - rect.top) * canvas.height / rect.height)) };
}

function cloneCanvas(source = byID('screenshot-editor-canvas')) {
  const copy = document.createElement('canvas'); copy.width = source.width; copy.height = source.height; copy.getContext('2d').drawImage(source, 0, 0); return copy;
}

function pushEditorHistory(snapshot = cloneCanvas()) {
  editorHistory.push(snapshot); if (editorHistory.length > 12) editorHistory.shift();
}

function editorPointerDown(event) {
  if (!activeScreenshot) return;
  const canvas = byID('screenshot-editor-canvas');
  const point = canvasPoint(event); const snapshot = cloneCanvas(); pushEditorHistory(snapshot);
  if (editorTool === 'text') {
    const value = window.prompt('Text to add:');
    if (!value) { editorHistory.pop(); return; }
    const context = canvas.getContext('2d');
    context.fillStyle = '#ff4f6d';
    context.font = `700 ${Math.max(18, Math.round(canvas.width / 45))}px "Segoe UI", sans-serif`;
    context.textBaseline = 'top';
    context.fillText(value.slice(0, 200), point.x, point.y);
    return;
  }
  canvas.setPointerCapture(event.pointerId);
  editorInteraction = { start: point, last: point, snapshot };
  if (editorTool === 'draw') {
    const context = canvas.getContext('2d'); context.strokeStyle = '#ff4f6d'; context.lineWidth = Math.max(3, canvas.width / 450); context.lineCap = 'round'; context.beginPath(); context.moveTo(point.x, point.y);
  }
}

function editorPointerMove(event) {
  if (!editorInteraction) return;
  const canvas = byID('screenshot-editor-canvas'); const context = canvas.getContext('2d'); const point = canvasPoint(event);
  if (editorTool === 'draw') { context.lineTo(point.x, point.y); context.stroke(); editorInteraction.last = point; return; }
  context.clearRect(0, 0, canvas.width, canvas.height); context.drawImage(editorInteraction.snapshot, 0, 0);
  const box = selectionBox(editorInteraction.start, point);
  if (editorTool === 'crop') { context.fillStyle = 'rgba(0,0,0,.5)'; context.fillRect(0, 0, canvas.width, canvas.height); context.drawImage(editorInteraction.snapshot, box.x, box.y, box.w, box.h, box.x, box.y, box.w, box.h); context.strokeStyle = '#22d3ee'; context.lineWidth = Math.max(2, canvas.width / 700); context.strokeRect(box.x, box.y, box.w, box.h); }
  if (editorTool === 'rectangle') { context.strokeStyle = '#ff4f6d'; context.lineWidth = Math.max(3, canvas.width / 450); context.strokeRect(box.x, box.y, box.w, box.h); }
  if (editorTool === 'arrow') drawEditorArrow(context, editorInteraction.start, point, Math.max(3, canvas.width / 450));
  if (editorTool === 'highlight') { context.fillStyle = 'rgba(255, 215, 64, .34)'; context.fillRect(box.x, box.y, box.w, box.h); }
  if (editorTool === 'blur') { context.filter = 'blur(12px)'; context.drawImage(editorInteraction.snapshot, box.x, box.y, box.w, box.h, box.x, box.y, box.w, box.h); context.filter = 'none'; }
  editorInteraction.last = point;
}

function editorPointerUp(event) {
  if (!editorInteraction) return;
  editorPointerMove(event);
  const canvas = byID('screenshot-editor-canvas');
  if (editorTool === 'crop') {
    const box = selectionBox(editorInteraction.start, editorInteraction.last);
    if (box.w >= 8 && box.h >= 8) {
      const source = editorInteraction.snapshot; const cropped = document.createElement('canvas');
      cropped.width = Math.round(box.w); cropped.height = Math.round(box.h);
      cropped.getContext('2d').drawImage(source, box.x, box.y, box.w, box.h, 0, 0, cropped.width, cropped.height);
      canvas.width = cropped.width; canvas.height = cropped.height; canvas.getContext('2d').drawImage(cropped, 0, 0);
    } else {
      canvas.getContext('2d').drawImage(editorInteraction.snapshot, 0, 0);
      editorHistory.pop();
    }
  }
  editorInteraction = null;
}

function selectionBox(start, end) { return { x: Math.min(start.x, end.x), y: Math.min(start.y, end.y), w: Math.abs(end.x - start.x), h: Math.abs(end.y - start.y) }; }

function drawEditorArrow(context, start, end, width) {
  const angle = Math.atan2(end.y - start.y, end.x - start.x);
  const head = Math.max(14, width * 4);
  context.strokeStyle = '#ff4f6d'; context.fillStyle = '#ff4f6d'; context.lineWidth = width; context.lineCap = 'round';
  context.beginPath(); context.moveTo(start.x, start.y); context.lineTo(end.x, end.y); context.stroke();
  context.beginPath(); context.moveTo(end.x, end.y);
  context.lineTo(end.x - head * Math.cos(angle - Math.PI / 6), end.y - head * Math.sin(angle - Math.PI / 6));
  context.lineTo(end.x - head * Math.cos(angle + Math.PI / 6), end.y - head * Math.sin(angle + Math.PI / 6));
  context.closePath(); context.fill();
}

function undoEditor() {
  const previous = editorHistory.pop(); if (!previous) return;
  const canvas = byID('screenshot-editor-canvas'); canvas.width = previous.width; canvas.height = previous.height; canvas.getContext('2d').drawImage(previous, 0, 0);
}

function resetEditor() {
  if (!editorImage) return;
  pushEditorHistory(); const canvas = byID('screenshot-editor-canvas'); canvas.width = editorImage.naturalWidth; canvas.height = editorImage.naturalHeight; canvas.getContext('2d').drawImage(editorImage, 0, 0);
}

async function saveEditor() {
  if (!activeScreenshot) return;
  setError('screenshot-editor-error'); byID('screenshot-editor-save').disabled = true;
  try {
    await saveScreenshotEdit(activeScreenshot.id, byID('screenshot-editor-canvas').toDataURL('image/png'));
    closeScreenshotEditor(); await loadScreenshotList(); byID('screenshot-status').textContent = 'Screenshot edit saved. The original is preserved.';
  } catch (error) { setError('screenshot-editor-error', messageOf(error, 'Screenshot edit could not be saved.')); }
  finally { byID('screenshot-editor-save').disabled = false; }
}

function initWorldClock() {
  byID('world-clock-add-form').addEventListener('submit', handleWorldClockAdd);
  byID('world-clock-list').addEventListener('click', handleWorldClockAction);
  byID('world-clock-24-hour').checked = localStorage.getItem('worldClock24Hour') !== 'false';
  byID('world-clock-24-hour').addEventListener('change', () => { localStorage.setItem('worldClock24Hour', String(byID('world-clock-24-hour').checked)); renderWorldClocks(); });
  byID('world-planner-form').addEventListener('submit', handleWorldPlanner);
  const now = new Date(); now.setSeconds(0, 0); byID('world-planner-at').value = localDateTimeInput(now);
}

async function loadWorldClock() {
  setError('world-clock-error');
  try {
    if (!timeZones.length) {
      timeZones = await listTimeZones();
      populateTimeZones(byID('world-clock-zone'));
      populateTimeZones(byID('world-planner-from'));
      const localZone = Intl.DateTimeFormat().resolvedOptions().timeZone;
      if (timeZones.some((zone) => zone.id === localZone)) { byID('world-clock-zone').value = localZone; byID('world-planner-from').value = localZone; }
    }
    worldClocks = await listWorldClocks();
    worldClocks.forEach((clock) => {
      if (!timeZones.some((zone) => zone.id === clock.time_zone_id)) {
        timeZones.push({ id: clock.time_zone_id, city: clock.label, region: 'Saved' });
      }
    });
    populateTimeZones(byID('world-planner-from'));
    renderWorldClocks();
    clearInterval(worldClockTicker); worldClockTicker = setInterval(updateWorldClockTimes, 1000);
  } catch (error) { setError('world-clock-error', messageOf(error, 'World clocks could not be loaded.')); }
}

function populateTimeZones(select) {
  if (select.tagName === 'INPUT') {
    byID('world-clock-zone-options').innerHTML = timeZones.map((zone) => `<option value="${escapeHtml(zone.id)}">${escapeHtml(zone.city)}</option>`).join('');
    return;
  }
  const previous = select.value;
  let region = '';
  let html = '';
  timeZones.forEach((zone) => {
    if (zone.region !== region) { if (region) html += '</optgroup>'; html += `<optgroup label="${escapeHtml(zone.region)}">`; region = zone.region; }
    html += `<option value="${escapeHtml(zone.id)}">${escapeHtml(zone.city)} — ${escapeHtml(zone.id)}</option>`;
  });
  select.innerHTML = html + (region ? '</optgroup>' : '');
  if ([...select.options].some((option) => option.value === previous)) select.value = previous;
}

async function handleWorldClockAdd(event) {
  event.preventDefault(); setError('world-clock-error');
  const zoneID = byID('world-clock-zone').value;
  const zone = timeZones.find((item) => item.id === zoneID);
  try {
    const request = {
      id: editingWorldClockID,
      label: byID('world-clock-label').value.trim() || zone?.city || zoneID,
      time_zone_id: zoneID,
      working_day_start_minutes: timeToMinutes(byID('world-clock-work-start').value),
      working_day_end_minutes: timeToMinutes(byID('world-clock-work-end').value),
    };
    if (editingWorldClockID) await updateWorldClock(request); else await createWorldClock(request);
    editingWorldClockID = 0;
    byID('world-clock-add-form').querySelector('button[type="submit"]').textContent = 'Add clock';
    byID('world-clock-label').value = '';
    worldClocks = await listWorldClocks();
    worldClocks.forEach((clock) => {
      if (!timeZones.some((item) => item.id === clock.time_zone_id)) timeZones.push({ id: clock.time_zone_id, city: clock.label, region: 'Saved' });
    });
    populateTimeZones(byID('world-planner-from'));
    renderWorldClocks();
  } catch (error) { setError('world-clock-error', messageOf(error, 'World clock could not be added.')); }
}

function renderWorldClocks() {
  byID('world-clock-list').innerHTML = worldClocks.length ? worldClocks.map((clock, index) => `
    <article class="world-clock-card" data-world-clock-id="${clock.id}" data-zone="${escapeHtml(clock.time_zone_id)}">
      <h3>${escapeHtml(clock.label)}</h3><span class="world-clock-zone">${escapeHtml(clock.time_zone_id)} · ${formatUTCOffset(clock.utc_offset_seconds)}</span>
      <strong class="world-clock-time">—</strong><span class="world-clock-date">—</span>
      <p class="world-clock-work">Working hours ${minutesToTime(clock.working_day_start_minutes)}–${minutesToTime(clock.working_day_end_minutes)}</p>
      <button class="danger-btn world-clock-delete" type="button" data-action="delete" aria-label="Delete">×</button>
      <div class="screenshot-card-actions"><button class="secondary-btn" type="button" data-action="edit">Edit</button><button class="secondary-btn" type="button" data-action="up" ${index === 0 ? 'disabled' : ''}>↑ Earlier</button><button class="secondary-btn" type="button" data-action="down" ${index === worldClocks.length - 1 ? 'disabled' : ''}>↓ Later</button></div>
    </article>`).join('') : '<div class="utility-empty">Add a location to start comparing times.</div>';
  updateWorldClockTimes();
}

function updateWorldClockTimes() {
  const hour12 = !byID('world-clock-24-hour').checked;
  const now = new Date();
  document.querySelectorAll('.world-clock-card').forEach((card) => {
    const zone = card.dataset.zone;
    try {
      card.querySelector('.world-clock-time').textContent = new Intl.DateTimeFormat(undefined, { timeZone: zone, hour: '2-digit', minute: '2-digit', second: '2-digit', hour12 }).format(now);
      card.querySelector('.world-clock-date').textContent = new Intl.DateTimeFormat(undefined, { timeZone: zone, weekday: 'short', year: 'numeric', month: 'short', day: 'numeric' }).format(now);
    } catch { card.querySelector('.world-clock-time').textContent = 'Unavailable'; }
  });
}

async function handleWorldClockAction(event) {
  const button = event.target.closest('[data-action]'); const card = button?.closest('[data-world-clock-id]'); if (!button || !card) return;
  const id = Number(card.dataset.worldClockId); const index = worldClocks.findIndex((clock) => clock.id === id);
  try {
    if (button.dataset.action === 'delete') {
      const clock = worldClocks[index];
      if (!await showConfirmation({ title: 'Delete world clock?', message: `“${clock?.label || 'This clock'}” will be removed from your saved locations.`, confirmLabel: 'Delete clock' })) return;
      await deleteWorldClock(id); worldClocks = await listWorldClocks(); renderWorldClocks(); return;
    }
    if (button.dataset.action === 'edit') {
      const clock = worldClocks[index]; if (!clock) return;
      editingWorldClockID = clock.id;
      byID('world-clock-label').value = clock.label;
      byID('world-clock-zone').value = clock.time_zone_id;
      byID('world-clock-work-start').value = minutesToTime(clock.working_day_start_minutes);
      byID('world-clock-work-end').value = minutesToTime(clock.working_day_end_minutes);
      byID('world-clock-add-form').querySelector('button[type="submit"]').textContent = 'Save clock';
      byID('world-clock-label').focus();
      return;
    }
    const swapIndex = button.dataset.action === 'up' ? index - 1 : index + 1;
    if (index < 0 || swapIndex < 0 || swapIndex >= worldClocks.length) return;
    [worldClocks[index], worldClocks[swapIndex]] = [worldClocks[swapIndex], worldClocks[index]];
    await reorderWorldClocks(worldClocks.map((clock) => clock.id)); worldClocks = await listWorldClocks(); renderWorldClocks();
  } catch (error) { setError('world-clock-error', messageOf(error, 'World clock action failed.')); }
}

async function handleWorldPlanner(event) {
  event.preventDefault(); setError('world-clock-error');
  if (!worldClocks.length) { byID('world-planner-results').innerHTML = '<div class="utility-empty">Add at least one saved clock first.</div>'; return; }
  try {
    const results = await convertWorldTime({ at: byID('world-planner-at').value, from_zone_id: byID('world-planner-from').value, to_zone_ids: worldClocks.map((clock) => clock.time_zone_id) });
    const hour12 = !byID('world-clock-24-hour').checked;
    byID('world-planner-results').innerHTML = results.map((result, index) => {
      const date = new Date(result.local_time); const clock = worldClocks[index];
      const dateText = new Intl.DateTimeFormat(undefined, { timeZone: result.time_zone_id, weekday: 'short', year: 'numeric', month: 'short', day: 'numeric' }).format(date);
      const timeText = new Intl.DateTimeFormat(undefined, { timeZone: result.time_zone_id, hour: '2-digit', minute: '2-digit', hour12 }).format(date);
      const timeParts = new Intl.DateTimeFormat('en-GB', { timeZone: result.time_zone_id, hour: '2-digit', minute: '2-digit', hourCycle: 'h23' }).formatToParts(date);
      const hour = Number(timeParts.find((part) => part.type === 'hour')?.value || 0);
      const minute = Number(timeParts.find((part) => part.type === 'minute')?.value || 0);
      const localMinutes = hour * 60 + minute;
      const working = clock && localMinutes >= clock.working_day_start_minutes && localMinutes < clock.working_day_end_minutes;
      return `<div class="world-planner-result ${working ? 'in-hours' : 'outside-hours'}"><span>${escapeHtml(clock?.label || result.time_zone_id)} · ${working ? 'Working hours' : 'Outside hours'}</span><strong>${escapeHtml(timeText)}</strong><span>${escapeHtml(dateText)} · ${formatUTCOffset(result.utc_offset_seconds)}</span></div>`;
    }).join('');
  } catch (error) { setError('world-clock-error', messageOf(error, 'Time could not be compared.')); }
}

async function copyText(value) {
  if (!value) return;
  await navigator.clipboard.writeText(value);
}

async function copyImage(url) {
  const response = await fetch(url); const blob = await response.blob();
  if (!window.ClipboardItem || !navigator.clipboard?.write) throw new Error('Image clipboard access is unavailable in this WebView.');
  await navigator.clipboard.write([new ClipboardItem({ [blob.type || 'image/png']: blob })]);
}

function timeToMinutes(value) { const [hours, minutes] = String(value || '00:00').split(':').map(Number); return hours * 60 + minutes; }
function minutesToTime(value) { const hours = Math.floor(Number(value) / 60); const minutes = Number(value) % 60; return `${String(hours).padStart(2, '0')}:${String(minutes).padStart(2, '0')}`; }
function localDateInput(date) { return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}`; }
function localDateTimeInput(date) { return `${localDateInput(date)}T${String(date.getHours()).padStart(2, '0')}:${String(date.getMinutes()).padStart(2, '0')}`; }
function formatBytes(value) { const bytes = Number(value) || 0; if (bytes < 1024) return `${bytes} B`; if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KiB`; return `${(bytes / 1024 / 1024).toFixed(1)} MiB`; }
function formatDateTime(value) { const date = new Date(value); return Number.isNaN(date.getTime()) ? String(value || '') : date.toLocaleString(); }
function capitalize(value) { return value ? value[0].toUpperCase() + value.slice(1) : ''; }
function formatUTCOffset(seconds) { const total = Number(seconds) || 0; const sign = total >= 0 ? '+' : '−'; const absolute = Math.abs(total); return `UTC${sign}${String(Math.floor(absolute / 3600)).padStart(2, '0')}:${String(Math.floor((absolute % 3600) / 60)).padStart(2, '0')}`; }
