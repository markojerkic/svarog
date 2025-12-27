htmx.on("htmx:afterSettle", function (evt) {
  const headers = evt.detail.xhr.getAllResponseHeaders();
  const isCloseDialog = headers.includes("close-dialog");
  console.log("isCloseDialog", isCloseDialog);
  if (!isCloseDialog) {
    return;
  }
  const dialog = evt.detail.elt.closest("[data-tui-dialog]");
  console.log("dialog", dialog);
  window.tui.dialog.close(dialog.id);
});
