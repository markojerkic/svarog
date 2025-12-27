/**
 *
 * @typedef {Object} ApiError
 * @property {string} message
 * @property {Object<string, string>} fields
 *
 */

htmx.on("htmx:responseError", function (evt) {
  const status = evt.detail.xhr.status;
  const contentType = evt.detail.xhr.getResponseHeader("content-type");
  const isJSON = contentType && contentType.includes("application/json");
  if (status >= 400 && status < 500 && isJSON) {
    /** @type {ApiError} */
    const error = JSON.parse(evt.detail.xhr.responseText);
    if (!("fields" in error)) {
      console.log("No fields in error", error);
      return;
    }

    const form = evt.detail.elt.closest("form") || evt.detail.elt;
    console.log("form", form);
    for (const [key, value] of Object.entries(error.fields)) {
      const field = form.querySelector(`[name="${key}"]`);
      console.log("field", field, "key", key, "error", value);
    }
  }
});
