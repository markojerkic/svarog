document.addEventListener("calendar-select", (event) => {
  if (!(event.target instanceof Element)) return;
  const popup = event.target.closest("[id$='-popover']");
  if (!popup) return;
  const trigger = document.querySelector(`[data-datepicker][data-templ-controls="${popup.id}"]`);
  const hidden = document.getElementById(popup.id.slice(0, -"-popover".length) + "-value");
  if (!(trigger instanceof HTMLElement) || !hidden) return;
  const date = event.detail?.selected;
  const label = trigger.querySelector("span");
  if (date instanceof Date && !Number.isNaN(date.getTime())) {
    const iso = `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, "0")}-${String(date.getDate()).padStart(2, "0")}`;
    hidden.value = iso;
    if (label) label.textContent = iso;
    trigger.removeAttribute("data-empty");
  } else {
    hidden.value = "";
    if (label) label.textContent = trigger.getAttribute("data-placeholder") || "Select a date";
    trigger.setAttribute("data-empty", "true");
  }
  window.templ?.popover?.close(popup);
});
