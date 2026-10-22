import { expect, test } from "@playwright/test";

const menuHTML = `
  <button data-log-menu-trigger data-log-id="42" data-log-content="message" data-instance-id="api 01 &amp; backup">Log actions</button>
  <div id="shared-log-menu" class="hidden" hx-target:inherited="body" hx-swap:inherited="innerHTML" hx-sync:inherited="body:replace">
    <button onclick="logMenu.copyLogLink()">Copy link</button>
    <a id="log-context-link" href="">Open context</a>
    <a id="filter-instance-link" href="">Show only logs of this instance</a>
  </div>`;
const baseURL = "http://svarog.test/logs/project/client";
const contextURL = `${baseURL}?logLine=42&instance=api+01+%26+backup`;

test.beforeEach(async ({ page }) => {
  await page.route("http://svarog.test/**", (route) =>
    route.fulfill({
      contentType: "text/html",
      body: `<html hx-boost:inherited="true"><head><style>.hidden { display: none; }</style></head><body>${menuHTML}</body></html>`,
    }),
  );
  await page.goto(
    `${baseURL}?search=error+%26+timeout&instance=old&instance=other&cursorTimeUs=123&cursorSequenceNumber=2&cursorId=7&direction=backward&logLine=99#old`,
  );
  await page.addScriptTag({
    content: `window.copiedLinks = []; window.toastCalls = [];
      window.showToast = (...args) => window.toastCalls.push(args);
      Object.defineProperty(navigator, "clipboard", { configurable: true, value: {
        writeText: async text => window.copiedLinks.push(text)
      }});`,
  });
  await page.addScriptTag({ path: "node_modules/htmx.org/dist/htmx.js" });
  await page.addScriptTag({ path: "internal/server/ui/assets/js/log-menu.js" });
});

test("log menu navigation uses HTMX to replace the complete view", async ({ page }) => {
  await page.getByRole("button", { name: "Log actions", exact: true }).click();
  const expectedURL = `${baseURL}?search=error+%26+timeout&instance=api+01+%26+backup`;
  const requestPromise = page.waitForRequest(expectedURL);
  await page.getByRole("link", { name: "Show only logs of this instance" }).click();
  const request = await requestPromise;
  expect(request.headers()["hx-request"]).toBe("true");
  expect(request.headers()["hx-request-type"]).toBe("full");
  await expect(page).toHaveURL(expectedURL);
  expect(
    await page.evaluate(() => typeof (window as typeof window & { logMenu: unknown }).logMenu),
  ).toBe("object");
  await page.getByRole("button", { name: "Log actions", exact: true }).click();
  await expect(page.getByRole("link", { name: "Open context", exact: true })).toHaveAttribute(
    "href",
    contextURL,
  );
  const contextRequestPromise = page.waitForRequest(contextURL);
  await page.getByRole("link", { name: "Open context", exact: true }).click();
  const contextRequest = await contextRequestPromise;
  expect(contextRequest.headers()["hx-request-type"]).toBe("full");
  await expect(page).toHaveURL(contextURL);
});

test("log menu preserves search for filtering and copies context for the selected instance", async ({
  page,
}) => {
  await page.getByRole("button", { name: "Log actions", exact: true }).click();
  await expect(page.getByRole("link", { name: "Show only logs of this instance" })).toHaveAttribute(
    "href",
    `${baseURL}?search=error+%26+timeout&instance=api+01+%26+backup`,
  );
  await page.getByRole("button", { name: "Copy link", exact: true }).click();
  await expect(page.locator("#shared-log-menu")).toBeHidden();
  expect(
    await page.evaluate(() => (window as typeof window & { copiedLinks: string[] }).copiedLinks),
  ).toEqual([contextURL]);
});

test("log menu uses the current DOM after an HTMX-style body replacement", async ({ page }) => {
  await page.getByRole("button", { name: "Log actions", exact: true }).click();
  await page.evaluate((html) => {
    history.replaceState(null, "", "/logs/project/client?logLine=42&cursorTime=123");
    document.body.innerHTML = html;
    document.querySelector("[data-log-menu-trigger]")!.setAttribute("data-log-id", "84");
  }, menuHTML);
  await page.getByRole("button", { name: "Log actions", exact: true }).click();
  await expect(page.locator("#shared-log-menu")).toBeVisible();
  await expect(page.getByRole("link", { name: "Open context", exact: true })).toHaveAttribute(
    "href",
    `${baseURL}?logLine=84&instance=api+01+%26+backup`,
  );
  await expect(page.getByRole("link", { name: "Show only logs of this instance" })).toHaveAttribute(
    "href",
    `${baseURL}?instance=api+01+%26+backup`,
  );
});

test("log menu reports clipboard errors without a false success toast", async ({ page }) => {
  await page.evaluate(() => {
    navigator.clipboard.writeText = async () => {
      throw new Error("Clipboard denied");
    };
  });
  await page.getByRole("button", { name: "Log actions", exact: true }).click();
  await page.getByRole("button", { name: "Copy link", exact: true }).click();
  await expect
    .poll(() =>
      page.evaluate(() => (window as typeof window & { toastCalls: string[][] }).toastCalls),
    )
    .toEqual([["Could not copy log link. Try again.", "error"]]);
});
