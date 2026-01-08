package filesystem

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const TRASH_HOME_PATH string = ".local/share/ground/trash"

var fileCopyNameRegex *regexp.Regexp

func SetupFileCopyNameRegex() error {
	re, err := regexp.Compile(`(.*)\(([0-9]+)\)$`)
	if err != nil {
		return errors.Join(errors.New("failed to compile regex"), err)
	}
	fileCopyNameRegex = re
	return nil
}

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

func GetDirectoryEntries(urlRelativePath string, urlRootPath string, isTrash bool) ([]DirectoryEntryData, error) {
	dirEntries, err := os.ReadDir(urlRootPath)
	if err != nil {
		return nil, errors.Join(errors.New("failed to read directory"), err)
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
		return DirectoryEntryData{}, errors.Join(errors.New("failed to get entry info"), err)
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
