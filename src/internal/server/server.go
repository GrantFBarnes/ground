package server

import (
	"embed"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strings"

	"github.com/grantfbarnes/ground/internal/server/api"
	"github.com/grantfbarnes/ground/internal/server/pages"
)

//go:embed static
var static embed.FS

func Run() {
	defer func() {
		if err := recover(); err != nil {
			slog.Error("panic occured", "error", err)
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
	http.Handle("GET /api/download/", api.Middleware(http.HandlerFunc(api.DownloadFile)))
	http.Handle("POST /api/trash/", api.Middleware(http.HandlerFunc(api.Trash)))
	http.Handle("DELETE /api/trash", api.Middleware(http.HandlerFunc(api.EmptyTrash)))
	http.Handle("POST /api/system/reboot", api.Middleware(http.HandlerFunc(api.SystemReboot)))
	http.Handle("POST /api/system/poweroff", api.Middleware(http.HandlerFunc(api.SystemPoweroff)))
	http.Handle("POST /api/user/{username}/create", api.Middleware(http.HandlerFunc(api.CreateUser)))
	http.Handle("POST /api/user/{username}/toggle-admin", api.Middleware(http.HandlerFunc(api.ToggleAdmin)))
	http.Handle("POST /api/user/{username}/impersonate", api.Middleware(http.HandlerFunc(api.Impersonate)))
	http.Handle("POST /api/user/{username}/password/reset", api.Middleware(http.HandlerFunc(api.ResetUserPassword)))
	http.Handle("POST /api/user/{username}/ssh-key", api.Middleware(http.HandlerFunc(api.AddUserSshKey)))
	http.Handle("DELETE /api/user/{username}/ssh-key/{index}", api.Middleware(http.HandlerFunc(api.DeleteUserSshKey)))
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
		slog.Error("failed to get local ip", "error", err)
		os.Exit(1)
	}
	port := ":3478"
	slog.Info("server is running", "url", fmt.Sprintf("http://%s%s", ip, port))
	err = http.ListenAndServe(port, nil)
	if err != nil {
		slog.Error("failed to run server", "error", err)
		os.Exit(1)
	}
}

func getLocalIPv4() (net.IP, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return nil, errors.Join(errors.New("failed to get hostname"), err)
	}

	ips, err := net.LookupIP(hostname)
	if err != nil {
		return nil, errors.Join(errors.New("failed to get lookup ip"), err)
	}

	for _, ip := range ips {
		if ip.To4() != nil {
			return ip.To4(), nil
		}
	}

	return nil, errors.New("failed to find ip")
}
