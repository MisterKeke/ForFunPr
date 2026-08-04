import { els } from './dom.js';

export function setBusy(busy) {
  els.loading.classList.toggle("hidden", !busy);
  els.singleSubmit.disabled = busy;
  els.allSubmit.disabled = busy;
  els.convSubmit.disabled = busy;
}

export function showError(message) {
  els.errorBox.textContent = message;
  els.errorBox.classList.remove("hidden");
  // Hide favorite message if visible
  const favoriteMsg = document.getElementById("favorite-message");
  if (favoriteMsg) {
    favoriteMsg.classList.add("hidden");
    favoriteMsg.textContent = "";
  }
}

export function hideError() {
  els.errorBox.classList.add("hidden");
}

// Уведомления для избранного
export function showFavoriteMessage(message, type = 'info') {
  let msgEl = els.favoriteMessage || document.getElementById('favorite-message');
  if (!msgEl) {
    msgEl = document.createElement('div');
    msgEl.id = 'favorite-message';
    msgEl.className = 'favorite-message hidden';
    els.singleResult.parentNode.insertBefore(msgEl, els.singleResult.nextSibling);
  }
  clearTimeout(msgEl._timeout);
  msgEl.classList.remove('hidden', 'success', 'warning', 'error', 'info');
  msgEl.className = `favorite-message ${type}`;
  msgEl.textContent = message;

  // hide error box
  if (els.errorBox) {
    els.errorBox.classList.add("hidden");
    els.errorBox.textContent = '';
  }

  msgEl._timeout = setTimeout(() => {
    msgEl.classList.add('hidden');
    msgEl.textContent = '';
  }, 3000);
}

export function showSuccess(message) {
  showFavoriteMessage(message, 'success');
}

export function showWarning(message) {
  showFavoriteMessage(message, 'warning');
}

export function showFavoriteError(message) {
  showFavoriteMessage(message, 'error');
}

let confirmationResolve = null;
let confirmationPreviousFocus = null;
let confirmationInitialized = false;

function closeConfirmation(confirmed) {
  if (!confirmationResolve) return;
  const resolve = confirmationResolve;
  confirmationResolve = null;
  els.confirmationModal.classList.add('hidden');
  els.confirmationModal.setAttribute('aria-hidden', 'true');
  resolve(confirmed);
  confirmationPreviousFocus?.focus?.();
  confirmationPreviousFocus = null;
}

function initConfirmation() {
  if (confirmationInitialized) return;
  confirmationInitialized = true;
  els.confirmationModalCancel.addEventListener('click', () => closeConfirmation(false));
  els.confirmationModalConfirm.addEventListener('click', () => closeConfirmation(true));
  els.confirmationModalBackdrop.addEventListener('click', () => closeConfirmation(false));
  document.addEventListener('keydown', (event) => {
    if (event.key !== 'Escape' || els.confirmationModal.classList.contains('hidden')) return;
    event.preventDefault();
    closeConfirmation(false);
  });
}

export function showConfirmation({ title, message, confirmLabel = 'Confirm' }) {
  initConfirmation();
  if (confirmationResolve) closeConfirmation(false);
  confirmationPreviousFocus = document.activeElement;
  els.confirmationModalHeading.textContent = title;
  els.confirmationModalMessage.textContent = message;
  els.confirmationModalConfirm.textContent = confirmLabel;
  els.confirmationModal.classList.remove('hidden');
  els.confirmationModal.setAttribute('aria-hidden', 'false');
  els.confirmationModalCancel.focus();
  return new Promise((resolve) => {
    confirmationResolve = resolve;
  });
}
