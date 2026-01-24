document.addEventListener("DOMContentLoaded", () => {
    const diskUsageElement = document.getElementById("disk-usage");
    if (!diskUsageElement) return;

    getDirectoryDiskUsage(pageRootPath).then((diskUsage) => {
        diskUsageElement.innerText = `Disk Usage: ${diskUsage}`;
    });
});

const selectedClassName = "highlighted-extra";
const hoverClassName = "highlighted-normal";
const tableContainerElement = document.getElementById("directory-entries-table-container");
let selectedRow = null;

function handleRowDragStart(event) {
    event.target.classList.add(selectedClassName);
    selectedRow = event.target;
}

function handleRowDragEnd(event) {
    event.target.classList.remove(selectedClassName);
    selectedRow = null;
}

function handleDirRowDragOver(event) {
    event.preventDefault();
    event.stopPropagation();
    tableContainerElement.classList.remove(hoverClassName);
    const row = event.target.closest("tr");
    if (row) row.classList.add(hoverClassName);
}

function handleDirRowDragLeave(event) {
    event.preventDefault();
    event.stopPropagation();
    const row = event.target.closest("tr");
    if (row) row.classList.remove(hoverClassName);
}

function handleDirRowDrop(event) {
    event.preventDefault();
    event.stopPropagation();
    const droppedRowElement = event.target.closest("tr");
    if (!droppedRowElement) return;
    droppedRowElement.classList.remove(hoverClassName);

    const dirName = droppedRowElement.dataset.name;
    if (selectedRow) {
        const selectedRowName = selectedRow.dataset.name;
        if (selectedRowName != dirName) {
            const source = pathJoin(pagePath, selectedRowName);
            const destination = pathJoin(pathJoin(pagePath, dirName), selectedRowName);
            moveFiles(source, destination);
        }
    } else if (event.dataTransfer.items) {
        const relHomePath = pathJoin(pagePath, dirName);
        uploadItems(relHomePath, event.dataTransfer.items);
    }
}

function handleTableContainerDragOver(event) {
    event.preventDefault();
    if (!selectedRow) {
        tableContainerElement.classList.add(hoverClassName);
    }
}

function handleTableContainerDragLeave(event) {
    event.preventDefault();
    if (!selectedRow) {
        tableContainerElement.classList.remove(hoverClassName);
    }
}

function handleTableContainerDrop(event) {
    event.preventDefault();
    if (!selectedRow) {
        tableContainerElement.classList.remove(hoverClassName);
        if (event.dataTransfer.items) {
            uploadItems(pagePath, event.dataTransfer.items);
        }
    }
}

function handleBreadcrumbDragOver(event) {
    event.preventDefault();
    const span = event.target.closest("span");
    if (span) span.classList.add(hoverClassName);
}

function handleBreadcrumbDragLeave(event) {
    event.preventDefault();
    const span = event.target.closest("span");
    if (span) span.classList.remove(hoverClassName);
}

function handleBreadcrumbDrop(event) {
    event.preventDefault();
    const droppedSpanElement = event.target.closest("span");
    if (!droppedSpanElement) return;
    droppedSpanElement.classList.remove(hoverClassName);

    const relHomePath = droppedSpanElement.dataset.path;
    if (selectedRow) {
        const selectedRowName = selectedRow.dataset.name;
        const source = pathJoin(pagePath, selectedRowName);
        const destination = pathJoin(relHomePath, selectedRowName);
        moveFiles(source, destination);
    } else if (event.dataTransfer.items) {
        uploadItems(relHomePath, event.dataTransfer.items);
    }
}

function uploadItems(relHomePath, items) {
    customConfirm(`Are you sure you want to upload this file/directory to ${relHomePath}?`).then(confirmed => {
        if (confirmed) {
            uploadDroppedFiles(relHomePath, items);
        }
    });
}

async function uploadDroppedFiles(relHomePath, items) {
    let files = [];
    for (const item of items) {
        const entry = item.webkitGetAsEntry();
        if (!entry) continue;
        files = files.concat(await traverseFileTree(entry));
    }
    upload(relHomePath, files);
}

