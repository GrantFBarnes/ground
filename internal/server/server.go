package server

import (
	"embed"
	"errors"
	"log"
	"net"
	"net/http"
	"os"
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
	http.HandleFunc("POST /api/login", login)
	http.HandleFunc("POST /api/logout", logout)
	http.HandleFunc("POST /api/compress/", compressDirectory)
	http.HandleFunc("GET /api/download/", downloadFile)

	// pages
	http.Handle("GET /{$}", pageMiddleware(http.HandlerFunc(getHomePage)))
	http.Handle("GET /login", pageMiddleware(http.HandlerFunc(getLoginPage)))
	http.Handle("GET /files/", pageMiddleware(http.HandlerFunc(getFilesPage)))
	http.Handle("GET /file/", pageMiddleware(http.HandlerFunc(getFilePage)))
	http.Handle("GET /", pageMiddleware(http.HandlerFunc(get404Page)))

	ip, err := getLocalIPv4()
	if err != nil {
		log.Fatalln(err)
	}
	port := ":3478"
	log.Printf("server is running on http://%s%s...\n", ip, port)
	err = http.ListenAndServe(port, nil)
	if err != nil {
		log.Fatalln(err)
	}
}

func getLocalIPv4() (net.IP, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return nil, err
	}

	ips, err := net.LookupIP(hostname)
	if err != nil {
		return nil, err
	}

	for _, ip := range ips {
		if ip.To4() != nil {
			return ip.To4(), nil
		}
	}

	return nil, errors.New("ip not found")
}
