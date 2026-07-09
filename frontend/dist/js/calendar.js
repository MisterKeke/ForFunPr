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

function sameDate(a, b) {
  return (
    a.getFullYear() === b.getFullYear() &&
    a.getMonth() === b.getMonth() &&
    a.getDate() === b.getDate()
  );
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
  const grid = document.getElementById("calendar-grid");
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

        const isToday = sameDate(date, today);
        const isWeekend = date.getDay() === 0 || date.getDay() === 6;
        const classes = [
          "calendar-day",
          isToday ? "is-today" : "",
          isWeekend ? "is-weekend" : "",
        ]
          .filter(Boolean)
          .join(" ");

        return `<span class="${classes}" title="${monthNames[month]} ${date.getDate()}, ${year}">${date.getDate()}</span>`;
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

export function initCalendar() {
  const todayButton = document.getElementById("calendar-today");
  const prevYearButton = document.getElementById("calendar-prev-year");
  const nextYearButton = document.getElementById("calendar-next-year");

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

  renderCalendar();
}
