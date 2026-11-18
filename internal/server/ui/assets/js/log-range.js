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
  form.querySelector("button[type=submit]").disabled = !from;
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

function localDateTime(date) {
  const parts = [date.getHours(), date.getMinutes(), date.getSeconds()];
  return `${isoDate(date)}T${parts.map((part) => String(part).padStart(2, "0")).join(":")}`;
}

document.addEventListener("focusin", (event) => {
  const form = event.target.closest("#log-time-range-form");
  if (!form || form.dataset.initialized) return;
  form.dataset.initialized = "true";
  for (const [name, selector] of [
    ["fromAt", "[data-log-time-from]"],
    ["toAt", "[data-log-time-to]"],
  ]) {
    if (form.elements[name].value) {
      form.querySelector(selector).value = localDateTime(new Date(form.elements[name].value));
    }
  }
});

document.addEventListener("click", (event) => {
  const preset = event.target.closest("[data-log-range-preset]");
  if (!preset) return;
  const form = preset.form;
  form.dataset.rangePreset = preset.dataset.logRangePreset;
  form.requestSubmit();
  delete form.dataset.rangePreset;
});

document.addEventListener(
  "submit",
  (event) => {
    const form = event.target;
    if (!(form instanceof HTMLFormElement) || form.id !== "log-time-range-form") return;
    const preset = form.dataset.rangePreset;
    const fromInput = form.querySelector("[data-log-time-from]");
    const toInput = form.querySelector("[data-log-time-to]");
    let from, to;
    if (preset === "today") {
      from = new Date();
      from.setHours(0, 0, 0, 0);
      to = new Date(from);
      to.setDate(to.getDate() + 1);
    } else if (preset) {
      to = new Date();
      from = new Date(to.getTime() - Number(preset) * 60_000);
    } else {
      from = new Date(fromInput.value);
      to = new Date(toInput.value);
    }
    if (!(from < to)) {
      event.preventDefault();
      toInput.setCustomValidity(window.svarogI18n("logs.invalid_range", "Invalid time range"));
      toInput.reportValidity();
      // A bad custom range should not prevent selecting a preset.
      toInput.setCustomValidity("");
      return;
    }
    form.elements.fromAt.value = from.toISOString();
    form.elements.toAt.value = to.toISOString();
  },
  true,
);
