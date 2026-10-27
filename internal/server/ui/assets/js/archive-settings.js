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

function setArchiveEditorOpen(open) {
  const editor = document.querySelector("#log-archive-editor");
  const edit = document.querySelector("#archive-edit");
  if (!editor || !edit) return;
  editor.hidden = !open;
  edit.setAttribute("aria-expanded", open ? "true" : "false");
  edit.querySelector("[data-archive-edit]").hidden = open;
  edit.querySelector("[data-archive-cancel]").hidden = !open;
}

function resetArchiveEditor(form) {
  if (!form) return;
  form.elements.namedItem("enabled").checked = form.dataset.savedEnabled === "true";
  form.elements.namedItem("archiveAfterWeeks").value = form.dataset.savedWeeks;
  updateArchiveSettings(form);
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

document.addEventListener("click", (event) => {
  const edit = event.target.closest("#archive-edit");
  if (!edit) return;
  const editor = document.querySelector("#log-archive-editor");
  const opening = editor.hidden;
  if (!opening) resetArchiveEditor(document.querySelector("#log-archive-settings-form"));
  setArchiveEditorOpen(opening);
});
