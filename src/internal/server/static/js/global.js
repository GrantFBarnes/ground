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
    let messageElement = document.getElementById(id);
    if (messageElement) {
        messageElement.remove();
    }

    messageElement = document.createElement("div");
    messageElement.id = id;
    messageElement.innerText = message;
    document.body.appendChild(messageElement);
    setTimeout(() => messageElement.remove(), 5000);
}

function logout() {
    if (confirm("Are you sure you want to logout?")) {
        fetch("/api/logout", { method: "POST" }).then(() => location.reload());
    }
}