package filesystem

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/grantfbarnes/ground/internal/auth"
	"github.com/grantfbarnes/ground/internal/users"
)

const TRASH_HOME_PATH string = ".local/share/ground/trash"

var fileCopyNameRegex = regexp.MustCompile(`(.*)\(([0-9]+)\)$`)

type DirectoryEntryData struct {
	IsDir        bool
	isTrash      bool
	Name         string
	Path         string
	size         int64
	LastModified string
	SymLinkPath  string
	UrlPath      string
	HumanSize    string
}

type FilePathBreadcrumb struct {
	Name   string
	Path   string
	IsHome bool
}

func CompressDirectory(username string, urlRelativePath string) error {
	urlRootPath := path.Join("/home", username, urlRelativePath)
	urlPathInfo, err := os.Stat(urlRootPath)
	if err != nil {
		return errors.New("Path not found.")
	}

	if !urlPathInfo.IsDir() {
		return errors.New("Path is not a directory.")
	}

	dirPath, dirName := path.Split(urlRootPath)
	fileName, err := GetAvailableFileName(dirPath, dirName+".tar.gz")
	if err != nil {
		return errors.New("Failed to find available file name.")
	}
	filePath := path.Join(dirPath, fileName)

	_, err = os.Stat(filePath)
	if err == nil {
		return errors.New("File already exists.")
	}

	cmd := exec.Command("su", "-c", "tar -zchf '"+filePath+"' --directory='"+urlRootPath+"' .", username)
	err = cmd.Run()
	if err != nil {
		return errors.New("Failed to compress directory.")
	}

	return nil
}

func CreateTrashDirectory(username string) error {
	uid, gid, err := users.GetUserIds(username)
	if err != nil {
		return err
	}

	homePath := path.Join("/home", username)
	err = CreateMissingDirectories(homePath, TRASH_HOME_PATH, uid, gid)
	if err != nil {
		return errors.New("Failed to create ground trash.")
	}

	return nil
}

func Trash(username string, urlRelativePath string) error {
	urlRootPath := path.Join("/home", username, urlRelativePath)
	_, err := os.Stat(urlRootPath)
	if err != nil {
		return errors.New("Path not found.")
	}

	uid, gid, err := users.GetUserIds(username)
	if err != nil {
		return err
	}

	homePath := path.Join("/home", username)
	trashTimestampHomePath := path.Join(TRASH_HOME_PATH, time.Now().Format("20060102150405.000"), path.Dir(urlRelativePath))
	err = CreateMissingDirectories(homePath, trashTimestampHomePath, uid, gid)
	if err != nil {
		return errors.New("Failed to create missing directories.")
	}

	cmd := exec.Command("mv", urlRootPath, path.Join(homePath, trashTimestampHomePath))
	err = cmd.Run()
	if err != nil {
		return errors.New("Failed to move to trash.")
	}

	return nil
}

func EmptyTrash(username string) error {
	trashRootPath := path.Join("/home", username, TRASH_HOME_PATH)

	dirEntries, err := os.ReadDir(trashRootPath)
	if err != nil {
		return errors.New("Failed to read trash.")
	}

	for _, entry := range dirEntries {
		entryFullPath := path.Join(trashRootPath, entry.Name())
		err = os.RemoveAll(entryFullPath)
		if err != nil {
			return errors.New("Failed empty trash.")
		}
	}

	return nil
}

func GetUserSshKeys(username string) []string {
	sshKeyPath := path.Join("/home", username, ".ssh", "authorized_keys")
	sshKeys, _ := getFileLines(sshKeyPath)
	return sshKeys
}

func getFileLines(filePath string) ([]string, error) {
	var lines []string

	file, err := os.Open(filePath)
	if err != nil {
		return lines, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		return lines, err
	}

	return lines, nil
}