function traverseFileTree(entry, path = "") {
    return new Promise((resolve, reject) => {
        if (entry.isFile) {
            entry.file((file) => {
                file.relativePath = path + file.name;
                resolve([file]);
            });
        } else if (entry.isDirectory) {
            const subDirReader = entry.createReader();
            subDirReader.readEntries(async (subEntries) => {
                let files = [];
                for (const subEntry of subEntries) {
                    files = files.concat(await traverseFileTree(subEntry, path + entry.name + "/"));
                }
                resolve(files);
            });
        } else {
            reject("entry not valid");
        }
    });
}

const formElement = document.getElementById("upload-form");

formElement.addEventListener("submit", (event) => {
    event.preventDefault();

    const fileUploadElement = document.getElementById("file-upload");
    if (!fileUploadElement) {
        notifyError("Failed to find file upload.");
        return;
    }

    const directoryUploadElement = document.getElementById("directory-upload");
    if (!directoryUploadElement) {
        notifyError("Failed to find directory upload.");
        return;
    }

    let files = [];
    for (const file of fileUploadElement.files) {
        files.push(file);
    }
    for (const file of directoryUploadElement.files) {
        files.push(file);
    }

    upload(pagePath, files);
});

function upload(relHomePath, files) {
    if (files.length === 0) {
        notifyError("No files available to upload.");
        return;
    }

    toggleLoading();

    let uploadCount = 0;
    let failedFiles = [];
    const uploadPromises = files.map(file => {
        const formData = new FormData();
        formData.append("file", file);
        return fetch(`/api/upload${relHomePath}`, { method: "POST", body: formData })
            .then((response) => {
                if (response.ok) {
                    uploadCount += 1;
                    notifyInfo(`Uploaded ${uploadCount} out of ${files.length} files...`);
                } else {
                    failedFiles.push(file.webkitRelativePath ?? file.name);
                }
            });
    });

    Promise.all(uploadPromises)
        .finally(() => {
            if (failedFiles.length) {
                failedFiles = failedFiles.sort();
                notifyError("The following files failed to upload:\n" + failedFiles.join("\n"), true);
                toggleLoading();
            } else {
                location.reload(true);
            }
        });
}

document.getElementById("create-directory-form").addEventListener("submit", function (event) {
    event.preventDefault();
    const formData = new FormData(this);
    const dirName = formData.get("dirName");
    customConfirm(`Are you sure you want to create directory '${dirName}'?`).then(confirmed => {
        if (confirmed) {
            document.getElementById("create-directory-dialog").close();
            toggleLoading();
            fetch("/api/directory", { method: "POST", body: formData }).then((response) => {
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

function moveFiles(source, destination) {
    customConfirm(`Are you sure you want to move '${source}' to '${destination}'?`).then(confirmed => {
        if (confirmed) {
            toggleLoading();
            const formData = new FormData();
            formData.append("sourceRelHomePath", source);
            formData.append("destinationRelHomePath", destination);
            fetch("/api/move", { method: "POST", body: formData }).then((response) => {
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

function compressDirectory(relHomePath) {
    customConfirm("Are you sure you want to compress this directory?").then(confirmed => {
        if (confirmed) {
            callFileApi("compress", relHomePath);
        }
    });
}

function extractFile(relHomePath) {
    customConfirm("Are you sure you want to extract this file?").then(confirmed => {
        if (confirmed) {
            callFileApi("extract", relHomePath);
        }
    });
}

function downloadFile(filePath) {
    const a = document.createElement("a");
    a.href = "/api/download" + filePath;
    a.download = true;
    a.click();
    a.remove();
}

function moveToTrash(relHomePath) {
    customConfirm("Are you sure you want to move this file/directory to the trash?").then(confirmed => {
        if (confirmed) {
            callFileApi("trash", relHomePath);
        }
    });
}

function pathJoin(left, right) {
    if (!left.endsWith("/")) {
        left += "/";
    }
    if (right.startsWith("/")) {
        right = right.slice(1);
    }
    return left + right;
}