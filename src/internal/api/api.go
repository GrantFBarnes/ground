package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/grantfbarnes/ground/internal/cookie"
	"github.com/grantfbarnes/ground/internal/filesystem"
	"github.com/grantfbarnes/ground/internal/system"
	"github.com/grantfbarnes/ground/internal/users"
)

const CONTEXT_KEY_USERNAME string = "username"

func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username, err := cookie.GetUsername(r)
		if err != nil {
			cookie.RemoveUsername(w)
			http.Error(w, "No login credentials found.", http.StatusUnauthorized)
			return
		}

		targetUsername := r.PathValue("username")
		if targetUsername != "" {
			err = users.Validate(targetUsername)
			if err != nil {
				http.Error(w, "Username is not valid.", http.StatusBadRequest)
				return
			}
		}

		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), CONTEXT_KEY_USERNAME, username)))
	})
}

func Login(w http.ResponseWriter, r *http.Request) {
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

	err = users.Login(body.Username, body.Password)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	login(w, body.Username)
}

func Logout(w http.ResponseWriter, r *http.Request) {
	cookie.RemoveUsername(w)
	w.WriteHeader(http.StatusOK)
}

func Impersonate(w http.ResponseWriter, r *http.Request) {
	username := r.Context().Value(CONTEXT_KEY_USERNAME).(string)

	if !users.IsAdmin(username) {
		http.Error(w, "Must be admin to impersonate.", http.StatusUnauthorized)
		return
	}

	login(w, r.PathValue("username"))
}

func login(w http.ResponseWriter, username string) {
	filesystem.CreateTrashDirectory(username)
	cookie.RemoveUsername(w)
	cookie.SetUsername(w, username)
	w.WriteHeader(http.StatusOK)
}

