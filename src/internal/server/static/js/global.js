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

function notifyInfo(message, keepOpen = false) {
    displayNotification("notification-info", message, keepOpen);
}

function notifyError(message, keepOpen = false) {
    displayNotification("notification-error", message, keepOpen);
}

function displayNotification(id, message, keepOpen) {
    let notificationBoxElement = document.getElementById(id);
    if (notificationBoxElement) {
        notificationBoxElement.remove();
    }

    notificationBoxElement = document.createElement("div");
    notificationBoxElement.id = id;
    notificationBoxElement.className = "notification";

    const notificationCloseElement = document.createElement("span");
    notificationCloseElement.className = "close-button";
    notificationCloseElement.onclick = () => { notificationBoxElement.remove() };
    notificationCloseElement.innerHTML = "&#128938;";

    const notificationMessageElement = document.createElement("span");
    notificationMessageElement.innerText = message;

    notificationBoxElement.appendChild(notificationCloseElement);
    notificationBoxElement.appendChild(notificationMessageElement);
    document.body.appendChild(notificationBoxElement);

    if (!keepOpen) {
        setTimeout(() => {
            if (notificationBoxElement) {
                notificationBoxElement.remove();
            }
        }, 5000);
    }
}

function customConfirm(message) {
    return new Promise((resolve) => {
        const dialogElement = document.getElementById("confirm-dialog");
        const messageElement = document.getElementById("confirm-message");
        const okElement = document.getElementById("confirm-ok");
        const cancelElement = document.getElementById("confirm-cancel");

        messageElement.textContent = message;

        okElement.onclick = () => {
            resolve(true);
            dialogElement.close();
        }

        cancelElement.onclick = () => {
            resolve(false);
            dialogElement.close();
        }

        dialogElement.showModal();
    });
}

function confirmLogout() {
    customConfirm("Are you sure you want to logout?").then(confirmed => {
        if (confirmed) {
            logout();
        }
    });
}

function logout() {
    fetch("/api/logout", { method: "POST" }).then(() => location.reload());
}