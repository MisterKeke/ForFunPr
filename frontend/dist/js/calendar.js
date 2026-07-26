import { getTodosByDueDate } from './api.js';
import { els } from './dom.js';
import { PRIORITY_LABELS } from './todoConstants.js';
import { escapeHtml, hasWailsBinding } from './utils.js';

const monthNames = [
  "January",
  "February",
  "March",
  "April",
  "May",
  "June",
  "July",
  "August",
  "September",
  "October",
  "November",
  "December",
];

const weekdayNames = ["Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"];

let visibleYear = new Date().getFullYear();
let selectedDateKey = "";
let selectedDateRequest = 0;

function sameDate(a, b) {
  return (
    a.getFullYear() === b.getFullYear() &&
    a.getMonth() === b.getMonth() &&
    a.getDate() === b.getDate()
  );
}

function dateKey(date) {
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, "0");
  const day = String(date.getDate()).padStart(2, "0");
  return `${year}-${month}-${day}`;
}

function dateFromKey(value) {
  const parts = String(value || "").split("-").map(Number);
  if (parts.length !== 3 || parts.some((part) => !Number.isInteger(part))) return null;

  const date = new Date(parts[0], parts[1] - 1, parts[2]);
  if (
    date.getFullYear() !== parts[0] ||
    date.getMonth() !== parts[1] - 1 ||
    date.getDate() !== parts[2]
  ) {
    return null;
  }
  return date;
}

function formatDateLabel(value) {
  const date = dateFromKey(value);
  if (!date) return value;
  return date.toLocaleDateString(undefined, {
    weekday: "long",
    year: "numeric",
    month: "long",
    day: "numeric",
  });
}

function getMonthCells(year, month) {
  const firstDay = new Date(year, month, 1);
  const daysInMonth = new Date(year, month + 1, 0).getDate();
  const leadingBlankDays = (firstDay.getDay() + 6) % 7;
  const cells = [];

  for (let index = 0; index < leadingBlankDays; index += 1) {
    cells.push(null);
  }

  for (let day = 1; day <= daysInMonth; day += 1) {
    cells.push(new Date(year, month, day));
  }

  return cells;
}

function renderCalendar() {
  const grid = els.calendarGrid;
  const title = document.getElementById("calendar-year-title");
  const rangeLabel = document.getElementById("calendar-range-label");
  if (!grid || !title || !rangeLabel) return;

  const today = new Date();
  const startMonth = visibleYear === today.getFullYear() ? today.getMonth() : 0;
  const monthsToRender = visibleYear === today.getFullYear() ? 12 - startMonth : 12;
  const monthCards = [];

  for (let offset = 0; offset < monthsToRender; offset += 1) {
    const monthDate = new Date(visibleYear, startMonth + offset, 1);
    const month = monthDate.getMonth();
    const year = monthDate.getFullYear();
    const cells = getMonthCells(year, month);
    const days = cells
      .map((date) => {
        if (!date) {
          return '<span class="calendar-day is-empty" aria-hidden="true"></span>';
        }

        const key = dateKey(date);
        const isToday = sameDate(date, today);
        const isWeekend = date.getDay() === 0 || date.getDay() === 6;
        const isSelected = key === selectedDateKey;
        const label = `${monthNames[month]} ${date.getDate()}, ${year}`;
        const classes = [
          "calendar-day",
          isToday ? "is-today" : "",
          isWeekend ? "is-weekend" : "",
          isSelected ? "is-selected" : "",
        ]
          .filter(Boolean)
          .join(" ");

        return `<button class="${classes}" type="button" data-date="${key}" title="${label}" aria-label="Show tasks for ${label}" aria-pressed="${isSelected}">${date.getDate()}</button>`;
      })
      .join("");

    monthCards.push(`
      <section class="calendar-month">
        <div class="calendar-month-header">
          <h3>${monthNames[month]}</h3>
          <span>${year}</span>
        </div>
        <div class="calendar-weekdays" aria-hidden="true">
          ${weekdayNames.map((day) => `<span>${day}</span>`).join("")}
        </div>
        <div class="calendar-days">
          ${days}
        </div>
      </section>
    `);
  }

  const lastMonth = new Date(visibleYear, startMonth + monthsToRender - 1, 1);
  title.textContent = `${visibleYear} Calendar`;
  rangeLabel.textContent = `${monthNames[startMonth]} ${visibleYear} to ${monthNames[lastMonth.getMonth()]} ${lastMonth.getFullYear()}`;
  grid.innerHTML = monthCards.join("");
}

function updateSelectedDateStyle() {
  els.calendarGrid?.querySelectorAll("button.calendar-day[data-date]").forEach((button) => {
    const selected = button.dataset.date === selectedDateKey;
    button.classList.toggle("is-selected", selected);
    button.setAttribute("aria-pressed", String(selected));
  });
}

