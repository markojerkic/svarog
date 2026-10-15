htmx.on("htmx:afterRequest", function (evt) {
  const headers = evt.detail.xhr.getAllResponseHeaders();
  const isCloseDialog = headers.includes("close-dialog");
  if (!isCloseDialog) {
    return;
  }
  const dialog = evt.detail.elt.closest('[role="dialog"]');
  if (dialog) window.templ.dialog.close(dialog.id);
});
