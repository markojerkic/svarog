import { expect, test } from "@playwright/test";

test("scroll and name search keep the selection across result pages", async ({ page }) => {
  const option = (name: string) =>
    `<label><input type="checkbox" name="instance" value="${name}"><span>${name}</span></label>`;
  const first = ["a", "b", "c", "d", "e"];
  const url = "http://logs.test/logs/project/client";
  await page.route(url, (route) =>
    route.fulfill({
      contentType: "text/html",
      body: `<form data-log-instance-form data-options-url="${url}/instances"
          data-unnamed="Unnamed" data-next='{"day":"2026-11-20","name":"e"}'>
        <div data-selected-instances><input type="hidden" name="instance" value=""></div>
        <input type="search"><template>${option("")}</template>
        <div class="log-instance-options" style="height:90px; overflow:auto">
          <div data-instance-rows>${first.map(option).join("")}</div>
          <button data-more type="button">More</button>
        </div>
        <p data-no-instances hidden>No matches</p><p data-instance-error hidden>Error</p>
        <style>label { display:block; height:25px }</style>
      </form>`,
    }),
  );
  await page.route(`${url}/instances?**`, (route) => {
    const q = new URL(route.request().url()).searchParams;
    const names = q.has("afterDay")
      ? [q.get("afterName") === "e" ? "worker" : "wrong cursor"]
      : q.get("q") === "worker"
        ? ["worker"]
        : [];
    return route.fulfill({ json: { names, next: null } });
  });
  await page.goto(url);
  await page.addScriptTag({ path: "internal/server/ui/components/baseui/lifecycle.js" });
  await page.addScriptTag({ path: "internal/server/ui/assets/js/log-instance-picker.js" });
  const checkboxes = page.getByRole("checkbox");
  await expect(checkboxes).toHaveCount(5);
  await page.getByLabel("a", { exact: true }).check();
  await page
    .locator(".log-instance-options")
    .evaluate((list) => (list.scrollTop = list.scrollHeight));
  const worker = page.getByLabel("worker", { exact: true });
  await worker.check();
  await expect(checkboxes).toHaveCount(6);

  const search = page.getByRole("searchbox");
  await search.fill("missing");
  await expect(page.getByText("No matches")).toBeVisible();
  await search.fill("worker");
  await expect(worker).toBeChecked();
  await worker.uncheck();
  const selected = await page
    .locator("form")
    .evaluate((form) => new FormData(form).getAll("instance"));
  expect(selected).toEqual(["", "a"]);
});

test("log colors fetch only newly visible names across live batches", async ({ page }) => {
  const base = "http://logs.test/logs/project/client";
  const batches: string[][] = [];
  await page.route(base, (route) =>
    route.fulfill({
      contentType: "text/html",
      body: `<div id="log-scroll-container" data-instance-hues-url="${base}/instance-hues">
      <pre data-instance-id="api"></pre><pre data-instance-id="api"></pre></div>`,
    }),
  );
  await page.route(`${base}/instance-hues`, (route) => {
    const names = route.request().postDataJSON() as string[];
    batches.push(names);
    return route.fulfill({
      json: Object.fromEntries(names.map((name) => [name, name === "api" ? 0 : 120])),
    });
  });
  await page.goto(base);
  await page.addScriptTag({ path: "internal/server/ui/components/baseui/lifecycle.js" });
  await page.addScriptTag({ path: "internal/server/ui/assets/js/log-instance-colors.js" });
  const api = page.locator('pre[data-instance-id="api"]').first();
  await expect(api).toHaveAttribute("style", /oklch/);
  const original = await api.getAttribute("style");
  await page.locator("#log-scroll-container").evaluate((list) => {
    list.insertAdjacentHTML(
      "beforeend",
      '<pre data-instance-id="api"></pre><pre data-instance-id="older"></pre>',
    );
  });
  const older = page.locator('pre[data-instance-id="older"]');
  await expect(older).toHaveAttribute("style", /oklch/);
  expect(await older.getAttribute("style")).not.toBe(original);
  expect(await api.getAttribute("style")).toBe(original);
  expect(batches).toEqual([["api"], ["older"]]);
});
