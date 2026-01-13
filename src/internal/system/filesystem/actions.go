package filesystem

import (
	"errors"
	"io"
	"mime/multipart"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/grantfbarnes/ground/internal/system/users"
)

func CreateDirectory(username string, relHomePath string, dirName string) error {
	rootDirPath := path.Join("/home", username, relHomePath)
	dirInfo, err := os.Stat(rootDirPath)
	if err != nil {
		return errors.Join(errors.New("failed to get path stat"), err)
	}

	if !dirInfo.IsDir() {
		return errors.New("path is not a directory")
	}

	uid, gid, err := users.GetUserIds(username)
	if err != nil {
		return errors.Join(errors.New("failed to get user ids"), err)
	}

	err = CreateMissingDirectories(rootDirPath, dirName, uid, gid)
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
	fileName, err := GetAvailableFileName(dirPath, dirName+".tar.gz")
	if err != nil {
		return errors.Join(errors.New("failed to find available file name"), err)
	}
	filePath := path.Join(dirPath, fileName)

	cmd := exec.Command("su", "-c", "tar -zchf '"+filePath+"' --directory='"+rootDirPath+"' .", username)
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

	uid, gid, err := users.GetUserIds(username)
	if err != nil {
		return errors.Join(errors.New("failed to get user ids"), err)
	}

	extractedDirName, err := GetAvailableFileName(fileDirPath, fileNameNoExt)
	if err != nil {
		return errors.Join(errors.New("failed to find available name"), err)
	}
	extractedDirPath := path.Join(fileDirPath, extractedDirName)

	err = CreateMissingDirectories(fileDirPath, extractedDirName, uid, gid)
	if err != nil {
		return errors.Join(errors.New("failed to create extract directory"), err)
	}

	cmd := exec.Command("su", "-c", "tar -xzf '"+rootFilePath+"' --directory='"+extractedDirPath+"'", username)
	err = cmd.Run()
	if err != nil {
		return errors.Join(errors.New("failed to extract file"), err)
	}

	return nil
}

func CreateRequiredFiles(username string) error {
	uid, gid, err := users.GetUserIds(username)
	if err != nil {
		return errors.Join(errors.New("failed to get user ids"), err)
	}

	homePath := path.Join("/home", username)

	err = CreateMissingDirectories(homePath, TRASH_HOME_PATH, uid, gid)
	if err != nil {
		return errors.Join(errors.New("failed to create trash directory"), err)
	}

	err = CreateMissingDirectories(homePath, ".ssh", uid, gid)
	if err != nil {
		return errors.Join(errors.New("failed to create ssh directory"), err)
	}

	err = createMissingFile(path.Join(homePath, ".ssh", "authorized_keys"), uid, gid)
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

	uid, gid, err := users.GetUserIds(username)
	if err != nil {
		return errors.Join(errors.New("failed to get user ids"), err)
	}

	trashRootPath := path.Join("/home", username, TRASH_HOME_PATH)
	trashTimestamp := time.Now().Format("20060102150405.000")
	err = CreateMissingDirectories(trashRootPath, trashTimestamp, uid, gid)
	if err != nil {
		return errors.Join(errors.New("failed to create missing directories"), err)
	}

	_, fileName := path.Split(rootDirPath)
	err = os.Rename(rootDirPath, path.Join(trashRootPath, trashTimestamp, fileName))
	if err != nil {
		return errors.Join(errors.New("failed to rename files"), err)
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

func CreateMissingDirectories(rootPath string, relDirPath string, uid int, gid int) error {
	relDirPathBuildUp := ""
	for dirName := range strings.SplitSeq(relDirPath, string(filepath.Separator)) {
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

		err = os.MkdirAll(dirPath, os.FileMode(0755))
		if err != nil {
			return errors.Join(errors.New("failed to make directories"), err)
		}

		err = os.Chown(dirPath, uid, gid)
		if err != nil {
			return errors.Join(errors.New("failed to change ownership"), err)
		}
	}
	return nil
}

func createMissingFile(filePath string, uid int, gid int) error {
	_, err := os.Stat(filePath)
	if err == nil {
		// file already exists
		return nil
	}

	createdFile, err := os.Create(filePath)
	if err != nil {
		return errors.Join(errors.New("failed to create file"), err)
	}
	defer createdFile.Close()

	err = os.Chown(filePath, uid, gid)
	if err != nil {
		return errors.Join(errors.New("failed to change ownership"), err)
	}

	return nil
}

func CreateMultipartFile(part *multipart.Part, filePath string, uid int, gid int) error {
	osFile, err := os.Create(filePath)
	if err != nil {
		return errors.Join(errors.New("failed to create file"), err)
	}
	defer osFile.Close()

	_, err = io.Copy(osFile, part)
	if err != nil {
		return errors.Join(errors.New("failed to copy file data"), err)
	}

	err = os.Chown(filePath, uid, gid)
	if err != nil {
		return errors.Join(errors.New("failed to change ownership"), err)
	}

	return nil
}
