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
  const save = document.querySelector("#archive-save-changes");
  if (save) save.disabled = invalid;
}

window.templ.lifecycle.register("#log-archive-settings-form", {
  init(form) {
    updateArchiveSettings(form);
  },
});

document.addEventListener("input", (event) => {
  const form = event.target.closest("#log-archive-settings-form");
  if (form) updateArchiveSettings(form, true);
});

document.addEventListener("htmx:responseError", (event) => {
  if (event.detail.ctx.elt.id !== "log-archive-settings-form") return;
  const message = window.svarogI18n(
    "archives.save_failed",
    "Could not save archive settings. Try again.",
  );
  window.templ.toast.add({ title: message, type: "error", timeout: 4000 });
});
