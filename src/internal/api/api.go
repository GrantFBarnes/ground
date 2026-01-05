package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/grantfbarnes/ground/internal/auth"
	"github.com/grantfbarnes/ground/internal/filesystem"
	"github.com/grantfbarnes/ground/internal/system"
	"github.com/grantfbarnes/ground/internal/users"
)

const CONTEXT_KEY_USERNAME string = "username"

func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username, err := auth.GetUsername(r)
		if err != nil {
			auth.RemoveUsername(w)
			http.Error(w, "No login credentials found.", http.StatusUnauthorized)
			return
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

	filesystem.CreateTrashDirectory(body.Username)

	auth.RemoveUsername(w)
	auth.SetUsername(w, body.Username)
	w.WriteHeader(http.StatusOK)
}

func Logout(w http.ResponseWriter, r *http.Request) {
	auth.RemoveUsername(w)
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

	fileIndex := 0
	for {
		done, err := createRequestFormFile(r, urlRootPath, uid, gid, fileIndex)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if done {
			break
		}

		fileIndex += 1
	}

	w.WriteHeader(http.StatusOK)
}

func createRequestFormFile(r *http.Request, urlRootPath string, uid int, gid int, fileIndex int) (bool, error) {
	file, fileHandler, err := r.FormFile(fmt.Sprintf("file%d", fileIndex))
	if err != nil {
		return true, nil
	}
	defer file.Close()

	fileDirRelPath, fileName, err := getFileHandlerDirPathFileName(fileHandler)
	if err != nil {
		return false, errors.New("Filename not found in header.")
	}

	err = filesystem.CreateMissingDirectories(urlRootPath, fileDirRelPath, uid, gid)
	if err != nil {
		return false, errors.New("Failed to create missing directories.")
	}

	fileDirPath := path.Join(urlRootPath, fileDirRelPath)
	fileName, err = filesystem.GetAvailableFileName(fileDirPath, fileName)
	if err != nil {
		return false, errors.New("Failed to find available file name.")
	}
	filePath := path.Join(fileDirPath, fileName)

	err = filesystem.CreateMultipartFile(file, filePath, uid, gid)
	if err != nil {
		return false, errors.New("Failed to create file.")
	}

	return false, nil
}

func getFileHandlerDirPathFileName(fileHandler *multipart.FileHeader) (dirPath string, fileName string, err error) {
	var filePath string

	contentDisposition := fileHandler.Header.Get("Content-Disposition")
	if strings.Contains(contentDisposition, "filename") {
		parts := strings.SplitSeq(contentDisposition, ";")
		for part := range parts {
			part = strings.TrimSpace(part)
			if filename, ok := strings.CutPrefix(part, "filename="); ok {
				filePath = strings.Trim(filename, `"`)
				break
			}
		}
	}

	if filePath == "" {
		return "", "", errors.New("filename not found in header")
	}

	dirPath, fileName = path.Split(filePath)

	return dirPath, fileName, nil
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
	err := system.Reboot(username)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func SystemPoweroff(w http.ResponseWriter, r *http.Request) {
	username := r.Context().Value(CONTEXT_KEY_USERNAME).(string)
	err := system.Poweroff(username)
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
	err := filesystem.DeleteUserSshKey(username, r.PathValue("username"), r.PathValue("lineNumber"))
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