function setCalendarTaskState({ loading = false, error } = {}) {
  els.calendarTaskLoading?.classList.toggle("hidden", !loading);
  if (els.calendarTaskError && error !== undefined) {
    els.calendarTaskError.classList.toggle("hidden", !error);
    els.calendarTaskError.textContent = error;
  }
}

function renderCalendarTasks(tasks) {
  if (!els.calendarTaskList) return;

  if (tasks.length === 0) {
    els.calendarTaskList.innerHTML = '<div class="dashboard-empty">No tasks are due on this date.</div>';
    return;
  }

  els.calendarTaskList.innerHTML = tasks
    .map((todo) => {
      const priority = String(todo.priority || "medium").toLowerCase();
      const priorityLabel = PRIORITY_LABELS[priority] || "Medium";
      const title = todo.title || "Untitled task";
      const description = todo.description || "";
      const completedClass = todo.done ? " is-complete" : "";
      const statusClass = todo.done ? " is-complete" : "";
      const status = todo.done ? "Completed" : "Incomplete";

      return `
        <article class="calendar-task-item${completedClass}">
          <div class="calendar-task-body">
            <div class="dashboard-task-title-row">
              <span class="todo-id-badge">#${escapeHtml(todo.id)}</span>
              <span class="calendar-task-title">${escapeHtml(title)}</span>
            </div>
            ${description ? `<span class="todo-desc">${escapeHtml(description)}</span>` : ""}
            <span class="calendar-task-status${statusClass}">${status}</span>
          </div>
          <span class="todo-priority-badge priority-${escapeHtml(priority)}">${escapeHtml(priorityLabel)}</span>
        </article>
      `;
    })
    .join("");
}

async function loadSelectedDateTasks(value) {
  if (!value || !els.calendarTaskPanel) return;

  const request = ++selectedDateRequest;
  els.calendarTaskPanel.classList.remove("hidden");
  if (els.calendarTaskTitle) {
    els.calendarTaskTitle.textContent = `Tasks for ${formatDateLabel(value)}`;
  }
  if (els.calendarTaskSummary) {
    els.calendarTaskSummary.textContent = "";
  }
  if (els.calendarTaskList) {
    els.calendarTaskList.innerHTML = "";
  }
  setCalendarTaskState({ loading: true, error: "" });

  if (!hasWailsBinding()) {
    setCalendarTaskState({ loading: false });
    if (els.calendarTaskList) {
      els.calendarTaskList.innerHTML = '<div class="dashboard-empty">Run the app to load tasks for this date.</div>';
    }
    return;
  }

  try {
    const result = await getTodosByDueDate(value);
    if (request !== selectedDateRequest) return;

    const tasks = Array.isArray(result) ? result : [];
    renderCalendarTasks(tasks);
    if (els.calendarTaskSummary) {
      els.calendarTaskSummary.textContent = tasks.length === 1
        ? "1 task scheduled"
        : `${tasks.length} tasks scheduled`;
    }
  } catch (err) {
    if (request !== selectedDateRequest) return;
    console.error(err);
    if (els.calendarTaskList) {
      els.calendarTaskList.innerHTML = "";
    }
    setCalendarTaskState({ error: err.message || String(err) });
  } finally {
    if (request === selectedDateRequest) {
      setCalendarTaskState({ loading: false });
    }
  }
}

function selectCalendarDate(value) {
  if (!dateFromKey(value)) return;
  selectedDateKey = value;
  updateSelectedDateStyle();
  void loadSelectedDateTasks(value);
}

export function initCalendar() {
  const todayButton = document.getElementById("calendar-today");
  const prevYearButton = document.getElementById("calendar-prev-year");
  const nextYearButton = document.getElementById("calendar-next-year");

  els.calendarGrid?.addEventListener("click", (event) => {
    const dayButton = event.target.closest("button.calendar-day[data-date]");
    if (!dayButton || !els.calendarGrid.contains(dayButton)) return;
    selectCalendarDate(dayButton.dataset.date);
  });

  todayButton?.addEventListener("click", () => {
    visibleYear = new Date().getFullYear();
    renderCalendar();
  });

  prevYearButton?.addEventListener("click", () => {
    visibleYear -= 1;
    renderCalendar();
  });

  nextYearButton?.addEventListener("click", () => {
    visibleYear += 1;
    renderCalendar();
  });

	document.addEventListener("todos:changed", (event) => {
		if (selectedDateKey) {
			if (Array.isArray(event.detail?.todos)) {
				selectedDateRequest += 1;
				const tasks = event.detail.todos.filter((todo) => todo.due_date === selectedDateKey);
				renderCalendarTasks(tasks);
				if (els.calendarTaskSummary) {
					els.calendarTaskSummary.textContent = tasks.length === 1
						? "1 task scheduled"
						: `${tasks.length} tasks scheduled`;
				}
				setCalendarTaskState({ loading: false, error: "" });
				return;
			}
			void loadSelectedDateTasks(selectedDateKey);
		}
	});

  renderCalendar();
}
