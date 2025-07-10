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

func middlewareForPages(next http.Handler) http.Handler {
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

func getPageFiles(w http.ResponseWriter, r *http.Request) {
	username, _ := auth.GetUsername(r)
	homePath := strings.TrimPrefix(r.URL.Path, "/files")
	fullPath := path.Join("/home", username, homePath)
	dirInfo, err := os.Stat(fullPath)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("Provided path not found."))
		return
	}

	if !dirInfo.IsDir() {
		http.Redirect(w, r, path.Join("/file", homePath), http.StatusSeeOther)
		return
	}

	dirEntries, err := os.ReadDir(fullPath)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("Failed to read directory entries."))
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
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("Failed to parse HTML."))
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

func getPageFile(w http.ResponseWriter, r *http.Request) {
	username, _ := auth.GetUsername(r)
	homePath := strings.TrimPrefix(r.URL.Path, "/file")
	fullPath := path.Join("/home", username, homePath)
	fileInfo, err := os.Stat(fullPath)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("Provided path not found."))
		return
	}

	if fileInfo.IsDir() {
		http.Redirect(w, r, path.Join("/files", homePath), http.StatusSeeOther)
		return
	}

	http.ServeFile(w, r, fullPath)
}

func getPageLogin(w http.ResponseWriter, r *http.Request) {
	tmpl, err := template.ParseFS(
		templates,
		"templates/pages/base.html",
		"templates/pages/bodies/login.html",
	)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("Failed to parse HTML."))
		return
	}

	_ = tmpl.ExecuteTemplate(w, "base", struct {
		PageTitle string
	}{
		PageTitle: "Ground - Login",
	})
}

func getPageHome(w http.ResponseWriter, r *http.Request) {
	tmpl, err := template.ParseFS(
		templates,
		"templates/pages/base.html",
		"templates/pages/bodies/home.html",
	)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("Failed to parse HTML."))
		return
	}

	_ = tmpl.ExecuteTemplate(w, "base", struct {
		PageTitle string
	}{
		PageTitle: "Ground - Home",
	})
}

func getPage404(w http.ResponseWriter, r *http.Request) {
	tmpl, err := template.ParseFS(
		templates,
		"templates/pages/base.html",
		"templates/pages/bodies/404.html",
	)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("Failed to parse HTML."))
		return
	}

	_ = tmpl.ExecuteTemplate(w, "base", struct {
		PageTitle string
	}{
		PageTitle: "Ground - 404 Not Found",
	})
}
