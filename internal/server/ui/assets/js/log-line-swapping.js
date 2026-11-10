const logPagePositions = new WeakMap();
const logRowKeys = new WeakMap();
const logRowBytes = new WeakMap();
const maxLiveRows = 2000;
const maxLiveBytes = 4 * 1024 * 1024;

document.addEventListener("htmx:before:swap", function (evt) {
  const ctx = evt.detail.ctx;
  const loader = ctx.sourceElement?.closest("[data-log-page]");
  const container = loader?.closest("#log-scroll-container");
  if (!container) return;

  const row =
    loader.dataset.logPage === "forward"
      ? loader.nextElementSibling
      : loader.previousElementSibling;
  if (!row?.matches("pre")) return;
  logPagePositions.set(ctx, { container, row, top: row.getBoundingClientRect().top });
});

document.addEventListener("htmx:after:swap", function (evt) {
  const ctx = evt.detail.ctx;
  const position = logPagePositions.get(ctx);
  logPagePositions.delete(ctx);
  if (!position?.row.isConnected) return;

  position.container.scrollTop += position.row.getBoundingClientRect().top - position.top;
});

document.addEventListener("htmx:before:settle", function (evt) {
  const { task, newContent } = evt.detail;
  if (task.type !== "oob" || task.target.id !== "log-scroll-container") return;

  const container = task.target;
  const elements = Array.from(newContent).filter((node) => node.nodeType === Node.ELEMENT_NODE);
  if (elements.some((node) => node.hasAttribute("data-log-resync"))) {
    elements.forEach((node) => node.remove());
    stopLiveLogs(container);
    return;
  }
  const incoming = elements.filter((node) => node.matches("pre[data-timestamp]"));
  incoming.forEach((row) => row.remove());
  if (container.dataset.liveStale === "true") return;
  mergeLogRows(incoming, container);
});

document.addEventListener("htmx:ws:close", function (evt) {
  if (evt.detail.reason === "removed") return;
  const container = evt.target.closest?.("#log-scroll-container[data-log-live]");
  if (container) stopLiveLogs(container);
});

function stopLiveLogs(container) {
  if (!container.hasAttribute("data-log-live") || container.dataset.liveStale === "true") return;
  container.dataset.liveStale = "true";
  const notice = document.getElementById("log-live-notice");
  if (notice) notice.hidden = false;
  const connection = container.querySelector("[data-log-stream]");
  if (connection) {
    window.htmx.swap({ sourceElement: connection, target: connection, swap: "delete", text: "" });
  }
}

function logRowKey(row) {
  if (!logRowKeys.has(row)) {
    logRowKeys.set(row, [
      BigInt(row.dataset.timestamp),
      BigInt(row.dataset.sequence || "0"),
      BigInt(row.dataset.logId || "0"),
    ]);
  }
  return logRowKeys.get(row);
}

function compareLogRows(a, b) {
  const left = logRowKey(a);
  const right = logRowKey(b);
  for (let i = 0; i < left.length; i++) {
    if (left[i] !== right[i]) return left[i] > right[i] ? -1 : 1;
  }
  return 0;
}

function rowBytes(row) {
  if (!logRowBytes.has(row)) {
    logRowBytes.set(row, new TextEncoder().encode(row.outerHTML).byteLength);
  }
  return logRowBytes.get(row);
}

function mergeLogRows(incoming, container) {
  const existing = Array.from(container.querySelectorAll(":scope > pre[data-timestamp]"));
  const ids = new Set(existing.map((row) => row.dataset.logId).filter((id) => id && id !== "0"));
  const olderLoader = container.querySelector('[data-log-page="backward"]');
  const oldest = existing.at(-1);
  const rows = incoming.filter((row) => {
    const id = row.dataset.logId;
    if (id && id !== "0") {
      if (ids.has(id)) return false;
      ids.add(id);
    }
    // Rows outside the loaded history window must arrive via its paginator.
    return !olderLoader || !oldest || compareLogRows(row, oldest) <= 0;
  });
  rows.sort(compareLogRows);
  if (!rows.length) return;

  const live = container.hasAttribute("data-log-live");
  const atLatest = Math.abs(container.scrollTop) < 32;
  const bytes = live ? [...existing, ...rows].reduce((sum, row) => sum + rowBytes(row), 0) : 0;
  if (live && !atLatest && (existing.length + rows.length > maxLiveRows || bytes > maxLiveBytes)) {
    stopLiveLogs(container);
    return;
  }

  const bounds = container.getBoundingClientRect();
  const anchor = atLatest
    ? null
    : existing.find((row) => row.getBoundingClientRect().bottom > bounds.top);
  const anchorTop = anchor?.getBoundingClientRect().top;
  const tail = olderLoader || container.querySelector("[data-log-stream]");
  let index = 0;
  let fragment = document.createDocumentFragment();
  let previousTarget;
  for (const row of rows) {
    while (index < existing.length && compareLogRows(existing[index], row) <= 0) index++;
    const target = existing[index] || tail;
    if (previousTarget !== undefined && target !== previousTarget) {
      container.insertBefore(fragment, previousTarget);
      fragment = document.createDocumentFragment();
    }
    fragment.append(row);
    previousTarget = target;
  }
  container.insertBefore(fragment, previousTarget || null);
  container.querySelector("[data-log-empty]")?.remove();

  if (live && atLatest) trimLiveRows(container, bytes);
  if (anchor?.isConnected) {
    container.scrollTop += anchor.getBoundingClientRect().top - anchorTop;
  }
}

function trimLiveRows(container, totalBytes) {
  const rows = Array.from(container.querySelectorAll(":scope > pre[data-timestamp]"));
  let trimmed = false;
  while (rows.length > 1 && (rows.length > maxLiveRows || totalBytes > maxLiveBytes)) {
    const row = rows.pop();
    totalBytes -= rowBytes(row);
    row.remove();
    trimmed = true;
  }
  if (!trimmed) return;

  let loader = container.querySelector('[data-log-page="backward"]');
  let created = false;
  if (!loader) {
    loader = document
      .getElementById("log-live-older-loader")
      ?.content.firstElementChild.cloneNode(true);
    if (!loader) return;
    created = true;
  }
  const last = rows.at(-1);
  const url = new URL(loader.getAttribute("hx-get"), document.baseURI);
  url.searchParams.delete("cursorTime");
  url.searchParams.set("cursorTimeUs", (BigInt(last.dataset.timestamp) / 1000n).toString());
  url.searchParams.set("cursorSequenceNumber", last.dataset.sequence || "0");
  url.searchParams.set("cursorId", last.dataset.logId);
  loader.setAttribute("hx-get", url.pathname + url.search);
  if (created) {
    container.insertBefore(loader, container.querySelector("[data-log-stream]"));
    window.htmx.process(loader);
  }
}
