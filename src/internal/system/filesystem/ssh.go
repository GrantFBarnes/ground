package filesystem

import (
	"errors"
	"os"
	"path"
	"regexp"

	"github.com/grantfbarnes/ground/internal/system/execute"
)

var sshKeyRegex *regexp.Regexp

func SetupSshKeyRegex() error {
	re, err := regexp.Compile(`^ssh-(rsa|ed25519) [a-zA-Z0-9+/]+[=]{0,3}( [^@]+@[^@]+)?$`)
	if err != nil {
		return errors.Join(errors.New("failed to compile regex"), err)
	}
	sshKeyRegex = re
	return nil
}

func GetUserSshKeys(username string) ([]string, error) {
	sshKeyPath := path.Join("/home", username, ".ssh", "authorized_keys")
	sshKeys, err := getFileLines(sshKeyPath)
	if err != nil {
		return nil, errors.Join(errors.New("failed to read file lines"), err)
	}
	return sshKeys, nil
}

func AddUserSshKey(username string, sshKey string) error {
	if !sshKeyRegex.MatchString(sshKey) {
		return errors.New("ssh key is not valid")
	}

	homePath := path.Join("/home", username)
	sshKeyPath := path.Join(homePath, ".ssh", "authorized_keys")
	_, err := os.Stat(sshKeyPath)
	if err != nil {
		err = execute.Touch(username, sshKeyPath)
		if err != nil {
			return errors.Join(errors.New("failed to create missing file"), err)
		}
	}

	sshKeyFile, err := os.OpenFile(sshKeyPath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return errors.Join(errors.New("failed to open file"), err)
	}
	defer sshKeyFile.Close()

	_, err = sshKeyFile.WriteString(sshKey + "\n")
	if err != nil {
		return errors.Join(errors.New("failed to write to file"), err)
	}

	return nil
}

func DeleteUserSshKey(username string, indexString string) error {
	err := execute.SedDeleteLine(path.Join("/home", username, ".ssh", "authorized_keys"), indexString)
	if err != nil {
		return errors.Join(errors.New("failed to remove line from file"), err)
	}

	return nil
}
