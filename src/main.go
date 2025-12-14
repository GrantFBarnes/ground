package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"

	"github.com/grantfbarnes/ground/internal/server"
)

const VERSION string = "v0.0.2"

func main() {
	settings := getSettingsFromArguments()

	if settings.help {
		printHelp()
		os.Exit(0)
	}

	if settings.version {
		fmt.Println(VERSION)
		os.Exit(0)
	}

	if !settings.run {
		printErrorMessage("nothing to run")
		os.Exit(1)
	}

	err := healthCheck()
	if err != nil {
		printErrorMessage(err.Error())
		os.Exit(1)
	}

	server.Run()
}

type settings struct {
	help    bool
	version bool
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

		case "run":
			args.run = true
		}
	}
	return args
}

func healthCheck() error {
	if os.Getuid() != 0 {
		return errors.New("not running as root")
	}

	dependencies := []string{
		"su",
		"sudo",
		"tar",
		"mv",
		"uptime",
		"systemctl",
	}
	for _, dependency := range dependencies {
		if missingRequiredDependencyProgram(dependency) {
			return errors.New("missing required dependency program '" + dependency + "'")
		}
	}

	return nil
}

func missingRequiredDependencyProgram(name string) bool {
	_, err := exec.LookPath(name)
	return err != nil
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
  help:    Print this message
  version: Print version
  run:     Run web server

Arguments:
  -h, --help:    Print this message
  -v, --version: Print version
`)
}
