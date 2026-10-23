// UI preview only: drafts reset when the sheet opens and never leave the browser.
function updateArchiveSettings(form, validate = false) {
  const enabled = form.elements.namedItem("enabled").checked;
  const weeks = form.elements.namedItem("archiveAfterWeeks");
  weeks.disabled = !enabled;
  form.querySelector("[data-archive-enabled]").hidden = !enabled;
  form.querySelector("[data-archive-disabled]").hidden = enabled;

  const invalid = enabled && validate && !weeks.validity.valid;
  form.querySelector("#archive-weeks-error").hidden = !invalid;
  if (invalid) {
    weeks.setAttribute("aria-invalid", "true");
  } else {
    weeks.removeAttribute("aria-invalid");
  }
}

window.templ.lifecycle.register("#log-archive-settings-form", {
  init(form) {
    updateArchiveSettings(form);
  },
});

document.addEventListener("dialog-open-change", (event) => {
  if (event.target.id !== "log-archive-sheet" || !event.detail.open) return;
  const form = event.target.querySelector("#log-archive-settings-form");
  form.reset();
  updateArchiveSettings(form);
});

document.addEventListener("input", (event) => {
  const form = event.target.closest("#log-archive-settings-form");
  if (form) updateArchiveSettings(form, true);
});

document.addEventListener("submit", (event) => {
  if (event.target.id === "log-archive-settings-form") event.preventDefault();
});
