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

  const simulateHtmxSwap = async (
    page: Page,
    timestamp: number,
    sequence?: number,
  ) => {
    await page.evaluate(
      ({ ts, seq }) => {
        const frag = document.createDocumentFragment();
        const el = document.createElement("pre");
        el.setAttribute("data-timestamp", String(ts));
        if (seq !== undefined) {
          el.setAttribute("data-sequence", String(seq));
        }
        el.innerText = `Log at ${ts}${seq !== undefined ? ` seq ${seq}` : ""}`;
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
        const fakeEvt = new CustomEvent("htmx:oobBeforeSwap", {
          bubbles: true,
          cancelable: true,
          detail: {
            target: { id: "some-other-container" },
            fragment: frag,
          },
        });
        document.body.dispatchEvent(fakeEvt);
      },
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
      .locator("#logs-container > pre")
      .evaluateAll((list) =>
        list.map((el) => parseInt(el.getAttribute("data-timestamp")!)),
      );

    console.log("Final DOM Order:", timestamps);
    expect(timestamps).toEqual([300, 200, 100, 50]);
  });

  test("sorts by sequence number when timestamps are equal", async ({
    page,
  }) => {
    // All logs have same timestamp (100), but different sequence numbers
    await simulateHtmxSwap(page, 100, 2);
    await simulateHtmxSwap(page, 100, 5);
    await simulateHtmxSwap(page, 100, 1);
    await simulateHtmxSwap(page, 100, 4);
    await simulateHtmxSwap(page, 100, 3);

    // ASSERT: Should be sorted by sequence number descending: 5 -> 4 -> 3 -> 2 -> 1
    const sequences = await page
      .locator("#logs-container > pre")
      .evaluateAll((list) =>
        list.map((el) => parseInt(el.getAttribute("data-sequence")!)),
      );

    console.log("Final DOM Order (by sequence):", sequences);
    expect(sequences).toEqual([5, 4, 3, 2, 1]);
  });
});
