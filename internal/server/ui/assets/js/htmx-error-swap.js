// Allow HTMX to swap content on 4xx error responses (e.g., validation errors)
document.body.addEventListener("htmx:beforeSwap", function (evt) {
  const status = evt.detail.xhr.status;
  const contentType = evt.detail.xhr.getResponseHeader("content-type");
  const isHTML = contentType && contentType.includes("text/html");
  if (status >= 400 && status < 500 && isHTML) {
    evt.detail.shouldSwap = true;
    evt.detail.isError = false;
  }
});
