const logPagePositions = new WeakMap();

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

  for (const logLine of newContent) {
    if (logLine.nodeType === Node.ELEMENT_NODE) {
      swapLogLine(logLine, task.target);
    }
  }
});

function swapLogLine(newLogEl, container) {
  newLogEl.remove();
  var children = container.children;
  var newTs = parseInt(newLogEl.getAttribute("data-timestamp"));
  var inserted = false;

  var newSeq = parseInt(newLogEl.getAttribute("data-sequence")) || 0;

  for (var i = 0; i < children.length; i++) {
    var childTs = parseInt(children[i].getAttribute("data-timestamp"));
    var childSeq = parseInt(children[i].getAttribute("data-sequence")) || 0;
    if (newTs > childTs || (newTs === childTs && newSeq > childSeq)) {
      container.insertBefore(newLogEl, children[i]);
      inserted = true;
      break;
    }
  }

  if (!inserted) {
    container.appendChild(newLogEl);
  }
}
