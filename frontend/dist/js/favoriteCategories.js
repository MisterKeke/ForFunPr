import { els } from './dom.js';
import { escapeHtml } from './utils.js';
import { createFavoriteCategory, listFavoriteCategories, renameFavoriteCategory } from './api.js';
import { showError } from './ui.js';
import { normalizeFavoriteSource } from './favoriteSources.js';

const UNCATEGORIZED_ID = "uncategorized";

let cachedCategories = {};
let modalState = null;
let managerSource = "telegram";

export function normalizeCategory(category) {
  if (!category) return null;
  const id = category.id ?? category.ID;
  const name = category.name ?? category.Name;
  if (id === undefined || id === null || !name) return null;
  return {
    id: Number(id),
    name: String(name),
    source: category.source ?? category.Source ?? "telegram",
    color: category.color ?? category.Color ?? "",
    created_at: category.created_at ?? category.CreatedAt ?? "",
  };
}

export async function loadFavoriteCategories(source = "telegram", force = false) {
  const normalizedSource = normalizeFavoriteSource(source);
  if (!force && cachedCategories[normalizedSource]?.length > 0) {
    return cachedCategories[normalizedSource];
  }
  const categories = await listFavoriteCategories(normalizedSource);
  cachedCategories[normalizedSource] = (Array.isArray(categories) ? categories : [])
    .map(normalizeCategory)
    .filter(Boolean);
  return cachedCategories[normalizedSource];
}

export function normalizeFavoriteItem(item, sourceField) {
  if (typeof item === "string") {
    return { sourceId: item, label: item, categoryId: null };
  }

  const sourceId =
    item?.[sourceField] ??
    item?.source_id ??
    item?.sourceId ??
    item?.username ??
    item?.channel_id ??
    item?.channelId ??
    "";
  const label =
    item?.display_name ??
    item?.displayName ??
    item?.label ??
    item?.username ??
    item?.Username ??
    sourceId;
  const categoryId = item?.category_id ?? item?.categoryId ?? item?.CategoryID ?? null;

  return {
    sourceId: String(sourceId || ""),
    label: String(label || sourceId || ""),
    categoryId: categoryId === 0 || categoryId ? Number(categoryId) : null,
  };
}

function getCategoryById(categories, categoryId) {
  return categories.find((category) => String(category.id) === String(categoryId));
}

function getGroupedFavorites(items, categories) {
  const groups = new Map();

  categories.forEach((category) => {
    groups.set(String(category.id), {
      id: String(category.id),
      name: category.name,
      items: [],
    });
  });

  groups.set(UNCATEGORIZED_ID, {
    id: UNCATEGORIZED_ID,
    name: "Uncategorized",
    items: [],
  });

  items.forEach((item) => {
    const key = item.categoryId && getCategoryById(categories, item.categoryId)
      ? String(item.categoryId)
      : UNCATEGORIZED_ID;
    groups.get(key).items.push(item);
  });

  return Array.from(groups.values()).filter((group) => group.items.length > 0);
}

function normalizeDisplayIdentifier(value) {
  return String(value || "").trim().replace(/^@/, "").toLowerCase();
}

export function renderFavoriteCategoryGroups({
  items,
  categories,
  activeCategoryId = "all",
  currentSourceId = "",
  sourcePrefix,
  emptyMessage,
  formatLabel,
}) {
  const normalizedItems = (Array.isArray(items) ? items : []).filter((item) => item.sourceId);

  if (normalizedItems.length === 0) {
    return `<div class="${sourcePrefix}-favorite-empty">${escapeHtml(emptyMessage)}</div>`;
  }

  const groups = getGroupedFavorites(normalizedItems, categories);
  const visibleGroups = activeCategoryId === "all"
    ? groups
    : groups.filter((group) => group.id === String(activeCategoryId));
  const filters = [
    { id: "all", name: "All" },
    ...groups.map((group) => ({ id: group.id, name: group.name })),
  ];
  const current = normalizeDisplayIdentifier(currentSourceId);

  const filtersHtml = filters.map((filter) => `
    <button
      class="favorite-category-filter ${String(activeCategoryId) === filter.id ? "active" : ""}"
      type="button"
      data-category-id="${escapeHtml(filter.id)}"
    >${escapeHtml(filter.name)}</button>
  `).join("");

  const groupsHtml = visibleGroups.map((group) => `
    <details class="favorite-category-group" data-category-id="${escapeHtml(group.id)}" open>
      <summary>
        <span>${escapeHtml(group.name)}</span>
        <span class="favorite-category-count">${group.items.length}</span>
      </summary>
      <div class="${sourcePrefix}-favorite-group-list favorite-category-chip-list">
        ${group.items.map((item) => {
          const sourceId = item.sourceId;
          const label = item.label || sourceId;
          const isActive =
            normalizeDisplayIdentifier(sourceId) === current ||
            normalizeDisplayIdentifier(label) === current;
          return `
            <div class="${sourcePrefix}-favorite-chip ${isActive ? "active" : ""}" data-channel="${escapeHtml(sourceId)}" data-display-channel="${escapeHtml(label)}">
              <span>${formatLabel(sourceId, item)}</span>
              <button class="${sourcePrefix}-favorite-remove" type="button" data-channel="${escapeHtml(sourceId)}" title="Remove from favorites">x</button>
            </div>
          `;
        }).join("")}
      </div>
    </details>
  `).join("");

  return `
    <div class="favorite-category-filters">${filtersHtml}</div>
    <div class="favorite-category-groups">${groupsHtml}</div>
  `;
}

