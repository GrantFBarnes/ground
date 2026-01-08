package power

import (
	"errors"
	"os/exec"
)

func Reboot() error {
	cmd := exec.Command("systemctl", "reboot")
	err := cmd.Run()
	if err != nil {
		return errors.Join(errors.New("failed to call reboot"), err)
	}

	return nil
}

func Poweroff() error {
	cmd := exec.Command("systemctl", "poweroff")
	err := cmd.Run()
	if err != nil {
		return errors.Join(errors.New("failed to call poweroff"), err)
	}

	return nil
}
