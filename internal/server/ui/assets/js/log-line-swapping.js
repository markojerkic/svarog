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
