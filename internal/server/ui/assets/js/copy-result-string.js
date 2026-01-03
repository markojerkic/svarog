/** @param {String | null} connString */
function copyConnString(connString) {
  if (!connString) {
    console.error("No connection string to copy");
    return;
  }

  navigator.clipboard.writeText(connString);
}

// Listen for HTMX custom event to copy to clipboard
document.addEventListener("copyToClipboard", function (evt) {
  if (evt.detail && evt.detail.value) {
    copyConnString(evt.detail.value);
  }
});
