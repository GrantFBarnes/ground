package server

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/grantfbarnes/ground/internal/auth"
)

const usernameRequestContextKey requestContextKey = "usernameRequestContextKey"

func middlewareForAPIs(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username, err := auth.GetUsername(r)
		if err == nil {
			r = r.WithContext(context.WithValue(r.Context(), usernameRequestContextKey, username))
		}
		next.ServeHTTP(w, r)
	})
}

func getUsername(r *http.Request) string {
	return r.Context().Value(usernameRequestContextKey).(string)
}

func downloadFile(w http.ResponseWriter, r *http.Request) {
	filePath := strings.TrimPrefix(r.URL.Path, "/download")
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

func checkLogin(w http.ResponseWriter, r *http.Request) {
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
		_, _ = w.Write([]byte("No username found."))
		return
	}

	if body.Password == "" {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("No password found."))
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
