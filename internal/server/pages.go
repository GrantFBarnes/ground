package server

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"os/exec"
	"path"
	"sort"
	"strings"

	"github.com/grantfbarnes/ground/internal/auth"
)

//go:embed templates
var templates embed.FS

type directoryEntryData struct {
	IsDir       bool
	Name        string
	Path        string
	Size        int64
	SymLinkPath string
	UrlPath     string
	HumanSize   string
}

type filePathBreadcrumb struct {
	Name   string
	Path   string
	IsHome bool
}

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

		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), CONTEXT_KEY_USERNAME, username)))
	})
}

func getFilesPage(w http.ResponseWriter, r *http.Request) {
	username := r.Context().Value(CONTEXT_KEY_USERNAME).(string)
	urlRelativePath := strings.TrimPrefix(r.URL.Path, "/files")
	urlRootPath := path.Join("/home", username, urlRelativePath)
	urlPathInfo, err := os.Stat(urlRootPath)
	if err != nil {
		getProblemPage(w, r, "the requested file path could not be found in your home directory")
		return
	}

	if !urlPathInfo.IsDir() {
		http.Redirect(w, r, path.Join("/file", urlRelativePath), http.StatusSeeOther)
		return
	}

	if urlTrashPath, ok := strings.CutPrefix(urlRelativePath, "/"+TRASH_HOME_PATH); ok {
		http.Redirect(w, r, path.Join("/trash", urlTrashPath), http.StatusSeeOther)
		return
	}

	directoryEntries, err := getDirectoryEntries(urlRelativePath, urlRootPath, false)
	if err != nil {
		getProblemPage(w, r, err.Error())
		return
	}

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
		Path:                urlRelativePath,
		FilePathBreadcrumbs: getBreadcrumbs("home", urlRelativePath),
		DirectoryEntries:    directoryEntries,
	})
}

func getFilePage(w http.ResponseWriter, r *http.Request) {
	username := r.Context().Value(CONTEXT_KEY_USERNAME).(string)
	urlRelativePath := strings.TrimPrefix(r.URL.Path, "/file")
	urlRootPath := path.Join("/home", username, urlRelativePath)
	urlPathInfo, err := os.Stat(urlRootPath)
	if err != nil {
		getProblemPage(w, r, "the requested file path could not be found in your home directory")
		return
	}

	if urlPathInfo.IsDir() {
		http.Redirect(w, r, path.Join("/files", urlRelativePath), http.StatusSeeOther)
		return
	}

	http.ServeFile(w, r, urlRootPath)
}

func getTrashPage(w http.ResponseWriter, r *http.Request) {
	username := r.Context().Value(CONTEXT_KEY_USERNAME).(string)
	urlRelativePath := strings.TrimPrefix(r.URL.Path, "/trash")
	urlRootPath := path.Join("/home", username, TRASH_HOME_PATH, urlRelativePath)
	urlPathInfo, err := os.Stat(urlRootPath)
	if err != nil || !urlPathInfo.IsDir() {
		http.Redirect(w, r, path.Join("/trash", path.Dir(urlRelativePath)), http.StatusSeeOther)
		return
	}

	directoryEntries, err := getDirectoryEntries(urlRelativePath, urlRootPath, true)
	if err != nil {
		getProblemPage(w, r, err.Error())
		return
	}

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
		Path:                urlRelativePath,
		FilePathBreadcrumbs: getBreadcrumbs("trash", urlRelativePath),
		DirectoryEntries:    directoryEntries,
	})
}

