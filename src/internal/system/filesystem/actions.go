package filesystem

import (
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"path"
	"strings"
	"time"

	"github.com/grantfbarnes/ground/internal/system/users"
)

func UploadFile(r *http.Request, dirPath string, username string) error {
	mediaType, contentParams, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || !strings.HasPrefix(mediaType, "multipart/") {
		return errors.Join(errors.New("failed to pares media type"), err)
	}

	boundary, ok := contentParams["boundary"]
	if !ok {
		return errors.New("failed to get file boundary")
	}

	multipartReader := multipart.NewReader(r.Body, boundary)
	for {
		part, err := multipartReader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return errors.Join(errors.New("failed to get next file part"), err)
		}

		err = createFileFromPart(part, dirPath, username)
		if err != nil {
			return errors.Join(errors.New("failed to create file"), err)
		}
	}

	return nil
}

func createFileFromPart(part *multipart.Part, dirPath string, username string) error {
	_, params, err := mime.ParseMediaType(part.Header.Get("Content-Disposition"))
	if err != nil {
		return errors.Join(errors.New("failed to get content disposition"), err)
	}

	fileRelPath, ok := params["filename"]
	if !ok {
		return errors.New("filename not found in content disposition")
	}
	fileDirRelPath, fileName := path.Split(fileRelPath)
	fileDirPath := path.Join(dirPath, fileDirRelPath)

	err = mkdir(fileDirPath, username)
	if err != nil {
		return errors.Join(errors.New("failed to create parent directory"), err)
	}

	fileName, err = getAvailableFileName(fileDirPath, fileName)
	if err != nil {
		return errors.Join(errors.New("failed to find available file name"), err)
	}
	filePath := path.Join(fileDirPath, fileName)

	err = createMultipartFile(part, filePath, username)
	if err != nil {
		return errors.Join(errors.New("failed to create multipart file"), err)
	}

	return nil
}

func createMultipartFile(part *multipart.Part, filePath string, username string) error {
	err := touch(filePath, username)
	if err != nil {
		return errors.Join(errors.New("failed to create file"), err)
	}

	osFile, err := os.OpenFile(filePath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return errors.Join(errors.New("failed to open file"), err)
	}
	defer osFile.Close()

	_, err = io.Copy(osFile, part)
	if err != nil {
		return errors.Join(errors.New("failed to copy file data"), err)
	}

	return nil
}

func CreateDirectory(username string, relHomePath string, dirName string) error {
	rootDirPath := path.Join("/home", username, relHomePath)
	dirInfo, err := os.Stat(rootDirPath)
	if err != nil {
		return errors.Join(errors.New("failed to get path stat"), err)
	}

	if !dirInfo.IsDir() {
		return errors.New("path is not a directory")
	}

	err = mkdir(path.Join(rootDirPath, dirName), username)
	if err != nil {
		return errors.Join(errors.New("failed to create directory"), err)
	}

	return nil
}

func CompressDirectory(username string, relHomePath string) error {
	rootDirPath := path.Join("/home", username, relHomePath)
	dirInfo, err := os.Stat(rootDirPath)
	if err != nil {
		return errors.Join(errors.New("failed to get path stat"), err)
	}

	if strings.ContainsAny(rootDirPath, "'") {
		return errors.New("path is not valid")
	}

	if !dirInfo.IsDir() {
		return errors.New("path is not a directory")
	}

	dirPath, dirName := path.Split(rootDirPath)
	fileName, err := getAvailableFileName(dirPath, dirName+".tar.gz")
	if err != nil {
		return errors.Join(errors.New("failed to find available file name"), err)
	}
	filePath := path.Join(dirPath, fileName)

	cmd := exec.Command("tar", "-zchf", filePath, "--directory", rootDirPath, ".")

	err = users.ExecuteAs(cmd, username)
	if err != nil {
		return errors.Join(errors.New("failed to set command executor"), err)
	}

	err = cmd.Run()
	if err != nil {
		return errors.Join(errors.New("failed to compress directory"), err)
	}

	return nil
}

func ExtractFile(username string, relHomePath string) error {
	rootFilePath := path.Join("/home", username, relHomePath)
	fileInfo, err := os.Stat(rootFilePath)
	if err != nil {
		return errors.Join(errors.New("failed to get path stat"), err)
	}

	if strings.ContainsAny(rootFilePath, "'") {
		return errors.New("path is not valid")
	}

	if fileInfo.IsDir() {
		return errors.New("path is a directory")
	}

	fileDirPath, fileName := path.Split(rootFilePath)
	fileNameNoExt, fileExt := getFileExtension(fileName)
	if fileExt != ".tar.gz" {
		return errors.New("file is not compressed")
	}

	extractedDirName, err := getAvailableFileName(fileDirPath, fileNameNoExt)
	if err != nil {
		return errors.Join(errors.New("failed to find available name"), err)
	}
	extractedDirPath := path.Join(fileDirPath, extractedDirName)

	err = mkdir(extractedDirPath, username)
	if err != nil {
		return errors.Join(errors.New("failed to create extract directory"), err)
	}

	cmd := exec.Command("tar", "-xzf", rootFilePath, "--directory", extractedDirPath)

	err = users.ExecuteAs(cmd, username)
	if err != nil {
		return errors.Join(errors.New("failed to set command executor"), err)
	}

	err = cmd.Run()
	if err != nil {
		return errors.Join(errors.New("failed to extract file"), err)
	}

	return nil
}

