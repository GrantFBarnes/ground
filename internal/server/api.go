package server

import (
	"encoding/json"
	"net/http"
	"os"
	"os/user"
	"path"

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

	_, err = user.Lookup(body.Username)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("User does not exist."))
		return
	}

	_, err = os.Stat(path.Join("/home", body.Username))
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("User has no home."))
		return
	}

	if !auth.CredentialsAreValid(body.Username, body.Password) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte("Invalid credentials provided."))
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
