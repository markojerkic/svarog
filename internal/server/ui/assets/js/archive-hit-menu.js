// One native popover serves every result, including pages appended by HTMX.
let trigger;

function openActions(button, point) {
  const menu = document.getElementById("archive-hit-menu");
  if (!menu) return;
  const wasOpen = menu.matches(":popover-open") && trigger === button;
  menu.hidePopover();
  if (wasOpen && !point) return;
  trigger = button;
  const search = new URL(menu.dataset.searchUrl || location.href, location.origin);
  search.searchParams.delete("older_cursor");
  const context = new URL(button.dataset.archiveUrl + "/context", location.origin);
  context.searchParams.set("entry", button.dataset.entry);
  context.searchParams.set("line", button.dataset.line);
  menu.dataset.permalink = context.href;
  const download = new URL(context);
  download.pathname += "/download";
  for (const name of ["from", "to", "text", "instance"]) {
    if (search.searchParams.has(name))
      context.searchParams.set(name, search.searchParams.get(name));
  }
  if (!search.searchParams.has("from") && menu.dataset.archiveStart) {
    search.searchParams.set("from", menu.dataset.archiveStart);
    search.searchParams.set("to", menu.dataset.archiveEnd);
  }
  search.searchParams.set("instance", button.dataset.instance);
  const urls = { context, instance: search, download, zip: button.dataset.archiveUrl };
  for (const link of menu.querySelectorAll("a[data-action]")) link.href = urls[link.dataset.action];
  menu.showPopover({ source: button });
  const rect = button.getBoundingClientRect();
  const width = menu.offsetWidth,
    height = menu.offsetHeight;
  const x = point ? point.x : rect.right - width;
  let y = point ? point.y : rect.bottom + 4;
  if (!point && y + height > innerHeight - 8) y = rect.top - height - 4;
  menu.style.left = `${Math.max(8, Math.min(x, innerWidth - width - 8))}px`;
  menu.style.top = `${Math.max(8, Math.min(y, innerHeight - height - 8))}px`;
  menu.querySelector("a").focus({ preventScroll: true });
}

document.addEventListener("click", async (event) => {
  const button = event.target.closest("[data-archive-url]");
  if (button) {
    event.preventDefault(); // openActions also positions the native popover.
    openActions(button);
    return;
  }
  const item = event.target.closest("#archive-hit-menu [data-action]");
  if (!item) return;
  const menu = item.closest("[popover]");
  menu.hidePopover();
  if (item.dataset.action !== "copy" && item.dataset.action !== "link") return;
  try {
    const link = item.dataset.action === "link";
    await navigator.clipboard.writeText(
      link
        ? menu.dataset.permalink
        : trigger.closest("[data-archive-row]").querySelector("[data-log-message]").textContent,
    );
    showToast(window.svarogI18n(link ? "logs.link_copied" : "archives.line_copied"));
  } catch {
    showToast(window.svarogI18n("common.copy_failed"), "error");
  }
});

document.addEventListener("contextmenu", (event) => {
  const button = event.target.closest("[data-archive-row]")?.querySelector("[data-archive-url]");
  if (!button) return;
  event.preventDefault();
  openActions(button, { x: event.clientX, y: event.clientY });
});