func CreateRequiredFiles(username string) error {
	homePath := path.Join("/home", username)

	err := mkdir(path.Join(homePath, TRASH_HOME_PATH), username)
	if err != nil {
		return errors.Join(errors.New("failed to create trash directory"), err)
	}

	err = touch(path.Join(homePath, ".ssh", "authorized_keys"), username)
	if err != nil {
		return errors.Join(errors.New("failed to create ssh keys file"), err)
	}

	return nil
}

func Move(username string, sourceRelHomePath string, destinationRelHomePath string) error {
	sourcePath := path.Join("/home", username, sourceRelHomePath)
	_, err := os.Stat(sourcePath)
	if err != nil {
		return errors.Join(errors.New("failed to get source path stat"), err)
	}

	destinationPath := path.Join("/home", username, destinationRelHomePath)
	_, err = os.Stat(destinationPath)
	if err == nil {
		return errors.New("destination already exists")
	}

	sourceParentPath, sourceName := path.Split(sourcePath)
	destinationParentPath, destinationName := path.Split(destinationPath)

	if sourceName != destinationName {
		return errors.New("source and destination names do not match")
	}

	if sourceParentPath == destinationParentPath {
		return errors.New("source and destination are the same")
	}

	err = os.Rename(sourcePath, destinationPath)
	if err != nil {
		return errors.Join(errors.New("failed to rename files"), err)
	}

	return nil
}

func Trash(username string, relHomePath string) error {
	rootDirPath := path.Join("/home", username, relHomePath)
	_, err := os.Stat(rootDirPath)
	if err != nil {
		return errors.Join(errors.New("failed to get path stat"), err)
	}

	trashRootPath := path.Join("/home", username, TRASH_HOME_PATH)
	trashTimestamp := time.Now().Format(systemTimeLayout)
	trashTimestampPath := path.Join(trashRootPath, trashTimestamp)
	err = mkdir(trashTimestampPath, username)
	if err != nil {
		return errors.Join(errors.New("failed to create timestamp directory"), err)
	}

	restorePath, fileName := path.Split(rootDirPath)

	if fileName == trashRestorePathFileName {
		fileName, _ = strings.CutPrefix(fileName, ".")
	}

	err = os.Rename(rootDirPath, path.Join(trashTimestampPath, fileName))
	if err != nil {
		return errors.Join(errors.New("failed to rename files"), err)
	}

	trashRestorePathFilePath := path.Join(trashTimestampPath, trashRestorePathFileName)
	err = touch(trashRestorePathFilePath, username)
	if err != nil {
		return errors.Join(errors.New("failed to create restore path file"), err)
	}

	err = os.WriteFile(trashRestorePathFilePath, []byte(restorePath), 0644)
	if err != nil {
		return errors.Join(errors.New("failed to write to restore path file"), err)
	}

	return nil
}

func Restore(username string, trashDirName string) error {
	if !systemTimeLayoutRegex.MatchString(trashDirName) {
		return errors.New("trash dir name is invalid")
	}

	trashDirPath := path.Join("/home", username, TRASH_HOME_PATH, trashDirName)
	_, err := os.Stat(trashDirPath)
	if err != nil {
		return errors.Join(errors.New("failed to find trash dir"), err)
	}

	restoreFilePath := path.Join(trashDirPath, trashRestorePathFileName)
	restorePathBytes, err := os.ReadFile(restoreFilePath)
	if err != nil {
		return errors.Join(errors.New("failed to read restore path"), err)
	}
	restorePath := string(restorePathBytes)

	err = mkdir(restorePath, username)
	if err != nil {
		return errors.Join(errors.New("failed to create restore path"), err)
	}

	dirEntries, err := os.ReadDir(trashDirPath)
	if err != nil {
		return errors.Join(errors.New("failed to read directory"), err)
	}

	for _, entry := range dirEntries {
		if entry.Name() == trashRestorePathFileName {
			continue
		}

		restoreName, err := getAvailableFileName(restorePath, entry.Name())
		if err != nil {
			return errors.Join(errors.New("failed to find available restore name"), err)
		}

		err = os.Rename(path.Join(trashDirPath, entry.Name()), path.Join(restorePath, restoreName))
		if err != nil {
			return errors.Join(errors.New("failed to rename files"), err)
		}
	}

	err = os.RemoveAll(trashDirPath)
	if err != nil {
		return errors.Join(errors.New("failed to remove dir path"), err)
	}

	return nil
}

func EmptyTrash(username string) error {
	trashRootPath := path.Join("/home", username, TRASH_HOME_PATH)

	dirEntries, err := os.ReadDir(trashRootPath)
	if err != nil {
		return errors.Join(errors.New("failed to read directory"), err)
	}

	for _, entry := range dirEntries {
		entryFullPath := path.Join(trashRootPath, entry.Name())
		err = os.RemoveAll(entryFullPath)
		if err != nil {
			return errors.Join(errors.New("failed to remove all files"), err)
		}
	}

	return nil
}
