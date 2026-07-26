import { els } from './dom.js';
import { escapeHtml, hasWailsBinding } from './utils.js';
import { getTodos, createTodo, updateTodo, toggleTodo, deleteTodo } from './api.js';
import { todos, setTodos } from './state.js';
import { showError } from './ui.js';
import { PRIORITY_LABELS } from './todoConstants.js';

const TODOS_CHANGED_EVENT = "todos:changed";

// Local UI state (not shared elsewhere, so it lives in this module)
let searchQuery = "";
let editingId = null; // null while adding a new task, otherwise the id being edited
let backendChangeRefresh = null;
let backendChangePending = false;
let notificationScheduled = false;
let pendingNotificationSource = "ui";

function notifyTodosChanged(source = "ui") {
	pendingNotificationSource = source;
	if (notificationScheduled) return;
	notificationScheduled = true;
	queueMicrotask(() => {
		notificationScheduled = false;
		document.dispatchEvent(new CustomEvent(TODOS_CHANGED_EVENT, {
			detail: { todos: todos.slice(), source: pendingNotificationSource },
		}));
	});
}

export function commitTodos(list, source = "ui") {
	setTodos(Array.isArray(list) ? list : []);
	renderTodos();
	notifyTodosChanged(source);
}

function refreshTodosAfterBackendChange() {
  backendChangePending = true;
  if (backendChangeRefresh) return backendChangeRefresh;

  backendChangeRefresh = (async () => {
    do {
      backendChangePending = false;
      await loadTodos();
			notifyTodosChanged("external");
    } while (backendChangePending);
  })().finally(() => {
    backendChangeRefresh = null;
  });

  return backendChangeRefresh;
}

function listenForBackendTodoChanges() {
  if (!hasWailsBinding() || !window.runtime?.EventsOn) return;

  window.runtime.EventsOn(TODOS_CHANGED_EVENT, () => {
    void refreshTodosAfterBackendChange();
  });
}

// ---------- formatting helpers ----------

