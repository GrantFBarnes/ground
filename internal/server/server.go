package server

import (
	"embed"
	"log"
	"net/http"
	"strings"
)

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

	// apis
	http.Handle("POST /login", middlewareForAPIs(http.HandlerFunc(checkLogin)))
	http.Handle("POST /download/", middlewareForAPIs(http.HandlerFunc(downloadFile)))

	// pages
	http.Handle("GET /{$}", middlewareForPages(http.HandlerFunc(getPageHome)))
	http.Handle("GET /login", middlewareForPages(http.HandlerFunc(getPageLogin)))
	http.Handle("GET /directory/", middlewareForPages(http.HandlerFunc(getPageDirectory)))
	http.Handle("GET /", middlewareForPages(http.HandlerFunc(getPage404)))

	port := ":3478"
	log.Printf("server is running on http://localhost%s...\n", port)
	err := http.ListenAndServe(port, nil)
	if err != nil {
		log.Fatalln(err)
	}
}
