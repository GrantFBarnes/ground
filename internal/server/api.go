package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/user"
	"path"
	"regexp"
	"strconv"
	"strings"

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
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), usernameContextKey, username)))
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

	_, err = user.Lookup(body.Username)
	if err != nil {
		http.Error(w, "User does not exist.", http.StatusBadRequest)
		return
	}

	_, err = os.Stat(path.Join("/home", body.Username))
	if err != nil {
		http.Error(w, "User has no home.", http.StatusBadRequest)
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

func compressDirectory(w http.ResponseWriter, r *http.Request) {
	username := r.Context().Value(usernameContextKey).(string)
	homePath := strings.TrimPrefix(r.URL.Path, "/api/compress")
	rootPath := path.Join("/home", username, homePath)
	pathInfo, err := os.Stat(rootPath)
	if err != nil {
		http.Error(w, "Path not found.", http.StatusBadRequest)
		return
	}

	if !pathInfo.IsDir() {
		http.Error(w, "Path is not a directory.", http.StatusBadRequest)
		return
	}

	parentDirName, dirName := path.Split(rootPath)
	fileName := getAvailableFileName(parentDirName, dirName+".tar.gz")
	filePath := path.Join(parentDirName, fileName)

	_, err = os.Stat(filePath)
	if err == nil {
		http.Error(w, "File already exists.", http.StatusBadRequest)
		return
	}

	cmd := exec.Command("su", "-c", "tar -zcf '"+filePath+"' --directory="+rootPath+" .", username)
	err = cmd.Run()
	if err != nil {
		http.Error(w, "Failed to compress directory.", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func uploadFiles(w http.ResponseWriter, r *http.Request) {
	username := r.Context().Value(usernameContextKey).(string)
	user, err := user.Lookup(username)
	if err != nil {
		http.Error(w, "Failed to lookup user.", http.StatusInternalServerError)
		return
	}

	homePath := strings.TrimPrefix(r.URL.Path, "/api/upload")
	rootPath := path.Join("/home", username, homePath)
	pathInfo, err := os.Stat(rootPath)
	if err != nil {
		http.Error(w, "Path not found.", http.StatusBadRequest)
		return
	}

	if !pathInfo.IsDir() {
		http.Error(w, "Path is not a directory.", http.StatusBadRequest)
		return
	}

	fileIndex := 0
	for {
		fileKey := fmt.Sprintf("file%d", fileIndex)
		formFile, formFileHandler, err := r.FormFile(fileKey)
		if err != nil {
			break
		}
		defer formFile.Close()

		formFileName := formFileHandler.Filename
		contentDisposition := formFileHandler.Header.Get("Content-Disposition")
		if strings.Contains(contentDisposition, "filename") {
			parts := strings.SplitSeq(contentDisposition, ";")
			for part := range parts {
				part = strings.TrimSpace(part)
				if fn, ok := strings.CutPrefix(part, "filename="); ok {
					formFileName = strings.Trim(fn, `"`)
					break
				}
			}
		}

		formFilePath, formFileName := path.Split(formFileName)
		formFilePath = path.Join(rootPath, formFilePath)
		fileName := getAvailableFileName(formFilePath, formFileName)
		filePath := path.Join(formFilePath, fileName)

		cmd := exec.Command("su", "-c", "mkdir -p '"+formFilePath+"'", username)
		err = cmd.Run()
		if err != nil {
			http.Error(w, "Failed to create directory.", http.StatusInternalServerError)
			return
		}

		osFile, err := os.Create(filePath)
		if err != nil {
			http.Error(w, "Failed to create file.", http.StatusInternalServerError)
			return
		}
		defer osFile.Close()

		_, err = io.Copy(osFile, formFile)
		if err != nil {
			http.Error(w, "Failed to write to file.", http.StatusInternalServerError)
			return
		}

		uid, _ := strconv.Atoi(user.Uid)
		gid, _ := strconv.Atoi(user.Gid)
		err = os.Chown(filePath, uid, gid)
		if err != nil {
			http.Error(w, "Failed to change ownership of file.", http.StatusInternalServerError)
			return
		}

		fileIndex += 1
	}

	w.WriteHeader(http.StatusOK)
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

func downloadFile(w http.ResponseWriter, r *http.Request) {
	username := r.Context().Value(usernameContextKey).(string)
	homePath := strings.TrimPrefix(r.URL.Path, "/api/download")
	rootPath := path.Join("/home", username, homePath)
	pathInfo, err := os.Stat(rootPath)
	if err != nil {
		http.Error(w, "Path not found.", http.StatusBadRequest)
		return
	}

	if pathInfo.IsDir() {
		http.Error(w, "Path is not a file.", http.StatusBadRequest)
		return
	}

	_, fileName := path.Split(rootPath)
	w.Header().Set("Content-Disposition", "attachment; filename="+fileName)
	w.Header().Set("Content-Type", "application/octet-stream")

	http.ServeFile(w, r, rootPath)
}
