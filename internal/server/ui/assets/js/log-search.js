// Loaded once by BaseLayout; delegated handlers also work after HTMX navigation.
window.templ.lifecycle.register("#log-search-trigger", {
  init(trigger) {
    const isMac = /Mac|iPhone|iPad/.test(navigator.platform);
    trigger.querySelector("kbd").textContent = isMac ? "⌘ K" : "Ctrl K";
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
