document.getElementById("change-password-form").addEventListener("submit", function (event) {
    event.preventDefault();
    const formData = new FormData(this);
    toggleLoading();
    fetch("/api/user/password/change", { method: "POST", body: formData }).then((response) => {
        if (response.ok) {
            if (currentUsername == targetUsername) {
                logout();
            } else {
                toggleLoading();
                notifyInfo("Password changed successfully.");
            }
        } else {
            response.text().then((text) => notifyError(text));
            toggleLoading();
        }
    });
});

document.getElementById("add-ssh-key-form").addEventListener("submit", function (event) {
    event.preventDefault();
    const formData = new FormData(this);
    customConfirm("Are you sure you want to add this SSH Key?").then(confirmed => {
        if (confirmed) {
            document.getElementById("add-ssh-key-dialog").close();
            toggleLoading();
            fetch("/api/user/ssh-key", { method: "POST", body: formData }).then((response) => {
                if (response.ok) {
                    location.reload();
                } else {
                    response.text().then((text) => notifyError(text));
                    toggleLoading();
                }
            });
        }
    });
});

function deleteSshKeyLine(index) {
    customConfirm("Are you sure you want to delete this SSH Key?").then(confirmed => {
        if (confirmed) {
            toggleLoading();
            const formData = new FormData();
            formData.append("username", targetUsername);
            formData.append("index", index);
            fetch("/api/user/ssh-key", { method: "DELETE", body: formData }).then((response) => {
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

function resetPassword() {
    customConfirm(`Are you sure you want to reset this user's password?`).then(confirmed => {
        if (confirmed) {
            toggleLoading();
            const formData = new FormData();
            formData.append("username", targetUsername);
            fetch("/api/user/password/reset", { method: "POST", body: formData }).then((response) => {
                if (response.ok) {
                    if (currentUsername == targetUsername) {
                        logout();
                    } else {
                        toggleLoading();
                        notifyInfo("Password reset successfully.");
                    }
                } else {
                    response.text().then((text) => notifyError(text));
                    toggleLoading();
                }
            });
        }
    });
}

function deleteUser() {
    customConfirm(`Are you sure you want to delete this user?`).then(confirmed => {
        if (confirmed) {
            toggleLoading();
            const formData = new FormData();
            formData.append("username", targetUsername);
            fetch("/api/user", { method: "DELETE", body: formData }).then((response) => {
                if (response.ok) {
                    if (currentUsername == targetUsername) {
                        logout();
                    } else {
                        location.href = "/admin";
                    }
                } else {
                    response.text().then((text) => notifyError(text));
                    toggleLoading();
                }
            });
        }
    });
}