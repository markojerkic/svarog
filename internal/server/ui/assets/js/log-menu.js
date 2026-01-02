(function () {
  let currentLogId = null;
  let currentLogContent = null;
  let currentInstanceId = null;
  let menu = null;

  function positionMenu(trigger) {
    if (!menu) return;

    const rect = trigger.getBoundingClientRect();
    const menuRect = menu.getBoundingClientRect();

    let left = rect.left - menuRect.width - 4;
    let top = rect.bottom - menuRect.height;

    if (left < 8) {
      left = rect.right + 4;
    }

    if (top < 8) {
      top = rect.top;
    }

    if (top + menuRect.height > window.innerHeight - 8) {
      top = window.innerHeight - menuRect.height - 8;
    }

    menu.style.left = `${left}px`;
    menu.style.top = `${top}px`;
  }

  function showMenu(trigger) {
    if (!menu) {
      menu = document.getElementById("shared-log-menu");
    }
    if (!menu) return;

    currentLogId = trigger.getAttribute("data-log-id");
    currentLogContent = trigger.getAttribute("data-log-content");
    currentInstanceId = trigger.getAttribute("data-instance-id");

    menu.style.display = "block";
    menu.classList.remove("hidden");

    requestAnimationFrame(() => positionMenu(trigger));
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

      const wasVisible = menu && menu.style.display === "block";
      hideMenu();

      if (!wasVisible) {
        showMenu(trigger);
      }
      return;
    }

    if (menu && !menu.contains(e.target)) {
      hideMenu();
    }
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
    copyLogLink: function () {
      if (currentLogId) {
        const url = new URL(window.location.href);
        url.searchParams.set("logLine", currentLogId);
        navigator.clipboard.writeText(url.toString());
        hideMenu();
        showToast("Copied log line link to clipboard");
      }
    },
    filterByInstance: function () {
      if (currentInstanceId) {
        const filterButton = document.getElementById("filter-instance-button");
        if (filterButton) {
          filterButton.setAttribute(
            "hx-vals",
            JSON.stringify({ instance: currentInstanceId }),
          );
          filterButton.click();
        }

        hideMenu();
      }
    },
  };
})();
