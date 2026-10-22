const palette = [
  "#3b82f6", // blue
  "#d97706", // orange
  "#a855f7", // purple
  "#0d9488", // teal
  "#db2777", // pink
  "#65a30d", // lime
  "#ef4444", // red
  "#64748b", // slate
];
const observers = new WeakMap();

function paletteFor(clientId) {
  // A client-specific hash gives a reproducible ordering without storing a seed.
  return palette
    .map((color) => {
      let rank = 2166136261;
      for (const char of `${clientId}:${color}`) {
        rank = Math.imul(rank ^ char.charCodeAt(0), 16777619);
      }
      return { color, rank: rank >>> 0 };
    })
    .sort((a, b) => a.rank - b.rank)
    .map(({ color }) => color);
}

window.templ.lifecycle.register("#log-scroll-container", {
  init(container) {
    const clientPalette = paletteFor(container.dataset.logClientId || "");
    const colors = new Map();
    function colorFor(instance) {
      if (!colors.has(instance)) {
        colors.set(instance, clientPalette[colors.size % clientPalette.length]);
      }
      return colors.get(instance);
    }
    // The complete client list keeps filtering and pagination from shifting colors.
    for (const instance of JSON.parse(container.dataset.logInstances || "[]") || []) {
      colorFor(instance);
    }
    function paint(row) {
      if (row instanceof Element && row.matches("pre[data-instance-id]")) {
        row.style.borderLeftColor = colorFor(row.dataset.instanceId);
      }
    }
    container.querySelectorAll("pre[data-instance-id]").forEach(paint);
    const observer = new MutationObserver((records) => {
      for (const record of records) record.addedNodes.forEach(paint);
    });
    observer.observe(container, { childList: true });
    observers.set(container, observer);
  },
  destroy(container) {
    observers.get(container)?.disconnect();
    observers.delete(container);
  },
});
