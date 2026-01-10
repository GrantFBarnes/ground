package api

import (
	"errors"
	"io"
	"log/slog"
	"mime"
	"mime/multipart"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/grantfbarnes/ground/internal/server/common"
	"github.com/grantfbarnes/ground/internal/server/cookie"
	"github.com/grantfbarnes/ground/internal/system/filesystem"
	"github.com/grantfbarnes/ground/internal/system/power"
	"github.com/grantfbarnes/ground/internal/system/users"
)

func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username, err := cookie.GetUsername(r)
		if err != nil {
			cookie.RemoveUsername(w)
			http.Error(w, "no login credentials found", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, common.GetRequestWithRequestor(r, username))
	})
}

func Login(w http.ResponseWriter, r *http.Request) {
	username := r.FormValue("username")
	password := r.FormValue("password")

	if !users.UserIsValid(username) {
		slog.Warn("username is not valid", "ip", r.RemoteAddr, "request", r.URL.Path, "username", username)
		http.Error(w, "username is not valid", http.StatusBadRequest)
		return
	}

	if password == "" {
		slog.Warn("password not provided", "ip", r.RemoteAddr, "request", r.URL.Path, "username", username)
		http.Error(w, "password not provided", http.StatusBadRequest)
		return
	}

	if strings.ContainsAny(password, "\n") {
		slog.Warn("password is not valid", "ip", r.RemoteAddr, "request", r.URL.Path, "username", username)
		http.Error(w, "password is not valid", http.StatusBadRequest)
		return
	}

	if !users.CredentialsAreValid(username, password) {
		slog.Warn("credentials are not valid", "ip", r.RemoteAddr, "request", r.URL.Path, "username", username)
		http.Error(w, "credentials are not valid", http.StatusBadRequest)
		return
	}

	login(w, username)
}

func Logout(w http.ResponseWriter, r *http.Request) {
	cookie.RemoveUsername(w)
	w.WriteHeader(http.StatusOK)
}

func Impersonate(w http.ResponseWriter, r *http.Request) {
	requestor := common.GetRequestor(r)
	username := r.FormValue("username")

	if !users.IsAdmin(requestor) {
		slog.Warn("non-admin request", "ip", r.RemoteAddr, "request", r.URL.Path, "requestor", requestor, "username", username)
		http.Error(w, "must be admin to impersonate", http.StatusUnauthorized)
		return
	}

	if !users.UserIsValid(username) {
		slog.Warn("username is not valid", "ip", r.RemoteAddr, "request", r.URL.Path, "requestor", requestor, "username", username)
		http.Error(w, "username is not valid", http.StatusBadRequest)
		return
	}

	if requestor == username {
		slog.Warn("cannot impersonate yourself", "ip", r.RemoteAddr, "request", r.URL.Path, "requestor", requestor, "username", username)
		http.Error(w, "cannot impersonate yourself", http.StatusBadRequest)
		return
	}

	login(w, username)
}

func login(w http.ResponseWriter, username string) {
	filesystem.CreateRequiredFiles(username)
	cookie.RemoveUsername(w)
	cookie.SetUsername(w, username)
	w.WriteHeader(http.StatusOK)
}

