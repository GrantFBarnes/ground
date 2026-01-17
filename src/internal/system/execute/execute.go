package execute

import (
	"errors"
	"os"
	"os/exec"
	"path"
	"strings"

	"github.com/grantfbarnes/ground/internal/system/users"
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

func Touch(username string, filePath string) error {
	filePath = path.Clean(filePath)

	if !strings.HasPrefix(filePath, path.Join("/home", username)) {
		return errors.New("file path is not in home directory")
	}

	_, err := os.Stat(filePath)
	if err == nil {
		// file already exists
		return nil
	}

	dirPath, _ := path.Split(filePath)
	err = Mkdir(username, dirPath)
	if err != nil {
		return errors.Join(errors.New("failed to create parent directory"), err)
	}

	cmd := exec.Command("touch", filePath)

	err = users.ExecuteAs(cmd, username)
	if err != nil {
		return errors.Join(errors.New("failed to set command executor"), err)
	}

	err = cmd.Run()
	if err != nil {
		return errors.Join(errors.New("failed to create file"), err)
	}

	return nil
}

func Mkdir(username string, dirPath string) error {
	dirPath = path.Clean(dirPath)

	if !strings.HasPrefix(dirPath, path.Join("/home", username)) {
		return errors.New("dir path is not in home directory")
	}

	cmd := exec.Command("mkdir", "-p", dirPath)

	err := users.ExecuteAs(cmd, username)
	if err != nil {
		return errors.Join(errors.New("failed to set command executor"), err)
	}

	err = cmd.Run()
	if err != nil {
		return errors.Join(errors.New("failed to create directory"), err)
	}

	return nil
}
