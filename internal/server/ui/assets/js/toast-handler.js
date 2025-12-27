// Toast handler for HX-Trigger events from HTMX
// Listens for 'toast' events triggered via HX-Trigger header

document.addEventListener('DOMContentLoaded', () => {
    document.body.addEventListener('toast', (event) => {
        const { message, level } = event.detail;
        showToast(message, level);
    });
});

function showToast(message, level = 'info') {
    const container = getOrCreateContainer();
    const toast = document.createElement('div');
    
    const bgColor = {
        success: 'bg-green-500',
        error: 'bg-red-500',
        warning: 'bg-yellow-500',
        info: 'bg-blue-500'
    }[level] || 'bg-gray-700';

    toast.className = `${bgColor} text-white px-4 py-3 rounded-lg shadow-lg mb-2 flex items-center gap-2 animate-in slide-in-from-right fade-in duration-300`;
    toast.innerHTML = `
        <span class="flex-1">${escapeHtml(message)}</span>
        <button class="opacity-70 hover:opacity-100" onclick="this.parentElement.remove()">
            <svg xmlns="http://www.w3.org/2000/svg" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                <path d="M18 6 6 18"></path>
                <path d="m6 6 12 12"></path>
            </svg>
        </button>
    `;

    container.appendChild(toast);

    setTimeout(() => {
        toast.classList.add('animate-out', 'slide-out-to-right', 'fade-out');
        setTimeout(() => toast.remove(), 300);
    }, 4000);
}

function getOrCreateContainer() {
    let container = document.getElementById('toast-container');
    if (!container) {
        container = document.createElement('div');
        container.id = 'toast-container';
        container.className = 'fixed bottom-4 right-4 z-50 flex flex-col items-end';
        document.body.appendChild(container);
    }
    return container;
}

function escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}
