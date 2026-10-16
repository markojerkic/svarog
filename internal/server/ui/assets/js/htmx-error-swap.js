// HTMX 4 swaps errors by default. Keep HTML validation errors, not JSON or 5xx.
document.addEventListener("htmx:before:response", function (evt) {
  const response = evt.detail.ctx.response;
  if (!response || response.status < 400) return;

  const isHTML = response.headers.get("content-type")?.includes("text/html");
  if (response.status >= 500 || !isHTML) {
    evt.preventDefault();
  }
});
