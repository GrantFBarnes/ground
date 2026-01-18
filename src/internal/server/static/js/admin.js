document.addEventListener("DOMContentLoaded", () => {
    const diskUsageElement = document.getElementById("disk-usage");
    if (!diskUsageElement) return;

    getDirectoryDiskUsage("/home").then((diskUsage) => {
        diskUsageElement.innerText = `Disk Usage: ${diskUsage}`;
    });

    for (const userDiskUsageElement of document.getElementsByClassName("user-disk-usage")) {
        const username = userDiskUsageElement.dataset.username;
        if (!username) continue;
        getDirectoryDiskUsage(`/home/${username}`).then((diskUsage) => {
            userDiskUsageElement.innerText = diskUsage;
        });
    }
});

function systemCall(callMethod) {
    customConfirm(`Are you sure you want to ${callMethod} the system?`).then(confirmed => {
        if (confirmed) {
            toggleLoading();
            const reloadTimeout = setTimeout(() => location.reload(), 5000);
            fetch(`/api/system/${callMethod}`, { method: "POST" }).then((response) => {
                if (response.ok) {
                    // success, wait for timeout reload
                } else {
                    clearTimeout(reloadTimeout);
                    response.text().then((text) => notifyError(text));
                    toggleLoading();
                }
            });
        }
    });
}

document.getElementById("create-user-form").addEventListener("submit", function (event) {
    event.preventDefault();
    const formData = new FormData(this);
    const username = formData.get("username");
    customConfirm(`Are you sure you want to create user '${username}'?`).then(confirmed => {
        if (confirmed) {
            document.getElementById("create-user-dialog").close();
            toggleLoading();
            fetch("/api/user", { method: "POST", body: formData }).then((response) => {
                if (response.ok) {
                    location.href = `/user/${username}`;
                } else {
                    response.text().then((text) => notifyError(text));
                    toggleLoading();
                }
            });
        }
    });
});

function toggleAdmin(selectElement, username) {
    customConfirm(`Are you sure you want to change the admin status for '${username}'?`).then(confirmed => {
        if (confirmed) {
            toggleLoading();
            const formData = new FormData();
            formData.append("username", username);
            fetch("/api/user/toggle-admin", { method: "POST", body: formData }).then((response) => {
                if (!response.ok) {
                    response.text().then((text) => notifyError(text));
                }
                toggleLoading();
            });
        } else {
            selectElement.value = selectElement.value == "yes" ? "no" : "yes";
        }
    });
}

function impersonateUser(username) {
    customConfirm(`Are you sure you want to impersonate user '${username}'?`).then(confirmed => {
        if (confirmed) {
            toggleLoading();
            const formData = new FormData();
            formData.append("username", username);
            fetch("/api/user/impersonate", { method: "POST", body: formData }).then((response) => {
                if (response.ok) {
                    location.reload();
                } else {
                    response.text().then((text) => notifyError(text));
                    toggleLoading();
                }
            });
        }
    });
}