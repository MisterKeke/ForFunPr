import { els } from './dom.js';
import { escapeHtml, hasWailsBinding } from './utils.js';
import { getTodos, createTodo, updateTodo, toggleTodo, deleteTodo } from './api.js';
import { todos, setTodos } from './state.js';
import { showError } from './ui.js';

const PRIORITY_LABELS = { low: "Low", medium: "Medium", high: "High" };

function formatTodoCreatedAt(value) {
  if (!value) return "";
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

export function renderTodos() {
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

export async function loadTodos() {
  if (!hasWailsBinding()) {
    els.todoList.innerHTML = '<div class="todo-empty">Backend unavailable — run the app to manage tasks (stored in database.db).</div>';
    setTodos([]);
    return;
  }
  try {
    const list = await getTodos();
    setTodos(Array.isArray(list) ? list : []);
  } catch (err) {
    console.error(err);
    setTodos([]);
  }
  renderTodos();
}

async function addTodo() {
  if (!hasWailsBinding()) {
    showError("Backend unavailable — cannot save tasks to database.db.");
    return;
  }
  const text = String(els.todoInput.value || "").trim();
  if (!text) return;
  const description = String(els.todoDescription ? els.todoDescription.value : "").trim();
  const priority = String(els.todoPriority ? els.todoPriority.value : "medium").trim() || "medium";
  try {
    const updated = await createTodo(text, description, priority);
    setTodos(Array.isArray(updated) ? updated : []);
  } catch (err) {
    console.error(err);
  }
  els.todoInput.value = "";
  if (els.todoDescription) els.todoDescription.value = "";
  if (els.todoPriority) els.todoPriority.value = "medium";
  renderTodos();
}

async function cycleTodoPriority(id) {
  if (!hasWailsBinding()) return;
  const todo = todos.find((item) => String(item.id) === String(id));
  if (!todo) return;
  const order = ["low", "medium", "high"];
  const current = (todo.priority || "medium").toLowerCase();
  const nextIndex = (order.indexOf(current) + 1) % order.length;
  const nextPriority = order[nextIndex];
  const title = todo.text || todo.title || "";
  const description = todo.details || todo.description || "";
  try {
    const updated = await updateTodo(Number(id), title, description, nextPriority);
    setTodos(Array.isArray(updated) ? updated : todos);
  } catch (err) {
    console.error(err);
  }
  renderTodos();
}

export function initTodos() {
  els.todoAdd.addEventListener("click", addTodo);
  els.todoInput.addEventListener("keydown", (event) => {
    if (event.key === "Enter") addTodo();
  });

  els.todoList.addEventListener("click", async (event) => {
    const item = event.target.closest(".todo-item");
    if (!item) return;
    const id = item.dataset.id;

    if (event.target.closest(".todo-remove")) {
      if (!hasWailsBinding()) return;
      try {
        const updated = await deleteTodo(Number(id));
        setTodos(Array.isArray(updated) ? updated : []);
      } catch (err) {
        console.error(err);
      }
      renderTodos();
      return;
    }

    if (event.target.closest(".todo-check")) {
      if (!hasWailsBinding()) return;
      try {
        const updated = await toggleTodo(Number(id));
        setTodos(Array.isArray(updated) ? updated : []);
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
}