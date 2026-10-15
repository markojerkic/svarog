document.addEventListener("htmx:confirm", function (e) {
  if (!e.detail.question) return;

  e.preventDefault();

  const message = e.detail.question;
  document.getElementById("confirm-message").textContent = message;

  const confirmButton = document.getElementById("confirm-button");
  const dialogElement = document.getElementById("confirm-dialog");

  window.templ.dialog.open("confirm-dialog");

  const handleConfirm = function () {
    e.detail.issueRequest(true);
    window.templ.dialog.close("confirm-dialog");
    confirmButton.removeEventListener("click", handleConfirm);
    dialogElement.removeEventListener("click", handleCancel);
  };

  const handleCancel = function (event) {
    if (event.target.closest("[data-templ-dialog-close]")) {
      window.templ.dialog.close("confirm-dialog");
      confirmButton.removeEventListener("click", handleConfirm);
      dialogElement.removeEventListener("click", handleCancel);
    }
  };

  confirmButton.addEventListener("click", handleConfirm, { once: true });
  dialogElement.addEventListener("click", handleCancel);
});
