package server

import (
	"context"
	"embed"
	"html/template"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/grantfbarnes/ground/internal/auth"
)

type requestContextKey string

const basePageDataRequestContextKey requestContextKey = "basePageDataRequestContextKey"

//go:embed templates
var templates embed.FS

type basePageData struct {
	PageTitle string
	Username  string
	LoggedIn  bool
}

func middlewareForPages(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data := basePageData{
			PageTitle: "Ground",
			Username:  "",
			LoggedIn:  false,
		}

		username, err := auth.GetUsername(r)
		if err != nil {
			auth.RemoveUsername(w)
		} else {
			data.Username = username
			data.LoggedIn = true
		}

		if r.URL.Path == "/login" {
			if data.LoggedIn {
				http.Redirect(w, r, auth.GetRedirectUrl(r), http.StatusSeeOther)
				return
			}
		} else if !data.LoggedIn {
			auth.SetRedirectUrl(w, r.URL.Path)
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		r = r.WithContext(context.WithValue(r.Context(), basePageDataRequestContextKey, data))

		next.ServeHTTP(w, r)
	})
}

func getBasePageData(r *http.Request) basePageData {
	return r.Context().Value(basePageDataRequestContextKey).(basePageData)
}

func getPageDirectory(w http.ResponseWriter, r *http.Request) {
	dirPath := strings.TrimPrefix(r.URL.Path, "/directory")
	fileInfo, err := os.Stat(dirPath)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("Provided path not found."))
		return
	}

	if !fileInfo.IsDir() {
		w.WriteHeader(http.StatusNotAcceptable)
		_, _ = w.Write([]byte("Provided path is not a directory."))
		return
	}

	dirEntries, err := os.ReadDir(dirPath)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("Failed read directory entries."))
		return
	}

	type rowData struct {
		Name string
		Path string
	}

	var directoryRows []rowData
	var fileRows []rowData

	if dirPath != "/" {
		directoryRows = append(directoryRows, rowData{
			Name: "..",
			Path: path.Join("/directory", dirPath, ".."),
		})
	}

	for _, entry := range dirEntries {
		if entry.IsDir() {
			row := rowData{
				Name: entry.Name(),
				Path: path.Join("/directory", dirPath, entry.Name()),
			}
			directoryRows = append(directoryRows, row)
		} else {
			row := rowData{
				Name: entry.Name(),
				Path: path.Join("/download", dirPath, entry.Name()),
			}
			fileRows = append(fileRows, row)
		}
	}

	tmpl, err := template.ParseFS(
		templates,
		"templates/pages/base.html",
		"templates/pages/bodies/directory.html",
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
		PageTitle:     "Ground - Directory",
		DirectoryRows: directoryRows,
		FileRows:      fileRows,
	})
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
		Body      template.HTML
	}{
		PageTitle: "Ground - Home",
		Body:      template.HTML("<h1>hello</h1>"),
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
