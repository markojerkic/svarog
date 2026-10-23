(() => {
  const textSelector = "[data-tui-tagsinput-text-input]";
  const hiddenSelector = '[data-tui-tagsinput-hidden-inputs] input[type="hidden"]';

  function editable(root) {
    const input = root.querySelector(textSelector);
    return input && !input.disabled && !input.readOnly;
  }

  function addTag(root) {
    if (!editable(root)) return;
    const input = root.querySelector(textSelector);
    const value = input.value.trim();
    if (!value) return;
    const existing = [...root.querySelectorAll(hiddenSelector)];
    if (existing.some((entry) => entry.value.toLowerCase() === value.toLowerCase())) {
      input.value = "";
      return;
    }

    const chip = document.createElement("div");
    chip.setAttribute("data-tui-tagsinput-chip", "");
    chip.className =
      "inline-flex items-center gap-2 rounded-md border px-2.5 py-0.5 text-xs font-semibold border-transparent bg-primary text-primary-foreground";
    // Only the static icon is HTML. User-provided tag names are always text.
    chip.innerHTML = `<span></span><button type="button"
      class="ml-1 text-current hover:text-destructive disabled:opacity-50 disabled:cursor-not-allowed cursor-pointer"
      data-tui-tagsinput-remove><svg xmlns="http://www.w3.org/2000/svg" class="h-3 w-3 pointer-events-none"
      fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
      <path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" /></svg></button>`;
    chip.querySelector("span").textContent = value;
    root.querySelector("[data-tui-tagsinput-container]").appendChild(chip);

    const hidden = document.createElement("input");
    hidden.type = "hidden";
    hidden.name = root.getAttribute("data-tui-tagsinput-name");
    hidden.value = value;
    const form = root.getAttribute("data-tui-tagsinput-form");
    if (form) hidden.setAttribute("form", form);
    root.querySelector("[data-tui-tagsinput-hidden-inputs]").appendChild(hidden);
    input.value = "";
  }

  function removeTag(button) {
    const chip = button.closest("[data-tui-tagsinput-chip]");
    const root = chip?.closest("[data-tui-tagsinput]");
    if (!root || !editable(root)) return;
    const value = chip.querySelector("span").textContent;
    // Do not interpolate names containing quotes/brackets into a CSS selector.
    [...root.querySelectorAll(hiddenSelector)].find((entry) => entry.value === value)?.remove();
    chip.remove();
  }

  document.addEventListener("keydown", (event) => {
    const input = event.target.closest(textSelector);
    const root = input?.closest("[data-tui-tagsinput]");
    if (!root || !editable(root) || event.isComposing) return;
    if (event.key === "Enter" || event.key === ",") {
      event.preventDefault();
      addTag(root);
    } else if (event.key === "Backspace" && input.value === "") {
      const last = root.querySelector(
        "[data-tui-tagsinput-chip]:last-child [data-tui-tagsinput-remove]",
      );
      if (last && !last.disabled) {
        event.preventDefault();
        removeTag(last);
      }
    }
  });

  document.addEventListener("click", (event) => {
    const remove = event.target.closest("[data-tui-tagsinput-remove]");
    if (remove) {
      event.preventDefault();
      event.stopPropagation();
      if (!remove.disabled) removeTag(remove);
      return;
    }
    const root = event.target.closest("[data-tui-tagsinput]");
    if (root && !event.target.closest("input")) root.querySelector(textSelector)?.focus();
  });

  // Capture before HTMX collects form values, including submission from the
  // dialog's external Save button or Enter in another field. Delegation also
  // covers forms replaced by server-side validation responses.
  document.addEventListener(
    "submit",
    (event) => {
      document.querySelectorAll("[data-tui-tagsinput]").forEach((root) => {
        if (root.querySelector(textSelector)?.form === event.target) addTag(root);
      });
    },
    true,
  );

  document.addEventListener("reset", (event) => {
    document.querySelectorAll("[data-tui-tagsinput]").forEach((root) => {
      if (root.querySelector(textSelector)?.form !== event.target) return;
      root.querySelectorAll("[data-tui-tagsinput-chip]").forEach((chip) => chip.remove());
      root.querySelectorAll(hiddenSelector).forEach((input) => input.remove());
      root.querySelector(textSelector).value = "";
    });
  });
})();
