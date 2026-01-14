package filesystem

import (
	"bufio"
	"errors"
	"fmt"
	"net/url"
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
	IsCompressed bool
	Name         string
	Path         string
	HumanSize    string
	LastModified string
	SymLinkPath  string
	UrlPath      string
}

type TrashEntryData struct {
	IsDir        bool
	IsCompressed bool
	Name         string
	Path         string
	HumanSize    string
	UrlPath      string
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

func GetDirectoryEntries(relDirPath string, rootDirPath string) ([]DirectoryEntryData, error) {
	dirEntries, err := os.ReadDir(rootDirPath)
	if err != nil {
		return nil, errors.Join(errors.New("failed to read directory"), err)
	}

	var entries []DirectoryEntryData
	for _, entry := range dirEntries {
		entry, err := getDirectoryEntry(entry, relDirPath, rootDirPath)
		if err != nil {
			continue
		}
		entries = append(entries, entry)
	}

	return sortDirectoryEntries(entries), nil
}

func getDirectoryEntry(dirEntry os.DirEntry, relDirPath string, rootDirPath string) (DirectoryEntryData, error) {
	entryInfo, err := dirEntry.Info()
	if err != nil {
		return DirectoryEntryData{}, errors.Join(errors.New("failed to get entry info"), err)
	}

	entry := DirectoryEntryData{
		IsDir:        dirEntry.IsDir(),
		IsCompressed: strings.HasSuffix(dirEntry.Name(), ".tar.gz"),
		Name:         dirEntry.Name(),
		Path:         path.Join(relDirPath, dirEntry.Name()),
		HumanSize:    getHumanSize(dirEntry.IsDir(), entryInfo.Size()),
		LastModified: entryInfo.ModTime().Format("2006-01-02 03:04:05 PM"),
	}

	entry.UrlPath, err = entry.getUrlPath()
	if err != nil {
		return DirectoryEntryData{}, errors.Join(errors.New("failed to get url path"), err)
	}

	symLinkPath, isSymLinkDir := entry.getSymLinkInfo(rootDirPath)
	entry.SymLinkPath = symLinkPath
	if isSymLinkDir {
		entry.IsDir = true
	}

	return entry, nil
}

func (entry DirectoryEntryData) getUrlPath() (string, error) {
	_, err := url.ParseRequestURI(entry.Path)
	if err != nil {
		return "", errors.Join(errors.New("failed to parse path to uri"), err)
	}

	if entry.IsDir {
		return url.JoinPath("/files", entry.Path)
	} else {
		return url.JoinPath("/file", entry.Path)
	}
}

func (entry DirectoryEntryData) getSymLinkInfo(rootPath string) (string, bool) {
	linkPath, err := os.Readlink(path.Join(rootPath, entry.Name))
	if err != nil {
		return "", false
	}

	if !strings.HasPrefix(linkPath, "/") {
		linkPath = path.Join(rootPath, linkPath)
	}

	linkInfo, err := os.Stat(linkPath)
	if err != nil {
		return "", false
	}

	return strings.TrimPrefix(linkPath, rootPath), linkInfo.IsDir()
}

func sortDirectoryEntries(entries []DirectoryEntryData) []DirectoryEntryData {
	sort.Slice(entries, func(i, j int) bool {
		a, b := entries[i], entries[j]

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

	return entries
}

func GetTrashEntries(relDirPath string, rootDirPath string) ([]TrashEntryData, error) {
	dirEntries, err := os.ReadDir(rootDirPath)
	if err != nil {
		return nil, errors.Join(errors.New("failed to read directory"), err)
	}

	var entries []TrashEntryData
	for _, entry := range dirEntries {
		entry, err := getTrashEntry(entry, relDirPath)
		if err != nil {
			continue
		}
		entries = append(entries, entry)
	}

	return sortTrashEntries(entries), nil
}

func getTrashEntry(dirEntry os.DirEntry, relDirPath string) (TrashEntryData, error) {
	entryInfo, err := dirEntry.Info()
	if err != nil {
		return TrashEntryData{}, errors.Join(errors.New("failed to get entry info"), err)
	}

	entry := TrashEntryData{
		IsDir:        dirEntry.IsDir(),
		IsCompressed: strings.HasSuffix(dirEntry.Name(), ".tar.gz"),
		Name:         dirEntry.Name(),
		Path:         path.Join(relDirPath, dirEntry.Name()),
		HumanSize:    getHumanSize(dirEntry.IsDir(), entryInfo.Size()),
	}

	entry.UrlPath, err = entry.getUrlPath()
	if err != nil {
		return TrashEntryData{}, errors.Join(errors.New("failed to get url path"), err)
	}
	entry.Path = path.Join("/", TRASH_HOME_PATH, entry.Path)

	return entry, nil
}

func (entry TrashEntryData) getUrlPath() (string, error) {
	_, err := url.ParseRequestURI(entry.Path)
	if err != nil {
		return "", errors.Join(errors.New("failed to parse path to uri"), err)
	}

	if entry.IsDir {
		return url.JoinPath("/trash", entry.Path)
	} else {
		return url.JoinPath("/file", TRASH_HOME_PATH, entry.Path)
	}
}

func sortTrashEntries(entries []TrashEntryData) []TrashEntryData {
	sort.Slice(entries, func(i, j int) bool {
		a, b := entries[i], entries[j]

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

	return entries
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

func GetFileBreadcrumbs(homeName string, relPath string) []FilePathBreadcrumb {
	breadcrumbPath := "/"
	FilePathBreadcrumbs := []FilePathBreadcrumb{
		{
			Name:   homeName,
			Path:   breadcrumbPath,
			IsHome: true,
		},
	}

	for breadcrumbDir := range strings.SplitSeq(relPath, string(filepath.Separator)) {
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
