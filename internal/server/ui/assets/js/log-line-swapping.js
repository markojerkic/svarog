document.body.addEventListener("htmx:oobBeforeSwap", function (evt) {
  if (evt.detail.target.id === "log-scroll-container") {
    evt.preventDefault();

    swapLogLine(evt);
  }
});

function swapLogLine(evt) {
  var newLogEl = evt.detail.fragment.firstElementChild;
  if (!newLogEl) return;

  newLogEl.removeAttribute("hx-swap-oob");

  var container = document.getElementById("log-scroll-container");
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
