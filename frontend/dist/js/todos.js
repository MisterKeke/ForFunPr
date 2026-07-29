import { els } from './dom.js';
import { escapeHtml, hasWailsBinding } from './utils.js';
import { getTodos, createTodo, updateTodo, toggleTodo, toggleTodoSubtask, deleteTodo } from './api.js';
import { todos, setTodos } from './state.js';
import { showError } from './ui.js';
import { DIFFICULTY_LABELS, PRIORITY_LABELS } from './todoConstants.js';

const TODOS_CHANGED_EVENT = "todos:changed";

// Local UI state (not shared elsewhere, so it lives in this module)
const todoFilters = {
	query: "",
	dueDate: "",
	priority: "",
	difficulty: "",
	tags: [],
};
let editingId = null; // null while adding a new task, otherwise the id being edited
let backendChangeRefresh = null;
let backendChangePending = false;
let notificationScheduled = false;
let pendingNotificationSource = "ui";
let draftSubtasks = [];

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
	const query = todoFilters.query.trim().toLowerCase();
	return todos.filter((todo) => {
		const title = String(todo.title || "").toLowerCase();
		const description = String(todo.description || "").toLowerCase();
		const tags = (Array.isArray(todo.tags) ? todo.tags : [])
			.map((tag) => String(tag).toLowerCase());
		const subtasks = (Array.isArray(todo.subtasks) ? todo.subtasks : [])
			.map((subtask) => subtask.title || "").join(" ").toLowerCase();
		const matchesQuery = !query || title.includes(query) || description.includes(query) ||
			tags.some((tag) => tag.includes(query)) || subtasks.includes(query);
		const difficulty = String(todo.difficulty || "").toLowerCase();
		const matchesDifficulty = !todoFilters.difficulty ||
			(todoFilters.difficulty === "unset" ? difficulty === "" : difficulty === todoFilters.difficulty);

		return matchesQuery &&
			(!todoFilters.dueDate || todo.due_date === todoFilters.dueDate) &&
			(!todoFilters.priority || String(todo.priority || "").toLowerCase() === todoFilters.priority) &&
			matchesDifficulty &&
			todoFilters.tags.every((tag) => tags.includes(tag));
  });
}

function syncTodoFilters() {
	todoFilters.query = String(els.todoSearch.value || "");
	todoFilters.dueDate = String(els.todoFilterDueDate.value || "");
	todoFilters.priority = String(els.todoFilterPriority.value || "").toLowerCase();
	todoFilters.difficulty = String(els.todoFilterDifficulty.value || "").toLowerCase();
	todoFilters.tags = parseTodoTags(els.todoFilterTags.value).map((tag) => tag.toLowerCase());
	els.todoFilterClear.disabled = !todoFilters.query.trim() && !todoFilters.dueDate &&
		!todoFilters.priority && !todoFilters.difficulty && todoFilters.tags.length === 0;
	renderTodos();
}

