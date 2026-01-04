package server

import (
	"embed"
	"errors"
	"log"
	"net"
	"net/http"
	"os"
	"strings"

	"github.com/grantfbarnes/ground/internal/api"
	"github.com/grantfbarnes/ground/internal/pages"
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
	http.HandleFunc("POST /api/login", api.Login)
	http.HandleFunc("POST /api/logout", api.Logout)
	http.Handle("POST /api/compress/", api.Middleware(http.HandlerFunc(api.CompressDirectory)))
	http.Handle("POST /api/upload/", api.Middleware(http.HandlerFunc(api.UploadFiles)))
	http.Handle("GET /api/download/", api.Middleware(http.HandlerFunc(api.Download)))
	http.Handle("POST /api/trash/", api.Middleware(http.HandlerFunc(api.Trash)))
	http.Handle("DELETE /api/trash", api.Middleware(http.HandlerFunc(api.EmptyTrash)))
	http.Handle("POST /api/system/reboot", api.Middleware(http.HandlerFunc(api.SystemReboot)))
	http.Handle("POST /api/system/poweroff", api.Middleware(http.HandlerFunc(api.SystemPoweroff)))
	http.Handle("POST /api/user/{username}/create", api.Middleware(http.HandlerFunc(api.CreateUser)))
	http.Handle("POST /api/user/{username}/password/reset", api.Middleware(http.HandlerFunc(api.ResetUserPassword)))
	http.Handle("POST /api/user/{username}/ssh-key", api.Middleware(http.HandlerFunc(api.AddUserSshKey)))
	http.Handle("DELETE /api/user/{username}/ssh-key/{lineNumber}", api.Middleware(http.HandlerFunc(api.DeleteUserSshKey)))
	http.Handle("DELETE /api/user/{username}", api.Middleware(http.HandlerFunc(api.DeleteUser)))

	// pages
	http.Handle("GET /{$}", pages.Middleware(http.HandlerFunc(pages.Home)))
	http.Handle("GET /login", pages.Middleware(http.HandlerFunc(pages.Login)))
	http.Handle("GET /admin", pages.Middleware(http.HandlerFunc(pages.Admin)))
	http.Handle("GET /user/{username}", pages.Middleware(http.HandlerFunc(pages.User)))
	http.Handle("GET /files/", pages.Middleware(http.HandlerFunc(pages.Files)))
	http.Handle("GET /file/", pages.Middleware(http.HandlerFunc(pages.File)))
	http.Handle("GET /trash/", pages.Middleware(http.HandlerFunc(pages.Trash)))
	http.Handle("GET /", pages.Middleware(http.HandlerFunc(pages.NotFound)))

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
