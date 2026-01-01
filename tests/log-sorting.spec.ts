import { test, expect, type Page } from "@playwright/test";
import fs from "node:fs";
import path from "node:path";

const scriptPath = path.join(
  process.cwd(),
  "internal/server/ui/assets/js/log-line-swapping.js",
);
const clientScript = fs.readFileSync(scriptPath, "utf8");

test.describe("Log Sorting Logic", () => {
  test.beforeEach(async ({ page }) => {
    await page.setContent(`
      <div id="logs-container"></div>
      <script>${clientScript}</script>
    `);
  });

  const simulateHtmxSwap = async (page: Page, timestamp: number) => {
    await page.evaluate((ts) => {
      const frag = document.createDocumentFragment();
      const el = document.createElement("pre");
      el.setAttribute("data-timestamp", String(ts));
      el.innerText = `Log at ${ts}`;
      frag.appendChild(el);

      const evt = new CustomEvent("htmx:oobBeforeSwap", {
        bubbles: true,
        cancelable: true,
        detail: {
          target: { id: "logs-container" },
          fragment: frag,
        },
      });
      document.body.dispatchEvent(evt);
    }, timestamp);
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
      .locator("#logs-container > pre")
      .evaluateAll((list) =>
        list.map((el) => parseInt(el.getAttribute("data-timestamp")!)),
      );

    console.log("Final DOM Order:", timestamps);
    expect(timestamps).toEqual([300, 200, 100, 50]);
  });
});
