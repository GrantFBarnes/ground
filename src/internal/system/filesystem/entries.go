package filesystem

import (
	"errors"
	"net/url"
	"os"
	"path"
	"sort"
	"strings"
	"time"
)

type DirectoryEntryData struct {
	IsDir        bool
	IsCompressed bool
	IconName     string
	Name         string
	Path         string
	size         int64
	HumanSize    string
	time         time.Time
	LastModified string
	SymLinkPath  string
	UrlPath      string
}

type TrashEntryData struct {
	DirName      string
	IsDir        bool
	IsCompressed bool
	IconName     string
	Name         string
	Path         string
	size         int64
	HumanSize    string
	trashedTime  time.Time
	TrashedOn    string
	UrlPath      string
}

func GetDirectoryEntries(relDirPath string, rootDirPath string, sortBy string, sortOrder string) ([]DirectoryEntryData, error) {
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

	return sortDirectoryEntries(entries, sortBy, sortOrder), nil
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
		Path:         path.Join("/", relDirPath, dirEntry.Name()),
		size:         entryInfo.Size(),
		time:         entryInfo.ModTime(),
		LastModified: entryInfo.ModTime().Format(displayTimeLayout),
	}

	entry.UrlPath, err = entry.getUrlPath()
	if err != nil {
		return entry, errors.Join(errors.New("failed to get url path"), err)
	}

	symLinkPath, isSymLinkDir := entry.getSymLinkInfo(rootDirPath)
	entry.SymLinkPath = symLinkPath
	if isSymLinkDir {
		entry.IsDir = true
	}

	entry.HumanSize = getHumanSize(entry.IsDir, entry.size)
	entry.IconName = getEntryIconName(entry.IsDir, entry.Name)

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

func sortDirectoryEntries(entries []DirectoryEntryData, sortBy string, sortOrder string) []DirectoryEntryData {
	sortOrderDesc := strings.ToLower(sortOrder) == "desc"
	sort.Slice(entries, func(i, j int) bool {
		a, b := entries[i], entries[j]

		var res bool
		var err error

		switch strings.ToLower(sortBy) {
		case "type":
			res, err = sortEntriesByType(a.IsDir, b.IsDir, sortOrderDesc)
			if err == nil {
				return res
			}
		case "name":
			res, err = sortEntriesByName(a.Name, b.Name, sortOrderDesc)
			if err == nil {
				return res
			}
		case "link":
			res, err = sortEntriesByLink(a.SymLinkPath, b.SymLinkPath, sortOrderDesc)
			if err == nil {
				return res
			}
		case "size":
			res, err = sortEntriesBySize(a.size, b.size, a.IsDir, b.IsDir, sortOrderDesc)
			if err == nil {
				return res
			}
		case "time":
			res, err = sortEntriesByTime(a.time, b.time, sortOrderDesc)
			if err == nil {
				return res
			}
		}

		res, err = sortEntriesByType(a.IsDir, b.IsDir, false)
		if err == nil {
			return res
		}
		res, err = sortEntriesByName(a.Name, b.Name, false)
		if err == nil {
			return res
		}
		res, err = sortEntriesByTime(a.time, b.time, false)
		if err == nil {
			return res
		}
		res, err = sortEntriesBySize(a.size, b.size, a.IsDir, b.IsDir, false)
		if err == nil {
			return res
		}
		res, err = sortEntriesByLink(a.SymLinkPath, b.SymLinkPath, false)
		if err == nil {
			return res
		}

		return false
	})

	return entries
}

func GetTrashEntries(username string, relTrashPath string, sortBy string, sortOrder string) ([]TrashEntryData, error) {
	var entries []TrashEntryData
	var err error

	if relTrashPath == "/" {
		dirEntries, err := os.ReadDir(path.Join("/home", username, TRASH_HOME_PATH))
		if err != nil {
			return entries, errors.Join(errors.New("failed to read directory"), err)
		}

		for _, entry := range dirEntries {
			subEntries, err := getTrashPathEntries(username, entry.Name())
			if err != nil {
				return entries, errors.Join(errors.New("failed to get trash path entries"), err)
			}
			entries = append(entries, subEntries...)
		}
	} else {
		entries, err = getTrashPathEntries(username, relTrashPath)
		if err != nil {
			return entries, errors.Join(errors.New("failed to get trash path entries"), err)
		}
	}

	return sortTrashEntries(entries, sortBy, sortOrder), err
}

