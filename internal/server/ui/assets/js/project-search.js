function scheduleProjectSearch(input) {
  const listId = input?.getAttribute("data-project-search");
  if (!listId) return;
  clearTimeout(input._projectSearchTimer);
  input._projectSearchTimer = setTimeout(() => {
    const list = document.getElementById(listId);
    if (list && window.htmx) htmx.trigger(list, "search");
  }, 200);
}

document.addEventListener("input", (event) => {
  scheduleProjectSearch(event.target);
});

document.addEventListener("focusin", (event) => {
  scheduleProjectSearch(event.target);
});

document.addEventListener("click", (event) => {
  const input = event.target
    .closest?.("[data-templ-combobox-anchor]")
    ?.querySelector("[data-project-search]");
  scheduleProjectSearch(input);
});

document.addEventListener("combobox-open-change", (event) => {
  if (!event.detail?.open) return;
  const input =
    event.target.closest?.("[data-project-search]") ||
    event.target.querySelector?.("[data-project-search]");
  scheduleProjectSearch(input);
});
