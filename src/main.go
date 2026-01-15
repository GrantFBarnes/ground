package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"

	"github.com/grantfbarnes/ground/internal/server"
	"github.com/grantfbarnes/ground/internal/server/cookie"
	"github.com/grantfbarnes/ground/internal/system/filesystem"
	"github.com/grantfbarnes/ground/internal/system/monitor"
	"github.com/grantfbarnes/ground/internal/system/users"
)

const VERSION string = "v0.2.8"

const COLOR_RED string = "\x1b[31m"
const COLOR_GREEN string = "\x1b[32m"
const COLOR_BLUE string = "\x1b[34m"
const COLOR_CYAN string = "\x1b[36m"
const COLOR_RESET string = "\x1b[0m"

func main() {
	var err error
	settings := getSettingsFromArguments()

	if settings.help {
		printHelp()
		os.Exit(0)
	}

	if settings.version {
		fmt.Println(VERSION)
		os.Exit(0)
	}

	if settings.service {
		err = printService()
		if err != nil {
			printErrorMessage(errors.Join(errors.New("failed to print service"), err).Error())
			os.Exit(1)
		}
		os.Exit(0)
	}

	if !settings.run {
		printErrorMessage("nothing to run")
		os.Exit(1)
	}

	err = healthCheck()
	if err != nil {
		printErrorMessage(errors.Join(errors.New("failed health check"), err).Error())
		os.Exit(1)
	}

	server.Run()
}

type settings struct {
	help    bool
	version bool
	service bool
	run     bool
}

func getSettingsFromArguments() settings {
	args := settings{}
	for i := 1; i < len(os.Args); i++ {
		switch os.Args[i] {
		case "help":
			fallthrough
		case "--help":
			fallthrough
		case "-h":
			args.help = true

		case "version":
			fallthrough
		case "--version":
			fallthrough
		case "-v":
			args.version = true

		case "service":
			args.service = true

		case "run":
			args.run = true
		}
	}
	return args
}

func healthCheck() (err error) {
	if os.Getuid() != 0 {
		return errors.New("not running as root")
	}

	dependencies := []string{
		"df",
		"du",
		"gpasswd",
		"groups",
		"passwd",
		"sed",
		"su",
		"systemctl",
		"tar",
		"uptime",
		"useradd",
		"userdel",
	}
	for _, dependency := range dependencies {
		if missingRequiredDependencyProgram(dependency) {
			return fmt.Errorf("missing required dependency program '%s'", dependency)
		}
	}

	err = cookie.SetupHashSecret()
	if err != nil {
		return errors.Join(errors.New("failed to setup hash secret"), err)
	}

	err = filesystem.SetupFileCopyNameRegex()
	if err != nil {
		return errors.Join(errors.New("failed to setup file copy name regex"), err)
	}

	err = filesystem.SetupSystemTimeLayoutRegex()
	if err != nil {
		return errors.Join(errors.New("failed to setup system time layout regex"), err)
	}

	err = filesystem.SetupSshKeyRegex()
	if err != nil {
		return errors.Join(errors.New("failed to setup ssh key regex"), err)
	}

	err = monitor.SetupDiskSize()
	if err != nil {
		return errors.Join(errors.New("failed to setup disk size"), err)
	}

	err = users.SetupUsernameRegex()
	if err != nil {
		return errors.Join(errors.New("failed to setup username regex"), err)
	}

	err = users.SetupAdminGroup()
	if err != nil {
		return errors.Join(errors.New("failed to setup admin group"), err)
	}

	return nil
}

func missingRequiredDependencyProgram(name string) bool {
	_, err := exec.LookPath(name)
	return err != nil
}

func printErrorMessage(msg string) {
	fmt.Printf("%sError:%s ", COLOR_RED, COLOR_RESET)
	fmt.Println(msg)
	fmt.Println("Run with -h/--help to print help.")
}

func printHelp() {
	fmt.Print(`ground

Methods:
  help:    Print this message
  version: Print version
  service: Print systemd service intructions
  run:     Run web server

Arguments:
  -h, --help:    Print this message
  -v, --version: Print version
`)
}

func printService() error {
	execPath, err := os.Executable()
	if err != nil {
		return errors.Join(errors.New("failed to get executable"), err)
	}

	servicePath := "/etc/systemd/system/ground.service"

	fmt.Println("The following instructions are to set up ground as a systemd service.")
	fmt.Println("Note, this is just an example, the actual service location/content can be modified.")
	fmt.Printf("Executable location: %s%s%s\n", COLOR_CYAN, execPath, COLOR_RESET)
	fmt.Printf("   Service location: %s%s%s", COLOR_CYAN, servicePath, COLOR_RESET)
	if _, err := os.Stat(servicePath); err == nil {
		fmt.Print(" (file already exists)")
	}
	fmt.Println()
	fmt.Println()

	fmt.Println("Example content of service file (uses current executable location):")
	fmt.Printf(`%s[Unit]
Description=Ground
After=network.target

[Service]
User=root
ExecStart=%s run
Restart=always

[Install]
WantedBy=multi-user.target
%s`, COLOR_BLUE, execPath, COLOR_RESET)

	fmt.Println()

	fmt.Println("After you have a service file defined, you can enable/start the service with the following:")
	fmt.Print(COLOR_GREEN)
	fmt.Println("sudo systemctl enable ground.service")
	fmt.Println("sudo systemctl start ground.service")
	fmt.Println("sudo systemctl reboot")
	fmt.Print(COLOR_RESET)

	fmt.Println()

	fmt.Println("You can stop/disable the service with the following:")
	fmt.Print(COLOR_GREEN)
	fmt.Println("sudo systemctl stop ground.service")
	fmt.Println("sudo systemctl disable ground.service")
	fmt.Print(COLOR_RESET)

	fmt.Println()

	fmt.Println("Simply update the binary to get a newer version running, no updates to service needed.")

	return nil
}
