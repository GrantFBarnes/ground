package filesystem

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"regexp"
	"strconv"
	"strings"

	"github.com/grantfbarnes/ground/internal/system/users"
)

const TRASH_HOME_PATH string = ".local/share/ground/trash"
const trashRestorePathFileName string = ".ground-trash-restore-path"
const displayTimeLayout string = "2006-01-02 03:04:05 PM"
const systemTimeLayout string = "20060102150405.000"

var fileCopyNameRegex *regexp.Regexp
var trashDirNameRegex *regexp.Regexp

func SetupFileCopyNameRegex() error {
	re, err := regexp.Compile(`(.*)\(([0-9]+)\)$`)
	if err != nil {
		return errors.Join(errors.New("failed to compile regex"), err)
	}
	fileCopyNameRegex = re
	return nil
}

func SetupTrashNameDirRegex() error {
	re, err := regexp.Compile(`^[0-9]{14}\.[0-9]{3}$`)
	if err != nil {
		return errors.Join(errors.New("failed to compile regex"), err)
	}
	trashDirNameRegex = re
	return nil
}

func getTopLevelDirName(fullPath string) string {
	parts := strings.SplitSeq(fullPath, string(os.PathSeparator))
	for p := range parts {
		if p == "" {
			continue
		}
		return p
	}
	return ""
}

func getFileLines(filePath string) ([]string, error) {
	var lines []string

	file, err := os.Open(filePath)
	if err != nil {
		return lines, errors.Join(errors.New("failed to open file"), err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		return lines, errors.Join(errors.New("failed to scan file"), err)
	}

	return lines, nil
}

func getAvailableFileName(fileDirPath string, fileName string) (string, error) {
	for {
		filePath := path.Join(fileDirPath, fileName)
		_, err := os.Stat(filePath)
		if err != nil {
			// file does not exist, return available name
			return fileName, nil
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

			if fileName == fileNameNoExt {
				return "", errors.New("failed to get copy file name")
			}
		} else {
			fileName = fmt.Sprintf("%s(1)%s", fileNameNoExt, fileExt)
		}
	}
}

func getFileExtension(fileName string) (string, string) {
	split := strings.Split(fileName, ".")
	isDotFile := strings.HasPrefix(fileName, ".")
	if isDotFile {
		split = strings.Split(fileName[1:], ".")
	}

	if len(split) == 0 {
		return "", ""
	}

	coreFileName := split[0]
	fileExtension := strings.Join(split[1:], ".")
	if fileExtension != "" {
		fileExtension = "." + fileExtension
	}

	if isDotFile {
		coreFileName = "." + coreFileName
	}

	return coreFileName, fileExtension
}

func touch(filePath string, username string) error {
	filePath = path.Clean(filePath)

	_, err := os.Stat(filePath)
	if err == nil {
		// file already exists
		return nil
	}

	dirPath, _ := path.Split(filePath)
	err = mkdir(dirPath, username)
	if err != nil {
		return errors.Join(errors.New("failed to create parent directory"), err)
	}

	cmd := exec.Command("touch", filePath)

	err = users.ExecuteAs(cmd, username)
	if err != nil {
		return errors.Join(errors.New("failed to set command executor"), err)
	}

	err = cmd.Run()
	if err != nil {
		return errors.Join(errors.New("failed to create file"), err)
	}

	return nil
}

func mkdir(dirPath string, username string) error {
	dirPath = path.Clean(dirPath)

	cmd := exec.Command("mkdir", "-p", dirPath)

	err := users.ExecuteAs(cmd, username)
	if err != nil {
		return errors.Join(errors.New("failed to set command executor"), err)
	}

	err = cmd.Run()
	if err != nil {
		return errors.Join(errors.New("failed to create directory"), err)
	}

	return nil
}

func getHumanSize(isDir bool, size int64) string {
	if isDir {
		return "-"
	}

	if size > 1000000000 {
		return fmt.Sprintf("%.3f GB", float64(size)/1000000000.0)
	}

	if size > 1000000 {
		return fmt.Sprintf("%.3f MB", float64(size)/1000000.0)
	}

	if size > 1000 {
		return fmt.Sprintf("%.3f KB", float64(size)/1000.0)
	}

	return fmt.Sprintf("%d B", size)
}
