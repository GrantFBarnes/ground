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

type contextKey string

const usernameContextKey contextKey = "username"

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
	homePath := strings.TrimPrefix(r.URL.Path, "/files")
	fullPath := path.Join("/home", username, homePath)
	dirInfo, err := os.Stat(fullPath)
	if err != nil {
		getProblemPage(w, r, "the requested file path could not be found in your home directory")
		return
	}

	if !dirInfo.IsDir() {
		http.Redirect(w, r, path.Join("/file", homePath), http.StatusSeeOther)
		return
	}

	dirEntries, err := os.ReadDir(fullPath)
	if err != nil {
		getProblemPage(w, r, "entries in the requested directory could not be read")
		return
	}

	type rowData struct {
		IsDir       bool
		Name        string
		Path        string
		SymLinkPath string
	}

	var rows []rowData

	if homePath != "/" {
		rows = append(rows, rowData{
			IsDir: true,
			Name:  "..",
			Path:  path.Join(homePath, ".."),
		})
	}

	for _, entry := range dirEntries {
		row := rowData{
			Name: entry.Name(),
			Path: path.Join(homePath, entry.Name()),
		}

		linkPath, err := os.Readlink(path.Join(fullPath, row.Name))
		if err == nil {
			if !strings.HasPrefix(linkPath, "/") {
				linkPath = path.Join(fullPath, linkPath)
			}
			linkInfo, err := os.Stat(linkPath)
			if err == nil {
				if linkInfo.IsDir() {
					row.IsDir = true
				}
				row.SymLinkPath = linkPath
			}
		}

		if entry.IsDir() {
			row.IsDir = true
		}

		rows = append(rows, row)
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
		PageTitle string
		Username  string
		Rows      []rowData
	}{
		PageTitle: "Ground - Files",
		Username:  username,
		Rows:      rows,
	})
}

func getFilePage(w http.ResponseWriter, r *http.Request) {
	username := r.Context().Value(usernameContextKey).(string)
	homePath := strings.TrimPrefix(r.URL.Path, "/file")
	fullPath := path.Join("/home", username, homePath)
	fileInfo, err := os.Stat(fullPath)
	if err != nil {
		getProblemPage(w, r, "the requested file path could not be found in your home directory")
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
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("Failed to generate HTML."))
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
