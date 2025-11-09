package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"

	"github.com/grantfbarnes/ground/internal/server"
)

func main() {
	settings := getSettingsFromArguments()

	if settings.help {
		printHelp()
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
	help bool
	run  bool
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

	if missingRequiredDependencyProgram("su") {
		return errors.New("missing required dependency program 'su'")
	}

	if missingRequiredDependencyProgram("tar") {
		return errors.New("missing required dependency program 'tar'")
	}

	if missingRequiredDependencyProgram("mv") {
		return errors.New("missing required dependency program 'mv'")
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
  help: Print this message
  run:  Run web server

Arguments:
  -h, --help: Print this message
`)
}
