import { els } from './dom.js';
import { normalizeCode, escapeHtml } from './utils.js';
import { callGetRate, callGetAllRates } from './api.js';
import { lastAllRates, setLastAllRates } from './state.js';
import { setBusy, showError, hideError } from './ui.js';

// ----- Single rate -----
export function renderSingleResult(result) {
  els.singleResult.classList.remove("hidden", "not-found");
  if (!result.found) {
    els.singleResult.classList.add("not-found");
    els.singleResult.innerHTML = `
      <div class="rate-value not-found-text">No rate found</div>
      <div class="rate-meta">${escapeHtml(result.base)} to ${escapeHtml(result.to)}</div>
    `;
    return;
  }
  els.singleResult.innerHTML = `
    <div class="rate-value">1 ${escapeHtml(result.base)} = ${Number(result.rate).toFixed(4)} ${escapeHtml(result.to)}</div>
    <div class="rate-meta">${result.date ? `Rate date: ${escapeHtml(result.date)}` : "Same currency"}</div>
  `;
}

export function renderConversionResult(result, amount) {
  els.convResult.classList.remove("hidden", "not-found");
  if (!result.found) {
    els.convResult.classList.add("not-found");
    els.convResult.innerHTML = `
      <div class="rate-value not-found-text">No rate found</div>
      <div class="rate-meta">${escapeHtml(result.base)} to ${escapeHtml(result.to)}</div>
    `;
    return;
  }
  const converted = Number(amount) * Number(result.rate);
  els.convResult.innerHTML = `
    <div class="rate-value">${escapeHtml(amount)} ${escapeHtml(result.base)} = ${converted.toFixed(4)} ${escapeHtml(result.to)}</div>
    <div class="rate-meta">${result.date ? `Rate date: ${escapeHtml(result.date)}` : "Same currency"}</div>
  `;
}

// ----- All rates -----
export function renderAllRates(filter = "") {
  if (!lastAllRates) return;
  const query = normalizeCode(filter);
  const codes = query
    ? lastAllRates.codes.filter((code) => code.includes(query))
    : lastAllRates.codes;
  if (codes.length === 0) {
    els.allResult.innerHTML = '<div class="meta">No matching currency codes.</div>';
    return;
  }
  els.allResult.innerHTML = codes
    .map((code) => `
      <div class="rate-item">
        <span class="code">${escapeHtml(code)}</span>
        <span>${Number(lastAllRates.rates[code]).toFixed(4)}</span>
      </div>
    `)
    .join("");
}

// ----- Init event listeners -----
export function initCurrency() {
  // Single rate
  els.singleSubmit.addEventListener("click", async () => {
    const base = normalizeCode(els.singleBase.value);
    const target = normalizeCode(els.singleTarget.value);
    if (!base || !target) {
      showError("Enter both currency codes.");
      return;
    }
    hideError();
    setBusy(true);
    try {
      const result = await callGetRate(base, target);
      renderSingleResult(result);
    } catch (err) {
      showError(err.message || String(err));
    } finally {
      setBusy(false);
    }
  });

  els.swapBtn.addEventListener("click", () => {
    const tmp = els.singleBase.value;
    els.singleBase.value = els.singleTarget.value;
    els.singleTarget.value = tmp;
  });

  // All rates
  els.allSubmit.addEventListener("click", async () => {
    const base = normalizeCode(els.allBase.value);
    if (!base) {
      showError("Enter a base currency code.");
      return;
    }
    hideError();
    setBusy(true);
    try {
      const data = await callGetAllRates(base);
      setLastAllRates(data);
      els.allMeta.textContent = `${data.codes.length} rates for ${data.base}${data.date ? ` on ${data.date}` : ""}`;
      els.allMeta.classList.remove("hidden");
      els.allSearchWrap.classList.remove("hidden");
      els.allSearch.value = "";
      renderAllRates();
    } catch (err) {
      showError(err.message || String(err));
    } finally {
      setBusy(false);
    }
  });

  els.allSearch.addEventListener("input", () => {
    renderAllRates(els.allSearch.value);
  });

  // Conversion
  els.convSubmit.addEventListener("click", async () => {
    const amount = Number(els.convAmount.value);
    const base = normalizeCode(els.convBase.value);
    const target = normalizeCode(els.convTarget.value);
    if (!amount || amount <= 0) {
      showError("Enter a valid amount.");
      return;
    }
    if (!base || !target) {
      showError("Enter both currency codes.");
      return;
    }
    hideError();
    setBusy(true);
    try {
      const result = await callGetRate(base, target);
      renderConversionResult(result, amount);
    } catch (err) {
      showError(err.message || String(err));
    } finally {
      setBusy(false);
    }
  });

  els.convSwapBtn.addEventListener("click", () => {
    const tmp = els.convBase.value;
    els.convBase.value = els.convTarget.value;
    els.convTarget.value = tmp;
  });
}