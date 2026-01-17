document.getElementById("login-form").addEventListener("submit", function (event) {
    event.preventDefault();
    const formData = new FormData(this);
    toggleLoading();
    fetch("/api/login", { method: "POST", body: formData }).then((response) => {
        if (response.ok) {
            location.reload();
        } else {
            response.text().then((text) => notifyError(text));
            toggleLoading();
        }
    });
});