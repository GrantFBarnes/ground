package server

import (
	"encoding/json"
	"net/http"
	"os"
	"os/user"
	"path"
	"strings"

	"github.com/grantfbarnes/ground/internal/auth"
)

func login(w http.ResponseWriter, r *http.Request) {
	type bodyStruct struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	var body bodyStruct

	defer r.Body.Close()

	err := json.NewDecoder(r.Body).Decode(&body)
	if err != nil {
		http.Error(w, "Invalid body provided.", http.StatusBadRequest)
		return
	}

	if body.Username == "" {
		http.Error(w, "No username provided.", http.StatusBadRequest)
		return
	}

	if body.Password == "" {
		http.Error(w, "No password provided.", http.StatusBadRequest)
		return
	}

	_, err = user.Lookup(body.Username)
	if err != nil {
		http.Error(w, "User does not exist.", http.StatusBadRequest)
		return
	}

	_, err = os.Stat(path.Join("/home", body.Username))
	if err != nil {
		http.Error(w, "User has no home.", http.StatusNotFound)
		return
	}

	if !auth.CredentialsAreValid(body.Username, body.Password) {
		http.Error(w, "Invalid credentials provided.", http.StatusUnauthorized)
		return
	}

	auth.RemoveUsername(w)
	auth.SetUsername(w, body.Username)
	w.WriteHeader(http.StatusOK)
}

func logout(w http.ResponseWriter, r *http.Request) {
	auth.RemoveUsername(w)
	w.WriteHeader(http.StatusOK)
}

func download(w http.ResponseWriter, r *http.Request) {
	username, err := auth.GetUsername(r)
	if err != nil {
		http.Error(w, "No login credentials found.", http.StatusUnauthorized)
		return
	}

	homePath := strings.TrimPrefix(r.URL.Path, "/api/download")
	fullPath := path.Join("/home", username, homePath)
	fileInfo, err := os.Stat(fullPath)
	if err != nil {
		http.Error(w, "File not found.", http.StatusNotFound)
		return
	}

	if fileInfo.IsDir() {
		http.Error(w, "Cannot download a directory.", http.StatusNotAcceptable)
		return
	}

	_, fileName := path.Split(homePath)
	w.Header().Set("Content-Disposition", "attachment; filename="+fileName)
	w.Header().Set("Content-Type", "application/octet-stream")

	http.ServeFile(w, r, fullPath)
}
