package main

import (
	"embed"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path"
	"strings"
)

//go:embed templates
var templates embed.FS

func main() {
	if len(os.Args) <= 1 {
		printErrorMessage("No arguments provided.")
		os.Exit(1)
	}

	for i := 1; i < len(os.Args); i++ {
		switch os.Args[i] {
		case "help":
			fallthrough
		case "--help":
			fallthrough
		case "-h":
			printHelp()
			os.Exit(0)
		case "run":
			run()
		default:
			printErrorMessage(fmt.Sprintf("Invalid argument provided: %s", os.Args[i]))
			os.Exit(1)
		}
	}
}

func printErrorMessage(msg string) {
	colorRed := "\x1b[31m"
	colorReset := "\x1b[0m"

	fmt.Printf("%sError:%s ", colorRed, colorReset)
	fmt.Println(msg)
	fmt.Println("Run with -h/--help to print help.")
}

func printHelp() {
	fmt.Print(`ground

Methods:
  help: Print this message
  run:  Run web server

Arguments:
  -h, --help: Print this message
`)
}

func run() {
	defer func() {
		if err := recover(); err != nil {
			log.Println("panic occurred:", err)
		}
	}()

	http.HandleFunc("GET /files/", getPageFiles)
	http.HandleFunc("GET /file/", getPageFile)
	http.HandleFunc("GET /{$}", getPageHome)
	http.HandleFunc("GET /", getPage404)

	port := ":3478"
	log.Printf("server is running on http://localhost%s...\n", port)
	err := http.ListenAndServe(port, nil)
	if err != nil {
		log.Fatalln(err)
	}
}

type FileRow struct {
	IsDir bool
	Name  string
	Path  string
}

func getPageFiles(w http.ResponseWriter, r *http.Request) {
	dirPath := strings.TrimPrefix(r.URL.Path, "/files")
	fileInfo, err := os.Stat(dirPath)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("Failed read path."))
		return
	}

	if !fileInfo.IsDir() {
		http.Redirect(w, r, path.Join("/file", dirPath), http.StatusSeeOther)
		return
	}

	dirEntries, err := os.ReadDir(dirPath)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("Failed read directory."))
		return
	}

	var fileRows []FileRow
	for _, entry := range dirEntries {
		row := FileRow{
			IsDir: entry.IsDir(),
			Name:  entry.Name(),
		}
		if row.IsDir {
			row.Path = path.Join("/files", dirPath, row.Name)
		} else {
			row.Path = path.Join("/file", dirPath, row.Name)
		}
		fileRows = append(fileRows, row)
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
		PageTitle string
		FileRows  []FileRow
	}{
		PageTitle: "Ground - Files",
		FileRows:  fileRows,
	})
}

func getPageFile(w http.ResponseWriter, r *http.Request) {
	filePath := strings.TrimPrefix(r.URL.Path, "/file")
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("Failed read path."))
		return
	}

	if fileInfo.IsDir() {
		http.Redirect(w, r, path.Join("/files", filePath), http.StatusSeeOther)
		return
	}

	http.ServeFile(w, r, filePath)
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