func ToggleAdmin(w http.ResponseWriter, r *http.Request) {
	username := r.Context().Value(CONTEXT_KEY_USERNAME).(string)

	if !users.IsAdmin(username) {
		http.Error(w, "Must be admin to change admin status.", http.StatusUnauthorized)
		return
	}

	err := users.ToggleAdmin(r.PathValue("username"))
	if err != nil {
		http.Error(w, "Failed to change admin status.", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func CompressDirectory(w http.ResponseWriter, r *http.Request) {
	username := r.Context().Value(CONTEXT_KEY_USERNAME).(string)
	urlRelativePath := strings.TrimPrefix(r.URL.Path, "/api/compress")
	err := filesystem.CompressDirectory(username, urlRelativePath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func UploadFiles(w http.ResponseWriter, r *http.Request) {
	username := r.Context().Value(CONTEXT_KEY_USERNAME).(string)
	urlRelativePath := strings.TrimPrefix(r.URL.Path, "/api/upload")

	urlRootPath := path.Join("/home", username, urlRelativePath)
	urlPathInfo, err := os.Stat(urlRootPath)
	if err != nil {
		http.Error(w, "Path not found.", http.StatusBadRequest)
		return
	}

	if !urlPathInfo.IsDir() {
		http.Error(w, "Path is not a directory.", http.StatusBadRequest)
		return
	}

	uid, gid, err := users.GetUserIds(username)
	if err != nil {
		http.Error(w, "Failed to get user ids.", http.StatusInternalServerError)
		return
	}

	mediaType, contentParams, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || !strings.HasPrefix(mediaType, "multipart/") {
		http.Error(w, "Failed to parse media type.", http.StatusInternalServerError)
		return
	}

	boundary, ok := contentParams["boundary"]
	if !ok {
		http.Error(w, "Failed to get file boundary.", http.StatusInternalServerError)
		return
	}

	multipartReader := multipart.NewReader(r.Body, boundary)
	for {
		part, err := multipartReader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			http.Error(w, "Failed to get next file part.", http.StatusInternalServerError)
			return
		}

		err = createFileFromPart(part, urlRootPath, uid, gid)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	w.WriteHeader(http.StatusOK)
}

func createFileFromPart(part *multipart.Part, urlRootPath string, uid int, gid int) error {
	_, params, err := mime.ParseMediaType(part.Header.Get("Content-Disposition"))
	if err != nil {
		return errors.New("Failed to get content disposition.")
	}

	fileRelPath, ok := params["filename"]
	if !ok {
		return errors.New("Filename not found in content disposition.")
	}
	fileDirRelPath, fileName := path.Split(fileRelPath)

	err = filesystem.CreateMissingDirectories(urlRootPath, fileDirRelPath, uid, gid)
	if err != nil {
		return errors.New("Failed to create missing directories.")
	}

	fileDirPath := path.Join(urlRootPath, fileDirRelPath)
	fileName, err = filesystem.GetAvailableFileName(fileDirPath, fileName)
	if err != nil {
		return errors.New("Failed to find available file name.")
	}
	filePath := path.Join(fileDirPath, fileName)

	err = filesystem.CreateMultipartFile(part, filePath, uid, gid)
	if err != nil {
		return errors.New("Failed to create file.")
	}

	return nil
}

func DownloadFile(w http.ResponseWriter, r *http.Request) {
	username := r.Context().Value(CONTEXT_KEY_USERNAME).(string)
	urlRelativePath := strings.TrimPrefix(r.URL.Path, "/api/download")

	urlRootPath := path.Join("/home", username, urlRelativePath)
	urlPathInfo, err := os.Stat(urlRootPath)
	if err != nil {
		http.Error(w, "Path not found.", http.StatusBadRequest)
		return
	}

	if urlPathInfo.IsDir() {
		http.Error(w, "Path is a directory.", http.StatusBadRequest)
		return
	}

	_, fileName := path.Split(urlRootPath)
	w.Header().Set("Content-Disposition", "attachment; filename="+fileName)
	w.Header().Set("Content-Type", "application/octet-stream")

	http.ServeFile(w, r, urlRootPath)
}

func Trash(w http.ResponseWriter, r *http.Request) {
	username := r.Context().Value(CONTEXT_KEY_USERNAME).(string)
	urlRelativePath := strings.TrimPrefix(r.URL.Path, "/api/trash")
	err := filesystem.Trash(username, urlRelativePath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func EmptyTrash(w http.ResponseWriter, r *http.Request) {
	username := r.Context().Value(CONTEXT_KEY_USERNAME).(string)
	err := filesystem.EmptyTrash(username)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func SystemReboot(w http.ResponseWriter, r *http.Request) {
	username := r.Context().Value(CONTEXT_KEY_USERNAME).(string)

	if !users.IsAdmin(username) {
		http.Error(w, "Must be admin to reboot.", http.StatusUnauthorized)
		return
	}

	err := system.Reboot()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func SystemPoweroff(w http.ResponseWriter, r *http.Request) {
	username := r.Context().Value(CONTEXT_KEY_USERNAME).(string)

	if !users.IsAdmin(username) {
		http.Error(w, "Must be admin to reboot.", http.StatusUnauthorized)
		return
	}

	err := system.Poweroff()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func CreateUser(w http.ResponseWriter, r *http.Request) {
	username := r.Context().Value(CONTEXT_KEY_USERNAME).(string)
	err := users.CreateUser(username, r.PathValue("username"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func ResetUserPassword(w http.ResponseWriter, r *http.Request) {
	username := r.Context().Value(CONTEXT_KEY_USERNAME).(string)
	err := users.ResetUserPassword(username, r.PathValue("username"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func AddUserSshKey(w http.ResponseWriter, r *http.Request) {
	username := r.Context().Value(CONTEXT_KEY_USERNAME).(string)
	err := filesystem.AddUserSshKey(username, r.PathValue("username"), r.FormValue("sshKey"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func DeleteUserSshKey(w http.ResponseWriter, r *http.Request) {
	username := r.Context().Value(CONTEXT_KEY_USERNAME).(string)
	err := filesystem.DeleteUserSshKey(username, r.PathValue("username"), r.PathValue("index"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func DeleteUser(w http.ResponseWriter, r *http.Request) {
	username := r.Context().Value(CONTEXT_KEY_USERNAME).(string)
	err := users.DeleteUser(username, r.PathValue("username"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}
