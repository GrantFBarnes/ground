package monitor

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

var diskSize string

func SetupDiskSize() error {
	cmd := exec.Command("df", "--human-readable", "--portability", "/home")
	outputBytes, err := cmd.Output()
	if err != nil {
		return errors.Join(errors.New("failed to run df"), err)
	}

	lines := strings.Split(string(outputBytes), "\n")
	if len(lines) < 2 {
		return errors.New("df output less than two lines")
	}

	fields := strings.Fields(lines[1])
	if len(fields) < 6 {
		return errors.New("df output less than six fields")
	}

	diskSize = fields[1]
	return nil
}

func GetUptime() (string, error) {
	cmd := exec.Command("uptime", "--pretty")
	outputBytes, err := cmd.Output()
	if err != nil {
		return "", errors.Join(errors.New("failed to run uptime"), err)
	}

	return string(outputBytes), nil
}

func GetDirectoryDiskUsage(dirPath string) string {
	directorySize := getDirectorySize(dirPath)
	return fmt.Sprintf("%s/%s", directorySize, diskSize)
}

func getDirectorySize(dirPath string) string {
	cmd := exec.Command("du", "--summarize", "--human-readable", dirPath)
	outputBytes, err := cmd.Output()
	if err != nil {
		return "?"
	}

	fields := strings.Fields(string(outputBytes))
	if len(fields) < 2 {
		return "?"
	}

	return fields[0]
}
