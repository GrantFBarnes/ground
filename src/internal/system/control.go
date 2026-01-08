package system

import (
	"errors"
	"os/exec"
)

func Reboot() error {
	cmd := exec.Command("systemctl", "reboot")
	err := cmd.Run()
	if err != nil {
		return errors.New("Failed to call reboot.")
	}

	return nil
}

func Poweroff() error {
	cmd := exec.Command("systemctl", "poweroff")
	err := cmd.Run()
	if err != nil {
		return errors.New("Failed to call poweroff.")
	}

	return nil
}
