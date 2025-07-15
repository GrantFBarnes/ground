function toggleLoader() {
    const loaderElement = document.getElementById("loader");
    if (loaderElement) {
        loaderElement.remove();
    } else {
        const newLoaderElement = document.createElement("div");
        newLoaderElement.id = "loader";
        newLoaderElement.className = "loader";
        document.body.appendChild(newLoaderElement);
    }
}

function logout() {
    if (confirm("Are you sure you want to logout?")) {
        fetch("/api/logout", { method: "POST" }).then(() => location.reload());
    }
}