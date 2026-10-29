// Private pages can enter the back-forward cache even with Cache-Control: no-store.
// Clear the snapshot before it freezes, and reload on restore so a dead session
// cannot show logs, admin chrome, or connection strings.
window.addEventListener("pagehide", (event) => {
  if (event.persisted === true) {
    document.title = "";
    document.documentElement.replaceChildren();
  }
});

window.addEventListener("pageshow", (event) => {
  if (event.persisted === true) {
    location.reload();
  }
});

// HX-Redirect is a full navigation. Without this, a boosted request can swap
// the login document into the previous private URL.
document.addEventListener("htmx:before:response", (event) => {
  const redirect = event.detail.ctx.response.headers.get("HX-Redirect");
  if (!redirect) return;
  event.preventDefault();
  location.assign(redirect);
});
