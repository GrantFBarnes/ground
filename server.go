package main

import (
	"embed"
	"html/template"
	"log"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
)

//go:embed templates
var templates embed.FS

//go:embed static
var static embed.FS

func run() {
	defer func() {
		if err := recover(); err != nil {
			log.Println("panic occurred:", err)
		}
	}()

	http.HandleFunc("GET /static/{fileType}/{fileName}", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFileFS(w, r, static, strings.TrimPrefix(r.URL.Path, "/"))
	})

	http.HandleFunc("GET /download/", downloadFile)
	http.HandleFunc("GET /directory/", getPageDirectory)
	http.HandleFunc("GET /login/check", checkLogin)
	http.HandleFunc("GET /login", getPageLogin)
	http.HandleFunc("GET /{$}", getPageHome)
	http.HandleFunc("GET /", getPage404)

	port := ":3478"
	log.Printf("server is running on http://localhost%s...\n", port)
	err := http.ListenAndServe(port, nil)
	if err != nil {
		log.Fatalln(err)
	}
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
	err := r.ParseForm()
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("Failed to parse form."))
		return
	}

	var username string
	var password string
	for key, val := range r.Form {
		if key == "username" {
			username = val[0]
		} else if key == "password" {
			password = val[0]
		}
	}

	if username == "" {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("No username found."))
		return
	}

	if password == "" {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("No password found."))
		return
	}

	valid, err := loginIsValid(username, password)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("Failed to check login credentials."))
		return
	}

	if !valid {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte("Invalid login credentials provided."))
		return
	}

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
