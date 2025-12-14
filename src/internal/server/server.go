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

type contextKey string

const CONTEXT_KEY_USERNAME contextKey = "username"
const TRASH_HOME_PATH string = ".local/share/ground/trash"

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
	http.Handle("POST /api/compress/", apiMiddleware(http.HandlerFunc(compressDirectory)))
	http.Handle("POST /api/upload/", apiMiddleware(http.HandlerFunc(uploadFiles)))
	http.Handle("GET /api/download/", apiMiddleware(http.HandlerFunc(download)))
	http.Handle("POST /api/trash/", apiMiddleware(http.HandlerFunc(trash)))
	http.Handle("DELETE /api/trash", apiMiddleware(http.HandlerFunc(emptyTrash)))
	http.Handle("POST /api/system/{method}", apiMiddleware(http.HandlerFunc(systemCallMethod)))

	// pages
	http.Handle("GET /{$}", pageMiddleware(http.HandlerFunc(getHomePage)))
	http.Handle("GET /login", pageMiddleware(http.HandlerFunc(getLoginPage)))
	http.Handle("GET /admin", pageMiddleware(http.HandlerFunc(getAdminPage)))
	http.Handle("GET /files/", pageMiddleware(http.HandlerFunc(getFilesPage)))
	http.Handle("GET /file/", pageMiddleware(http.HandlerFunc(getFilePage)))
	http.Handle("GET /trash/", pageMiddleware(http.HandlerFunc(getTrashPage)))
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
