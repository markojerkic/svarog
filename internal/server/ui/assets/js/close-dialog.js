document.addEventListener("htmx:after:request", function (evt) {
  const { response, sourceElement } = evt.detail.ctx;
  if (response.headers.get("close-dialog") !== "true") {
    return;
  }
  const dialog = sourceElement.closest('[role="dialog"]');
  if (dialog) window.templ.dialog.close(dialog.id);
});
