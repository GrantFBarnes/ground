document.addEventListener("DOMContentLoaded", () => {
    const urlParams = new URLSearchParams(window.location.search);
    if (!urlParams) return;

    let sortBy = urlParams.get("sortBy");
    if (!sortBy) return;
    sortBy = sortBy.toLowerCase();

    const options = ["type", "name", "link", "size", "time"];
    if (options.indexOf(sortBy) < 0) return;

    const tableSortIconElement = document.getElementById(`table-sort-icon-${sortBy}`);
    if (!tableSortIconElement) return;

    const imgElement = document.createElement("img");
    if (urlParams.get("sortOrder") == "desc") {
        imgElement.src = "/static/symbols/sort-desc.svg";
    } else {
        imgElement.src = "/static/symbols/sort-asc.svg";
    }
    imgElement.alt = "Sort Icon"
    imgElement.width = 16;
    imgElement.height = 16;
    tableSortIconElement.appendChild(imgElement);
});

function reloadPageWithSortBy(sortBy) {
    const url = new URL(window.location.href);
    const prevSortBy = url.searchParams.get("sortBy");
    const prevSortOrder = url.searchParams.get("sortOrder");
    url.searchParams.set("sortBy", sortBy);
    if (sortBy == prevSortBy) {
        if (prevSortOrder == "desc") {
            url.search = "";
        } else {
            url.searchParams.set("sortOrder", "desc");
        }
    } else {
        url.searchParams.set("sortOrder", "asc");
    }
    window.location.href = url.toString();
}

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
    notificationCloseElement.innerHTML = "&times;";

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

function callFileApi(api, relHomePath) {
    toggleLoading();
    const formData = new FormData();
    formData.append("relHomePath", relHomePath);
    fetch("/api/" + api, { method: "POST", body: formData }).then((response) => {
        if (response.ok) {
            location.reload();
        } else {
            response.text().then((text) => notifyError(text));
            toggleLoading();
        }
    });
}

function getDirectoryDiskUsage(dirPath) {
    return new Promise((resolve, reject) => {
        fetch(`/api/disk-usage${dirPath}`, { method: "GET" })
            .then((response) => {
                if (response.ok) {
                    resolve(response.text());
                } else {
                    reject();
                }
            })
            .catch(() => reject());
    });
}