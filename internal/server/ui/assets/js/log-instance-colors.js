function hash(value) {
  let rank = 2166136261;
  for (const char of String(value)) {
    rank = Math.imul(rank ^ char.charCodeAt(0), 16777619);
  }
  return rank >>> 0;
}

// Mid lightness and chroma stay readable on the white card and on the dark card.
function paint(hue) {
  const wrapped = ((hue % 360) + 360) % 360;
  return `oklch(0.6 0.16 ${wrapped.toFixed(1)})`;
}

// The next live instance sits in the middle of the largest open arc.
function widestHue(hues) {
  const sorted = [...hues].sort((a, b) => a - b);
  let bestGap = -1;
  let best = 0;
  for (let i = 0; i < sorted.length; i++) {
    const start = sorted[i];
    const end = sorted[(i + 1) % sorted.length];
    const gap = (end - start + 360) % 360 || 360;
    if (gap > bestGap) {
      bestGap = gap;
      best = (start + gap / 2) % 360;
    }
  }
  return best;
}

const observers = new WeakMap();

window.templ.lifecycle.register("#log-scroll-container", {
  init(container) {
    // The full client list is the set being colored. Even steps around the
    // wheel keep every pair apart; a short palette folded by list index made
    // unrelated names identical and put warm hues next to each other.
    const offset = hash(container.dataset.logClientId || "") % 360;
    let names = [];
    try {
      const parsed = JSON.parse(container.dataset.logInstances || "[]");
      if (Array.isArray(parsed)) names = parsed;
    } catch {
      names = [];
    }
    const seen = new Set();
    const unique = [];
    for (const name of names) {
      if (typeof name === "string" && !seen.has(name)) {
        seen.add(name);
        unique.push(name);
      }
    }
    const hues = new Map();
    const step = unique.length > 0 ? 360 / unique.length : 0;
    const taken = unique.map((name, index) => {
      const hue = (offset + index * step) % 360;
      hues.set(name, hue);
      return hue;
    });
    function colorFor(instance) {
      if (!hues.has(instance)) {
        const hue = taken.length === 0 ? (offset + hash(instance)) % 360 : widestHue(taken);
        hues.set(instance, hue);
        taken.push(hue);
      }
      return paint(hues.get(instance));
    }
    function paintRow(row) {
      if (row instanceof Element && row.matches("pre[data-instance-id]")) {
        row.style.borderLeftColor = colorFor(row.dataset.instanceId);
      }
    }
    container.querySelectorAll("pre[data-instance-id]").forEach(paintRow);
    const observer = new MutationObserver((records) => {
      for (const record of records) record.addedNodes.forEach(paintRow);
    });
    observer.observe(container, { childList: true });
    observers.set(container, observer);
  },
  destroy(container) {
    observers.get(container)?.disconnect();
    observers.delete(container);
  },
});
