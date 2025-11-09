package server

import (
	"context"
	"embed"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"path"
	"sort"
	"strings"

	"github.com/grantfbarnes/ground/internal/auth"
)

//go:embed templates
var templates embed.FS

func pageMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username, err := auth.GetUsername(r)
		loggedIn := err == nil
		if !loggedIn {
			auth.RemoveUsername(w)
		}

		if r.URL.Path == "/login" {
			if loggedIn {
				http.Redirect(w, r, auth.GetRedirectUrl(r), http.StatusSeeOther)
				return
			}
		} else if !loggedIn {
			auth.SetRedirectUrl(w, r.URL.Path)
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), usernameContextKey, username)))
	})
}

func getFilesPage(w http.ResponseWriter, r *http.Request) {
	username := r.Context().Value(usernameContextKey).(string)
	urlHomePath := strings.TrimPrefix(r.URL.Path, "/files")
	urlRootPath := path.Join("/home", username, urlHomePath)
	urlPathInfo, err := os.Stat(urlRootPath)
	if err != nil {
		getProblemPage(w, r, "the requested file path could not be found in your home directory")
		return
	}

	if !urlPathInfo.IsDir() {
		http.Redirect(w, r, path.Join("/file", urlHomePath), http.StatusSeeOther)
		return
	}

	if strings.HasPrefix(urlHomePath, "/"+trashHomePath) {
		http.Redirect(w, r, path.Join("/trash", strings.TrimPrefix(urlHomePath, "/"+trashHomePath)), http.StatusSeeOther)
		return
	}

	dirEntries, err := os.ReadDir(urlRootPath)
	if err != nil {
		getProblemPage(w, r, "entries in the requested directory could not be read")
		return
	}

	type filePathBreadcrumb struct {
		Name   string
		Path   string
		IsHome bool
	}

	breadcrumbPath := "/"
	filePathBreadcrumbs := []filePathBreadcrumb{
		{
			Name:   "home",
			Path:   breadcrumbPath,
			IsHome: true,
		},
	}
	for breadcrumbDir := range strings.SplitSeq(urlHomePath, "/") {
		if breadcrumbDir == "" {
			continue
		}

		breadcrumbPath = path.Join(breadcrumbPath, breadcrumbDir)
		filePathBreadcrumbs = append(filePathBreadcrumbs, filePathBreadcrumb{
			Name:   breadcrumbDir,
			Path:   breadcrumbPath,
			IsHome: false,
		})
	}

	type directoryEntryData struct {
		IsDir       bool
		Name        string
		Path        string
		Size        int64
		SymLinkPath string
		UrlPath     string
		HumanSize   string
	}

	var directoryEntries []directoryEntryData
	for _, entry := range dirEntries {
		entryInfo, err := entry.Info()
		if err != nil {
			continue
		}

		directoryEntry := directoryEntryData{
			IsDir: entry.IsDir(),
			Name:  entry.Name(),
			Path:  path.Join(urlHomePath, entry.Name()),
			Size:  entryInfo.Size(),
		}

		linkPath, err := os.Readlink(path.Join(urlRootPath, directoryEntry.Name))
		if err == nil {
			if !strings.HasPrefix(linkPath, "/") {
				linkPath = path.Join(urlRootPath, linkPath)
			}
			linkInfo, err := os.Stat(linkPath)
			if err == nil {
				if linkInfo.IsDir() {
					directoryEntry.IsDir = true
				}
				linkPath = strings.TrimPrefix(linkPath, urlRootPath)
				directoryEntry.SymLinkPath = linkPath
			}
		}

		if directoryEntry.IsDir {
			directoryEntry.UrlPath = path.Join("/files", directoryEntry.Path)
			directoryEntry.HumanSize = "-"
		} else {
			directoryEntry.UrlPath = path.Join("/file", directoryEntry.Path)
			if directoryEntry.Size > 1000000000 {
				directoryEntry.HumanSize = fmt.Sprintf("%.3f GB", float64(directoryEntry.Size)/1000000000.0)
			} else if directoryEntry.Size > 1000000 {
				directoryEntry.HumanSize = fmt.Sprintf("%.3f MB", float64(directoryEntry.Size)/1000000.0)
			} else if directoryEntry.Size > 1000 {
				directoryEntry.HumanSize = fmt.Sprintf("%.3f KB", float64(directoryEntry.Size)/1000.0)
			} else {
				directoryEntry.HumanSize = fmt.Sprintf("%d B", directoryEntry.Size)
			}
		}

		directoryEntries = append(directoryEntries, directoryEntry)
	}

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

	tmpl, err := template.ParseFS(
		templates,
		"templates/pages/base.html",
		"templates/pages/bodies/files.html",
	)
	if err != nil {
		getProblemPage(w, r, "failed to generate html for the requested page")
		return
	}

	_ = tmpl.ExecuteTemplate(w, "base", struct {
		PageTitle           string
		Username            string
		Path                string
		FilePathBreadcrumbs []filePathBreadcrumb
		DirectoryEntries    []directoryEntryData
	}{
		PageTitle:           "Ground - Files",
		Username:            username,
		Path:                urlHomePath,
		FilePathBreadcrumbs: filePathBreadcrumbs,
		DirectoryEntries:    directoryEntries,
	})
}