function clearTodoFilters() {
	els.todoSearch.value = "";
	els.todoFilterDueDate.value = "";
	els.todoFilterPriority.value = "";
	els.todoFilterDifficulty.value = "";
	els.todoFilterTags.value = "";
	syncTodoFilters();
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
    els.todoList.innerHTML = '<div class="todo-empty">No tasks match these filters</div>';
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
		const difficulty = String(todo.difficulty || "").toLowerCase();
		const tags = Array.isArray(todo.tags) ? todo.tags : [];
		const subtasks = Array.isArray(todo.subtasks) ? todo.subtasks : [];
		const completedSubtasks = subtasks.filter((subtask) => subtask.done).length;
		const tagsHtml = tags.length > 0 ? `<div class="todo-tags">${tags.map((tag) =>
			`<span class="todo-tag">${escapeHtml(tag)}</span>`).join("")}</div>` : "";
		const subtasksHtml = difficulty === "hard" && subtasks.length > 0 ? `
			<div class="todo-subtasks" aria-label="Subtasks">
				<div class="todo-subtask-progress">${completedSubtasks}/${subtasks.length} subtasks complete</div>
				${subtasks.map((subtask) => `
					<button class="todo-subtask ${subtask.done ? "done" : ""}" type="button"
						data-action="toggle-subtask" data-subtask-id="${escapeHtml(subtask.id)}">
						<span class="todo-subtask-check">${subtask.done ? "v" : ""}</span>
						<span>${escapeHtml(subtask.title)}</span>
					</button>
				`).join("")}
			</div>` : "";

      return `
      <div class="todo-item ${todo.done ? "done" : ""}" data-id="${escapeHtml(todo.id)}">
        <button class="todo-check" type="button" title="Toggle task">${todo.done ? "v" : ""}</button>
        <div class="todo-body" data-action="edit" title="Click to edit">
          <div class="todo-title-row">
            <span class="todo-id-badge">#${escapeHtml(todo.id)}</span>
            <span class="todo-text">${escapeHtml(todo.title)}</span>
          </div>
          ${description ? `<span class="todo-desc">${escapeHtml(description)}</span>` : ""}
		  ${tagsHtml}
		  ${subtasksHtml}
          <div class="todo-meta-row">
            ${createdLabel ? `<span class="todo-created">Created ${escapeHtml(createdLabel)}</span>` : ""}
            ${dueLabel ? `<span class="todo-due ${overdue ? "overdue" : ""}">Due ${escapeHtml(dueLabel)}</span>` : ""}
            <span class="todo-done-label">${todo.done ? "Completed" : "Pending"}</span>
          </div>
        </div>
		<div class="todo-badge-stack">
			${difficulty ? `<span class="todo-difficulty-badge difficulty-${escapeHtml(difficulty)}">${escapeHtml(DIFFICULTY_LABELS[difficulty] || difficulty)}</span>` : ""}
			<button class="todo-priority-badge priority-${escapeHtml(priority)}" type="button" data-action="cycle-priority" title="Click to change priority">${escapeHtml(priorityLabel)}</button>
		</div>
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

export function openTodoModal(todo = null) {
  editingId = todo ? todo.id : null;
  els.todoModalHeading.textContent = todo ? "Edit task" : "Add new task";
  els.todoModalTitle.value = todo ? todo.title : "";
  els.todoModalDescription.value = todo ? todo.description : "";
  els.todoModalDueDate.value = todo ? todo.due_date : "";
  els.todoModalPriority.value = todo ? (todo.priority || "medium").toLowerCase() : "medium";
	els.todoModalDifficulty.value = todo ? String(todo.difficulty || "").toLowerCase() : "";
	els.todoModalTags.value = todo && Array.isArray(todo.tags) ? todo.tags.join(", ") : "";
	draftSubtasks = todo && Array.isArray(todo.subtasks)
		? todo.subtasks.map((subtask, position) => ({ ...subtask, position }))
		: [];
	renderTodoSubtaskEditor();
	updateTodoSubtaskVisibility();

  els.todoModal.classList.remove("hidden");
  els.todoModal.setAttribute("aria-hidden", "false");
  els.todoModalTitle.focus();
}

function closeTodoModal() {
  els.todoModal.classList.add("hidden");
  els.todoModal.setAttribute("aria-hidden", "true");
	editingId = null;
	draftSubtasks = [];
}

function updateTodoSubtaskVisibility() {
	const isHard = els.todoModalDifficulty.value === "hard";
	els.todoModalSubtasksWrap.classList.toggle("hidden", !isHard);
}

function renderTodoSubtaskEditor() {
	if (!els.todoModalSubtasks) return;
	if (draftSubtasks.length === 0) {
		els.todoModalSubtasks.innerHTML = '<div class="todo-subtask-editor-empty">No subtasks yet.</div>';
		return;
	}
	els.todoModalSubtasks.innerHTML = draftSubtasks.map((subtask, index) => `
		<div class="todo-subtask-editor-row" data-index="${index}">
			<input type="checkbox" data-action="draft-subtask-done" ${subtask.done ? "checked" : ""} aria-label="Subtask complete" />
			<input type="text" data-action="draft-subtask-title" value="${escapeHtml(subtask.title || "")}" placeholder="Subtask title" />
			<button class="secondary-btn small-btn" type="button" data-action="remove-draft-subtask">Remove</button>
		</div>
	`).join("");
}

function parseTodoTags(value) {
	const seen = new Set();
	return String(value || "").split(",").map((tag) => tag.trim()).filter((tag) => {
		const key = tag.toLowerCase();
		if (!key || seen.has(key)) return false;
		seen.add(key);
		return true;
	});
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
	const difficulty = String(els.todoModalDifficulty.value || "").trim().toLowerCase();
	const tags = parseTodoTags(els.todoModalTags.value);
	const subtasks = draftSubtasks.map((subtask, position) => ({
		id: Number(subtask.id || 0),
		title: String(subtask.title || "").trim(),
		done: Boolean(subtask.done),
		position,
	}));
	if (subtasks.some((subtask) => !subtask.title)) {
		showError("Enter a title for every subtask.");
		return;
	}
	if (subtasks.length > 0 && difficulty !== "hard") {
		showError("Subtasks are only available for hard tasks.");
		return;
	}
	const request = {
		id: editingId ? Number(editingId) : undefined,
		title,
		description,
		priority,
		due_date: dueDate,
		difficulty,
		tags,
		subtasks,
	};

  try {
    const updated = editingId
		? await updateTodo(request)
		: await createTodo(request);
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
  try {
		const updated = await updateTodo({
			id: Number(id),
			title: todo.title,
			description: todo.description,
			priority: nextPriority,
			due_date: todo.due_date,
			difficulty: todo.difficulty || "",
			tags: Array.isArray(todo.tags) ? todo.tags : [],
			subtasks: Array.isArray(todo.subtasks) ? todo.subtasks : [],
		});
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

  // Search and filters
	[
		els.todoSearch,
		els.todoFilterDueDate,
		els.todoFilterPriority,
		els.todoFilterDifficulty,
		els.todoFilterTags,
	].forEach((control) => control.addEventListener("input", syncTodoFilters));
	els.todoFilterClear.addEventListener("click", clearTodoFilters);
	syncTodoFilters();

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
	els.todoModalDifficulty.addEventListener("change", () => {
		if (els.todoModalDifficulty.value !== "hard" && draftSubtasks.length > 0) {
			if (window.confirm("Changing difficulty will clear all subtasks. Continue?")) {
				draftSubtasks = [];
				renderTodoSubtaskEditor();
			} else {
				els.todoModalDifficulty.value = "hard";
			}
		}
		updateTodoSubtaskVisibility();
	});
	els.todoModalSubtaskAdd.addEventListener("click", () => {
		draftSubtasks.push({ id: 0, title: "", done: false, position: draftSubtasks.length });
		renderTodoSubtaskEditor();
		els.todoModalSubtasks.querySelector('[data-index]:last-child input[type="text"]')?.focus();
	});
	els.todoModalSubtasks.addEventListener("input", (event) => {
		const row = event.target.closest("[data-index]");
		if (!row) return;
		const index = Number(row.dataset.index);
		if (event.target.matches('[data-action="draft-subtask-title"]')) {
			draftSubtasks[index].title = event.target.value;
		}
	});
	els.todoModalSubtasks.addEventListener("change", (event) => {
		const row = event.target.closest("[data-index]");
		if (!row) return;
		const index = Number(row.dataset.index);
		if (event.target.matches('[data-action="draft-subtask-done"]')) {
			draftSubtasks[index].done = event.target.checked;
		}
	});
	els.todoModalSubtasks.addEventListener("click", (event) => {
		const button = event.target.closest('[data-action="remove-draft-subtask"]');
		if (!button) return;
		const row = button.closest("[data-index]");
		draftSubtasks.splice(Number(row.dataset.index), 1);
		renderTodoSubtaskEditor();
	});

  // Task list interactions: toggle done, remove, cycle priority, edit
  els.todoList.addEventListener("click", async (event) => {
    const item = event.target.closest(".todo-item");
    if (!item) return;
    const id = item.dataset.id;

	const subtaskButton = event.target.closest('[data-action="toggle-subtask"]');
	if (subtaskButton) {
		if (!hasWailsBinding()) return;
		try {
			const updated = await toggleTodoSubtask(Number(id), Number(subtaskButton.dataset.subtaskId));
			commitTodos(updated);
		} catch (err) {
			showError(err.message || String(err));
		}
		return;
	}

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
