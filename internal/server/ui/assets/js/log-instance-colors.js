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

const cleanups = new WeakMap();

window.templ.lifecycle.register("#log-scroll-container", {
  init(container) {
    // The server computes positions in the full client palette, but sends
    // only the colors needed to paint the visible logs, including new rows.
    const offset = hash(container.dataset.logClientId || "") % 360;
    const colors = new Map();
    const taken = new Set();
    const pending = new Set();
    const controller = new AbortController();
    let loading = false;

    function paintRow(row) {
      if (!(row instanceof Element) || !row.matches("pre[data-instance-id]")) return;
      const name = row.dataset.instanceId;
      if (colors.has(name)) row.style.borderLeftColor = colors.get(name);
      else pending.add(name);
    }
    async function loadColors() {
      if (loading || !pending.size) return;
      loading = true;
      while (pending.size && !controller.signal.aborted) {
        const names = [...pending].slice(0, 300);
        let palette = {};
        try {
          const response = await fetch(container.dataset.instanceHuesUrl, {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify(names),
            signal: controller.signal,
          });
          if (response.ok) palette = await response.json();
        } catch {
          // The fallback also covers live names not yet found in stored logs.
        }
        if (controller.signal.aborted) return;
        for (const name of names) {
          let hue = Object.hasOwn(palette, name) ? (offset + palette[name]) % 360 : null;
          if (hue === null || taken.has(hue)) {
            hue = taken.size ? widestHue(taken) : (offset + hash(name)) % 360;
          }
          colors.set(name, paint(hue));
          taken.add(hue);
          pending.delete(name);
        }
        container.querySelectorAll("pre[data-instance-id]").forEach(paintRow);
      }
      loading = false;
    }
    container.querySelectorAll("pre[data-instance-id]").forEach(paintRow);
    loadColors();
    const observer = new MutationObserver((records) => {
      for (const record of records) record.addedNodes.forEach(paintRow);
      loadColors();
    });
    observer.observe(container, { childList: true });
    cleanups.set(container, () => {
      controller.abort();
      observer.disconnect();
    });
  },
  destroy(container) {
    cleanups.get(container)?.();
    cleanups.delete(container);
  },
});
