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

document.addEventListener("copyLoginToken", function (evt) {
  if (evt.detail && evt.detail.value) {
    const url = new URL(window.location.href);
    url.protocol = window.location.protocol;
    url.hostname = window.location.hostname;
    url.port = window.location.port;
    url.pathname = "/login";
    url.searchParams.set("token", evt.detail.value);
    copyConnString(url.toString());
  }
});
