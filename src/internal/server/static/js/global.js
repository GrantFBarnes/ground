function toggleLoading() {
    let overlayElement = document.getElementById("overlay");
    let loadingWheelElement = document.getElementById("loading-wheel");

    if (overlayElement || loadingWheelElement) {
        overlayElement.remove();
        loadingWheelElement.remove();
    } else {
        overlayElement = document.createElement("div");
        overlayElement.id = "overlay";
        document.body.appendChild(overlayElement);

        loadingWheelElement = document.createElement("div");
        loadingWheelElement.id = "loading-wheel";
        document.body.appendChild(loadingWheelElement);
    }
}

function displayInfoMessage(message) {
    displayMessage("info-message", message);
}

function displayErrorMessage(message) {
    displayMessage("error-message", message);
}

function displayMessage(id, message) {
    let messageBoxElement = document.getElementById(id);
    if (messageBoxElement) {
        messageBoxElement.remove();
    }

    messageBoxElement = document.createElement("div");
    messageBoxElement.id = id;

    const messageCloseElement = document.createElement("span");
    messageCloseElement.className = "close-button";
    messageCloseElement.onclick = () => { messageBoxElement.remove() };
    messageCloseElement.innerHTML = "&#128938;";

    const messageTextElement = document.createElement("span");
    messageTextElement.innerText = message;

    messageBoxElement.appendChild(messageCloseElement);
    messageBoxElement.appendChild(messageTextElement);
    document.body.appendChild(messageBoxElement);
    setTimeout(() => {
        if (messageBoxElement) {
            messageBoxElement.remove();
        }
    }, 5000);
}

function logout() {
    if (confirm("Are you sure you want to logout?")) {
        fetch("/api/logout", { method: "POST" }).then(() => location.reload());
    }
}