function secretText(field) {
  if (!field) return "";
  if (field instanceof HTMLTextAreaElement || field instanceof HTMLInputElement) {
    return field.value;
  }
  return field.textContent || "";
}

function writeClipboard(text) {
  if (!text || !navigator.clipboard || typeof navigator.clipboard.writeText !== "function") {
    return Promise.reject(new Error("clipboard unavailable"));
  }
  return navigator.clipboard.writeText(text);
}

function showCopyFailed() {
  const message =
    (window.svarogI18n &&
      window.svarogI18n(
        "common.copy_failed",
        "Could not copy. Select the value and copy it manually.",
      )) ||
    "Could not copy. Select the value and copy it manually.";
  if (typeof showToast === "function") {
    showToast(message, "error");
    return;
  }
  if (window.templ && window.templ.toast) {
    window.templ.toast.add({ title: message, type: "error", timeout: 4000 });
  }
}

document.addEventListener("click", function (evt) {
  const button = evt.target instanceof Element ? evt.target.closest("[data-copy-target]") : null;
  if (!button) return;
  evt.preventDefault();
  const field = document.getElementById(button.getAttribute("data-copy-target"));
  writeClipboard(secretText(field)).catch(showCopyFailed);
});

// Best-effort copy after the secret is already in the page. A rejected
// clipboard write must not be reported as success; the visible field remains.
document.addEventListener("htmx:after:swap", function (evt) {
  const target = evt.detail && evt.detail.ctx && evt.detail.ctx.target;
  if (!(target instanceof Element)) return;

  // open() is a no-op when the dialog is already open, and shows the secret
  // if the admin closed the dialog before the response arrived.
  if (
    target.id === "login-link-result" ||
    target.id === "login-link-url" ||
    target.querySelector("#login-link-url")
  ) {
    window.templ.dialog.open("login-link-dialog");
  }
  if (target.id === "connection-string-value" || target.querySelector("#connection-string-value")) {
    window.templ.dialog.open("connection-string-dialog");
  }

  const field = target.matches("[data-copy-source]")
    ? target
    : target.querySelector("[data-copy-source]");
  if (!field) return;

  requestAnimationFrame(function () {
    if (typeof field.focus === "function") field.focus();
    if (typeof field.select === "function") field.select();
  });
  writeClipboard(secretText(field)).catch(function () {});
});
