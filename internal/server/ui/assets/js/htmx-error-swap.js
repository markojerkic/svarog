// Allow HTMX to swap content on 4xx error responses (e.g., validation errors)
document.body.addEventListener("htmx:beforeSwap", function (evt) {
  const status = evt.detail.xhr.status;
  if (status >= 400 && status < 500) {
    evt.detail.shouldSwap = true;
    evt.detail.isError = false;
  }
});