func AddUserSshKey(username string, targetUsername string, sshKey string) error {
	if !auth.IsAdmin(username) {
		return errors.New("Must be admin to add user SSH Keys.")
	}

	if sshKey == "" {
		return errors.New("SSH Key was not provided.")
	}

	homePath := path.Join("/home", targetUsername)
	sshKeyPath := path.Join(homePath, ".ssh", "authorized_keys")
	_, err := os.Stat(sshKeyPath)
	if err != nil {
		uid, gid, err := users.GetUserIds(targetUsername)
		if err != nil {
			return err
		}

		err = CreateMissingDirectories(homePath, ".ssh", uid, gid)
		if err != nil {
			return errors.New("Failed to create SSH folder.")
		}

		err = createMissingFile(sshKeyPath, uid, gid)
		if err != nil {
			return errors.New("Failed to create SSH file.")
		}
	}

	sshKeyFile, err := os.OpenFile(sshKeyPath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return errors.New("Failed to open SSH file.")
	}
	defer sshKeyFile.Close()

	_, err = sshKeyFile.WriteString(sshKey + "\n")
	if err != nil {
		return errors.New("Failed to write to SSH file.")
	}

	return nil
}

func DeleteUserSshKey(username string, targetUsername string, indexString string) error {
	if !auth.IsAdmin(username) {
		return errors.New("Must be admin to delete user SSH Keys.")
	}

	index, err := strconv.Atoi(indexString)
	if err != nil {
		return errors.New("Index is not a number.")
	}

	if index < 0 {
		return errors.New("Index is not valid.")
	}

	homePath := path.Join("/home", targetUsername)
	sshKeyPath := path.Join(homePath, ".ssh", "authorized_keys")
	_, err = os.Stat(sshKeyPath)
	if err != nil {
		return errors.New("SSH file does not exist.")
	}

	cmd := exec.Command("sed", "-i", fmt.Sprintf("%dd", index+1), sshKeyPath)

	err = cmd.Start()
	if err != nil {
		return errors.New("Failed to delete SSH Key.")
	}

	err = cmd.Wait()
	if err != nil {
		return errors.New("Failed to delete SSH Key.")
	}

	return nil
}

func GetAvailableFileName(fileDirPath string, fileName string) (string, error) {
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
				return "", errors.New("Failed to get copy file name.")
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
			return err
		}

		err = os.Chown(dirPath, uid, gid)
		if err != nil {
			return err
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
		return err
	}
	defer createdFile.Close()

	err = os.Chown(filePath, uid, gid)
	if err != nil {
		return err
	}

	return nil
}

func CreateMultipartFile(part *multipart.Part, filePath string, uid int, gid int) error {
	osFile, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer osFile.Close()

	_, err = io.Copy(osFile, part)
	if err != nil {
		return err
	}

	err = os.Chown(filePath, uid, gid)
	if err != nil {
		return err
	}

	return nil
}

func GetDirectoryEntries(urlRelativePath string, urlRootPath string, isTrash bool) ([]DirectoryEntryData, error) {
	dirEntries, err := os.ReadDir(urlRootPath)
	if err != nil {
		return nil, errors.New("entries in the requested directory could not be read")
	}

	var directoryEntries []DirectoryEntryData
	for _, entry := range dirEntries {
		directoryEntry, err := getDirectoryEntry(entry, urlRelativePath, urlRootPath, isTrash)
		if err != nil {
			continue
		}
		directoryEntries = append(directoryEntries, directoryEntry)
	}

	return sortDirectoryEntries(directoryEntries), nil
}

