function isoDate(date) {
  return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, "0")}-${String(date.getDate()).padStart(2, "0")}`;
}

document.addEventListener("calendar-select", (event) => {
  if (!(event.target instanceof Element) || !event.target.closest("#log-range-calendar")) return;
  const selected = event.detail?.selected;
  const from = selected?.from instanceof Date ? selected.from : null;
  const to = selected?.to instanceof Date ? selected.to : null;
  if (event.target.closest("#archive-range")) {
    setArchiveDays(document.getElementById("archive-search-form"), from, to);
    return;
  }
  const form = event.target.closest("form");
  if (!(form instanceof HTMLFormElement)) return;
  form.elements.from.value = from ? isoDate(from) : "";
  form.elements.to.value = to ? isoDate(to) : "";
  form.querySelector("button[type=submit]").disabled = !from;
});

function setArchiveDays(form, from, to) {
  form.dataset.fromDay = from ? isoDate(from) : "";
  form.dataset.toDay = to ? isoDate(to) : form.dataset.fromDay;
  const trigger = document.getElementById("archive-range-trigger");
  if (from) {
    document.getElementById("archive-range-error").hidden = true;
    trigger.removeAttribute("aria-invalid");
    trigger.removeAttribute("aria-describedby");
  }
  const croatian = document.documentElement.lang === "hr";
  const format = new Intl.DateTimeFormat(croatian ? "hr-HR" : "en-GB", {
    day: "numeric",
    month: croatian ? "numeric" : "short",
    year: "numeric",
  });
  let label = from ? format.format(from) : window.svarogI18n("common.select_date");
  if (to && form.dataset.fromDay !== form.dataset.toDay) label += ` – ${format.format(to)}`;
  trigger.querySelector("span").textContent = label;
}

window.templ.lifecycle.register("#archive-search-form", {
  init(form) {
    if (!form.elements.from.value || !form.elements.to.value) return;
    const from = new Date(Number(form.elements.from.value) * 1000);
    const to = new Date(Number(form.elements.to.value) * 1000);
    const fromClock = localDateTime(from).split("T")[1];
    const toClock = localDateTime(to).split("T")[1];
    const fromInput = form.elements["archive-from-time"];
    const toInput = form.elements["archive-to-time"];
    fromInput.value = fromClock === "00:00:00" ? "" : fromClock;
    toInput.value = toClock === "00:00:00" ? "" : toClock;
    toInput.closest("details").open = Boolean(fromInput.value || toInput.value);
    // The calendar includes the last day; the URL excludes its end instant.
    if (!toInput.value) to.setTime(to.getTime() - 1000);
    from.setHours(0, 0, 0, 0);
    to.setHours(0, 0, 0, 0);
    setArchiveDays(form, from, to);
    const calendar = form.querySelector("#log-range-calendar");
    window.templ.calendar.setMonth(calendar, from);
    window.templ.calendar.setSelected(calendar, { from, to });
  },
});

function archiveUnixSeconds(day, clock, previous) {
  const date = new Date(`${day}T${clock}`);
  // A repeated local hour can refer to two instants. Keep the original one
  // when a shared search is submitted again without changing that boundary.
  if (previous && localDateTime(new Date(Number(previous) * 1000)) === localDateTime(date)) {
    return Number(previous);
  }
  return date.getTime() / 1000;
}

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
  if (event.target.closest("[data-archive-range-done]")) {
    window.templ.popover.close("archive-range");
    return;
  }
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
    if (
      !(form instanceof HTMLFormElement) ||
      !form.matches("#log-time-range-form, #archive-search-form")
    )
      return;
    if (form.id === "archive-search-form") {
      if (!form.dataset.fromDay) {
        event.preventDefault();
        const trigger = document.getElementById("archive-range-trigger");
        const warning = document.getElementById("archive-range-error");
        warning.hidden = false;
        trigger.setAttribute("aria-invalid", "true");
        trigger.setAttribute("aria-describedby", warning.id);
        trigger.focus();
        return;
      }
      const toInput = form.elements["archive-to-time"];
      const from = archiveUnixSeconds(
        form.dataset.fromDay,
        form.elements["archive-from-time"].value || "00:00",
        form.elements.from.value,
      );
      const to = archiveUnixSeconds(
        form.dataset.toDay,
        toInput.value || "24:00",
        form.elements.to.value,
      );
      if (!(from < to)) {
        event.preventDefault();
        window.templ.popover.open("archive-range");
        toInput.closest("details").open = true;
        toInput.setCustomValidity(window.svarogI18n("logs.invalid_range"));
        toInput.reportValidity();
        toInput.setCustomValidity("");
        return;
      }
      form.elements.from.value = String(from);
      form.elements.to.value = String(to);
      form.querySelector("button[type=submit]").disabled = true;
      form.setAttribute("aria-busy", "true");
      return;
    }
    const preset = form.dataset.rangePreset;
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
      from = new Date(form.querySelector("[data-log-time-from]").value);
      to = new Date(toInput.value);
    }
    if (!(from < to)) {
      event.preventDefault();
      toInput.closest("details").open = true;
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

window.addEventListener("pageshow", (event) => {
  const form = document.querySelector("#archive-search-form[aria-busy]");
  if (!event.persisted || !form) return;
  form.removeAttribute("aria-busy");
  form.querySelector("button[type=submit]").disabled = false;
});
