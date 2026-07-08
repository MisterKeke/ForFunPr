(function () {
  "use strict";

  const els = {
    tabs: document.querySelectorAll(".tab-btn"),
    panels: document.querySelectorAll(".tab-panel"),
    menuButtons: document.querySelectorAll(".menu-btn"),
    views: document.querySelectorAll(".app-view"),
    singleBase: document.getElementById("single-base"),
    singleTarget: document.getElementById("single-target"),
    singleSubmit: document.getElementById("single-submit"),
    singleResult: document.getElementById("single-result"),
    swapBtn: document.getElementById("swap-btn"),
    allBase: document.getElementById("all-base"),
    allSubmit: document.getElementById("all-submit"),
    allMeta: document.getElementById("all-meta"),
    allSearchWrap: document.getElementById("all-search-wrap"),
    allSearch: document.getElementById("all-search"),
    allResult: document.getElementById("all-result"),
    loading: document.getElementById("loading"),
    errorBox: document.getElementById("error-box"),
    favoriteAdd: document.getElementById("favorite-add"),
    favoriteMessage: document.getElementById("favorite-message"),
    favoriteList: document.getElementById("favorite-list"),
    favoriteRefresh: document.getElementById("favorite-refresh"),
    todoInput: document.getElementById("todo-input"),
    todoDescription: document.getElementById("todo-description"),
    todoPriority: document.getElementById("todo-priority"),
    todoAdd: document.getElementById("todo-add"),
    todoList: document.getElementById("todo-list"),
    telegramChannel: document.getElementById("telegram-channel"),
    telegramLoad: document.getElementById("telegram-load"),
    telegramRefresh: document.getElementById("telegram-refresh"),
    telegramFavoriteAdd: document.getElementById("telegram-favorite-add"),
    telegramFavoriteList: document.getElementById("telegram-favorite-list"),
    telegramPosts: document.getElementById("telegram-posts"),
    telegramLoading: document.getElementById("telegram-loading"),
    telegramError: document.getElementById("telegram-error"),
    convAmount: document.getElementById("conv-amount"),
    convBase: document.getElementById("conv-base"),
    convTarget: document.getElementById("conv-target"),
    convSubmit: document.getElementById("conv-submit"),
    convResult: document.getElementById("conv-result"),
    convSwapBtn: document.getElementById("conv-swap-btn"),
  };

  let lastAllRates = null;
  let todos = [];

  function hasWailsBinding() {
    return Boolean(window.go && window.go.main && window.go.main.App);
  }

  function normalizeCode(value) {
    return String(value || "").trim().toUpperCase();
  }

  function escapeHtml(str) {
    return String(str)
      .replace(/&/g, "&amp;")
      .replace(/</g, "&lt;")
      .replace(/>/g, "&gt;")
      .replace(/"/g, "&quot;");
  }

  function setBusy(busy) {
    els.loading.classList.toggle("hidden", !busy);
    els.singleSubmit.disabled = busy;
    els.allSubmit.disabled = busy;
    els.convSubmit.disabled = busy;
  }

  function showError(message) {
    els.errorBox.textContent = message;
    els.errorBox.classList.remove("hidden");
    const favoriteMsg = document.getElementById("favorite-message");
    if (favoriteMsg) {
      favoriteMsg.classList.add("hidden");
      favoriteMsg.textContent = "";
    }
  }

  function hideError() {
    els.errorBox.classList.add("hidden");
  }

  // ---- Уведомления для избранного ----
  function showFavoriteMessage(message, type = 'info') {
    let msgEl = els.favoriteMessage || document.getElementById('favorite-message');

    if (!msgEl) {
      msgEl = document.createElement('div');
      msgEl.id = 'favorite-message';
      msgEl.className = 'favorite-message hidden';
      els.singleResult.parentNode.insertBefore(msgEl, els.singleResult.nextSibling);
    }

    clearTimeout(msgEl._timeout);
    msgEl.classList.remove('hidden');
    msgEl.classList.remove('success', 'warning', 'error', 'info');
    msgEl.className = `favorite-message ${type}`;
    msgEl.textContent = message;

    if (els.errorBox) {
      els.errorBox.classList.add('hidden');
      els.errorBox.textContent = '';
    }

    msgEl._timeout = setTimeout(() => {
      msgEl.classList.add('hidden');
      msgEl.textContent = '';
    }, 3000);
  }

  function showSuccess(message) {
    showFavoriteMessage(message, 'success');
  }

  function showWarning(message) {
    showFavoriteMessage(message, 'warning');
  }

  function showFavoriteError(message) {
    showFavoriteMessage(message, 'error');
  }

  // ---- Переключение видов ----
  function switchView(viewName) {
    els.menuButtons.forEach((btn) => {
      btn.classList.toggle("active", btn.dataset.view === viewName);
    });
    els.views.forEach((view) => {
      view.classList.toggle("active", view.id === `view-${viewName}`);
    });
    hideError();

    // Автоматическая загрузка при переключении на Telegram
    if (viewName === 'telegram') {
      loadTelegramFavorites();
      const channel = normalizeCode(els.telegramChannel.value) || "durov";
      if (!els.telegramPosts.querySelector('.telegram-post')) {
        loadTelegramPosts(channel);
      }
    }
  }

  // ---- Основные методы валют ----
  async function callGetRate(base, target) {
    if (hasWailsBinding()) {
      return window.go.main.App.GetRate(base, target);
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
    if (!res.ok) {
      throw new Error(`Request failed: ${res.status}`);
    }

    const data = await res.json();
    return {
      base: data.base,
      date: data.date,
      to: data.quote,
      rate: data.rate,
      found: true,
    };
  }

  async function callGetAllRates(base) {
    if (hasWailsBinding()) {
      return window.go.main.App.GetAllRates(base);
    }

    const res = await fetch(
      `https://api.frankfurter.dev/v2/rates?base=${encodeURIComponent(base)}`
    );

    if (!res.ok) {
      throw new Error(`Request failed: ${res.status}`);
    }

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

  function renderSingleResult(result) {
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
  function renderConversionResult(result, amount) {
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

  async function convertCurrency() {
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
  }
  els.convSubmit.addEventListener("click", convertCurrency);

  els.convSwapBtn.addEventListener("click", () => {
    const currentBase = els.convBase.value;
    els.convBase.value = els.convTarget.value;
    els.convTarget.value = currentBase;
  });

  function renderAllRates(filter = "") {
    if (!lastAllRates) {
      return;
    }

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

  // ---- Избранное ----
  function favoriteKey(base, target) {
    return `${normalizeCode(base)}:${normalizeCode(target)}`;
  }

  function splitFavoriteKey(key) {
    const parts = String(key || "").split(":");
    return {
      base: normalizeCode(parts[0]),
      target: normalizeCode(parts[1]),
    };
  }

  function renderFavorites(keys, rates = []) {
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

  async function loadFavorites() {
    try {
      if (hasWailsBinding()) {
        const codes = await window.go.main.App.ListFavorites();
        const payloadStr = await window.go.main.App.GetFavoriteswithRates();
        const payload = JSON.parse(payloadStr || "{}");
        renderFavorites(codes, payload.favorites || []);
        return;
      }

      const codes = JSON.parse(localStorage.getItem("favorites") || "[]");
      renderFavorites(codes, []);
    } catch (err) {
      showError(err.message || String(err));
    }
  }

  // ---- To-Do (backed by database.db via the Go/Wails backend) ----
  const PRIORITY_LABELS = { low: "Low", medium: "Medium", high: "High" };

  async function loadTodos() {
    if (!hasWailsBinding()) {
      els.todoList.innerHTML = '<div class="todo-empty">Backend unavailable — run the app to manage tasks (stored in database.db).</div>';
      todos = [];
      return;
    }

    try {
      const list = await window.go.main.App.GetTodos();
      todos = Array.isArray(list) ? list : [];
    } catch (err) {
      console.error(err);
      todos = [];
    }
    renderTodos();
  }

  function formatTodoCreatedAt(value) {
    if (!value) {
      return "";
    }
    // SQLite CURRENT_TIMESTAMP is "YYYY-MM-DD HH:MM:SS" (UTC, no timezone marker).
    // Normalize so Date can parse it consistently across browsers.
    const isoLike = /^\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}$/.test(value)
      ? `${value.replace(" ", "T")}Z`
      : value;

    const date = new Date(isoLike);
    if (Number.isNaN(date.getTime())) {
      return value;
    }
    return date.toLocaleString(undefined, {
      year: "numeric",
      month: "short",
      day: "numeric",
      hour: "2-digit",
      minute: "2-digit",
    });
  }

  function renderTodos() {
    if (todos.length === 0) {
      els.todoList.innerHTML = '<div class="todo-empty">No tasks yet</div>';
      return;
    }

    els.todoList.innerHTML = todos
      .map((todo) => {
        const priority = (todo.priority || "medium").toLowerCase();
        const priorityLabel = PRIORITY_LABELS[priority] || "Medium";
        const description = todo.details || todo.description || "";
        const createdLabel = formatTodoCreatedAt(todo.created_at || todo.CreatedAt);

        return `
        <div class="todo-item ${todo.done ? "done" : ""}" data-id="${escapeHtml(todo.id)}">
          <button class="todo-check" type="button" title="Toggle task">${todo.done ? "v" : ""}</button>
          <div class="todo-body">
            <span class="todo-text">${escapeHtml(todo.text || todo.title || "")}</span>
            ${description ? `<span class="todo-desc">${escapeHtml(description)}</span>` : ""}
            ${createdLabel ? `<span class="todo-created">Created ${escapeHtml(createdLabel)}</span>` : ""}
          </div>
          <button class="todo-priority-badge priority-${escapeHtml(priority)}" type="button" data-action="cycle-priority" title="Click to change priority">${escapeHtml(priorityLabel)}</button>
          <button class="todo-remove" type="button" title="Remove">x</button>
        </div>
      `;
      })
      .join("");
  }

  async function addTodo() {
    if (!hasWailsBinding()) {
      showError("Backend unavailable — cannot save tasks to database.db.");
      return;
    }

    const text = String(els.todoInput.value || "").trim();
    if (!text) {
      return;
    }

    const description = String(els.todoDescription ? els.todoDescription.value : "").trim();
    const priority = String(els.todoPriority ? els.todoPriority.value : "medium").trim() || "medium";

    try {
      const updated = await window.go.main.App.CreateTodo(text, description, priority);
      todos = Array.isArray(updated) ? updated : [];
    } catch (err) {
      console.error(err);
    }

    els.todoInput.value = "";
    if (els.todoDescription) els.todoDescription.value = "";
    if (els.todoPriority) els.todoPriority.value = "medium";
    renderTodos();
  }

  async function cycleTodoPriority(id) {
    if (!hasWailsBinding()) {
      return;
    }

    const todo = todos.find((item) => String(item.id) === String(id));
    if (!todo) {
      return;
    }

    const order = ["low", "medium", "high"];
    const current = (todo.priority || "medium").toLowerCase();
    const nextIndex = (order.indexOf(current) + 1) % order.length;
    const nextPriority = order[nextIndex];
    const title = todo.text || todo.title || "";
    const description = todo.details || todo.description || "";

    try {
      const updated = await window.go.main.App.UpdateTodo(Number(id), title, description, nextPriority);
      todos = Array.isArray(updated) ? updated : todos;
    } catch (err) {
      console.error(err);
    }
    renderTodos();
  }

  // ---- TELEGRAM ФУНКЦИИ (теперь внутри IIFE) ----
  async function loadTelegramPosts(channel, forceRefresh = false) {
    const loadingEl = els.telegramLoading;
    const errorEl = els.telegramError;
    const postsEl = els.telegramPosts;

    loadingEl.classList.remove("hidden");
    errorEl.classList.add("hidden");
    errorEl.textContent = "";
    postsEl.innerHTML = "";

    try {
      let posts;
      if (hasWailsBinding()) {
      // Если принудительное обновление – очищаем кэш
        if (forceRefresh) {
          await window.go.main.App.TelegramCacheClear();
        }
        posts = await window.go.main.App.GetChannelPosts(channel);
        // ... остальное без изменений
      } else {
        posts = await fetchTelegramPostsFallback(channel);
    }

      if (!posts || posts.length === 0) {
        postsEl.innerHTML = '<div class="telegram-post-empty">No posts found in this channel</div>';
        return;
      }

      renderTelegramPosts(posts);
    } catch (err) {
      errorEl.textContent = err.message || String(err);
      errorEl.classList.remove("hidden");
      postsEl.innerHTML = '';
    } finally {
      loadingEl.classList.add("hidden");
    }
  }

  function renderTelegramPosts(posts) {
    const postsEl = els.telegramPosts;
    postsEl.innerHTML = posts.map(post => {
      const imagesHtml = post.images && post.images.length > 0
        ? `<div class="telegram-post-images">${post.images.map(img => 
           `<img src="${escapeHtml(img)}" alt="Post image" loading="lazy" />`
          ).join('')}</div>`
        : '';

      const metaHtml = `
        <div class="telegram-post-meta">
          ${post.date ? `<span>📅 ${escapeHtml(formatTelegramDate(post.date))}</span>` : ''}
          ${post.views ? `<span>👁️ ${escapeHtml(post.views)}</span>` : ''}
        </div>
      `;

    return `
      <div class="telegram-post">
        ${post.text ? `<div class="telegram-post-text">${escapeHtml(post.text)}</div>` : ''}
        ${imagesHtml}
        ${metaHtml}
      </div>
    `;
  }).join('');
}

  // ---- Telegram favorite channels (backed by database.db) ----
  async function loadTelegramFavorites() {
    if (!hasWailsBinding()) {
      els.telegramFavoriteList.innerHTML = '<div class="telegram-favorite-empty">Backend unavailable — favorites are stored in database.db.</div>';
      return;
    }

    try {
      const channels = await window.go.main.App.ListTelegramFavorites();
      renderTelegramFavorites(Array.isArray(channels) ? channels : []);
    } catch (err) {
      console.error(err);
    }
  }

  function renderTelegramFavorites(channels) {
    const currentChannel = normalizeCode(els.telegramChannel.value).toLowerCase();

    if (!channels || channels.length === 0) {
      els.telegramFavoriteList.innerHTML = '<div class="telegram-favorite-empty">No favorite channels yet</div>';
      return;
    }

    els.telegramFavoriteList.innerHTML = channels
      .map((channel) => `
        <div class="telegram-favorite-chip ${channel === currentChannel ? "active" : ""}" data-channel="${escapeHtml(channel)}">
          <span>@${escapeHtml(channel)}</span>
          <button class="telegram-favorite-remove" type="button" data-channel="${escapeHtml(channel)}" title="Remove from favorites">x</button>
        </div>
      `)
      .join("");
  }

  function formatTelegramDate(dateStr) {
    try {
      const date = new Date(dateStr);
      return date.toLocaleString();
    } catch {
      return dateStr;
    }
  }

  // fallback для разработки без бэкенда
  async function fetchTelegramPostsFallback(channel) {
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

  // ---- Обработчики событий ----
  els.menuButtons.forEach((btn) => {
    btn.addEventListener("click", () => {
      switchView(btn.dataset.view);
    });
  });

  els.tabs.forEach((btn) => {
    btn.addEventListener("click", () => {
      els.tabs.forEach((item) => item.classList.remove("active"));
      els.panels.forEach((panel) => panel.classList.remove("active"));
      btn.classList.add("active");
      document.getElementById(`tab-${btn.dataset.tab}`).classList.add("active");
      hideError();
    });
  });

  els.swapBtn.addEventListener("click", () => {
    const currentBase = els.singleBase.value;
    els.singleBase.value = els.singleTarget.value;
    els.singleTarget.value = currentBase;
  });

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

  els.allSubmit.addEventListener("click", async () => {
    const base = normalizeCode(els.allBase.value);

    if (!base) {
      showError("Enter a base currency code.");
      return;
    }

    hideError();
    setBusy(true);

    try {
      lastAllRates = await callGetAllRates(base);
      els.allMeta.textContent = `${lastAllRates.codes.length} rates for ${lastAllRates.base}${lastAllRates.date ? ` on ${lastAllRates.date}` : ""}`;
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
      if (hasWailsBinding()) {
        const result = await window.go.main.App.AddFavorite(key);
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
      } else {
        const codes = JSON.parse(localStorage.getItem("favorites") || "[]");
        if (codes.includes(key)) {
          showWarning(`"${key}" is already in your favorites!`);
          renderFavorites(codes, []);
          return;
        }
        codes.push(key);
        codes.sort();
        localStorage.setItem("favorites", JSON.stringify(codes));
        renderFavorites(codes, []);
        showSuccess(`"${key}" added to favorites!`);
      }
    } catch (err) {
      console.error('Error:', err);
      showFavoriteError(err.message || String(err));
    }
  });

  els.favoriteRefresh.addEventListener("click", loadFavorites);

  els.favoriteList.addEventListener("click", async (event) => {
    const btn = event.target.closest("button");
    if (!btn) {
      return;
    }

    const code = btn.dataset.code;
    if (!code) {
      return;
    }

    if (btn.classList.contains("remove")) {
      try {
        if (hasWailsBinding()) {
          await window.go.main.App.RemoveFavorite(code);
          await loadFavorites();
        } else {
          const codes = JSON.parse(localStorage.getItem("favorites") || "[]")
            .filter((item) => item !== code);
          localStorage.setItem("favorites", JSON.stringify(codes));
          renderFavorites(codes, []);
        }
      } catch (err) {
        showError(err.message || String(err));
      }
      return;
    }

    const pair = splitFavoriteKey(code);
    els.singleBase.value = pair.base;
    els.singleTarget.value = pair.target;
    els.allBase.value = pair.base;
  });

  els.todoAdd.addEventListener("click", addTodo);

  els.todoInput.addEventListener("keydown", (event) => {
    if (event.key === "Enter") {
      addTodo();
    }
  });

  els.todoList.addEventListener("click", async (event) => {
    const item = event.target.closest(".todo-item");
    if (!item) {
      return;
    }

    const id = item.dataset.id;

    if (event.target.closest(".todo-remove")) {
      if (!hasWailsBinding()) return;
      try {
        const updated = await window.go.main.App.DeleteTodo(Number(id));
        todos = Array.isArray(updated) ? updated : [];
      } catch (err) {
        console.error(err);
      }
      renderTodos();
      return;
    }

    if (event.target.closest(".todo-check")) {
      if (!hasWailsBinding()) return;
      try {
        const updated = await window.go.main.App.ToggleTodo(Number(id));
        todos = Array.isArray(updated) ? updated : [];
      } catch (err) {
        console.error(err);
      }
      renderTodos();
      return;
    }

    if (event.target.closest('[data-action="cycle-priority"]')) {
      await cycleTodoPriority(id);
    }
  });

  // ---- Обработчики Telegram ----
  els.telegramLoad.addEventListener("click", () => {
    const channel = normalizeCode(els.telegramChannel.value) || "durov";
    loadTelegramPosts(channel, true); // <-- передаём true
  });

  els.telegramRefresh.addEventListener("click", () => {
    const channel = normalizeCode(els.telegramChannel.value) || "durov";
    if (hasWailsBinding()) {
      // Очищаем кэш на бэкенде
      window.go.main.App.TelegramCacheClear?.();
    }
    loadTelegramPosts(channel, true);
  });

  els.telegramChannel.addEventListener("keydown", (event) => {
    if (event.key === "Enter") {
      els.telegramLoad.click();
    }
  });

  els.telegramFavoriteAdd.addEventListener("click", async () => {
    const channel = normalizeCode(els.telegramChannel.value).toLowerCase();
    if (!channel) {
      showError("Enter a channel username before saving to favorites.");
      return;
    }

    if (!hasWailsBinding()) {
      showError("Backend unavailable — cannot save favorites to database.db.");
      return;
    }

    try {
      await window.go.main.App.AddTelegramFavorite(channel);
      await loadTelegramFavorites();
    } catch (err) {
      showError(err.message || String(err));
    }
  });

  els.telegramFavoriteList.addEventListener("click", async (event) => {
    const removeBtn = event.target.closest(".telegram-favorite-remove");
    if (removeBtn) {
      const channel = removeBtn.dataset.channel;
      if (hasWailsBinding() && channel) {
        try {
          await window.go.main.App.RemoveTelegramFavorite(channel);
          await loadTelegramFavorites();
        } catch (err) {
          showError(err.message || String(err));
        }
      }
      return;
    }

    const chip = event.target.closest(".telegram-favorite-chip");
    if (chip) {
      const channel = chip.dataset.channel;
      if (channel) {
        els.telegramChannel.value = channel;
        loadTelegramPosts(channel, true);
        renderTelegramFavorites(
          Array.from(els.telegramFavoriteList.querySelectorAll(".telegram-favorite-chip")).map(
            (el) => el.dataset.channel
          )
        );
      }
    }
  });

  // ---- Инициализация ----
  loadFavorites();
  loadTodos();
})();