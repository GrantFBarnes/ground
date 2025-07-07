package main

import (
	"embed"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
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

	// root path only
	http.HandleFunc("GET /{$}", getPageHome)

	// catch all other paths
	http.HandleFunc("GET /", getPage404)

	port := ":3478"
	log.Printf("server is running on http://localhost%s...\n", port)
	err := http.ListenAndServe(port, nil)
	if err != nil {
		log.Fatalln(err)
	}
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