func getFilePage(w http.ResponseWriter, r *http.Request) {
	username := r.Context().Value(usernameContextKey).(string)
	urlHomePath := strings.TrimPrefix(r.URL.Path, "/file")
	urlRootPath := path.Join("/home", username, urlHomePath)
	urlPathInfo, err := os.Stat(urlRootPath)
	if err != nil {
		getProblemPage(w, r, "the requested file path could not be found in your home directory")
		return
	}

	if urlPathInfo.IsDir() {
		http.Redirect(w, r, path.Join("/files", urlHomePath), http.StatusSeeOther)
		return
	}

	http.ServeFile(w, r, urlRootPath)
}

func getTrashPage(w http.ResponseWriter, r *http.Request) {
	username := r.Context().Value(usernameContextKey).(string)
	urlHomePath := strings.TrimPrefix(r.URL.Path, "/trash")
	urlRootPath := path.Join("/home", username, trashHomePath, urlHomePath)
	urlPathInfo, err := os.Stat(urlRootPath)
	if err != nil || !urlPathInfo.IsDir() {
		http.Redirect(w, r, path.Join("/trash", path.Dir(urlHomePath)), http.StatusSeeOther)
		return
	}

	dirEntries, err := os.ReadDir(urlRootPath)
	if err != nil {
		getProblemPage(w, r, "entries in the requested directory could not be read")
		return
	}

	type filePathBreadcrumb struct {
		Name   string
		Path   string
		IsHome bool
	}

	breadcrumbPath := "/"
	filePathBreadcrumbs := []filePathBreadcrumb{
		{
			Name:   "trash",
			Path:   breadcrumbPath,
			IsHome: true,
		},
	}
	for breadcrumbDir := range strings.SplitSeq(urlHomePath, "/") {
		if breadcrumbDir == "" {
			continue
		}

		breadcrumbPath = path.Join(breadcrumbPath, breadcrumbDir)
		filePathBreadcrumbs = append(filePathBreadcrumbs, filePathBreadcrumb{
			Name:   breadcrumbDir,
			Path:   breadcrumbPath,
			IsHome: false,
		})
	}

	type directoryEntryData struct {
		IsDir       bool
		Name        string
		Path        string
		Size        int64
		SymLinkPath string
		UrlPath     string
		HumanSize   string
	}

	var directoryEntries []directoryEntryData
	for _, entry := range dirEntries {
		entryInfo, err := entry.Info()
		if err != nil {
			continue
		}

		directoryEntry := directoryEntryData{
			IsDir: entry.IsDir(),
			Name:  entry.Name(),
			Path:  path.Join("/", trashHomePath, urlHomePath, entry.Name()),
			Size:  entryInfo.Size(),
		}

		linkPath, err := os.Readlink(path.Join(urlRootPath, directoryEntry.Name))
		if err == nil {
			if !strings.HasPrefix(linkPath, "/") {
				linkPath = path.Join(urlRootPath, linkPath)
			}
			linkInfo, err := os.Stat(linkPath)
			if err == nil {
				if linkInfo.IsDir() {
					directoryEntry.IsDir = true
				}
				linkPath = strings.TrimPrefix(linkPath, urlRootPath)
				directoryEntry.SymLinkPath = linkPath
			}
		}

		if directoryEntry.IsDir {
			directoryEntry.UrlPath = path.Join("/trash", path.Join(urlHomePath, entry.Name()))
			directoryEntry.HumanSize = "-"
		} else {
			directoryEntry.UrlPath = path.Join("/file", directoryEntry.Path)
			if directoryEntry.Size > 1000000000 {
				directoryEntry.HumanSize = fmt.Sprintf("%.3f GB", float64(directoryEntry.Size)/1000000000.0)
			} else if directoryEntry.Size > 1000000 {
				directoryEntry.HumanSize = fmt.Sprintf("%.3f MB", float64(directoryEntry.Size)/1000000.0)
			} else if directoryEntry.Size > 1000 {
				directoryEntry.HumanSize = fmt.Sprintf("%.3f KB", float64(directoryEntry.Size)/1000.0)
			} else {
				directoryEntry.HumanSize = fmt.Sprintf("%d B", directoryEntry.Size)
			}
		}

		directoryEntries = append(directoryEntries, directoryEntry)
	}

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

	tmpl, err := template.ParseFS(
		templates,
		"templates/pages/base.html",
		"templates/pages/bodies/trash.html",
	)
	if err != nil {
		getProblemPage(w, r, "failed to generate html for the requested page")
		return
	}

	_ = tmpl.ExecuteTemplate(w, "base", struct {
		PageTitle           string
		Username            string
		Path                string
		FilePathBreadcrumbs []filePathBreadcrumb
		DirectoryEntries    []directoryEntryData
	}{
		PageTitle:           "Ground - Trash",
		Username:            username,
		Path:                urlHomePath,
		FilePathBreadcrumbs: filePathBreadcrumbs,
		DirectoryEntries:    directoryEntries,
	})
}

