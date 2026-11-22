const logColumnsCookie = "svarog_log_columns";
const logWrapLinesCookie = "svarog_log_wrap_lines";
const locale = document.documentElement.lang || undefined;
const dateFormatter = new Intl.DateTimeFormat(locale, {
  year: "numeric",
  month: "2-digit",
  day: "2-digit",
});
const timeFormatter = new Intl.DateTimeFormat(locale, {
  hour: "2-digit",
  minute: "2-digit",
  second: "2-digit",
  hourCycle: "h23",
  fractionalSecondDigits: 3,
});
const observers = new WeakMap();

function formatLogRow(row) {
  if (!(row instanceof Element) || !row.matches("pre[data-timestamp]")) return;
  const time = row.querySelector("[data-log-time]");
  if (!time || time.textContent.trim()) return;
  const date = new Date(time.dateTime);
  if (Number.isNaN(date.getTime())) return;
  time.title = time.dateTime;
  time.textContent = `${dateFormatter.format(date)} ${timeFormatter.format(date)}`;
}

window.templ.lifecycle.register("#log-scroll-container", {
  init(container) {
    container.querySelectorAll("pre[data-timestamp]").forEach(formatLogRow);
    const anchor = container.querySelector("[data-log-anchor]");
    if (anchor) {
      // Center within the log viewport, without scrolling the surrounding page.
      requestAnimationFrame(() => {
        if (!anchor.isConnected) return;
        const row = anchor.getBoundingClientRect();
        const view = container.getBoundingClientRect();
        container.scrollTop += row.top - view.top + (row.height - view.height) / 2;
      });
    }
    const observer = new MutationObserver((records) => {
      for (const record of records) record.addedNodes.forEach(formatLogRow);
    });
    observer.observe(container, { childList: true });
    observers.set(container, observer);
  },
  destroy(container) {
    observers.get(container)?.disconnect();
    observers.delete(container);
  },
});

function saveLogPreference(name, value) {
  const secure = location.protocol === "https:" ? "; Secure" : "";
  document.cookie = `${name}=${value}; Path=/; Max-Age=31536000; SameSite=Lax${secure}`;
}

document.addEventListener("click", (event) => {
  const button = event.target.closest("[data-log-option]");
  if (!button) return;
  const container = document.getElementById("log-scroll-container");
  if (!container) return;
  const option = button.dataset.logOption;
  const enabled = button.getAttribute("aria-pressed") !== "true";
  if (option === "wrap") {
    container.dataset.wrapLines = String(enabled);
    saveLogPreference(logWrapLinesCookie, String(enabled));
  } else {
    container.toggleAttribute(`data-show-${option}`, enabled);
    const columns = ["time", "instance"].filter((name) =>
      container.hasAttribute(`data-show-${name}`),
    );
    saveLogPreference(logColumnsCookie, columns.join(","));
  }
  document.querySelectorAll(`[data-log-option="${option}"]`).forEach((el) => {
    el.setAttribute("aria-pressed", String(enabled));
  });
});
