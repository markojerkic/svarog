import { expect, test, type Page, type WebSocketRoute } from "@playwright/test";
import type { Htmx } from "htmx.org";

async function liveView(page: Page) {
  const sockets: WebSocketRoute[] = [];
  await page.routeWebSocket("**/ws/p/c?instance=api", (socket) => {
    sockets.push(socket);
  });
  await page.route("http://logs.test/logs/**", (route) =>
    route.fulfill({
      contentType: "text/html",
      body: `<span id="htmx-global-indicator" class="htmx-indicator"><svg width="16" height="16"></svg></span>
           <span id="log-live-status" data-live-label="Live" data-resync-label="Refresh required" data-memory-label="Memory limit" data-paused-label="Paused"></span>
           <button data-log-pause>Pause</button>
           <div id="log-live-notice" hidden><p></p></div>
           <div id="log-scroll-container" data-log-live>
             <pre data-timestamp="1">before drop</pre>
             <div data-log-stream hx-ws:connect="/ws/p/c?instance=api" hx-config="ws.pauseOnBackground:false ws.reconnectMaxAttempts:6"></div>
           </div>`,
    }),
  );
  await page.goto("http://logs.test/logs/p/c?instance=api");
  await page.addStyleTag({ path: "internal/server/ui/assets/css/input.css" });
  await page.addScriptTag({ path: "node_modules/htmx.org/dist/htmx.js" });
  await page.addScriptTag({ path: "internal/server/ui/assets/js/log-line-swapping.js" });
  await page.addScriptTag({ path: "node_modules/htmx.org/dist/ext/hx-ws.js" });
  await page.evaluate(() => (window as typeof window & { htmx: Htmx }).htmx.process(document.body));
  await expect(page.locator("#log-live-status")).toHaveText("Live");
  await page.clock.install({ time: new Date("2026-01-01T00:00:00Z") });
  await page.clock.pauseAt(new Date("2026-01-01T00:01:00Z"));
  return sockets;
}

test("transport loss retries the filtered stream with only the global spinner", async ({ page }) => {
  const sockets = await liveView(page);
  const indicator = page.locator("#htmx-global-indicator");
  const notice = page.locator("#log-live-notice");
  await expect(indicator).toBeHidden();
  await sockets[0]!.close({ code: 1006 });
  await expect(indicator).toBeVisible();
  await expect(page.locator("#log-live-status")).toBeHidden();
  await expect(notice).toBeHidden();
  await page.clock.fastForward(1000);
  await expect(indicator).toBeHidden();
  await expect(page.locator("#log-live-status")).toBeVisible();
  await expect(notice).toBeHidden();
  expect(sockets).toHaveLength(2);
  sockets[1]!.send(
    '<div hx-swap-oob="afterbegin:#log-scroll-container"><pre data-timestamp="2">after reconnect</pre></div>',
  );
  await expect(page.locator("#log-scroll-container > pre")).toHaveText([
    "after reconnect",
    "before drop",
  ]);
});

for (const [code, label] of [
  [4001, "Refresh required"],
  [4002, "Memory limit"],
] as const) {
  test(`server close for ${label} requires manual resume`, async ({ page }) => {
    const sockets = await liveView(page);
    await sockets[0]!.close({ code });
    await expect(page.locator("#log-live-status")).toHaveText(label);
    await expect(page.locator("[data-log-stream]")).toHaveCount(0);
    await page.clock.fastForward(60_000);
    expect(sockets).toHaveLength(1);
  });
}

test("pausing during reconnect cancels the scheduled socket", async ({ page }) => {
  const sockets = await liveView(page);
  await sockets[0]!.close({ code: 1012 });
  await expect(page.locator("#htmx-global-indicator")).toBeVisible();
  await page.getByRole("button", { name: "Pause" }).click();
  await expect(page.locator("#log-live-status")).toHaveText("Paused");
  await expect(page.locator("#htmx-global-indicator")).toBeHidden();
  await expect(page.locator("[data-log-stream]")).toHaveCount(0);
  await page.clock.fastForward(60_000);
  expect(sockets).toHaveLength(1);
});
