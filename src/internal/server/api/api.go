package api

import (
	"log/slog"
	"net/http"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/grantfbarnes/ground/internal/server/common"
	"github.com/grantfbarnes/ground/internal/server/cookie"
	"github.com/grantfbarnes/ground/internal/system/filesystem"
	"github.com/grantfbarnes/ground/internal/system/power"
	"github.com/grantfbarnes/ground/internal/system/users"
)

var loginAttemptMutex sync.Mutex
var loginAttempts map[string][]time.Time = make(map[string][]time.Time)

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

	if tooManyLoginAttempts(r) {
		slog.Warn("too many login attempts", "ip", r.RemoteAddr, "request", r.URL.Path, "username", username)
		http.Error(w, "too many login attempts", http.StatusTooManyRequests)
		return
	}

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

func UploadFiles(w http.ResponseWriter, r *http.Request) {
	requestor := common.GetRequestor(r)
	urlRelativePath := strings.TrimPrefix(r.URL.Path, "/api/upload")

	homePath := path.Join("/home", requestor)
	urlRootPath := path.Join(homePath, urlRelativePath)
	urlRootPath = path.Clean(urlRootPath)

	if !strings.HasPrefix(urlRootPath, homePath) {
		slog.Warn("path outside of home", "ip", r.RemoteAddr, "request", r.URL.Path, "requestor", requestor)
		http.Error(w, "path outside of home", http.StatusBadRequest)
		return
	}

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

	err = filesystem.UploadFile(r, urlRootPath, requestor)
	if err != nil {
		slog.Error("failed to upload file", "ip", r.RemoteAddr, "request", r.URL.Path, "requestor", requestor, "error", err)
		http.Error(w, "failed to upload file", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func DownloadFile(w http.ResponseWriter, r *http.Request) {
	requestor := common.GetRequestor(r)
	urlRelativePath := strings.TrimPrefix(r.URL.Path, "/api/download")

	homePath := path.Join("/home", requestor)
	urlRootPath := path.Join(homePath, urlRelativePath)
	urlRootPath = path.Clean(urlRootPath)

	if !strings.HasPrefix(urlRootPath, homePath) {
		slog.Warn("path outside of home", "ip", r.RemoteAddr, "request", r.URL.Path, "requestor", requestor)
		http.Error(w, "path outside of home", http.StatusBadRequest)
		return
	}

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

func CreateDirectory(w http.ResponseWriter, r *http.Request) {
	requestor := common.GetRequestor(r)
	relHomePath := r.FormValue("relHomePath")
	dirName := r.FormValue("dirName")

	if relHomePath == "" {
		slog.Warn("path not provided", "ip", r.RemoteAddr, "request", r.URL.Path, "requestor", requestor)
		http.Error(w, "path not provided", http.StatusBadRequest)
		return
	}

	if dirName == "" {
		slog.Warn("directory name not provided", "ip", r.RemoteAddr, "request", r.URL.Path, "requestor", requestor)
		http.Error(w, "directory name not provided", http.StatusBadRequest)
		return
	}

	homePath := path.Join("/home", requestor)
	dirPath := path.Join(homePath, relHomePath, dirName)
	dirPath = path.Clean(dirPath)
	if !strings.HasPrefix(dirPath, homePath) {
		slog.Warn("path outside of home", "ip", r.RemoteAddr, "request", r.URL.Path, "requestor", requestor)
		http.Error(w, "path outside of home", http.StatusBadRequest)
		return
	}

	err := filesystem.CreateDirectory(requestor, relHomePath, dirName)
	if err != nil {
		slog.Error("failed to create directory", "ip", r.RemoteAddr, "request", r.URL.Path, "requestor", requestor, "error", err)
		http.Error(w, "failed to create directory", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func CompressDirectory(w http.ResponseWriter, r *http.Request) {
	requestor := common.GetRequestor(r)
	relHomePath := r.FormValue("relHomePath")
	if relHomePath == "" {
		slog.Warn("path not provided", "ip", r.RemoteAddr, "request", r.URL.Path, "requestor", requestor)
		http.Error(w, "path not provided", http.StatusBadRequest)
		return
	}

	homePath := path.Join("/home", requestor)
	fullPath := path.Join(homePath, relHomePath)
	fullPath = path.Clean(fullPath)
	if !strings.HasPrefix(fullPath, homePath) {
		slog.Warn("path outside of home", "ip", r.RemoteAddr, "request", r.URL.Path, "requestor", requestor)
		http.Error(w, "path outside of home", http.StatusBadRequest)
		return
	}

	err := filesystem.CompressDirectory(requestor, relHomePath)
	if err != nil {
		slog.Error("failed to compress directory", "ip", r.RemoteAddr, "request", r.URL.Path, "requestor", requestor, "error", err)
		http.Error(w, "failed to compress directory", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func ExtractFile(w http.ResponseWriter, r *http.Request) {
	requestor := common.GetRequestor(r)
	relHomePath := r.FormValue("relHomePath")
	if relHomePath == "" {
		slog.Warn("path not provided", "ip", r.RemoteAddr, "request", r.URL.Path, "requestor", requestor)
		http.Error(w, "path not provided", http.StatusBadRequest)
		return
	}

	homePath := path.Join("/home", requestor)
	fullPath := path.Join(homePath, relHomePath)
	fullPath = path.Clean(fullPath)
	if !strings.HasPrefix(fullPath, homePath) {
		slog.Warn("path outside of home", "ip", r.RemoteAddr, "request", r.URL.Path, "requestor", requestor)
		http.Error(w, "path outside of home", http.StatusBadRequest)
		return
	}

	err := filesystem.ExtractFile(requestor, relHomePath)
	if err != nil {
		slog.Error("failed to extract file", "ip", r.RemoteAddr, "request", r.URL.Path, "requestor", requestor, "error", err)
		http.Error(w, "failed to extract file", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func MoveFiles(w http.ResponseWriter, r *http.Request) {
	requestor := common.GetRequestor(r)
	sourceRelHomePath := r.FormValue("sourceRelHomePath")
	destinationRelHomePath := r.FormValue("destinationRelHomePath")

	if sourceRelHomePath == "" {
		slog.Warn("source path not provided", "ip", r.RemoteAddr, "request", r.URL.Path, "requestor", requestor)
		http.Error(w, "source path not provided", http.StatusBadRequest)
		return
	}

	if destinationRelHomePath == "" {
		slog.Warn("destination path not provided", "ip", r.RemoteAddr, "request", r.URL.Path, "requestor", requestor)
		http.Error(w, "destination path not provided", http.StatusBadRequest)
		return
	}

	homePath := path.Join("/home", requestor)

	fullSourcePath := path.Join(homePath, sourceRelHomePath)
	fullSourcePath = path.Clean(fullSourcePath)
	if !strings.HasPrefix(fullSourcePath, homePath) {
		slog.Warn("source path outside of home", "ip", r.RemoteAddr, "request", r.URL.Path, "requestor", requestor)
		http.Error(w, "source path outside of home", http.StatusBadRequest)
		return
	}

	fullDestinationPath := path.Join(homePath, destinationRelHomePath)
	fullDestinationPath = path.Clean(fullDestinationPath)
	if !strings.HasPrefix(fullDestinationPath, homePath) {
		slog.Warn("destination path outside of home", "ip", r.RemoteAddr, "request", r.URL.Path, "requestor", requestor)
		http.Error(w, "destination path outside of home", http.StatusBadRequest)
		return
	}

	err := filesystem.Move(requestor, sourceRelHomePath, destinationRelHomePath)
	if err != nil {
		slog.Error("failed to move files", "ip", r.RemoteAddr, "request", r.URL.Path, "requestor", requestor, "source", sourceRelHomePath, "destination", destinationRelHomePath, "error", err)
		http.Error(w, "failed to move files", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func Trash(w http.ResponseWriter, r *http.Request) {
	requestor := common.GetRequestor(r)
	relHomePath := r.FormValue("relHomePath")
	if relHomePath == "" {
		slog.Warn("path not provided", "ip", r.RemoteAddr, "request", r.URL.Path, "requestor", requestor)
		http.Error(w, "path not provided", http.StatusBadRequest)
		return
	}

	homePath := path.Join("/home", requestor)
	fullPath := path.Join(homePath, relHomePath)
	fullPath = path.Clean(fullPath)
	if !strings.HasPrefix(fullPath, homePath) {
		slog.Warn("path outside of home", "ip", r.RemoteAddr, "request", r.URL.Path, "requestor", requestor)
		http.Error(w, "path outside of home", http.StatusBadRequest)
		return
	}

	err := filesystem.Trash(requestor, relHomePath)
	if err != nil {
		slog.Error("failed to trash", "ip", r.RemoteAddr, "request", r.URL.Path, "requestor", requestor, "error", err)
		http.Error(w, "failed to trash", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func Restore(w http.ResponseWriter, r *http.Request) {
	requestor := common.GetRequestor(r)
	trashDirName := r.FormValue("trashDirName")
	if trashDirName == "" {
		slog.Warn("trash dir name not provided", "ip", r.RemoteAddr, "request", r.URL.Path, "requestor", requestor)
		http.Error(w, "trash dir name not provided", http.StatusBadRequest)
		return
	}

	err := filesystem.Restore(requestor, trashDirName)
	if err != nil {
		slog.Error("failed restore trash dir", "ip", r.RemoteAddr, "request", r.URL.Path, "requestor", requestor, "error", err)
		http.Error(w, "failed restore trash dir", http.StatusInternalServerError)
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

func tooManyLoginAttempts(r *http.Request) bool {
	loginAttemptMutex.Lock()
	defer loginAttemptMutex.Unlock()

	cleanUpLoginAttempts()

	if _, ok := loginAttempts[r.RemoteAddr]; !ok {
		loginAttempts[r.RemoteAddr] = make([]time.Time, 0)
	}
	loginAttempts[r.RemoteAddr] = append(loginAttempts[r.RemoteAddr], time.Now())

	return len(loginAttempts[r.RemoteAddr]) > 10
}

func cleanUpLoginAttempts() {
	expiry := time.Now().Add(-time.Hour)

	newLoginAttempts := make(map[string][]time.Time)
	for ip, times := range loginAttempts {
		newTimes := make([]time.Time, 0)
		for _, t := range times {
			if t.After(expiry) {
				newTimes = append(newTimes, t)
			}
		}
		if len(newTimes) > 0 {
			newLoginAttempts[ip] = newTimes
		}
	}

	loginAttempts = newLoginAttempts
}

func login(w http.ResponseWriter, username string) {
	filesystem.CreateRequiredFiles(username)
	cookie.RemoveUsername(w)
	cookie.SetUsername(w, username)
	w.WriteHeader(http.StatusOK)
}