func getLoginPage(w http.ResponseWriter, r *http.Request) {
	tmpl, err := template.ParseFS(
		templates,
		"templates/pages/base.html",
		"templates/pages/bodies/login.html",
	)
	if err != nil {
		getProblemPage(w, r, "failed to generate html for the requested page")
		return
	}

	_ = tmpl.ExecuteTemplate(w, "base", struct {
		PageTitle string
		Username  string
	}{
		PageTitle: "Ground - Login",
		Username:  "",
	})
}

func getHomePage(w http.ResponseWriter, r *http.Request) {
	username := r.Context().Value(usernameContextKey).(string)

	tmpl, err := template.ParseFS(
		templates,
		"templates/pages/base.html",
		"templates/pages/bodies/home.html",
	)
	if err != nil {
		getProblemPage(w, r, "failed to generate html for the requested page")
		return
	}

	_ = tmpl.ExecuteTemplate(w, "base", struct {
		PageTitle string
		Username  string
	}{
		PageTitle: "Ground - Home",
		Username:  username,
	})
}

func get404Page(w http.ResponseWriter, r *http.Request) {
	getProblemPage(w, r, "the requested url path is not valid")
}

func getProblemPage(w http.ResponseWriter, r *http.Request, problemMessage string) {
	username := r.Context().Value(usernameContextKey).(string)

	tmpl, err := template.ParseFS(
		templates,
		"templates/pages/base.html",
		"templates/pages/bodies/problem.html",
	)
	if err != nil {
		http.Error(w, "Failed to generate HTML.", http.StatusInternalServerError)
		return
	}

	_ = tmpl.ExecuteTemplate(w, "base", struct {
		PageTitle      string
		Username       string
		ProblemMessage string
	}{
		PageTitle:      "Ground - Error",
		Username:       username,
		ProblemMessage: problemMessage,
	})
}
