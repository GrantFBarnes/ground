package system

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/grantfbarnes/ground/internal/auth"
)

func GetUptime() (string, error) {
	cmd := exec.Command("uptime", "--pretty")
	outputBytes, err := cmd.Output()
	if err != nil {
		return "", err
	}

	return string(outputBytes), nil
}

func GetDirectoryDiskUsage(dirPath string) string {
	directorySize := getDirectorySize(dirPath)
	diskSize := getDiskSize()
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

func getDiskSize() string {
	cmd := exec.Command("df", "--human-readable", "--portability", "/home")
	outputBytes, err := cmd.Output()
	if err != nil {
		return "?"
	}

	lines := strings.Split(string(outputBytes), "\n")
	if len(lines) < 2 {
		return "?"
	}

	fields := strings.Fields(lines[1])
	if len(fields) < 6 {
		return "?"
	}

	return fields[1]
}

func Reboot(username string) error {
	if !auth.IsAdmin(username) {
		return errors.New("Must be admin to reboot.")
	}

	cmd := exec.Command("systemctl", "reboot")
	err := cmd.Run()
	if err != nil {
		return errors.New("Failed to call reboot.")
	}

	return nil
}

func Poweroff(username string) error {
	if !auth.IsAdmin(username) {
		return errors.New("Must be admin to poweroff.")
	}

	cmd := exec.Command("systemctl", "poweroff")
	err := cmd.Run()
	if err != nil {
		return errors.New("Failed to call poweroff.")
	}

	return nil
}
