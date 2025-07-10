package server

import (
	"embed"
	"html/template"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/grantfbarnes/ground/internal/auth"
)

//go:embed templates
var templates embed.FS

func pageMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		loggedIn := auth.IsLoggedIn(r)
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

		next.ServeHTTP(w, r)
	})
}

func getFilesPage(w http.ResponseWriter, r *http.Request) {
	username, _ := auth.GetUsername(r)
	homePath := strings.TrimPrefix(r.URL.Path, "/files")
	fullPath := path.Join("/home", username, homePath)
	dirInfo, err := os.Stat(fullPath)
	if err != nil {
		getErrorPage(w, template.HTML("<p>Could not find provided path.</p>"))
		return
	}

	if !dirInfo.IsDir() {
		http.Redirect(w, r, path.Join("/file", homePath), http.StatusSeeOther)
		return
	}

	dirEntries, err := os.ReadDir(fullPath)
	if err != nil {
		getErrorPage(w, template.HTML("<p>Could not read directory.</p>"))
		return
	}

	type rowData struct {
		Name        string
		ApiPath     string
		SymLinkPath string
	}

	var directoryRows []rowData
	var fileRows []rowData

	if homePath != "/" {
		directoryRows = append(directoryRows, rowData{
			Name:    "..",
			ApiPath: path.Join("/files", homePath, ".."),
		})
	}

	for _, entry := range dirEntries {
		linkPath, err := os.Readlink(path.Join(fullPath, entry.Name()))
		if err == nil {
			if !strings.HasPrefix(linkPath, "/") {
				linkPath = path.Join(fullPath, linkPath)
			}
			linkInfo, err := os.Stat(linkPath)
			if err == nil {
				if linkInfo.IsDir() {
					row := rowData{
						Name:        entry.Name(),
						ApiPath:     path.Join("/files", homePath, entry.Name()),
						SymLinkPath: linkPath,
					}
					directoryRows = append(directoryRows, row)
					continue
				} else {
					row := rowData{
						Name:        entry.Name(),
						ApiPath:     path.Join("/file", homePath, entry.Name()),
						SymLinkPath: linkPath,
					}
					fileRows = append(fileRows, row)
					continue
				}
			}
		}

		if entry.IsDir() {
			row := rowData{
				Name:    entry.Name(),
				ApiPath: path.Join("/files", homePath, entry.Name()),
			}
			directoryRows = append(directoryRows, row)
		} else {
			row := rowData{
				Name:    entry.Name(),
				ApiPath: path.Join("/file", homePath, entry.Name()),
			}
			fileRows = append(fileRows, row)
		}
	}

	tmpl, err := template.ParseFS(
		templates,
		"templates/pages/base.html",
		"templates/pages/bodies/files.html",
	)
	if err != nil {
		getErrorPage(w, template.HTML("<p>Failed to generate HTML.</p>"))
		return
	}

	_ = tmpl.ExecuteTemplate(w, "base", struct {
		PageTitle     string
		DirectoryRows []rowData
		FileRows      []rowData
	}{
		PageTitle:     "Ground - Files",
		DirectoryRows: directoryRows,
		FileRows:      fileRows,
	})
}

func getFilePage(w http.ResponseWriter, r *http.Request) {
	username, _ := auth.GetUsername(r)
	homePath := strings.TrimPrefix(r.URL.Path, "/file")
	fullPath := path.Join("/home", username, homePath)
	fileInfo, err := os.Stat(fullPath)
	if err != nil {
		getErrorPage(w, template.HTML("<p>Could not find provided path.</p>"))
		return
	}

	if fileInfo.IsDir() {
		http.Redirect(w, r, path.Join("/files", homePath), http.StatusSeeOther)
		return
	}

	http.ServeFile(w, r, fullPath)
}

func getLoginPage(w http.ResponseWriter, r *http.Request) {
	tmpl, err := template.ParseFS(
		templates,
		"templates/pages/base.html",
		"templates/pages/bodies/login.html",
	)
	if err != nil {
		getErrorPage(w, template.HTML("<p>Failed to generate HTML.</p>"))
		return
	}

	_ = tmpl.ExecuteTemplate(w, "base", struct {
		PageTitle string
	}{
		PageTitle: "Ground - Login",
	})
}

func getHomePage(w http.ResponseWriter, r *http.Request) {
	tmpl, err := template.ParseFS(
		templates,
		"templates/pages/base.html",
		"templates/pages/bodies/home.html",
	)
	if err != nil {
		getErrorPage(w, template.HTML("<p>Failed to generate HTML.</p>"))
		return
	}

	_ = tmpl.ExecuteTemplate(w, "base", struct {
		PageTitle string
	}{
		PageTitle: "Ground - Home",
	})
}

func get404Page(w http.ResponseWriter, r *http.Request) {
	getErrorPage(w, template.HTML("<p>404 - Path Not Found</p>"))
}

func getErrorPage(w http.ResponseWriter, errorHtml template.HTML) {
	tmpl, err := template.ParseFS(
		templates,
		"templates/pages/base.html",
		"templates/pages/bodies/error.html",
	)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("Failed to generate HTML."))
		return
	}

	_ = tmpl.ExecuteTemplate(w, "base", struct {
		PageTitle string
		Html      template.HTML
	}{
		PageTitle: "Ground - Error",
		Html:      errorHtml,
	})
}