function formatTodoCreatedAt(value) {
  if (!value) return "";
  // SQLite CURRENT_TIMESTAMP is "YYYY-MM-DD HH:MM:SS" (UTC, no timezone marker).
  const isoLike = /^\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}$/.test(value)
    ? `${value.replace(" ", "T")}Z`
    : value;
  const date = new Date(isoLike);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleString(undefined, {
    year: "numeric",
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}

function formatDueDate(value) {
  if (!value) return "";
  const date = new Date(value.length === 10 ? `${value}T00:00:00` : value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleDateString(undefined, {
    year: "numeric",
    month: "short",
    day: "numeric",
  });
}

function isOverdue(todo) {
  if (!todo.due_date || todo.done) return false;
  const due = new Date(`${todo.due_date}T23:59:59`);
  if (Number.isNaN(due.getTime())) return false;
  return due.getTime() < Date.now();
}

// ---------- filtering ----------

function getFilteredTodos() {
  const query = searchQuery.trim().toLowerCase();
  if (!query) return todos;
  return todos.filter((todo) => {
    const title = todo.title.toLowerCase();
    const description = todo.description.toLowerCase();
    return title.includes(query) || description.includes(query);
  });
}

// ---------- rendering ----------
// Renders the canonical Todo shape: id, title, description, done,
// created_at, due_date, and priority.

export function renderTodos() {
  const filtered = getFilteredTodos();

  if (todos.length === 0) {
    els.todoList.innerHTML = '<div class="todo-empty">No tasks yet</div>';
    return;
  }

  if (filtered.length === 0) {
    els.todoList.innerHTML = '<div class="todo-empty">No tasks match your search</div>';
    return;
  }

  els.todoList.innerHTML = filtered
    .map((todo) => {
      const priority = (todo.priority || "medium").toLowerCase();
      const priorityLabel = PRIORITY_LABELS[priority] || "Medium";
      const description = todo.description;
      const createdLabel = formatTodoCreatedAt(todo.created_at);
      const dueLabel = formatDueDate(todo.due_date);
      const overdue = isOverdue(todo);

      return `
      <div class="todo-item ${todo.done ? "done" : ""}" data-id="${escapeHtml(todo.id)}">
        <button class="todo-check" type="button" title="Toggle task">${todo.done ? "v" : ""}</button>
        <div class="todo-body" data-action="edit" title="Click to edit">
          <div class="todo-title-row">
            <span class="todo-id-badge">#${escapeHtml(todo.id)}</span>
            <span class="todo-text">${escapeHtml(todo.title)}</span>
          </div>
          ${description ? `<span class="todo-desc">${escapeHtml(description)}</span>` : ""}
          <div class="todo-meta-row">
            ${createdLabel ? `<span class="todo-created">Created ${escapeHtml(createdLabel)}</span>` : ""}
            ${dueLabel ? `<span class="todo-due ${overdue ? "overdue" : ""}">Due ${escapeHtml(dueLabel)}</span>` : ""}
            <span class="todo-done-label">${todo.done ? "Completed" : "Pending"}</span>
          </div>
        </div>
        <button class="todo-priority-badge priority-${escapeHtml(priority)}" type="button" data-action="cycle-priority" title="Click to change priority">${escapeHtml(priorityLabel)}</button>
        <button class="todo-remove" type="button" title="Remove">x</button>
      </div>
    `;
    })
    .join("");
}

// ---------- data loading ----------

export async function loadTodos() {
  if (!hasWailsBinding()) {
    els.todoList.innerHTML = '<div class="todo-empty">Backend unavailable — run the app to manage tasks (stored in database.db).</div>';
    setTodos([]);
    return;
  }
  try {
    const list = await getTodos();
    setTodos(list);
  } catch (err) {
    console.error(err);
    showError(err.message || String(err));
    return;
  }
  renderTodos();
}

// ---------- modal (add / edit) ----------

function openTodoModal(todo = null) {
  editingId = todo ? todo.id : null;
  els.todoModalHeading.textContent = todo ? "Edit task" : "Add new task";
  els.todoModalTitle.value = todo ? todo.title : "";
  els.todoModalDescription.value = todo ? todo.description : "";
  els.todoModalDueDate.value = todo ? todo.due_date : "";
  els.todoModalPriority.value = todo ? (todo.priority || "medium").toLowerCase() : "medium";

  els.todoModal.classList.remove("hidden");
  els.todoModal.setAttribute("aria-hidden", "false");
  els.todoModalTitle.focus();
}

function closeTodoModal() {
  els.todoModal.classList.add("hidden");
  els.todoModal.setAttribute("aria-hidden", "true");
  editingId = null;
}

async function saveTodoFromModal() {
  const title = String(els.todoModalTitle.value || "").trim();
  if (!title) {
    showError("Enter a task title.");
    els.todoModalTitle.focus();
    return;
  }

  const description = String(els.todoModalDescription.value || "").trim();
  const dueDate = String(els.todoModalDueDate.value || "").trim();
  const priority = String(els.todoModalPriority.value || "medium").trim() || "medium";

  try {
    const updated = editingId
      ? await updateTodo(editingId, title, description, priority, dueDate)
      : await createTodo(title, description, priority, dueDate);
		commitTodos(updated);
  } catch (err) {
    console.error(err);
    showError(err.message || String(err));
    return;
  }

  closeTodoModal();
}

// ---------- quick actions ----------

async function cycleTodoPriority(id) {
  if (!hasWailsBinding()) return;
  const todo = todos.find((item) => String(item.id) === String(id));
  if (!todo) return;
  const order = ["low", "medium", "high"];
  const current = (todo.priority || "medium").toLowerCase();
  const nextIndex = (order.indexOf(current) + 1) % order.length;
  const nextPriority = order[nextIndex];
  const title = todo.title;
  const description = todo.description;
  const dueDate = todo.due_date;
  try {
    const updated = await updateTodo(id, title, description, nextPriority, dueDate);
		commitTodos(updated);
  } catch (err) {
    console.error(err);
    showError(err.message || String(err));
    return;
  }
}

// ---------- init ----------

export function initTodos() {
  listenForBackendTodoChanges();

  // Search
  els.todoSearch.addEventListener("input", () => {
    searchQuery = els.todoSearch.value || "";
    renderTodos();
  });

  // Open "add task" modal
  els.todoNew.addEventListener("click", () => openTodoModal());

  // Close modal
  els.todoModalClose.addEventListener("click", closeTodoModal);
  els.todoModalCancel.addEventListener("click", closeTodoModal);
  if (els.todoModalBackdrop) {
    els.todoModalBackdrop.addEventListener("click", closeTodoModal);
  }
  document.addEventListener("keydown", (event) => {
    if (event.key === "Escape" && !els.todoModal.classList.contains("hidden")) {
      closeTodoModal();
    }
  });

  // Save (create or edit)
  els.todoModalSave.addEventListener("click", saveTodoFromModal);
  els.todoModalTitle.addEventListener("keydown", (event) => {
    if (event.key === "Enter") saveTodoFromModal();
  });

  // Task list interactions: toggle done, remove, cycle priority, edit
  els.todoList.addEventListener("click", async (event) => {
    const item = event.target.closest(".todo-item");
    if (!item) return;
    const id = item.dataset.id;

    if (event.target.closest(".todo-remove")) {
      if (!hasWailsBinding()) return;
      try {
        const updated = await deleteTodo(Number(id));
			commitTodos(updated);
      } catch (err) {
        console.error(err);
        showError(err.message || String(err));
        return;
      }
		return;
    }

    if (event.target.closest(".todo-check")) {
      if (!hasWailsBinding()) return;
      try {
        const updated = await toggleTodo(Number(id));
			commitTodos(updated);
      } catch (err) {
        console.error(err);
        showError(err.message || String(err));
        return;
      }
      return;
    }

    if (event.target.closest('[data-action="cycle-priority"]')) {
      await cycleTodoPriority(id);
      return;
    }

    if (event.target.closest('[data-action="edit"]')) {
      const todo = todos.find((item) => String(item.id) === String(id));
      if (todo) openTodoModal(todo);
    }
  });
}