func ToggleAdmin(w http.ResponseWriter, r *http.Request) {
	requestor := common.GetRequestor(r)
	username := r.FormValue("username")

	if !users.IsAdmin(requestor) {
		slog.Warn("non-admin request", "ip", r.RemoteAddr, "request", r.URL.Path, "requestor", requestor, "username", username)
		http.Error(w, "must be admin to change admin status", http.StatusUnauthorized)
		return
	}

	if !users.UserIsValid(username) {
		slog.Warn("username is not valid", "ip", r.RemoteAddr, "request", r.URL.Path, "requestor", requestor, "username", username)
		http.Error(w, "username is not valid", http.StatusBadRequest)
		return
	}

	err := users.ToggleAdmin(username)
	if err != nil {
		slog.Error("failed to change admin status", "ip", r.RemoteAddr, "request", r.URL.Path, "requestor", requestor, "username", username, "error", err)
		http.Error(w, "failed to change admin status", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func CompressDirectory(w http.ResponseWriter, r *http.Request) {
	requestor := common.GetRequestor(r)
	urlRelativePath := strings.TrimPrefix(r.URL.Path, "/api/compress")
	err := filesystem.CompressDirectory(requestor, urlRelativePath)
	if err != nil {
		slog.Error("failed to compress directory", "ip", r.RemoteAddr, "request", r.URL.Path, "requestor", requestor, "error", err)
		http.Error(w, "failed to compress directory", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func ExtractFile(w http.ResponseWriter, r *http.Request) {
	requestor := common.GetRequestor(r)
	urlRelativePath := strings.TrimPrefix(r.URL.Path, "/api/extract")
	err := filesystem.ExtractFile(requestor, urlRelativePath)
	if err != nil {
		slog.Error("failed to extract file", "ip", r.RemoteAddr, "request", r.URL.Path, "requestor", requestor, "error", err)
		http.Error(w, "failed to extract file", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func UploadFiles(w http.ResponseWriter, r *http.Request) {
	requestor := common.GetRequestor(r)
	urlRelativePath := strings.TrimPrefix(r.URL.Path, "/api/upload")

	urlRootPath := path.Join("/home", requestor, urlRelativePath)
	urlPathInfo, err := os.Stat(urlRootPath)
	if err != nil {
		slog.Warn("path not found", "ip", r.RemoteAddr, "request", r.URL.Path, "requestor", requestor, "error", err)
		http.Error(w, "path not found", http.StatusBadRequest)
		return
	}

	if !urlPathInfo.IsDir() {
		slog.Warn("path is not a directory", "ip", r.RemoteAddr, "request", r.URL.Path, "requestor", requestor)
		http.Error(w, "path is not a directory", http.StatusBadRequest)
		return
	}

	uid, gid, err := users.GetUserIds(requestor)
	if err != nil {
		slog.Error("failed to get user ids", "ip", r.RemoteAddr, "request", r.URL.Path, "requestor", requestor, "error", err)
		http.Error(w, "failed to get user ids", http.StatusInternalServerError)
		return
	}

	mediaType, contentParams, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || !strings.HasPrefix(mediaType, "multipart/") {
		slog.Error("failed to pares media type", "ip", r.RemoteAddr, "request", r.URL.Path, "requestor", requestor, "error", err)
		http.Error(w, "failed to parse media type", http.StatusInternalServerError)
		return
	}

	boundary, ok := contentParams["boundary"]
	if !ok {
		slog.Warn("failed to get file boundary", "ip", r.RemoteAddr, "request", r.URL.Path, "requestor", requestor)
		http.Error(w, "failed to get file boundary", http.StatusInternalServerError)
		return
	}

	multipartReader := multipart.NewReader(r.Body, boundary)
	for {
		part, err := multipartReader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			slog.Error("failed to get next file part", "ip", r.RemoteAddr, "request", r.URL.Path, "requestor", requestor, "error", err)
			http.Error(w, "failed to get next file part", http.StatusInternalServerError)
			return
		}

		err = createFileFromPart(part, urlRootPath, uid, gid)
		if err != nil {
			slog.Error("failed to create file", "ip", r.RemoteAddr, "request", r.URL.Path, "requestor", requestor, "error", err)
			http.Error(w, "failed to create file", http.StatusInternalServerError)
			return
		}
	}

	w.WriteHeader(http.StatusOK)
}

func createFileFromPart(part *multipart.Part, urlRootPath string, uid int, gid int) error {
	_, params, err := mime.ParseMediaType(part.Header.Get("Content-Disposition"))
	if err != nil {
		return errors.Join(errors.New("failed to get content disposition"), err)
	}

	fileRelPath, ok := params["filename"]
	if !ok {
		return errors.New("filename not found in content disposition")
	}
	fileDirRelPath, fileName := path.Split(fileRelPath)

	err = filesystem.CreateMissingDirectories(urlRootPath, fileDirRelPath, uid, gid)
	if err != nil {
		return errors.Join(errors.New("failed to create missing directories"), err)
	}

	fileDirPath := path.Join(urlRootPath, fileDirRelPath)
	fileName, err = filesystem.GetAvailableFileName(fileDirPath, fileName)
	if err != nil {
		return errors.Join(errors.New("failed to find available file name"), err)
	}
	filePath := path.Join(fileDirPath, fileName)

	err = filesystem.CreateMultipartFile(part, filePath, uid, gid)
	if err != nil {
		return errors.Join(errors.New("failed to create multipart file"), err)
	}

	return nil
}

func DownloadFile(w http.ResponseWriter, r *http.Request) {
	requestor := common.GetRequestor(r)
	urlRelativePath := strings.TrimPrefix(r.URL.Path, "/api/download")

	urlRootPath := path.Join("/home", requestor, urlRelativePath)
	urlPathInfo, err := os.Stat(urlRootPath)
	if err != nil {
		slog.Warn("path not found", "ip", r.RemoteAddr, "request", r.URL.Path, "requestor", requestor, "error", err)
		http.Error(w, "path not found", http.StatusBadRequest)
		return
	}

	if urlPathInfo.IsDir() {
		slog.Warn("path is a directory", "ip", r.RemoteAddr, "request", r.URL.Path, "requestor", requestor)
		http.Error(w, "path is a directory", http.StatusBadRequest)
		return
	}

	_, fileName := path.Split(urlRootPath)
	w.Header().Set("Content-Disposition", "attachment; filename="+fileName)
	w.Header().Set("Content-Type", "application/octet-stream")

	http.ServeFile(w, r, urlRootPath)
}

func Trash(w http.ResponseWriter, r *http.Request) {
	requestor := common.GetRequestor(r)
	urlRelativePath := strings.TrimPrefix(r.URL.Path, "/api/trash")
	err := filesystem.Trash(requestor, urlRelativePath)
	if err != nil {
		slog.Error("failed to trash", "ip", r.RemoteAddr, "request", r.URL.Path, "requestor", requestor, "error", err)
		http.Error(w, "failed to trash", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func EmptyTrash(w http.ResponseWriter, r *http.Request) {
	requestor := common.GetRequestor(r)
	err := filesystem.EmptyTrash(requestor)
	if err != nil {
		slog.Error("failed to emtpy trash", "ip", r.RemoteAddr, "request", r.URL.Path, "requestor", requestor, "error", err)
		http.Error(w, "failed to emtpy trash", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func SystemReboot(w http.ResponseWriter, r *http.Request) {
	requestor := common.GetRequestor(r)

	if !users.IsAdmin(requestor) {
		slog.Warn("non-admin request", "ip", r.RemoteAddr, "request", r.URL.Path, "requestor", requestor)
		http.Error(w, "must be admin to reboot", http.StatusUnauthorized)
		return
	}

	err := power.Reboot()
	if err != nil {
		slog.Error("failed to reboot", "ip", r.RemoteAddr, "request", r.URL.Path, "requestor", requestor, "error", err)
		http.Error(w, "failed to reboot", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func SystemPoweroff(w http.ResponseWriter, r *http.Request) {
	requestor := common.GetRequestor(r)

	if !users.IsAdmin(requestor) {
		slog.Warn("non-admin request", "ip", r.RemoteAddr, "request", r.URL.Path, "requestor", requestor)
		http.Error(w, "must be admin to poweroff", http.StatusUnauthorized)
		return
	}

	err := power.Poweroff()
	if err != nil {
		slog.Error("failed to poweroff", "ip", r.RemoteAddr, "request", r.URL.Path, "requestor", requestor, "error", err)
		http.Error(w, "failed to poweroff", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func CreateUser(w http.ResponseWriter, r *http.Request) {
	requestor := common.GetRequestor(r)
	username := r.FormValue("username")

	if !users.IsAdmin(requestor) {
		slog.Warn("non-admin request", "ip", r.RemoteAddr, "request", r.URL.Path, "requestor", requestor, "username", username)
		http.Error(w, "must be admin to create users", http.StatusUnauthorized)
		return
	}

	if !users.UsernameIsValid(username) {
		slog.Warn("username is not valid", "ip", r.RemoteAddr, "request", r.URL.Path, "requestor", requestor, "username", username)
		http.Error(w, "username is not valid", http.StatusBadRequest)
		return
	}

	err := users.CreateUser(username)
	if err != nil {
		slog.Error("failed to create user", "ip", r.RemoteAddr, "request", r.URL.Path, "requestor", requestor, "username", username, "error", err)
		http.Error(w, "failed to create user", http.StatusInternalServerError)
		return
	}

	err = filesystem.CreateRequiredFiles(username)
	if err != nil {
		slog.Error("failed to create required files", "ip", r.RemoteAddr, "request", r.URL.Path, "requestor", requestor, "username", username, "error", err)
		http.Error(w, "failed to create required files", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func ResetUserPassword(w http.ResponseWriter, r *http.Request) {
	requestor := common.GetRequestor(r)
	username := r.FormValue("username")

	if requestor != username && !users.IsAdmin(requestor) {
		slog.Warn("non-admin request", "ip", r.RemoteAddr, "request", r.URL.Path, "requestor", requestor, "username", username)
		http.Error(w, "must be admin to reset passwords for other users", http.StatusUnauthorized)
		return
	}

	if !users.UserIsValid(username) {
		slog.Warn("username is not valid", "ip", r.RemoteAddr, "request", r.URL.Path, "requestor", requestor, "username", username)
		http.Error(w, "username is not valid", http.StatusBadRequest)
		return
	}

	err := users.ResetUserPassword(username)
	if err != nil {
		slog.Error("failed to reset password", "ip", r.RemoteAddr, "request", r.URL.Path, "requestor", requestor, "username", username, "error", err)
		http.Error(w, "failed to reset password", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func ChangeUserPassword(w http.ResponseWriter, r *http.Request) {
	requestor := common.GetRequestor(r)
	username := r.FormValue("username")
	currentPassword := r.FormValue("currentPassword")
	newPassword := r.FormValue("newPassword")
	newPasswordConfirm := r.FormValue("newPasswordConfirm")

	if requestor != username && !users.IsAdmin(requestor) {
		slog.Warn("non-admin request", "ip", r.RemoteAddr, "request", r.URL.Path, "requestor", requestor, "username", username)
		http.Error(w, "must be admin to change passwords for other users", http.StatusUnauthorized)
		return
	}

	if !users.UserIsValid(username) {
		slog.Warn("username is not valid", "ip", r.RemoteAddr, "request", r.URL.Path, "requestor", requestor, "username", username)
		http.Error(w, "username is not valid", http.StatusBadRequest)
		return
	}

	if newPassword == "" {
		slog.Warn("password not provided", "ip", r.RemoteAddr, "request", r.URL.Path, "requestor", requestor, "username", username)
		http.Error(w, "password not provided", http.StatusBadRequest)
		return
	}

	if strings.ContainsAny(newPassword, "\n") {
		slog.Warn("password is not valid", "ip", r.RemoteAddr, "request", r.URL.Path, "requestor", requestor, "username", username)
		http.Error(w, "password is not valid", http.StatusBadRequest)
		return
	}

	if newPassword != newPasswordConfirm {
		slog.Warn("password confirm does not match", "ip", r.RemoteAddr, "request", r.URL.Path, "requestor", requestor, "username", username)
		http.Error(w, "password confirm does not match", http.StatusBadRequest)
		return
	}

	if !users.CredentialsAreValid(username, currentPassword) {
		slog.Warn("credentials are not valid", "ip", r.RemoteAddr, "request", r.URL.Path, "requestor", requestor, "username", username)
		http.Error(w, "credentials are not valid", http.StatusBadRequest)
		return
	}

	err := users.SetUserPassword(username, newPassword)
	if err != nil {
		slog.Error("failed to change password", "ip", r.RemoteAddr, "request", r.URL.Path, "requestor", requestor, "username", username, "error", err)
		http.Error(w, "failed to change password", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func AddUserSshKey(w http.ResponseWriter, r *http.Request) {
	requestor := common.GetRequestor(r)
	username := r.FormValue("username")
	sshKey := r.FormValue("sshKey")

	if requestor != username && !users.IsAdmin(requestor) {
		slog.Warn("non-admin request", "ip", r.RemoteAddr, "request", r.URL.Path, "requestor", requestor, "username", username)
		http.Error(w, "must be admin to add SSH keys for other users", http.StatusUnauthorized)
		return
	}

	if !users.UserIsValid(username) {
		slog.Warn("username is not valid", "ip", r.RemoteAddr, "request", r.URL.Path, "requestor", requestor, "username", username)
		http.Error(w, "username is not valid", http.StatusBadRequest)
		return
	}

	err := filesystem.AddUserSshKey(username, sshKey)
	if err != nil {
		slog.Error("failed to add ssh key", "ip", r.RemoteAddr, "request", r.URL.Path, "requestor", requestor, "username", username, "error", err)
		http.Error(w, "failed to add ssh key", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func DeleteUserSshKey(w http.ResponseWriter, r *http.Request) {
	requestor := common.GetRequestor(r)
	username := r.FormValue("username")
	index := r.FormValue("index")

	if requestor != username && !users.IsAdmin(requestor) {
		slog.Warn("non-admin request", "ip", r.RemoteAddr, "request", r.URL.Path, "requestor", requestor, "username", username)
		http.Error(w, "must be admin to delete SSH keys for other users", http.StatusUnauthorized)
		return
	}

	if !users.UserIsValid(username) {
		slog.Warn("username is not valid", "ip", r.RemoteAddr, "request", r.URL.Path, "requestor", requestor, "username", username)
		http.Error(w, "username is not valid", http.StatusBadRequest)
		return
	}

	err := filesystem.DeleteUserSshKey(username, index)
	if err != nil {
		slog.Error("failed to delete ssh key", "ip", r.RemoteAddr, "request", r.URL.Path, "requestor", requestor, "username", username, "error", err)
		http.Error(w, "failed to delete ssh key", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func DeleteUser(w http.ResponseWriter, r *http.Request) {
	requestor := common.GetRequestor(r)
	username := r.FormValue("username")

	if requestor != username && !users.IsAdmin(requestor) {
		slog.Warn("non-admin request", "ip", r.RemoteAddr, "request", r.URL.Path, "requestor", requestor, "username", username)
		http.Error(w, "must be admin to delete other users", http.StatusUnauthorized)
		return
	}

	if !users.UserIsValid(username) {
		slog.Warn("username is not valid", "ip", r.RemoteAddr, "request", r.URL.Path, "requestor", requestor, "username", username)
		http.Error(w, "username is not valid", http.StatusBadRequest)
		return
	}

	err := users.DeleteUser(username)
	if err != nil {
		slog.Error("failed to delete user", "ip", r.RemoteAddr, "request", r.URL.Path, "requestor", requestor, "username", username, "error", err)
		http.Error(w, "failed to delete user", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}
