document.addEventListener("htmx:confirm", function (e) {
  if (!e.detail.question) return;

  e.preventDefault();

  const message = e.detail.question;
  document.getElementById("confirm-message").textContent = message;

  const confirmButton = document.getElementById("confirm-button");
  const dialogElement = document.querySelector(
    '[data-tui-dialog][id="confirm-dialog"]',
  );

  window.tui.dialog.open("confirm-dialog");

  const handleConfirm = function () {
    e.detail.issueRequest(true);
    window.tui.dialog.close("confirm-dialog");
    confirmButton.removeEventListener("click", handleConfirm);
    dialogElement.removeEventListener("click", handleCancel);
  };

  const handleCancel = function (event) {
    if (event.target.hasAttribute("data-tui-dialog-close")) {
      window.tui.dialog.close("confirm-dialog");
      confirmButton.removeEventListener("click", handleConfirm);
      dialogElement.removeEventListener("click", handleCancel);
    }
  };

  confirmButton.addEventListener("click", handleConfirm, { once: true });
  dialogElement.addEventListener("click", handleCancel);
});
