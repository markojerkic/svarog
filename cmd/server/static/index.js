function scrollDown() {
    /** @type {HTMLDivElement | null} */
    const messagesDiv = document.querySelector("#logs-container")
    if (!messagesDiv) {
        return
    }
    messagesDiv.scrollTop = messagesDiv.scrollHeight;
}

scrollDown()
