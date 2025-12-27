document.addEventListener("DOMContentLoaded", () => {
  document.body.addEventListener("toast", (event) => {
    const { message, level } = event.detail;
    showToast(message, level);
  });
});

const icons = {
  success: `<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="lucide lucide-check-circle text-green-500"><path d="M22 11.08V12a10 10 0 1 1-5.93-9.14"/><path d="m9 11 3 3L22 4"/></svg>`,
  error: `<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="lucide lucide-circle-x text-destructive"><circle cx="12" cy="12" r="10"/><path d="m15 9-6 6"/><path d="m9 9 6 6"/></svg>`,
  warning: `<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="lucide lucide-triangle-alert text-yellow-500"><path d="m21.73 18-8-14a2 2 0 0 0-3.48 0l-8 14A2 2 0 0 0 4 21h16a2 2 0 0 0 1.73-3Z"/><path d="M12 9v4"/><path d="M12 17h.01"/></svg>`,
  info: `<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="lucide lucide-info text-blue-500"><circle cx="12" cy="12" r="10"/><path d="M12 16v-4"/><path d="M12 8h.01"/></svg>`,
};

function showToast(message, level = "info") {
  const icon = icons[level] || icons.info;

  const borderColors = {
    success: "!border-green-500/40",
    error: "!border-destructive/40",
    warning: "!border-yellow-500/40",
    info: "!border-blue-500/40",
  };

  const borderColor = borderColors[level] || borderColors.info;

  // Construct the HTML content for the toast
  // We use a flex container for the layout
  const content = `
        <div class="flex items-start gap-3 w-full">
            <div class="mt-0.5 shrink-0">
                ${icon}
            </div>
            <div class="grid gap-1">
                <p class="text-sm font-medium leading-none text-foreground text-left">
                    ${message}
                </p>
            </div>
        </div>
    `;

  Toastify({
    text: content,
    duration: 4000,
    gravity: "bottom",
    position: "right",
    stopOnFocus: true,
    close: false, // Sonner usually doesn't have a visible close button by default, acts on click/timeout
    className: `!bg-background !text-foreground !shadow-xl !border ${borderColor} !rounded-lg !p-4 !max-w-[420px] !w-full !flex !items-center !gap-4`,
    style: {
      background: "unset",
      boxShadow: "unset",
      border: "unset",
      padding: "unset",
      maxWidth: "unset",
      minWidth: "300px", // Ensure it has some width
    },
    escapeMarkup: false,
    onClick: function () {}, // Callback after click
  }).showToast();
}
