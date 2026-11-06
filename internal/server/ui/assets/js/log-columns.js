const logColumnsCookie = "svarog_log_columns";
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
  const date = new Date(Number(row.dataset.timestamp.slice(0, -6)));
  if (Number.isNaN(date.getTime())) return;
  time.textContent = `${dateFormatter.format(date)} ${timeFormatter.format(date)}`;
}

window.templ.lifecycle.register("#log-scroll-container", {
  init(container) {
    container.querySelectorAll("pre[data-timestamp]").forEach(formatLogRow);
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

document.addEventListener("click", (event) => {
  const button = event.target.closest("[data-log-column]");
  if (!button) return;
  const container = document.getElementById("log-scroll-container");
  if (!container) return;
  const column = button.dataset.logColumn;
  const enabled = container.toggleAttribute(`data-show-${column}`);
  button.setAttribute("aria-pressed", String(enabled));
  const columns = ["time", "instance"].filter((name) =>
    container.hasAttribute(`data-show-${name}`),
  );
  const secure = location.protocol === "https:" ? "; Secure" : "";
  document.cookie = `${logColumnsCookie}=${columns.join(",")}; Path=/; Max-Age=31536000; SameSite=Lax${secure}`;
});
