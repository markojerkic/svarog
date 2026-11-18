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

// Settle fires on the mounted target even when the request's original button
// was replaced. Wait one frame for the dialog portal to initialize.
const shownCopyResults = new WeakSet();
document.addEventListener("htmx:after:settle", function (evt) {
  const target = evt.detail.task.target;
  if (!(target instanceof Element)) return;
  const field = target.matches("[data-copy-source]")
    ? target
    : target.querySelector("[data-copy-source]");
  if (!field || shownCopyResults.has(field)) return;

  const dialog = field.closest('[role="dialog"]');
  if (!dialog) return;
  shownCopyResults.add(field);
  writeClipboard(secretText(field)).catch(function () {});
  requestAnimationFrame(function () {
    if (!field.isConnected) return;
    window.templ.dialog.open(dialog.id);
    field.focus();
    field.select();
  });
});
