package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/grantfbarnes/ground/internal/server"
)

func main() {
	err := healthCheck()
	if err != nil {
		printErrorMessage(err.Error())
		os.Exit(1)
	}

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
			server.Run()
		default:
			printErrorMessage(fmt.Sprintf("Invalid argument provided: %s", os.Args[i]))
			os.Exit(1)
		}
	}
}

func healthCheck() error {
	if os.Getuid() != 0 {
		return errors.New("not running as root")
	}

	secret := os.Getenv("GROUND_SECRET")
	if secret == "" {
		return errors.New("no secret defined")
	}

	return nil
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
