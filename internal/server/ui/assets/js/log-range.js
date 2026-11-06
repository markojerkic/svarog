function isoDate(date) {
  return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, "0")}-${String(date.getDate()).padStart(2, "0")}`;
}

document.addEventListener("calendar-select", (event) => {
  if (!(event.target instanceof Element) || !event.target.closest("#log-range-calendar")) return;
  const form = document.getElementById("log-range-form");
  if (!(form instanceof HTMLFormElement)) return;
  const selected = event.detail?.selected;
  const from = selected?.from instanceof Date ? selected.from : null;
  const to = selected?.to instanceof Date ? selected.to : null;
  form.elements.from.value = from ? isoDate(from) : "";
  form.elements.to.value = to ? isoDate(to) : "";
});

document.addEventListener(
  "submit",
  (event) => {
    const form = event.target;
    if (!(form instanceof HTMLFormElement) || form.id !== "log-range-form") return;
    form.elements.offset.value = String(new Date().getTimezoneOffset());
    if (!form.elements.from.value) event.preventDefault();
  },
  true,
);
