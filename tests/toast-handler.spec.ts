import { expect, test } from "@playwright/test";

test("forwards toast events to shadcn after HTMX replaces the body", async ({ page }) => {
  await page.setContent("<main>Original page</main>");
  await page.addScriptTag({
    content: `window.toastCalls = [];
      window.templ = { toast: { add: options => window.toastCalls.push(options) } };`,
  });
  await page.addScriptTag({
    path: "internal/server/ui/assets/js/toast-handler.js",
  });

  const calls = await page.evaluate(() => {
    const notify = (message: string, level?: string) => {
      document.body.dispatchEvent(
        new CustomEvent("toast", { bubbles: true, detail: { message, level } }),
      );
    };
    notify("Saved", "success");
    document.body.replaceWith(document.createElement("body"));
    notify("<b>Invalid name</b>", "error");
    notify("Copied link");
    return (window as typeof window & { toastCalls: unknown[] }).toastCalls;
  });

  expect(
    calls,
    "Toast events must survive body replacement and preserve plain-text messages",
  ).toEqual([
    { title: "Saved", type: "success", timeout: 4000 },
    { title: "<b>Invalid name</b>", type: "error", timeout: 4000 },
    { title: "Copied link", type: "info", timeout: 4000 },
  ]);
});
