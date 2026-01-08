package users

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"path"
)

func CreateUser(username string) error {
	homePath := path.Join("/home", username)
	_, err := os.Stat(homePath)
	if err == nil {
		return errors.New("user already exists")
	}

	cmd := exec.Command("useradd", "--create-home", username)

	err = cmd.Run()
	if err != nil {
		return errors.Join(errors.New("failed to create user"), err)
	}

	err = setUserPassword(username, "password")
	if err != nil {
		return errors.Join(errors.New("failed to set user password"), err)
	}

	return nil
}

func DeleteUser(username string) error {
	cmd := exec.Command("userdel", "--remove", username)

	err := cmd.Run()
	if err != nil {
		return errors.Join(errors.New("failed to delete user"), err)
	}

	return nil
}

func ResetUserPassword(username string) error {
	err := setUserPassword(username, "password")
	if err != nil {
		return errors.Join(errors.New("failed to set user password"), err)
	}

	return nil
}

func setUserPassword(username string, password string) error {
	cmd := exec.Command("passwd", "--stdin", username)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return errors.Join(errors.New("failed to read stdin"), err)
	}

	go func() {
		defer stdin.Close()
		io.WriteString(stdin, password+"\n")
	}()

	err = cmd.Run()
	if err != nil {
		return errors.Join(errors.New("failed to run passwd"), err)
	}

	return nil
}
