package system

import (
	"errors"
	"os/exec"

	"github.com/grantfbarnes/ground/internal/auth"
)

func GetUptime() (string, error) {
	cmd := exec.Command("uptime", "--pretty")
	uptimeBytes, err := cmd.Output()
	if err != nil {
		return "", err
	}

	return string(uptimeBytes), nil
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
