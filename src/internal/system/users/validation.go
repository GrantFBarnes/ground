package users

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"os/user"
	"path"
	"regexp"
)

var usernameRegex *regexp.Regexp

func SetupUsernameRegex() error {
	// contains only letters, numbers, or ._-
	// cannot start with -
	// cannot be exclusively numbers
	// can end with $
	// length between 1 and 256
	re, err := regexp.Compile(`^([a-zA-Z0-9._]*[a-zA-Z._][a-zA-Z0-9._-]*[$]?){1,256}$`)
	if err != nil {
		return errors.Join(errors.New("failed to compile regex"), err)
	}
	usernameRegex = re
	return nil
}

func UsernameIsValid(username string) bool {
	return usernameRegex.MatchString(username)
}

func UserIsValid(username string) bool {
	if !UsernameIsValid(username) {
		return false
	}

	_, err := user.Lookup(username)
	if err != nil {
		return false
	}

	homePath := path.Join("/home", username)
	_, err = os.Stat(homePath)
	if err != nil {
		return false
	}

	return true
}

func CredentialsAreValid(username string, password string) bool {
	// since program is run as root, standard su doesn't require password
	// use su to run su as that user checking for password
	cmd := exec.Command("su", "-c", "su -c exit "+username, username)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return false
	}

	go func() {
		defer stdin.Close()
		io.WriteString(stdin, password+"\n")
	}()

	return cmd.Run() == nil
}
