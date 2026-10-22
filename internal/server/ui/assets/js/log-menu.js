(function () {
  let currentLogId = null;
  let currentLogContent = null;
  let currentInstanceId = null;
  let menu = null;

  function contextURL() {
    const url = new URL(window.location.pathname, window.location.origin);
    url.searchParams.set("logLine", currentLogId);
    url.searchParams.set("instance", currentInstanceId);
    return url;
  }

  function positionMenu(trigger, point) {
    if (!menu) return;

    const menuRect = menu.getBoundingClientRect();
    let left;
    let top;

    if (point) {
      left = point.x;
      top = point.y;
      if (left + menuRect.width > window.innerWidth - 8) {
        left = window.innerWidth - menuRect.width - 8;
      }
      if (top + menuRect.height > window.innerHeight - 8) {
        top = window.innerHeight - menuRect.height - 8;
      }
      if (left < 8) left = 8;
      if (top < 8) top = 8;
    } else {
      const rect = trigger.getBoundingClientRect();
      left = rect.left - menuRect.width - 4;
      top = rect.bottom - menuRect.height;

      if (left < 8) {
        left = rect.right + 4;
      }

      if (top < 8) {
        top = rect.top;
      }

      if (top + menuRect.height > window.innerHeight - 8) {
        top = window.innerHeight - menuRect.height - 8;
      }
    }

    menu.style.left = `${left}px`;
    menu.style.top = `${top}px`;
  }

  function showMenu(trigger, point) {
    menu = document.getElementById("shared-log-menu");
    if (!menu) return;

    currentLogId = trigger.getAttribute("data-log-id");
    currentLogContent = trigger.getAttribute("data-log-content");
    currentInstanceId = trigger.getAttribute("data-instance-id");

    menu.querySelector("#log-context-link").href = contextURL().href;
    const filterURL = new URL(window.location.pathname, window.location.origin);
    const search = new URLSearchParams(window.location.search).get("search")?.trim();
    if (search) filterURL.searchParams.set("search", search);
    filterURL.searchParams.set("instance", currentInstanceId);
    menu.querySelector("#filter-instance-link").href = filterURL.href;

    menu.style.display = "block";
    menu.classList.remove("hidden");

    requestAnimationFrame(() => positionMenu(trigger, point));
  }

  function hideMenu() {
    if (menu) {
      menu.style.display = "none";
      menu.classList.add("hidden");
    }
  }

  document.addEventListener("click", function (e) {
    const trigger = e.target.closest("[data-log-menu-trigger]");

    if (trigger) {
      e.preventDefault();
      e.stopPropagation();

      const wasVisible = menu && menu.isConnected && menu.style.display === "block";
      hideMenu();

      if (!wasVisible) {
        showMenu(trigger);
      }
      return;
    }

    if (menu && (!menu.contains(e.target) || e.target.closest("a"))) {
      hideMenu();
    }
  });

  document.addEventListener("contextmenu", function (e) {
    const el = e.target instanceof Element ? e.target : e.target.parentElement;
    if (!el) return;
    if (menu && menu.contains(el)) return;

    const trigger =
      el.closest("[data-log-menu-trigger]") ||
      el.closest("pre")?.querySelector("[data-log-menu-trigger]");
    if (!trigger) {
      hideMenu();
      return;
    }

    e.preventDefault();
    showMenu(trigger, { x: e.clientX, y: e.clientY });
  });

  document.addEventListener("keydown", function (e) {
    if (e.key === "Escape") {
      hideMenu();
    }
  });

  window.logMenu = {
    getCurrentLogId: function () {
      return currentLogId;
    },
    getCurrentLogContent: function () {
      return currentLogContent;
    },
    getCurrentInstanceId: function () {
      return currentInstanceId;
    },
    copyLogContent: function () {
      if (currentLogContent) {
        navigator.clipboard.writeText(currentLogContent);
        hideMenu();
      }
    },
    copyLogLink: async function () {
      if (currentLogId) {
        try {
          await navigator.clipboard.writeText(contextURL().href);
          hideMenu();
          showToast(
            window.svarogI18n?.("logs.link_copied", "Copied log line link to clipboard") ||
              "Copied log line link to clipboard",
          );
        } catch {
          showToast(
            window.svarogI18n?.("logs.copy_failed", "Could not copy log link. Try again.") ||
              "Could not copy log link. Try again.",
            "error",
          );
        }
      }
    },
  };
})();
