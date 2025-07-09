package server

import (
	"encoding/json"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/grantfbarnes/ground/internal/auth"
)

func download(w http.ResponseWriter, r *http.Request) {
	if !auth.IsLoggedIn(r) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte("Not logged in."))
		return
	}

	filePath := strings.TrimPrefix(r.URL.Path, "/api/download")
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("Provided path not found."))
		return
	}

	if fileInfo.IsDir() {
		w.WriteHeader(http.StatusNotAcceptable)
		_, _ = w.Write([]byte("Provided path is a directory."))
		return
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", "attachment; filename="+strconv.Quote(fileInfo.Name()))

	http.ServeFile(w, r, filePath)
}

func login(w http.ResponseWriter, r *http.Request) {
	type bodyStruct struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	var body bodyStruct

	defer r.Body.Close()

	err := json.NewDecoder(r.Body).Decode(&body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("Invalid body provided."))
		return
	}

	if body.Username == "" {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("No username provided."))
		return
	}

	if body.Password == "" {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("No password provided."))
		return
	}

	if !auth.CredentialsAreValid(body.Username, body.Password) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte("Invalid login credentials provided."))
		return
	}

	auth.SetUsername(w, body.Username)

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("Login credentials valid."))
}