func getDirectoryEntry(entry os.DirEntry, urlRelativePath string, urlRootPath string, isTrash bool) (DirectoryEntryData, error) {
	entryInfo, err := entry.Info()
	if err != nil {
		return DirectoryEntryData{}, err
	}

	directoryEntry := DirectoryEntryData{
		IsDir:        entry.IsDir(),
		isTrash:      isTrash,
		Name:         entry.Name(),
		Path:         path.Join(urlRelativePath, entry.Name()),
		size:         entryInfo.Size(),
		LastModified: entryInfo.ModTime().Format("2006-01-02 03:04:05 PM"),
	}

	directoryEntry.UrlPath = directoryEntry.getUrlPath()
	directoryEntry.HumanSize = directoryEntry.getHumanSize()

	if directoryEntry.isTrash {
		directoryEntry.Path = path.Join("/", TRASH_HOME_PATH, directoryEntry.Path)
	} else {
		symLinkPath, isSymLinkDir := directoryEntry.getSymLinkInfo(urlRootPath)
		directoryEntry.SymLinkPath = symLinkPath
		if isSymLinkDir {
			directoryEntry.IsDir = true
		}
	}

	return directoryEntry, nil
}

func (directoryEntry DirectoryEntryData) getUrlPath() string {
	if directoryEntry.isTrash {
		if directoryEntry.IsDir {
			return path.Join("/trash", directoryEntry.Path)
		} else {
			return path.Join("/file", TRASH_HOME_PATH, directoryEntry.Path)
		}
	} else {
		if directoryEntry.IsDir {
			return path.Join("/files", directoryEntry.Path)
		} else {
			return path.Join("/file", directoryEntry.Path)
		}
	}
}

func (directoryEntry DirectoryEntryData) getHumanSize() string {
	if directoryEntry.IsDir {
		return "-"
	}

	if directoryEntry.size > 1000000000 {
		return fmt.Sprintf("%.3f GB", float64(directoryEntry.size)/1000000000.0)
	}

	if directoryEntry.size > 1000000 {
		return fmt.Sprintf("%.3f MB", float64(directoryEntry.size)/1000000.0)
	}

	if directoryEntry.size > 1000 {
		return fmt.Sprintf("%.3f KB", float64(directoryEntry.size)/1000.0)
	}

	return fmt.Sprintf("%d B", directoryEntry.size)
}

func (directoryEntry DirectoryEntryData) getSymLinkInfo(urlRootPath string) (string, bool) {
	linkPath, err := os.Readlink(path.Join(urlRootPath, directoryEntry.Name))
	if err != nil {
		return "", false
	}

	if !strings.HasPrefix(linkPath, "/") {
		linkPath = path.Join(urlRootPath, linkPath)
	}

	linkInfo, err := os.Stat(linkPath)
	if err != nil {
		return "", false
	}

	return strings.TrimPrefix(linkPath, urlRootPath), linkInfo.IsDir()
}

func sortDirectoryEntries(directoryEntries []DirectoryEntryData) []DirectoryEntryData {
	sort.Slice(directoryEntries, func(i, j int) bool {
		a, b := directoryEntries[i], directoryEntries[j]

		if a.IsDir != b.IsDir {
			return a.IsDir
		}

		aDot := strings.HasPrefix(a.Name, ".")
		bDot := strings.HasPrefix(b.Name, ".")
		if aDot != bDot {
			return bDot
		}

		return strings.ToLower(a.Name) < strings.ToLower(b.Name)
	})

	return directoryEntries
}

func GetFileBreadcrumbs(homeName string, urlPath string) []FilePathBreadcrumb {
	breadcrumbPath := "/"
	FilePathBreadcrumbs := []FilePathBreadcrumb{
		{
			Name:   homeName,
			Path:   breadcrumbPath,
			IsHome: true,
		},
	}

	for breadcrumbDir := range strings.SplitSeq(urlPath, string(filepath.Separator)) {
		if breadcrumbDir == "" {
			continue
		}

		breadcrumbPath = path.Join(breadcrumbPath, breadcrumbDir)
		FilePathBreadcrumbs = append(FilePathBreadcrumbs, FilePathBreadcrumb{
			Name:   breadcrumbDir,
			Path:   breadcrumbPath,
			IsHome: false,
		})
	}

	return FilePathBreadcrumbs
}