function setModalHidden(hidden) {
  els.favoriteCategoryModal.classList.toggle("hidden", hidden);
  els.favoriteCategoryModal.setAttribute("aria-hidden", hidden ? "true" : "false");
}

function closeFavoriteCategoryModal(value = null) {
  if (modalState?.resolve) {
    modalState.resolve(value);
  }
  modalState = null;
  setModalHidden(true);
}

function getSelectedCategoryMode() {
  return els.favoriteCategoryOptions.querySelector('input[name="favorite-category-choice"]:checked')?.value || "new";
}

function renderFavoriteCategoryModal() {
  if (!modalState) return;
  const categories = modalState.categories;
  const hasCategories = categories.length > 0;
  const oneCategory = categories.length === 1;
  const selectedValue = !hasCategories || modalState.showNew ? "new" : String(categories[0].id);

  let optionsHtml = "";
  if (!hasCategories) {
    optionsHtml = '<div class="favorite-category-empty">Create a category for this favorite.</div>';
  } else {
    optionsHtml = categories.map((category) => `
      <label class="favorite-category-choice">
        <input
          type="radio"
          name="favorite-category-choice"
          value="${escapeHtml(category.id)}"
          ${String(category.id) === selectedValue ? "checked" : ""}
        />
        <span>${escapeHtml(category.name)}</span>
      </label>
    `).join("");

    if (oneCategory && !modalState.showNew) {
      optionsHtml += '<button class="favorite-category-link" type="button" data-action="show-new-category">Add to a different category</button>';
    } else {
      optionsHtml += `
        <label class="favorite-category-choice">
          <input
            type="radio"
            name="favorite-category-choice"
            value="new"
            ${selectedValue === "new" ? "checked" : ""}
          />
          <span>New category</span>
        </label>
      `;
    }
  }

  els.favoriteCategoryOptions.innerHTML = optionsHtml;
  els.favoriteCategoryNewWrap.classList.toggle("hidden", hasCategories && selectedValue !== "new");
  els.favoriteCategoryNewName.value = "";

  if (!hasCategories || selectedValue === "new") {
    requestAnimationFrame(() => els.favoriteCategoryNewName.focus());
  }
}

async function saveFavoriteCategoryModal() {
  if (!modalState) return;
  const selectedValue = getSelectedCategoryMode();

  try {
    if (selectedValue === "new") {
      const category = await createFavoriteCategory(els.favoriteCategoryNewName.value, modalState.source);
      await loadFavoriteCategories(modalState.source, true);
      closeFavoriteCategoryModal(normalizeCategory(category));
      return;
    }

    const category = getCategoryById(modalState.categories, selectedValue);
    if (!category) {
      showError("Choose a category.");
      return;
    }
    closeFavoriteCategoryModal(category);
  } catch (err) {
    showError(err.message || String(err));
  }
}

export async function openFavoriteCategoryModal({ title, targetLabel, source = "telegram" }) {
  source = normalizeFavoriteSource(source);
  const categories = await loadFavoriteCategories(source, true);

  setModalHidden(false);
  els.favoriteCategoryModalHeading.textContent = title || "Add favorite";
  els.favoriteCategoryModalTarget.textContent = targetLabel || "";

  return new Promise((resolve) => {
    modalState = {
      categories,
      source,
      showNew: categories.length === 0,
      resolve,
    };
    renderFavoriteCategoryModal();
  });
}

function setManagerHidden(hidden) {
	els.favoriteCategoryManagerModal?.classList.toggle("hidden", hidden);
	els.favoriteCategoryManagerModal?.setAttribute("aria-hidden", hidden ? "true" : "false");
}

function closeFavoriteCategoryManager() {
	setManagerHidden(true);
}

