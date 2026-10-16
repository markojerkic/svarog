document.addEventListener("htmx:confirm", function (e) {
  e.preventDefault();

  const { ctx, issueRequest, dropRequest } = e.detail;
  document.getElementById("confirm-message").textContent = ctx.confirm;

  const confirmButton = document.getElementById("confirm-button");
  const dialogElement = document.getElementById("confirm-dialog");

  window.templ.dialog.open("confirm-dialog");

  const controller = new AbortController();
  confirmButton.addEventListener(
    "click",
    function () {
      controller.abort();
      issueRequest();
      window.templ.dialog.close("confirm-dialog");
    },
    { once: true, signal: controller.signal },
  );
  dialogElement.addEventListener(
    "dialog-close",
    function () {
      controller.abort();
      dropRequest();
    },
    { once: true, signal: controller.signal },
  );
});