func getTrashPathEntries(username string, relTrashPath string) ([]TrashEntryData, error) {
	var entries []TrashEntryData

	trashDirName := getTopLevelDirName(relTrashPath)
	if !trashDirNameRegex.MatchString(trashDirName) {
		return entries, errors.New("trash dir name is not valid")
	}

	trashedTime, err := time.Parse(systemTimeLayout, trashDirName)
	if err != nil {
		return entries, errors.Join(errors.New("failed to parse trash time"), err)
	}
	trashedOn := trashedTime.Format(displayTimeLayout)

	dirEntries, err := os.ReadDir(path.Join("/home", username, TRASH_HOME_PATH, relTrashPath))
	if err != nil {
		return nil, errors.Join(errors.New("failed to read directory"), err)
	}

	for _, entry := range dirEntries {
		if entry.Name() == trashRestorePathFileName {
			continue
		}

		entry, err := getTrashEntry(entry, relTrashPath)
		if err != nil {
			continue
		}

		entry.DirName = trashDirName
		entry.trashedTime = trashedTime
		entry.TrashedOn = trashedOn

		entries = append(entries, entry)
	}

	return entries, nil
}

func getTrashEntry(dirEntry os.DirEntry, relTrashPath string) (TrashEntryData, error) {
	entryInfo, err := dirEntry.Info()
	if err != nil {
		return TrashEntryData{}, errors.Join(errors.New("failed to get entry info"), err)
	}

	entry := TrashEntryData{
		IsDir:        dirEntry.IsDir(),
		IsCompressed: strings.HasSuffix(dirEntry.Name(), ".tar.gz"),
		IconName:     getEntryIconName(dirEntry.IsDir(), dirEntry.Name()),
		Name:         dirEntry.Name(),
		Path:         path.Join("/", relTrashPath, dirEntry.Name()),
		size:         entryInfo.Size(),
		HumanSize:    getHumanSize(dirEntry.IsDir(), entryInfo.Size()),
	}

	entry.UrlPath, err = entry.getUrlPath()
	if err != nil {
		return entry, errors.Join(errors.New("failed to get url path"), err)
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

func sortTrashEntries(entries []TrashEntryData, sortBy string, sortOrder string) []TrashEntryData {
	sortOrderDesc := strings.ToLower(sortOrder) == "desc"
	sort.Slice(entries, func(i, j int) bool {
		a, b := entries[i], entries[j]

		var res bool
		var err error

		switch strings.ToLower(sortBy) {
		case "type":
			res, err = sortEntriesByType(a.IsDir, b.IsDir, sortOrderDesc)
			if err == nil {
				return res
			}
		case "name":
			res, err = sortEntriesByName(a.Name, b.Name, sortOrderDesc)
			if err == nil {
				return res
			}
		case "size":
			res, err = sortEntriesBySize(a.size, b.size, a.IsDir, b.IsDir, sortOrderDesc)
			if err == nil {
				return res
			}
		case "time":
			res, err = sortEntriesByTime(a.trashedTime, b.trashedTime, sortOrderDesc)
			if err == nil {
				return res
			}
		}

		res, err = sortEntriesByTime(a.trashedTime, b.trashedTime, false)
		if err == nil {
			return res
		}
		res, err = sortEntriesByType(a.IsDir, b.IsDir, false)
		if err == nil {
			return res
		}
		res, err = sortEntriesByName(a.Name, b.Name, false)
		if err == nil {
			return res
		}
		res, err = sortEntriesBySize(a.size, b.size, a.IsDir, b.IsDir, false)
		if err == nil {
			return res
		}

		return false
	})

	return entries
}

func getEntryIconName(isDir bool, name string) string {
	if isDir {
		switch strings.ToLower(name) {
		case "desktop":
			return "folder-desktop"
		case "documents":
			return "folder-documents"
		case "downloads":
			return "folder-downloads"
		case "music":
			return "folder-music"
		case "pictures":
			return "folder-pictures"
		case "videos":
			return "folder-videos"
		}

		return "folder"
	}

	_, fileExt := getFileExtension(name)
	switch strings.ToLower(fileExt) {
	case ".apng":
		fallthrough
	case ".avif":
		fallthrough
	case ".gif":
		fallthrough
	case ".jpeg":
		fallthrough
	case ".jpg":
		fallthrough
	case ".png":
		fallthrough
	case ".svg":
		fallthrough
	case ".webp":
		return "file-image"

	case ".avi":
		fallthrough
	case ".mkv":
		fallthrough
	case ".mov":
		fallthrough
	case ".mp4":
		fallthrough
	case ".mpeg":
		fallthrough
	case ".webm":
		fallthrough
	case ".wmv":
		return "file-video"

	case ".mp3":
		fallthrough
	case ".ogg":
		return "file-audio"

	case ".txt":
		fallthrough
	case ".md":
		return "file-text"

	case ".sh":
		fallthrough
	case ".ps1":
		fallthrough
	case ".bat":
		return "file-script"

	case ".html":
		return "file-html"

	case ".doc":
		fallthrough
	case ".docx":
		fallthrough
	case ".odt":
		return "file-document"

	case ".csv":
		fallthrough
	case ".xlsx":
		fallthrough
	case ".xlsm":
		fallthrough
	case ".xlsb":
		fallthrough
	case ".xltx":
		fallthrough
	case ".xltm":
		fallthrough
	case ".xls":
		fallthrough
	case ".xlt":
		return "file-spreadsheet"

	case ".pptx":
		fallthrough
	case ".pptm":
		return "file-slide"
	}

	return "file"
}

func sortEntriesByType(aIsDir bool, bIsDir bool, desc bool) (bool, error) {
	if aIsDir == bIsDir {
		return false, errors.New("same value")
	}

	if desc {
		return bIsDir, nil
	} else {
		return aIsDir, nil
	}
}

func sortEntriesByName(aName string, bName string, desc bool) (bool, error) {
	aLower := strings.ToLower(aName)
	bLower := strings.ToLower(bName)
	if aLower == bLower {
		return false, errors.New("same value")
	}

	aDot := strings.HasPrefix(aLower, ".")
	bDot := strings.HasPrefix(bLower, ".")
	if aDot != bDot {
		if desc {
			return aDot, nil
		} else {
			return bDot, nil
		}
	}

	if desc {
		return aLower > bLower, nil
	} else {
		return aLower < bLower, nil
	}
}

func sortEntriesByLink(aSymLinkPath string, bSymLinkPath string, desc bool) (bool, error) {
	aLink := aSymLinkPath != ""
	bLink := bSymLinkPath != ""
	if aLink == bLink {
		return false, errors.New("same value")
	}

	if desc {
		return bLink, nil
	} else {
		return aLink, nil
	}
}

func sortEntriesBySize(aSize int64, bSize int64, aIsDir bool, bIsDir bool, desc bool) (bool, error) {
	if aIsDir {
		aSize = 0
	}
	if bIsDir {
		bSize = 0
	}

	if aSize == bSize {
		return false, errors.New("same value")
	}

	if desc {
		return aSize < bSize, nil
	} else {
		return aSize > bSize, nil
	}
}

func sortEntriesByTime(aTime time.Time, bTime time.Time, desc bool) (bool, error) {
	if aTime.Equal(bTime) {
		return false, errors.New("same value")
	}

	if desc {
		return aTime.Before(bTime), nil
	} else {
		return aTime.After(bTime), nil
	}
}