function renderFavoriteCategoryManager(categories) {
	if (!els.favoriteCategoryManagerList) return;
	if (categories.length === 0) {
		els.favoriteCategoryManagerList.innerHTML = '<div class="favorite-category-empty">No categories yet.</div>';
		return;
	}
	els.favoriteCategoryManagerList.innerHTML = categories.map((category) => `
		<div class="favorite-category-manager-row" data-category-id="${escapeHtml(category.id)}">
			<label for="favorite-category-rename-${escapeHtml(category.id)}">#${escapeHtml(category.id)}</label>
			<input id="favorite-category-rename-${escapeHtml(category.id)}" type="text" value="${escapeHtml(category.name)}" />
			<button class="secondary-btn small-btn" type="button" data-action="rename-category">Rename</button>
		</div>
	`).join("");
}

export async function openFavoriteCategoryManager(source = "telegram") {
	managerSource = normalizeFavoriteSource(source);
	const categories = await loadFavoriteCategories(managerSource, true);
	if (els.favoriteCategoryManagerHeading) {
		els.favoriteCategoryManagerHeading.textContent = `Manage ${managerSource === "youtube" ? "YouTube" : "Telegram"} categories`;
	}
	renderFavoriteCategoryManager(categories);
	setManagerHidden(false);
}

export function initFavoriteCategoryModal() {
  if (!els.favoriteCategoryModal) return;
	if (window.runtime?.EventsOn) {
		window.runtime.EventsOn("favorite-categories:changed", (source) => {
			const normalizedSource = normalizeFavoriteSource(source);
			delete cachedCategories[normalizedSource];
			document.dispatchEvent(new CustomEvent("favorite-categories:changed", {
				detail: { source: normalizedSource },
			}));
		});
	}

  els.favoriteCategoryModalClose.addEventListener("click", () => closeFavoriteCategoryModal(null));
  els.favoriteCategoryModalCancel.addEventListener("click", () => closeFavoriteCategoryModal(null));
  els.favoriteCategoryModalSave.addEventListener("click", saveFavoriteCategoryModal);
  els.favoriteCategoryModalBackdrop.addEventListener("click", () => closeFavoriteCategoryModal(null));

  els.favoriteCategoryOptions.addEventListener("click", (event) => {
    if (event.target.closest('[data-action="show-new-category"]')) {
      modalState.showNew = true;
      renderFavoriteCategoryModal();
    }
  });

  els.favoriteCategoryOptions.addEventListener("change", () => {
    const selectedValue = getSelectedCategoryMode();
    els.favoriteCategoryNewWrap.classList.toggle("hidden", selectedValue !== "new");
    if (selectedValue === "new") {
      els.favoriteCategoryNewName.focus();
    }
  });

  els.favoriteCategoryNewName.addEventListener("keydown", (event) => {
    if (event.key === "Enter") saveFavoriteCategoryModal();
  });

  document.addEventListener("keydown", (event) => {
    if (event.key === "Escape" && !els.favoriteCategoryModal.classList.contains("hidden")) {
      closeFavoriteCategoryModal(null);
    }
	if (event.key === "Escape" && !els.favoriteCategoryManagerModal?.classList.contains("hidden")) {
		closeFavoriteCategoryManager();
	}
  });

	els.telegramCategoryManage?.addEventListener("click", () => {
		void openFavoriteCategoryManager("telegram").catch((err) => showError(err.message || String(err)));
	});
	els.youtubeCategoryManage?.addEventListener("click", () => {
		void openFavoriteCategoryManager("youtube").catch((err) => showError(err.message || String(err)));
	});
	els.favoriteCategoryManagerClose?.addEventListener("click", closeFavoriteCategoryManager);
	els.favoriteCategoryManagerDone?.addEventListener("click", closeFavoriteCategoryManager);
	els.favoriteCategoryManagerBackdrop?.addEventListener("click", closeFavoriteCategoryManager);
	els.favoriteCategoryManagerList?.addEventListener("click", async (event) => {
		const button = event.target.closest('[data-action="rename-category"]');
		if (!button) return;
		const row = button.closest("[data-category-id]");
		const input = row?.querySelector("input");
		if (!row || !input) return;
		button.disabled = true;
		try {
			await renameFavoriteCategory(Number(row.dataset.categoryId), input.value, managerSource);
			const categories = await loadFavoriteCategories(managerSource, true);
			renderFavoriteCategoryManager(categories);
			document.dispatchEvent(new CustomEvent("favorite-categories:changed", {
				detail: { source: managerSource },
			}));
		} catch (err) {
			showError(err.message || String(err));
		} finally {
			button.disabled = false;
		}
	});
}
