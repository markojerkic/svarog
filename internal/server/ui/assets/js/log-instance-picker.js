const cleanups = new WeakMap();

window.templ.lifecycle.register("[data-log-instance-form]", {
  init(form) {
    const search = form.querySelector('[type="search"]');
    const list = form.querySelector(".log-instance-options");
    const rows = form.querySelector("[data-instance-rows]");
    const more = form.querySelector("[data-more]");
    const empty = form.querySelector("[data-no-instances]");
    const error = form.querySelector("[data-instance-error]");
    const selected = form.querySelector("[data-selected-instances]");
    const template = form.querySelector("template");
    const fields = new Map([...selected.children].map((field) => [field.value, field]));
    let next = JSON.parse(form.dataset.next);
    let request;
    let timer;

    async function load() {
      if (request) return;
      const current = new AbortController();
      request = current;
      const url = new URL(form.dataset.optionsUrl, location.href);
      url.searchParams.set("q", search.value.trim());
      if (next) {
        url.searchParams.set("afterDay", next.day);
        url.searchParams.set("afterName", next.name);
      }
      list.setAttribute("aria-busy", "true");
      error.hidden = true;
      try {
        const response = await fetch(url, { signal: current.signal });
        if (!response.ok) throw new Error(response.statusText);
        const page = await response.json();
        if (current.signal.aborted) return;
        for (const name of page.names) {
          const option = template.content.cloneNode(true);
          const box = option.querySelector("input");
          box.value = name;
          if (fields.has(name)) {
            box.checked = true;
            fields.get(name).remove();
            fields.delete(name);
          }
          option.querySelector("span").textContent = name || form.dataset.unnamed;
          rows.append(option);
        }
        next = page.next;
        more.hidden = !next;
        empty.hidden = rows.childElementCount !== 0;
      } catch {
        if (!current.signal.aborted) {
          error.hidden = false;
          more.hidden = false;
        }
      } finally {
        if (request === current) {
          request = null;
          list.removeAttribute("aria-busy");
        }
      }
    }

    search.addEventListener("input", () => {
      clearTimeout(timer);
      request?.abort();
      request = null;
      next = null;
      // Keep checked names in the form even when their result rows go away.
      for (const box of rows.querySelectorAll("input:checked")) {
        box.type = "hidden";
        selected.append(box);
        fields.set(box.value, box);
      }
      rows.replaceChildren();
      more.hidden = empty.hidden = error.hidden = true;
      list.scrollTop = 0;
      timer = setTimeout(load, 180);
    });
    search.addEventListener("keydown", (event) => {
      if (event.key === "Enter") event.preventDefault();
    });
    list.addEventListener("scroll", () => {
      if (
        next &&
        error.hidden &&
        list.scrollTop > 0 &&
        list.scrollTop + list.clientHeight >= list.scrollHeight - more.offsetHeight
      ) {
        load();
      }
    });
    more.addEventListener("click", load);
    cleanups.set(form, () => {
      clearTimeout(timer);
      request?.abort();
    });
  },
  destroy(form) {
    cleanups.get(form)?.();
    cleanups.delete(form);
  },
});
