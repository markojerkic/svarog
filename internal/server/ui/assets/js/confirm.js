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
      if (ctx.sourceElement.hasAttribute("data-confirm-project-name")) {
        dialogElement.addEventListener(
          "dialog-close",
          () => confirmProjectDeletion(ctx, issueRequest, dropRequest),
          { once: true },
        );
      } else {
        issueRequest();
      }
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

function confirmProjectDeletion(ctx, issueRequest, dropRequest) {
  const projectName = ctx.sourceElement.dataset.confirmProjectName;
  const dialogElement = document.getElementById("delete-project-dialog");
  const form = document.getElementById("delete-project-confirmation-form");
  const input = document.getElementById("delete-project-name");
  const button = document.getElementById("delete-project-button");
  document.getElementById("delete-project-expected-name").textContent = projectName;
  form.reset();
  button.disabled = true;

  const controller = new AbortController();
  input.addEventListener(
    "input",
    () => {
      button.disabled = input.value !== projectName;
    },
    { signal: controller.signal },
  );
  form.addEventListener(
    "submit",
    (event) => {
      event.preventDefault();
      if (input.value !== projectName) return;

      controller.abort();
      button.disabled = true;
      // HTMX 4 has already encoded DELETE parameters before htmx:confirm.
      const url = new URL(ctx.request.action, document.baseURI);
      url.searchParams.set("confirmationName", input.value);
      ctx.request.action = url.pathname + url.search;
      issueRequest();
      window.templ.dialog.close("delete-project-dialog");
    },
    { signal: controller.signal },
  );
  dialogElement.addEventListener(
    "dialog-close",
    () => {
      controller.abort();
      form.reset();
      button.disabled = true;
      dropRequest();
    },
    { once: true, signal: controller.signal },
  );

  window.templ.dialog.open("delete-project-dialog");
}
