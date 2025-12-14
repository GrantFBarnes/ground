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

function displayNotification(message) {
    let notificationElement = document.getElementById("notification");
    if (notificationElement) {
        notificationElement.remove();
    }

    notificationElement = document.createElement("div");
    notificationElement.id = "notification";
    notificationElement.innerText = message;
    document.body.appendChild(notificationElement);
    setTimeout(() => notificationElement.remove(), 5000);
}

function logout() {
    if (confirm("Are you sure you want to logout?")) {
        fetch("/api/logout", { method: "POST" }).then(() => location.reload());
    }
}