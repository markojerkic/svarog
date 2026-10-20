// Loaded once by BaseLayout; delegated handlers also work after HTMX navigation.
window.templ.lifecycle.register("#log-search-trigger", {
  init(trigger) {
    const shortcut = trigger.querySelector("kbd");
    if (!shortcut) return;
    const isMac = /Mac|iPhone|iPad/.test(navigator.platform);
    shortcut.textContent = isMac ? "⌘ K" : "Ctrl K";
  },
});

document.addEventListener("keydown", (event) => {
  if (
    event.defaultPrevented ||
    event.isComposing ||
    event.repeat ||
    !(event.ctrlKey || event.metaKey) ||
    event.altKey ||
    event.shiftKey ||
    event.key.toLowerCase() !== "k"
  ) {
    return;
  }
  if (
    event.target.closest?.(
      "input, textarea, select, [contenteditable]:not([contenteditable='false']), [role='textbox']",
    ) ||
    document.querySelector('[role="dialog"][data-open], [role="alertdialog"][data-open]')
  ) {
    return;
  }

  const trigger = document.getElementById("log-search-trigger");
  if (!trigger || !trigger.getClientRects().length) return;
  event.preventDefault();
  trigger.click();
});

// Dialogs normally keep the virtual keyboard closed on touch. Search should
// focus its input immediately without changing other dialogs' focus behavior.
document.addEventListener("focusin", (event) => {
  if (event.target.id === "log-search-dialog") {
    event.target.querySelector("input").focus();
  }
});

document.addEventListener("dialog-open-change", (event) => {
  if (event.target.id !== "log-search-dialog") return;
  if (event.detail.open) {
    const input = event.target.querySelector("input[name='search']");
    input.value = input.defaultValue;
  } else {
    window.htmx.trigger(document.getElementById("log-search-form"), "htmx:abort");
  }
});

document.addEventListener("htmx:before:response", (event) => {
  const { sourceElement, response } = event.detail.ctx;
  if (sourceElement.matches("[data-log-page]") && !sourceElement.isConnected) {
    event.preventDefault();
    return;
  }
  if (
    sourceElement.matches(
      "#log-search-form, [data-log-instance-filter], [data-log-context], [data-log-page]",
    ) &&
    response.status >= 400
  ) {
    event.preventDefault();
    showToast("Could not load logs. Try again.", "error");
  }
});

document.addEventListener("htmx:error", (event) => {
  const { ctx, error } = event.detail;
  if (
    ctx.sourceElement.matches(
      "#log-search-form, [data-log-instance-filter], [data-log-context], [data-log-page]",
    ) &&
    error.name !== "AbortError"
  ) {
    showToast("Could not load logs. Try again.", "error");
  }
});
