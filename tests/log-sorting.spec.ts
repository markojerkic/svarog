import { test, expect, type Page } from "@playwright/test";
import type { Htmx } from "htmx.org";

test.describe("Log Sorting Logic", () => {
  test.beforeEach(async ({ page }) => {
    await page.setContent(
      '<base href="http://logs.test/"><div id="log-scroll-container"></div><div id="other-container"></div>',
    );
    await page.addScriptTag({ path: "node_modules/htmx.org/dist/htmx.js" });
    await page.addScriptTag({ path: "internal/server/ui/assets/js/log-line-swapping.js" });
  });

  const simulateHtmxSwap = async (page: Page, timestamp: number, sequence?: number) => {
    await page.evaluate(
      ({ ts, seq }) =>
        (window as typeof window & { htmx: Htmx }).htmx.swap({
          text: `<div hx-swap-oob="afterbegin:#log-scroll-container"><pre data-timestamp="${ts}" data-sequence="${seq || 0}">Log at ${ts}</pre></div>
          <div id="other-container" hx-swap-oob="true">Other update ${ts}</div>`,
          sourceElement: document.body,
        }),
      { ts: timestamp, seq: sequence },
    );
  };

  test("handles out-of-order log arrival correctly", async ({ page }) => {
    // 1. Receive First Log (Time: 100)
    await simulateHtmxSwap(page, 100);

    // 2. Receive NEWER Log (Time: 300) -> Should go to TOP
    await simulateHtmxSwap(page, 300);

    // 3. Receive OLDER Log (Time: 50) -> Should go to BOTTOM
    await simulateHtmxSwap(page, 50);

    // 4. Receive MIDDLE Log (Time: 200) -> Should insert BETWEEN 300 and 100
    await simulateHtmxSwap(page, 200);

    // ASSERT: Check the DOM order
    // We expect Descending: 300 -> 200 -> 100 -> 50
    const timestamps = await page
      .locator("#log-scroll-container > pre")
      .evaluateAll((list) => list.map((el) => parseInt(el.getAttribute("data-timestamp")!)));

    console.log("Final DOM Order:", timestamps);
    expect(timestamps).toEqual([300, 200, 100, 50]);
    await expect(page.locator("#other-container")).toHaveText("Other update 200");
  });

  test("sorts by sequence number when timestamps are equal", async ({ page }) => {
    // All logs have same timestamp (100), but different sequence numbers
    await simulateHtmxSwap(page, 100, 2);
    await simulateHtmxSwap(page, 100, 5);
    await simulateHtmxSwap(page, 100, 1);
    await simulateHtmxSwap(page, 100, 4);
    await simulateHtmxSwap(page, 100, 3);

    // ASSERT: Should be sorted by sequence number descending: 5 -> 4 -> 3 -> 2 -> 1
    const sequences = await page
      .locator("#log-scroll-container > pre")
      .evaluateAll((list) => list.map((el) => parseInt(el.getAttribute("data-sequence")!)));

    console.log("Final DOM Order (by sequence):", sequences);
    expect(sequences).toEqual([5, 4, 3, 2, 1]);
  });

  test("merges batches with exact timestamps, stable IDs and loader boundaries", async ({
    page,
  }) => {
    await page.locator("#log-scroll-container").evaluate((container) => {
      container.innerHTML =
        '<pre data-log-id="5" data-timestamp="1770000000000000000" data-sequence="3">existing</pre>' +
        '<pre data-log-id="2" data-timestamp="1769999999999999000" data-sequence="0">oldest</pre>' +
        '<div data-log-page="backward">older loader</div><div data-log-stream></div>';
    });
    await page.evaluate(() =>
      (window as typeof window & { htmx: Htmx }).htmx.swap({
        sourceElement: document.body,
        text: `<div hx-swap-oob="afterbegin:#log-scroll-container">
          <pre data-log-id="5" data-timestamp="1770000000000000000" data-sequence="3">duplicate existing</pre>
          <pre data-log-id="6" data-timestamp="1770000000000000000" data-sequence="3">tie by ID</pre>
          <pre data-log-id="7" data-timestamp="1770000000000000001" data-sequence="0">newer by one ns</pre>
          <pre data-log-id="7" data-timestamp="1770000000000000001" data-sequence="0">duplicate batch</pre>
          <pre data-log-id="1" data-timestamp="1769999999999998000" data-sequence="0">outside loaded window</pre>
        </div>`,
      }),
    );
    await expect(page.locator("#log-scroll-container > pre")).toHaveText([
      "newer by one ns",
      "tie by ID",
      "existing",
      "oldest",
    ]);
    expect(
      await page
        .locator('[data-log-page="backward"]')
        .evaluate((element) => (element.previousElementSibling as HTMLElement)?.dataset.logId),
    ).toBe("2");
  });

  test("bounds live history and keeps pagination at the retained boundary", async ({ page }) => {
    await page.locator("#log-scroll-container").evaluate((container) => {
      container.setAttribute("data-log-live", "");
      container.setAttribute(
        "style",
        "display:flex;flex-direction:column-reverse;overflow:auto;height:120px",
      );
      container.insertAdjacentHTML(
        "afterend",
        `<template id="log-live-older-loader"><div data-log-page="backward" hx-get="/logs/p/c?direction=backward" hx-trigger="click">Older</div></template>`,
      );
    });
    await sendRows(page, 1, 2100);
    await expect(page.locator("#log-scroll-container > pre")).toHaveCount(2000);
    const boundary = await page.locator('[data-log-page="backward"]').evaluate((loader) => ({
      previous: (loader.previousElementSibling as HTMLElement)?.dataset.logId,
      query: loader.getAttribute("hx-get"),
    }));
    expect(boundary.previous).toBe("101");
    expect(new URL(boundary.query!, "http://logs.test").searchParams.get("cursorId")).toBe("101");
    expect(new URL(boundary.query!, "http://logs.test").searchParams.get("cursorTimeUs")).toBe(
      "1770000000000101",
    );
  });

  test("stops live updates instead of evicting the history being read", async ({ page }) => {
    await page.locator("#log-scroll-container").evaluate((container) => {
      container.setAttribute("data-log-live", "");
      container.setAttribute(
        "style",
        "display:flex;flex-direction:column-reverse;overflow:auto;height:120px",
      );
      container.insertAdjacentHTML("beforebegin", '<div id="log-live-notice" hidden>Refresh</div>');
      container.innerHTML = "<div data-log-stream></div>";
    });
    await sendRows(page, 1, 2000);
    await page.locator("#log-scroll-container").evaluate((container) => {
      container.scrollTop = -1000;
    });
    await sendRows(page, 2001, 100);
    await expect(page.locator("#log-scroll-container > pre")).toHaveCount(2000);
    await expect(page.locator('pre[data-log-id="1"]')).toBeAttached();
    await expect(page.locator('pre[data-log-id="2100"]')).toHaveCount(0);
    await expect(page.locator("#log-live-notice")).toBeVisible();
    await expect(page.locator("[data-log-stream]")).toHaveCount(0);
  });
});

async function sendRows(page: Page, start: number, count: number) {
  for (let offset = 0; offset < count; offset += 100) {
    await page.evaluate(
      ({ start, count }) => {
        const rows = Array.from({ length: count }, (_, index) => {
          const id = start + index;
          const timestamp = 1770000000000000000n + BigInt(id) * 1000n;
          return `<pre style="height:20px;flex-shrink:0" data-log-id="${id}" data-timestamp="${timestamp}" data-sequence="0">row ${id}</pre>`;
        }).join("");
        return (window as typeof window & { htmx: Htmx }).htmx.swap({
          sourceElement: document.body,
          text: `<div hx-swap-oob="afterbegin:#log-scroll-container">${rows}</div>`,
        });
      },
      { start: start + offset, count: Math.min(100, count - offset) },
    );
  }
}
