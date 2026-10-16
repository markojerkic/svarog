document.addEventListener("toast", (event) => {
  const { message, level } = event.detail;
  showToast(message, level);
});

function showToast(message, level = "info") {
  window.templ.toast.add({ title: message, type: level, timeout: 4000 });
}
