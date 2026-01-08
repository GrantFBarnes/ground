package users

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"path"
)

func CreateUser(username string, newUsername string) error {
	if !IsAdmin(username) {
		return errors.New("Must be admin to create new users.")
	}

	homePath := path.Join("/home", newUsername)
	_, err := os.Stat(homePath)
	if err == nil {
		return errors.New("User already exists.")
	}

	cmd := exec.Command("useradd", "--create-home", newUsername)

	err = cmd.Run()
	if err != nil {
		return errors.New("Failed to create user.")
	}

	err = setUserPassword(newUsername, "password")
	if err != nil {
		return errors.New("Failed to set password.")
	}

	return nil
}

func DeleteUser(username string, targetUsername string) error {
	if username != targetUsername && !IsAdmin(username) {
		return errors.New("Must be admin to delete other users.")
	}

	cmd := exec.Command("userdel", "--remove", targetUsername)

	err := cmd.Run()
	if err != nil {
		return errors.New("Failed to delete user.")
	}

	return nil
}

func ResetUserPassword(username string, targetUsername string) error {
	if username != targetUsername && !IsAdmin(username) {
		return errors.New("Must be admin to reset other user passwords.")
	}

	err := setUserPassword(targetUsername, "password")
	if err != nil {
		return errors.New("Failed to set password.")
	}

	return nil
}

func setUserPassword(username string, password string) error {
	cmd := exec.Command("passwd", "--stdin", username)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}

	go func() {
		defer stdin.Close()
		io.WriteString(stdin, password+"\n")
	}()

	err = cmd.Run()
	if err != nil {
		return err
	}

	return nil
}
