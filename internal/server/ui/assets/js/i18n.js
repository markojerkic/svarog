const translations = JSON.parse(document.documentElement.dataset.i18n || "{}");
window.svarogI18n = (key, fallback = key) => translations[key] || fallback;

document.addEventListener("submit", (event) => {
  const form = event.target;
  if (!(form instanceof HTMLFormElement) || !form.matches("[data-language-switcher]")) return;
  const returnTo = form.elements.namedItem("returnTo");
  if (returnTo instanceof HTMLInputElement) {
    returnTo.value = window.location.pathname + window.location.search;
  }
});

document.addEventListener("htmx:before:response", (event) => {
  const headers = event.detail.ctx.response.headers;
  const language = headers.get("Content-Language");
  if (language && language !== document.documentElement.lang && !headers.has("HX-Redirect")) {
    event.preventDefault();
    window.location.reload();
  }
});
