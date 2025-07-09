package server

import (
	"context"
	"embed"
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"

	"github.com/grantfbarnes/ground/internal/auth"
)

type requestContextKey string

const basePageDataRequestContextKey requestContextKey = "basePageDataRequestContextKey"
const usernameRequestContextKey requestContextKey = "usernameRequestContextKey"

//go:embed templates
var templates embed.FS

//go:embed static
var static embed.FS

func Run() {
	defer func() {
		if err := recover(); err != nil {
			log.Println("panic occurred:", err)
		}
	}()

	// static files
	http.HandleFunc("GET /static/{fileType}/{fileName}", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFileFS(w, r, static, strings.TrimPrefix(r.URL.Path, "/"))
	})

	// pages
	http.Handle("GET /{$}", middlewareForPages(http.HandlerFunc(getPageHome)))
	http.Handle("GET /login", middlewareForPages(http.HandlerFunc(getPageLogin)))
	http.Handle("GET /directory/", middlewareForPages(http.HandlerFunc(getPageDirectory)))
	http.Handle("GET /", middlewareForPages(http.HandlerFunc(getPage404)))

	// apis
	http.Handle("POST /login", middlewareForAPIs(http.HandlerFunc(checkLogin)))
	http.Handle("POST /download/", middlewareForAPIs(http.HandlerFunc(downloadFile)))

	port := ":3478"
	log.Printf("server is running on http://localhost%s...\n", port)
	err := http.ListenAndServe(port, nil)
	if err != nil {
		log.Fatalln(err)
	}
}

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

func middlewareForAPIs(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username, err := auth.GetUsername(r)
		if err == nil {
			r = r.WithContext(context.WithValue(r.Context(), usernameRequestContextKey, username))
		}
		next.ServeHTTP(w, r)
	})
}

func getUsername(r *http.Request) string {
	return r.Context().Value(usernameRequestContextKey).(string)
}

func downloadFile(w http.ResponseWriter, r *http.Request) {
	filePath := strings.TrimPrefix(r.URL.Path, "/download")
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("Provided path not found."))
		return
	}

	if fileInfo.IsDir() {
		w.WriteHeader(http.StatusNotAcceptable)
		_, _ = w.Write([]byte("Provided path is a directory."))
		return
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", "attachment; filename="+strconv.Quote(fileInfo.Name()))

	http.ServeFile(w, r, filePath)
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

func checkLogin(w http.ResponseWriter, r *http.Request) {
	type bodyStruct struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	var body bodyStruct

	defer r.Body.Close()

	err := json.NewDecoder(r.Body).Decode(&body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("Invalid body provided."))
		return
	}

	if body.Username == "" {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("No username found."))
		return
	}

	if body.Password == "" {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("No password found."))
		return
	}

	if !auth.CredentialsAreValid(body.Username, body.Password) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte("Invalid login credentials provided."))
		return
	}

	auth.SetUsername(w, body.Username)

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("Login credentials valid."))
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
