package server

import (
	"archive/zip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"os/user"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/grantfbarnes/ground/internal/auth"
)

var fileCopyNameRegex = regexp.MustCompile(`(.*)\(([0-9]+)\)$`)

func apiMiddleware(next http.Handler) http.Handler {
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

	user, err := user.Lookup(body.Username)
	if err != nil {
		http.Error(w, "User does not exist.", http.StatusBadRequest)
		return
	}

	if !auth.CredentialsAreValid(body.Username, body.Password) {
		http.Error(w, "Invalid credentials provided.", http.StatusUnauthorized)
		return
	}

	homePath := path.Join("/home", body.Username)
	_, err = os.Stat(homePath)
	if err != nil {
		http.Error(w, "User has no home.", http.StatusBadRequest)
		return
	}

	uid, _ := strconv.Atoi(user.Uid)
	gid, _ := strconv.Atoi(user.Gid)
	err = createMissingDirectories(homePath, TRASH_HOME_PATH, uid, gid)
	if err != nil {
		http.Error(w, "Failed to create ground trash.", http.StatusInternalServerError)
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

func compressDirectory(w http.ResponseWriter, r *http.Request) {
	username := r.Context().Value(CONTEXT_KEY_USERNAME).(string)
	urlRelativePath := strings.TrimPrefix(r.URL.Path, "/api/compress")
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

	dirPath, dirName := path.Split(urlRootPath)
	fileName := getAvailableFileName(dirPath, dirName+".tar.gz")
	filePath := path.Join(dirPath, fileName)

	_, err = os.Stat(filePath)
	if err == nil {
		http.Error(w, "File already exists.", http.StatusBadRequest)
		return
	}

	cmd := exec.Command("su", "-c", "tar -zcf '"+filePath+"' --directory='"+urlRootPath+"' .", username)
	err = cmd.Run()
	if err != nil {
		http.Error(w, "Failed to compress directory.", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func uploadFiles(w http.ResponseWriter, r *http.Request) {
	username := r.Context().Value(CONTEXT_KEY_USERNAME).(string)
	user, err := user.Lookup(username)
	if err != nil {
		http.Error(w, "Failed to lookup user.", http.StatusInternalServerError)
		return
	}
	uid, _ := strconv.Atoi(user.Uid)
	gid, _ := strconv.Atoi(user.Gid)

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

	fileIndex := 0
	for {
		file, fileHandler, err := r.FormFile(fmt.Sprintf("file%d", fileIndex))
		if err != nil {
			break
		}
		defer file.Close()

		fileDirRelPath, fileName, err := getDirectoryPathFileName(fileHandler)
		if err != nil {
			http.Error(w, "Filename not found in header.", http.StatusBadRequest)
			return
		}

		err = createMissingDirectories(urlRootPath, fileDirRelPath, uid, gid)
		if err != nil {
			http.Error(w, "Failed to create missing directories.", http.StatusInternalServerError)
			return
		}

		fileDirPath := path.Join(urlRootPath, fileDirRelPath)
		fileName = getAvailableFileName(fileDirPath, fileName)
		filePath := path.Join(fileDirPath, fileName)

		err = createFile(file, filePath, uid, gid)
		if err != nil {
			http.Error(w, "Failed to create file.", http.StatusInternalServerError)
			return
		}

		fileIndex += 1
	}

	w.WriteHeader(http.StatusOK)
}

func getDirectoryPathFileName(fileHandler *multipart.FileHeader) (dirPath string, fileName string, err error) {
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

func createMissingDirectories(rootPath string, relDirPath string, uid int, gid int) error {
	relDirPathBuildUp := ""
	for dirName := range strings.SplitSeq(relDirPath, "/") {
		if dirName == "" {
			continue
		}

		relDirPathBuildUp = path.Join(relDirPathBuildUp, dirName)
		dirPath := path.Join(rootPath, relDirPathBuildUp)

		_, err := os.Stat(dirPath)
		if err == nil {
			// directory already exists
			continue
		}

		err = os.Mkdir(dirPath, os.FileMode(0755))
		if err != nil {
			return errors.New("failed to create directory")
		}

		err = os.Chown(dirPath, uid, gid)
		if err != nil {
			return errors.New("failed to change directory ownership")
		}
	}
	return nil
}

func createFile(file multipart.File, filePath string, uid int, gid int) error {
	osFile, err := os.Create(filePath)
	if err != nil {
		return errors.New("failed to create file")
	}
	defer osFile.Close()

	_, err = io.Copy(osFile, file)
	if err != nil {
		return errors.New("failed to write to file")
	}

	err = os.Chown(filePath, uid, gid)
	if err != nil {
		return errors.New("failed to change file ownership")
	}

	return nil
}

func getAvailableFileName(fileDirPath string, fileName string) string {
	for {
		filePath := path.Join(fileDirPath, fileName)
		_, err := os.Stat(filePath)
		if err != nil {
			break
		}

		fileNameNoExt, fileExt := getFileExtension(fileName)

		if fileCopyNameRegex.MatchString(fileNameNoExt) {
			fileName = fileCopyNameRegex.ReplaceAllStringFunc(fileNameNoExt, func(s string) string {
				matches := fileCopyNameRegex.FindStringSubmatch(s)
				if len(matches) != 3 {
					return s
				}

				coreFileName := matches[1]
				fileNameCopyNumber := matches[2]
				copyNumber, err := strconv.Atoi(fileNameCopyNumber)
				if err != nil {
					return s
				}

				return fmt.Sprintf("%s(%d)%s", coreFileName, copyNumber+1, fileExt)
			})
		} else {
			fileName = fmt.Sprintf("%s(1)%s", fileNameNoExt, fileExt)
		}

	}

	return fileName
}

func getFileExtension(fileName string) (coreFileName, fileExtension string) {
	split := strings.Split(fileName, ".")
	if strings.HasPrefix(fileName, ".") {
		split = strings.Split(fileName[1:], ".")
	}

	if len(split) == 0 {
		return "", ""
	}

	coreFileName = split[0]
	fileExtension = strings.Join(split[1:], ".")
	if fileExtension != "" {
		fileExtension = "." + fileExtension
	}

	return coreFileName, fileExtension
}

func download(w http.ResponseWriter, r *http.Request) {
	username := r.Context().Value(CONTEXT_KEY_USERNAME).(string)
	urlRelativePath := strings.TrimPrefix(r.URL.Path, "/api/download")
	urlRootPath := path.Join("/home", username, urlRelativePath)
	urlPathInfo, err := os.Stat(urlRootPath)
	if err != nil {
		http.Error(w, "Path not found.", http.StatusBadRequest)
		return
	}

	if urlPathInfo.IsDir() {
		_, dirName := path.Split(urlRootPath)
		w.Header().Set("Content-Disposition", "attachment; filename="+dirName+".zip")
		w.Header().Set("Content-Type", "application/zip")

		zipWriter := zip.NewWriter(w)
		defer zipWriter.Close()

		filepath.Walk(urlRootPath, func(filePath string, fileInfo os.FileInfo, _ error) error {
			if fileInfo.IsDir() {
				return nil
			}

			relPath, err := filepath.Rel(urlRootPath, filePath)
			if err != nil {
				return err
			}

			file, err := os.Open(filePath)
			if err != nil {
				return err
			}
			defer file.Close()

			fileWriter, err := zipWriter.Create(relPath)
			if err != nil {
				return err
			}

			io.Copy(fileWriter, file)

			return nil
		})
	} else {
		_, fileName := path.Split(urlRootPath)
		w.Header().Set("Content-Disposition", "attachment; filename="+fileName)
		w.Header().Set("Content-Type", "application/octet-stream")

		http.ServeFile(w, r, urlRootPath)
	}
}

func trash(w http.ResponseWriter, r *http.Request) {
	username := r.Context().Value(CONTEXT_KEY_USERNAME).(string)
	user, err := user.Lookup(username)
	if err != nil {
		http.Error(w, "Failed to lookup user.", http.StatusInternalServerError)
		return
	}
	uid, _ := strconv.Atoi(user.Uid)
	gid, _ := strconv.Atoi(user.Gid)

	urlRelativePath := strings.TrimPrefix(r.URL.Path, "/api/trash")
	urlRootPath := path.Join("/home", username, urlRelativePath)
	_, err = os.Stat(urlRootPath)
	if err != nil {
		http.Error(w, "Path not found.", http.StatusBadRequest)
		return
	}

	homePath := path.Join("/home", username)
	trashTimestampHomePath := path.Join(TRASH_HOME_PATH, time.Now().Format("20060102150405.000"), path.Dir(urlRelativePath))
	err = createMissingDirectories(homePath, trashTimestampHomePath, uid, gid)
	if err != nil {
		http.Error(w, "Failed to create missing directories.", http.StatusInternalServerError)
		return
	}

	cmd := exec.Command("mv", urlRootPath, path.Join(homePath, trashTimestampHomePath))
	err = cmd.Run()
	if err != nil {
		http.Error(w, "Failed to move to trash.", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func emptyTrash(w http.ResponseWriter, r *http.Request) {
	username := r.Context().Value(CONTEXT_KEY_USERNAME).(string)
	trashRootPath := path.Join("/home", username, TRASH_HOME_PATH)

	dirEntries, err := os.ReadDir(trashRootPath)
	if err != nil {
		http.Error(w, "Failed to read trash.", http.StatusInternalServerError)
		return
	}

	for _, entry := range dirEntries {
		entryFullPath := path.Join(trashRootPath, entry.Name())
		err = os.RemoveAll(entryFullPath)
		if err != nil {
			http.Error(w, "Failed empty trash.", http.StatusInternalServerError)
			return
		}
	}

	w.WriteHeader(http.StatusOK)
}

func systemCallMethod(w http.ResponseWriter, r *http.Request) {
	username := r.Context().Value(CONTEXT_KEY_USERNAME).(string)

	if !auth.IsAdmin(username) {
		http.Error(w, "Must be admin to make system calls.", http.StatusForbidden)
		return
	}

	method := r.PathValue("method")
	switch method {
	case "reboot":
		fallthrough
	case "poweroff":
		cmd := exec.Command("systemctl", method)
		err := cmd.Run()
		if err != nil {
			http.Error(w, "Failed to make system call.", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	default:
		http.Error(w, "System method not recognized.", http.StatusBadRequest)
	}
}

func createUser(w http.ResponseWriter, r *http.Request) {
	username := r.Context().Value(CONTEXT_KEY_USERNAME).(string)

	if !auth.IsAdmin(username) {
		http.Error(w, "Must be admin to create new users.", http.StatusForbidden)
		return
	}

	newUsername := r.PathValue("username")

	homePath := path.Join("/home", newUsername)
	_, err := os.Stat(homePath)
	if err == nil {
		http.Error(w, "User already exists.", http.StatusBadRequest)
		return
	}

	cmd := exec.Command("useradd", "--create-home", newUsername)

	err = cmd.Start()
	if err != nil {
		http.Error(w, "Failed to create user.", http.StatusInternalServerError)
		return
	}

	err = cmd.Wait()
	if err != nil {
		http.Error(w, "Failed to create user.", http.StatusInternalServerError)
		return
	}

	err = setUserPassword(newUsername, "password")
	if err != nil {
		http.Error(w, "Failed to set password.", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func resetUserPassword(w http.ResponseWriter, r *http.Request) {
	username := r.Context().Value(CONTEXT_KEY_USERNAME).(string)

	if !auth.IsAdmin(username) {
		http.Error(w, "Must be admin to reset user passwords.", http.StatusForbidden)
		return
	}

	err := setUserPassword(r.PathValue("username"), "password")
	if err != nil {
		http.Error(w, "Failed to reset password.", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func deleteUser(w http.ResponseWriter, r *http.Request) {
	username := r.Context().Value(CONTEXT_KEY_USERNAME).(string)

	if !auth.IsAdmin(username) {
		http.Error(w, "Must be admin to delete users.", http.StatusForbidden)
		return
	}

	targetUsername := r.PathValue("username")

	cmd := exec.Command("userdel", "--remove", targetUsername)

	err := cmd.Start()
	if err != nil {
		http.Error(w, "Failed to delete user.", http.StatusInternalServerError)
		return
	}

	err = cmd.Wait()
	if err != nil {
		http.Error(w, "Failed to delete user.", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func setUserPassword(username string, password string) error {
	cmd := exec.Command("passwd", "--stdin", username)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}

	go func() {
		defer stdin.Close()
		io.WriteString(stdin, password+"\n")
	}()

	err = cmd.Start()
	if err != nil {
		return err
	}

	err = cmd.Wait()
	if err != nil {
		return err
	}

	return nil
}
