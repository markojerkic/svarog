import { expect, test } from "@playwright/test";
import fs from "node:fs";

const toasterHTML = fs.readFileSync("tests/fixtures/toaster.html", "utf8");

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

test("disposes active toasts when HTMX replaces the body", async ({ page }) => {
  await page.setContent(`<main>First page</main>${toasterHTML}`);
  for (const script of [
    "baseui/lifecycle.js",
    "baseui/tabbable.js",
    "baseui/floating_focus_manager.js",
    "baseui/use_transition_status.js",
    "toast/toast.js",
  ]) {
    await page.addScriptTag({ path: `internal/server/ui/components/${script}` });
  }
  await page.addScriptTag({ path: "internal/server/ui/assets/js/toast-handler.js" });

  const viewport = page.getByRole("region", { name: "Notifications" });
  await page.evaluate(() => (window as any).showToast("Project saved", "success"));
  await expect(viewport.getByRole("heading", { name: "Project saved" })).toBeVisible();
  await viewport.getByRole("heading", { name: "Project saved" }).hover();
  await page.mouse.down();

  await page.evaluate((nextHTML) => {
    const viewport = document.querySelector('[data-slot="toast-viewport"]') as any;
    const provider = viewport._templToastProvider;
    const toast = [...provider.toasts.values()][0] as any;
    const originalRender = provider.render;
    let detachedRenders = 0;
    // Bound the old bug so the failing test reports it instead of starving the browser's event loop.
    provider.render = () => {
      if (!viewport.isConnected && ++detachedRenders > 20) return;
      originalRender();
    };
    let disconnected = 0;
    for (const observer of toast.observers) {
      const disconnect = observer.disconnect.bind(observer);
      observer.disconnect = () => {
        disconnected++;
        disconnect();
      };
    }
    let detachedMutations = 0;
    const mutationObserver = new MutationObserver((records) => {
      if (!viewport.isConnected) {
        detachedMutations += records.reduce((count, record) => count + record.addedNodes.length, 0);
      }
    });
    mutationObserver.observe(toast.content, { childList: true, subtree: true });
    const drag = toast.dragAbortController;
    const body = document.createElement("body");
    body.innerHTML = `<main>Second page</main>${nextHTML}`;
    (window as any).oldToaster = {
      viewport,
      provider,
      drag,
      mutationObserver,
      disconnected: () => disconnected,
      detachedMutations: () => detachedMutations,
    };
    // Queue work before the swap, as resize/content updates can do during navigation.
    provider.render();
    document.body.replaceWith(body);
  }, toasterHTML);

  await expect(page.getByText("Second page")).toBeVisible();
  await page.mouse.up();
  await expect(viewport.getByRole("heading", { name: "Project saved" })).toHaveCount(0);
  expect(
    await page.evaluate(() => {
      const old = (window as any).oldToaster;
      return {
        disconnected: old.disconnected(),
        dragAborted: old.drag.signal.aborted,
        toasts: old.provider.toasts.size,
        timers: old.provider.store.timers.size,
        storedToasts: old.provider.store.state.toasts.length,
        detachedMutations: old.detachedMutations(),
      };
    }),
    "Unmount must disconnect both observers, abort dragging and discard old toasts and timers",
  ).toEqual({
    disconnected: 2,
    dragAborted: true,
    toasts: 0,
    timers: 0,
    storedToasts: 0,
    detachedMutations: 0,
  });
  await page.evaluate(() => (window as any).oldToaster.mutationObserver.disconnect());

  await page.evaluate(() => (window as any).showToast("Next page works", "success"));
  await expect(viewport.getByRole("heading", { name: "Next page works" })).toBeVisible();
  await viewport.hover();
  await page.getByRole("button", { name: "Close toast" }).click();
  await expect(viewport.getByRole("heading", { name: "Next page works" })).toHaveCount(0);
});
