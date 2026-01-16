package filesystem

import (
	"os"
	"path"
	"strings"
)

type FilePathBreadcrumb struct {
	Name   string
	Path   string
	IsHome bool
}

func GetFileBreadcrumbs(relPath string) []FilePathBreadcrumb {
	return getBreadcrumbs("home", relPath)
}

func GetTrashBreadcrumbs(relPath string) []FilePathBreadcrumb {
	return getBreadcrumbs("trash", relPath)
}

func getBreadcrumbs(homeName string, relPath string) []FilePathBreadcrumb {
	breadcrumbPath := "/"
	FilePathBreadcrumbs := []FilePathBreadcrumb{
		{
			Name:   homeName,
			Path:   breadcrumbPath,
			IsHome: true,
		},
	}

	for breadcrumbDir := range strings.SplitSeq(relPath, string(os.PathSeparator)) {
		if breadcrumbDir == "" {
			continue
		}

		if homeName == "trash" && breadcrumbPath == "/" {
			breadcrumbPath = path.Join(breadcrumbPath, breadcrumbDir)
		} else {
			breadcrumbPath = path.Join(breadcrumbPath, breadcrumbDir)
			FilePathBreadcrumbs = append(FilePathBreadcrumbs, FilePathBreadcrumb{
				Name:   breadcrumbDir,
				Path:   breadcrumbPath,
				IsHome: false,
			})
		}
	}

	return FilePathBreadcrumbs
}
