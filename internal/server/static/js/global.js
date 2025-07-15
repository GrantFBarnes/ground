function logout() {
    if (confirm("Are you sure you want to logout?")) {
        callPostAPI("/api/logout", null).then(() => location.reload());
    }
}

async function callPostAPI(url, body) {
    try {
        const response = await fetch(url, { method: "POST", body: body });
        const text = await response.text();
        return { status: response.status, data: text };
    } catch (error) {
        return { status: "error", data: error };
    }
}

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