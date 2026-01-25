package pages

import (
	"embed"
	"html/template"
	"log/slog"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/grantfbarnes/ground/internal/server/common"
	"github.com/grantfbarnes/ground/internal/server/cookie"
	"github.com/grantfbarnes/ground/internal/system/filesystem"
	"github.com/grantfbarnes/ground/internal/system/monitor"
	"github.com/grantfbarnes/ground/internal/system/users"
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

		next.ServeHTTP(w, common.GetRequestWithRequestor(r, username))
	})
}

func Home(w http.ResponseWriter, r *http.Request) {
	requestor := common.GetRequestor(r)

	tmpl, err := template.ParseFS(
		templates,
		"templates/pages/base.html",
		"templates/pages/bodies/home.html",
	)
	if err != nil {
		slog.Error("failed to generate html", "ip", r.RemoteAddr, "request", r.URL.Path, "requestor", requestor, "error", err)
		getProblemPage(w, r, "There was a problem generating the HTML for the requested page.")
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

func Login(w http.ResponseWriter, r *http.Request) {
	tmpl, err := template.ParseFS(
		templates,
		"templates/pages/base.html",
		"templates/pages/bodies/login.html",
	)
	if err != nil {
		slog.Error("failed to generate html", "ip", r.RemoteAddr, "request", r.URL.Path, "error", err)
		getProblemPage(w, r, "There was a problem generating the HTML for the requested page.")
		return
	}

	_ = tmpl.ExecuteTemplate(w, "base", struct {
		PageTitle string
		Username  string
		IsAdmin   bool
	}{
		PageTitle: "Ground - Login",
		Username:  "",
		IsAdmin:   false,
	})
}

func Files(w http.ResponseWriter, r *http.Request) {
	requestor := common.GetRequestor(r)
	urlRelativePath := strings.TrimPrefix(r.URL.Path, "/files")
	searchFilter := r.URL.Query().Get("searchFilter")
	sortBy := r.URL.Query().Get("sortBy")
	sortOrder := r.URL.Query().Get("sortOrder")

	homePath := path.Join("/home", requestor)
	urlRootPath := path.Join(homePath, urlRelativePath)
	urlRootPath = path.Clean(urlRootPath)

	if !strings.HasPrefix(urlRootPath, homePath) {
		slog.Warn("path outside of home", "ip", r.RemoteAddr, "request", r.URL.Path, "requestor", requestor)
		getProblemPage(w, r, "The requested file path is not in your home directory.")
		return
	}

	urlPathInfo, err := os.Stat(urlRootPath)
	if err != nil {
		slog.Warn("failed to find path", "ip", r.RemoteAddr, "request", r.URL.Path, "requestor", requestor, "error", err)
		getProblemPage(w, r, "The requested file path could not be found in your home directory.")
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

	directoryEntries, err := filesystem.GetDirectoryEntries(urlRelativePath, urlRootPath, searchFilter, sortBy, sortOrder)
	if err != nil {
		slog.Error("failed to get directory entries", "ip", r.RemoteAddr, "request", r.URL.Path, "requestor", requestor, "error", err)
		getProblemPage(w, r, "There was a problem getting the directory entries for this requested file path.")
		return
	}

	tmpl, err := template.ParseFS(
		templates,
		"templates/pages/base.html",
		"templates/pages/bodies/files.html",
	)
	if err != nil {
		slog.Error("failed to generate html", "ip", r.RemoteAddr, "request", r.URL.Path, "requestor", requestor, "error", err)
		getProblemPage(w, r, "There was a problem generating the HTML for the requested page.")
		return
	}

	_ = tmpl.ExecuteTemplate(w, "base", struct {
		PageTitle           string
		Username            string
		IsAdmin             bool
		Path                string
		RootPath            string
		FilePathBreadcrumbs []filesystem.FilePathBreadcrumb
		DirectoryEntries    []filesystem.DirectoryEntryData
	}{
		PageTitle:           "Ground - Files",
		Username:            requestor,
		IsAdmin:             users.IsAdmin(requestor),
		Path:                urlRelativePath,
		RootPath:            urlRootPath,
		FilePathBreadcrumbs: filesystem.GetFileBreadcrumbs(urlRelativePath),
		DirectoryEntries:    directoryEntries,
	})
}

func File(w http.ResponseWriter, r *http.Request) {
	requestor := common.GetRequestor(r)
	urlRelativePath := strings.TrimPrefix(r.URL.Path, "/file")

	homePath := path.Join("/home", requestor)
	urlRootPath := path.Join(homePath, urlRelativePath)
	urlRootPath = path.Clean(urlRootPath)

	if !strings.HasPrefix(urlRootPath, homePath) {
		slog.Warn("path outside of home", "ip", r.RemoteAddr, "request", r.URL.Path, "requestor", requestor)
		getProblemPage(w, r, "The requested file path is not in your home directory.")
		return
	}

	urlPathInfo, err := os.Stat(urlRootPath)
	if err != nil {
		slog.Warn("failed to find path", "ip", r.RemoteAddr, "request", r.URL.Path, "requestor", requestor, "error", err)
		getProblemPage(w, r, "The requested file path could not be found in your home directory.")
		return
	}

	if urlPathInfo.IsDir() {
		http.Redirect(w, r, path.Join("/files", urlRelativePath), http.StatusSeeOther)
		return
	}

	http.ServeFile(w, r, urlRootPath)
}

func Trash(w http.ResponseWriter, r *http.Request) {
	requestor := common.GetRequestor(r)
	urlRelativePath := strings.TrimPrefix(r.URL.Path, "/trash")
	sortBy := r.URL.Query().Get("sortBy")
	sortOrder := r.URL.Query().Get("sortOrder")

	homePath := path.Join("/home", requestor, filesystem.TRASH_HOME_PATH)
	urlRootPath := path.Join(homePath, urlRelativePath)
	urlRootPath = path.Clean(urlRootPath)

	if !strings.HasPrefix(urlRootPath, homePath) {
		slog.Warn("path outside of home", "ip", r.RemoteAddr, "request", r.URL.Path, "requestor", requestor)
		getProblemPage(w, r, "The requested file path is not in your home directory.")
		return
	}

	urlPathInfo, err := os.Stat(urlRootPath)
	if err != nil || !urlPathInfo.IsDir() {
		http.Redirect(w, r, path.Join("/trash", path.Dir(urlRelativePath)), http.StatusSeeOther)
		return
	}

	trashEntries, err := filesystem.GetTrashEntries(requestor, urlRelativePath, sortBy, sortOrder)
	if err != nil {
		slog.Error("failed to get trash entries", "ip", r.RemoteAddr, "request", r.URL.Path, "requestor", requestor, "error", err)
		getProblemPage(w, r, "There was a problem getting the trash entries for this requested file path.")
		return
	}

	tmpl, err := template.ParseFS(
		templates,
		"templates/pages/base.html",
		"templates/pages/bodies/trash.html",
	)
	if err != nil {
		slog.Error("failed to generate html", "ip", r.RemoteAddr, "request", r.URL.Path, "requestor", requestor, "error", err)
		getProblemPage(w, r, "There was a problem generating the HTML for the requested page.")
		return
	}

	_ = tmpl.ExecuteTemplate(w, "base", struct {
		PageTitle           string
		Username            string
		IsAdmin             bool
		Path                string
		RootPath            string
		FilePathBreadcrumbs []filesystem.FilePathBreadcrumb
		TrashEntries        []filesystem.TrashEntryData
	}{
		PageTitle:           "Ground - Trash",
		Username:            requestor,
		IsAdmin:             users.IsAdmin(requestor),
		Path:                urlRelativePath,
		RootPath:            urlRootPath,
		FilePathBreadcrumbs: filesystem.GetTrashBreadcrumbs(urlRelativePath),
		TrashEntries:        trashEntries,
	})
}

func User(w http.ResponseWriter, r *http.Request) {
	requestor := common.GetRequestor(r)

	targetUsername := r.PathValue("username")

	if !users.UserIsValid(targetUsername) {
		slog.Warn("user is not valid", "ip", r.RemoteAddr, "request", r.URL.Path, "requestor", requestor)
		getProblemPage(w, r, "The requested user is not valid.")
		return
	}

	if requestor != targetUsername && !users.IsAdmin(requestor) {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	err := filesystem.CreateRequiredFiles(targetUsername)
	if err != nil {
		slog.Error("failed to create required files", "ip", r.RemoteAddr, "request", r.URL.Path, "requestor", requestor, "error", err)
		getProblemPage(w, r, "There was a problem creating the required files for this user.")
		return
	}

	sshKeys, err := filesystem.GetUserSshKeys(targetUsername)
	if err != nil {
		slog.Error("failed to get ssh keys", "ip", r.RemoteAddr, "request", r.URL.Path, "requestor", requestor, "error", err)
		getProblemPage(w, r, "There was a problem getting the SSH Keys for this user.")
		return
	}

	tmpl, err := template.ParseFS(
		templates,
		"templates/pages/base.html",
		"templates/pages/bodies/user.html",
	)
	if err != nil {
		slog.Error("failed to generate html", "ip", r.RemoteAddr, "request", r.URL.Path, "requestor", requestor, "error", err)
		getProblemPage(w, r, "There was a problem generating the HTML for the requested page.")
		return
	}

	_ = tmpl.ExecuteTemplate(w, "base", struct {
		PageTitle      string
		Username       string
		IsAdmin        bool
		TargetUsername string
		SshKeys        []string
	}{
		PageTitle:      "Ground - User Manage",
		Username:       requestor,
		IsAdmin:        users.IsAdmin(requestor),
		TargetUsername: targetUsername,
		SshKeys:        sshKeys,
	})
}

func Admin(w http.ResponseWriter, r *http.Request) {
	requestor := common.GetRequestor(r)

	if !users.IsAdmin(requestor) {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	userListItems, err := users.GetUserListItems()
	if err != nil {
		slog.Error("failed to users", "ip", r.RemoteAddr, "request", r.URL.Path, "requestor", requestor, "error", err)
		getProblemPage(w, r, "There was a problem getting the list of system users.")
		return
	}

	tmpl, err := template.ParseFS(
		templates,
		"templates/pages/base.html",
		"templates/pages/bodies/admin.html",
	)
	if err != nil {
		slog.Error("failed to generate html", "ip", r.RemoteAddr, "request", r.URL.Path, "requestor", requestor, "error", err)
		getProblemPage(w, r, "There was a problem generating the HTML for the requested page.")
		return
	}

	_ = tmpl.ExecuteTemplate(w, "base", struct {
		PageTitle     string
		Username      string
		IsAdmin       bool
		Uptime        string
		UserListItems []users.UserListItem
	}{
		PageTitle:     "Ground - Admin",
		Username:      requestor,
		IsAdmin:       users.IsAdmin(requestor),
		Uptime:        monitor.GetUptime(),
		UserListItems: userListItems,
	})
}

func NotFound(w http.ResponseWriter, r *http.Request) {
	getProblemPage(w, r, "The requested url path is not valid.")
}

func getProblemPage(w http.ResponseWriter, r *http.Request, problemMessage string) {
	requestor := common.GetRequestor(r)

	tmpl, err := template.ParseFS(
		templates,
		"templates/pages/base.html",
		"templates/pages/bodies/problem.html",
	)
	if err != nil {
		slog.Error("failed to generate html", "ip", r.RemoteAddr, "request", r.URL.Path, "requestor", requestor, "error", err)
		http.Error(w, "Failed to generate HTML.", http.StatusInternalServerError)
		return
	}

	_ = tmpl.ExecuteTemplate(w, "base", struct {
		PageTitle      string
		Username       string
		IsAdmin        bool
		ProblemMessage string
	}{
		PageTitle:      "Ground - Error",
		Username:       requestor,
		IsAdmin:        users.IsAdmin(requestor),
		ProblemMessage: problemMessage,
	})
}