func getAdminPage(w http.ResponseWriter, r *http.Request) {
	username := r.Context().Value(CONTEXT_KEY_USERNAME).(string)

	if !auth.IsAdmin(username) {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	cmd := exec.Command("uptime", "--pretty")
	uptime, err := cmd.Output()
	if err != nil {
		getProblemPage(w, r, "failed to get server uptime")
		return
	}

	tmpl, err := template.ParseFS(
		templates,
		"templates/pages/base.html",
		"templates/pages/bodies/admin.html",
	)
	if err != nil {
		getProblemPage(w, r, "failed to generate html for the requested page")
		return
	}

	_ = tmpl.ExecuteTemplate(w, "base", struct {
		PageTitle string
		Username  string
		Uptime    string
	}{
		PageTitle: "Ground - Admin",
		Username:  username,
		Uptime:    string(uptime),
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
	username := r.Context().Value(CONTEXT_KEY_USERNAME).(string)

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
		IsAdmin   bool
	}{
		PageTitle: "Ground - Home",
		Username:  username,
		IsAdmin:   auth.IsAdmin(username),
	})
}

func get404Page(w http.ResponseWriter, r *http.Request) {
	getProblemPage(w, r, "the requested url path is not valid")
}

func getProblemPage(w http.ResponseWriter, r *http.Request, problemMessage string) {
	username := r.Context().Value(CONTEXT_KEY_USERNAME).(string)

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

func getDirectoryEntries(urlRelativePath string, urlRootPath string, isTrash bool) ([]directoryEntryData, error) {
	dirEntries, err := os.ReadDir(urlRootPath)
	if err != nil {
		return nil, errors.New("entries in the requested directory could not be read")
	}

	var directoryEntries []directoryEntryData
	for _, entry := range dirEntries {
		directoryEntry, err := getDirectoryEntry(entry, urlRelativePath, urlRootPath, isTrash)
		if err != nil {
			continue
		}
		directoryEntries = append(directoryEntries, directoryEntry)
	}
	return sortEntries(directoryEntries), nil
}

func getDirectoryEntry(entry os.DirEntry, urlRelativePath string, urlRootPath string, isTrash bool) (directoryEntryData, error) {
	entryInfo, err := entry.Info()
	if err != nil {
		return directoryEntryData{}, err
	}

	directoryEntry := directoryEntryData{
		IsDir: entry.IsDir(),
		Name:  entry.Name(),
		Path:  path.Join(urlRelativePath, entry.Name()),
		Size:  entryInfo.Size(),
	}

	if isTrash {
		directoryEntry.Path = path.Join("/", TRASH_HOME_PATH, directoryEntry.Path)
	}

	symLinkPath, isSymLinkDir := directoryEntry.getSymLinkInfo(urlRootPath)
	directoryEntry.SymLinkPath = symLinkPath
	if isSymLinkDir {
		directoryEntry.IsDir = true
	}
	directoryEntry.UrlPath = directoryEntry.getUrlPath(urlRelativePath, isTrash)
	directoryEntry.HumanSize = directoryEntry.getHumanSize()

	return directoryEntry, nil
}

func (directoryEntry directoryEntryData) getSymLinkInfo(urlRootPath string) (string, bool) {
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

func (directoryEntry directoryEntryData) getUrlPath(urlRelativePath string, isTrash bool) string {
	if !directoryEntry.IsDir {
		return path.Join("/file", directoryEntry.Path)
	}

	if isTrash {
		return path.Join("/trash", path.Join(urlRelativePath, directoryEntry.Name))
	}

	return path.Join("/files", directoryEntry.Path)
}

func (directoryEntry directoryEntryData) getHumanSize() string {
	if directoryEntry.IsDir {
		return "-"
	}

	if directoryEntry.Size > 1000000000 {
		return fmt.Sprintf("%.3f GB", float64(directoryEntry.Size)/1000000000.0)
	}

	if directoryEntry.Size > 1000000 {
		return fmt.Sprintf("%.3f MB", float64(directoryEntry.Size)/1000000.0)
	}

	if directoryEntry.Size > 1000 {
		return fmt.Sprintf("%.3f KB", float64(directoryEntry.Size)/1000.0)
	}

	return fmt.Sprintf("%d B", directoryEntry.Size)
}

func sortEntries(directoryEntries []directoryEntryData) []directoryEntryData {
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

func getBreadcrumbs(homeName string, urlPath string) []filePathBreadcrumb {
	breadcrumbPath := "/"
	filePathBreadcrumbs := []filePathBreadcrumb{
		{
			Name:   homeName,
			Path:   breadcrumbPath,
			IsHome: true,
		},
	}

	for breadcrumbDir := range strings.SplitSeq(urlPath, "/") {
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

	return filePathBreadcrumbs
}
