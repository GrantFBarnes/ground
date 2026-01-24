document.addEventListener("DOMContentLoaded", () => {
    const diskUsageElement = document.getElementById("disk-usage");
    if (!diskUsageElement) return;

    getDirectoryDiskUsage(pageRootPath).then((diskUsage) => {
        diskUsageElement.innerText = `Disk Usage: ${diskUsage}`;
    });
});

const selectedClassName = "highlighted-extra";
let selectedRow = null;

function selectRow(element) {
    if (selectedRow) {
        selectedRow.classList.remove(selectedClassName);
    }
    selectedRow = element;
    selectedRow.classList.add(selectedClassName);
}

function emptyTrash() {
    customConfirm("Are you sure you want to empty the trash?\nThis is permanent and cannot be undone.").then(confirmed => {
        if (confirmed) {
            toggleLoading();
            fetch("/api/trash", { method: "DELETE" }).then((response) => {
                if (response.ok) {
                    location.href = "/trash";
                } else {
                    response.text().then((text) => notifyError(text));
                    toggleLoading();
                }
            });
        }
    });
}

function restoreDir(trashDirName, trashedOn) {
    customConfirm(`Are you sure you want to restore all files/directories trashed on ${trashedOn}?`).then(confirmed => {
        if (confirmed) {
            toggleLoading();
            const formData = new FormData();
            formData.append("trashDirName", trashDirName);
            fetch("/api/restore", { method: "POST", body: formData }).then((response) => {
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