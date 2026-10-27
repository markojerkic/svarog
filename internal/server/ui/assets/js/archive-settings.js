function updateArchiveSettings(form, validate = false) {
  const enabled = form.elements.namedItem("enabled").checked;
  const weeks = form.elements.namedItem("archiveAfterWeeks");
  weeks.disabled = !enabled;

  const invalid = enabled && validate && !weeks.validity.valid;
  const error = form.querySelector("#archive-weeks-error");
  if (error) error.hidden = !invalid;
  if (invalid) {
    weeks.setAttribute("aria-invalid", "true");
  } else {
    weeks.removeAttribute("aria-invalid");
  }
  const save = form.querySelector("#archive-save-changes");
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
