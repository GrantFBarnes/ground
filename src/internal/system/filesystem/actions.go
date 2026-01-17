package filesystem

import (
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	"github.com/grantfbarnes/ground/internal/system/execute"
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

	err = execute.MakeDirectory(username, fileDirPath)
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
	err := execute.TouchFile(username, filePath)
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

	err = execute.MakeDirectory(username, path.Join(rootDirPath, dirName))
	if err != nil {
		return errors.Join(errors.New("failed to create directory"), err)
	}

	return nil
}

func CompressDirectory(username string, relHomePath string) error {
	dirPath := path.Join("/home", username, relHomePath)
	dirParentPath, dirName := path.Split(dirPath)
	fileName, err := getAvailableFileName(dirParentPath, dirName+".tar.gz")
	if err != nil {
		return errors.Join(errors.New("failed to find available file name"), err)
	}
	filePath := path.Join(dirParentPath, fileName)

	err = execute.TarCompressDirectory(username, dirPath, filePath)
	if err != nil {
		return errors.Join(errors.New("failed to execute tar compress"), err)
	}

	return nil
}

func ExtractFile(username string, relHomePath string) error {
	filePath := path.Join("/home", username, relHomePath)
	fileParentPath, fileName := path.Split(filePath)
	fileNameNoExt, _ := getFileExtension(fileName)
	dirName, err := getAvailableFileName(fileParentPath, fileNameNoExt)
	if err != nil {
		return errors.Join(errors.New("failed to find available dir name"), err)
	}
	dirPath := path.Join(fileParentPath, dirName)

	err = execute.TarExtractFile(username, filePath, dirPath)
	if err != nil {
		return errors.Join(errors.New("failed to execute tar extract"), err)
	}

	return nil
}

func CreateRequiredFiles(username string) error {
	homePath := path.Join("/home", username)

	err := execute.MakeDirectory(username, path.Join(homePath, TRASH_HOME_PATH))
	if err != nil {
		return errors.Join(errors.New("failed to create trash directory"), err)
	}

	err = execute.TouchFile(username, path.Join(homePath, ".ssh", "authorized_keys"))
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

	err = execute.Move(username, sourcePath, destinationPath)
	if err != nil {
		return errors.Join(errors.New("failed to move files"), err)
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
	err = execute.MakeDirectory(username, trashTimestampPath)
	if err != nil {
		return errors.Join(errors.New("failed to create timestamp directory"), err)
	}

	restorePath, fileName := path.Split(rootDirPath)

	if fileName == trashRestorePathFileName {
		fileName, _ = strings.CutPrefix(fileName, ".")
	}

	err = execute.Move(username, rootDirPath, path.Join(trashTimestampPath, fileName))
	if err != nil {
		return errors.Join(errors.New("failed to move files"), err)
	}

	trashRestorePathFilePath := path.Join(trashTimestampPath, trashRestorePathFileName)
	err = execute.TouchFile(username, trashRestorePathFilePath)
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
	if !trashDirNameRegex.MatchString(trashDirName) {
		return errors.New("trash dir name is not valid")
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

	err = execute.MakeDirectory(username, restorePath)
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

		err = execute.Move(username, path.Join(trashDirPath, entry.Name()), path.Join(restorePath, restoreName))
		if err != nil {
			return errors.Join(errors.New("failed to move files"), err)
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
