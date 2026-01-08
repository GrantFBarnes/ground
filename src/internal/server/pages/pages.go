package pages

import (
	"context"
	"embed"
	"html/template"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/grantfbarnes/ground/internal/filesystem"
	"github.com/grantfbarnes/ground/internal/server/cookie"
	"github.com/grantfbarnes/ground/internal/system"
	"github.com/grantfbarnes/ground/internal/users"
)

//go:embed templates
var templates embed.FS

func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username, err := cookie.GetUsername(r)
		loggedIn := err == nil
		if !loggedIn {
			cookie.RemoveUsername(w)
		}

		if r.URL.Path == "/login" {
			if loggedIn {
				http.Redirect(w, r, cookie.GetRedirectUrl(r), http.StatusSeeOther)
				return
			}
		} else if !loggedIn {
			cookie.SetRedirectUrl(w, r.URL.Path)
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), "requestor", username)))
	})
}

func Files(w http.ResponseWriter, r *http.Request) {
	requestor := users.GetRequestor(r)
	urlRelativePath := strings.TrimPrefix(r.URL.Path, "/files")
	urlRootPath := path.Join("/home", requestor, urlRelativePath)
	urlPathInfo, err := os.Stat(urlRootPath)
	if err != nil {
		getProblemPage(w, r, "the requested file path could not be found in your home directory")
		return
	}

	if !urlPathInfo.IsDir() {
		http.Redirect(w, r, path.Join("/file", urlRelativePath), http.StatusSeeOther)
		return
	}

	if urlTrashPath, ok := strings.CutPrefix(urlRelativePath, "/"+filesystem.TRASH_HOME_PATH); ok {
		http.Redirect(w, r, path.Join("/trash", urlTrashPath), http.StatusSeeOther)
		return
	}

	directoryEntries, err := filesystem.GetDirectoryEntries(urlRelativePath, urlRootPath, false)
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
		FilePathBreadcrumbs []filesystem.FilePathBreadcrumb
		DiskUsage           string
		DirectoryEntries    []filesystem.DirectoryEntryData
	}{
		PageTitle:           "Ground - Files",
		Username:            requestor,
		Path:                urlRelativePath,
		FilePathBreadcrumbs: filesystem.GetFileBreadcrumbs("home", urlRelativePath),
		DiskUsage:           system.GetDirectoryDiskUsage(urlRootPath),
		DirectoryEntries:    directoryEntries,
	})
}

func File(w http.ResponseWriter, r *http.Request) {
	requestor := users.GetRequestor(r)
	urlRelativePath := strings.TrimPrefix(r.URL.Path, "/file")
	urlRootPath := path.Join("/home", requestor, urlRelativePath)
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

func Trash(w http.ResponseWriter, r *http.Request) {
	requestor := users.GetRequestor(r)
	urlRelativePath := strings.TrimPrefix(r.URL.Path, "/trash")
	urlRootPath := path.Join("/home", requestor, filesystem.TRASH_HOME_PATH, urlRelativePath)
	urlPathInfo, err := os.Stat(urlRootPath)
	if err != nil || !urlPathInfo.IsDir() {
		http.Redirect(w, r, path.Join("/trash", path.Dir(urlRelativePath)), http.StatusSeeOther)
		return
	}

	directoryEntries, err := filesystem.GetDirectoryEntries(urlRelativePath, urlRootPath, true)
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
		FilePathBreadcrumbs []filesystem.FilePathBreadcrumb
		DiskUsage           string
		DirectoryEntries    []filesystem.DirectoryEntryData
	}{
		PageTitle:           "Ground - Trash",
		Username:            requestor,
		Path:                urlRelativePath,
		FilePathBreadcrumbs: filesystem.GetFileBreadcrumbs("trash", urlRelativePath),
		DiskUsage:           system.GetDirectoryDiskUsage(urlRootPath),
		DirectoryEntries:    directoryEntries,
	})
}

func Admin(w http.ResponseWriter, r *http.Request) {
	requestor := users.GetRequestor(r)

	if !users.IsAdmin(requestor) {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	uptime, err := system.GetUptime()
	if err != nil {
		getProblemPage(w, r, "failed to get server uptime")
		return
	}

	userListItems, err := users.GetUserListItems()
	if err != nil {
		getProblemPage(w, r, "failed to get users")
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
		PageTitle     string
		Username      string
		DiskUsage     string
		Uptime        string
		UserListItems []users.UserListItem
	}{
		PageTitle:     "Ground - Admin",
		Username:      requestor,
		DiskUsage:     system.GetDirectoryDiskUsage("/home"),
		Uptime:        uptime,
		UserListItems: userListItems,
	})
}

func User(w http.ResponseWriter, r *http.Request) {
	requestor := users.GetRequestor(r)

	targetUsername := r.PathValue("username")

	if !users.UserIsValid(targetUsername) {
		getProblemPage(w, r, "requested user is not valid")
		return
	}

	if requestor != targetUsername && !users.IsAdmin(requestor) {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	tmpl, err := template.ParseFS(
		templates,
		"templates/pages/base.html",
		"templates/pages/bodies/user.html",
	)
	if err != nil {
		getProblemPage(w, r, "failed to generate html for the requested page")
		return
	}

	_ = tmpl.ExecuteTemplate(w, "base", struct {
		PageTitle      string
		Username       string
		TargetUsername string
		SshKeys        []string
	}{
		PageTitle:      "Ground - User Manage",
		Username:       requestor,
		TargetUsername: targetUsername,
		SshKeys:        filesystem.GetUserSshKeys(targetUsername),
	})
}

func Login(w http.ResponseWriter, r *http.Request) {
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

func Home(w http.ResponseWriter, r *http.Request) {
	requestor := users.GetRequestor(r)

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
		Username:  requestor,
		IsAdmin:   users.IsAdmin(requestor),
	})
}

func NotFound(w http.ResponseWriter, r *http.Request) {
	getProblemPage(w, r, "the requested url path is not valid")
}

func getProblemPage(w http.ResponseWriter, r *http.Request, problemMessage string) {
	requestor := users.GetRequestor(r)

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
		Username:       requestor,
		ProblemMessage: problemMessage,
	})
}
