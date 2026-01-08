package filesystem

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"regexp"
	"strconv"

	"github.com/grantfbarnes/ground/internal/system/users"
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
		uid, gid, err := users.GetUserIds(username)
		if err != nil {
			return errors.Join(errors.New("failed to get user ids"), err)
		}

		err = CreateMissingDirectories(homePath, ".ssh", uid, gid)
		if err != nil {
			return errors.Join(errors.New("failed to create missing directories"), err)
		}

		err = createMissingFile(sshKeyPath, uid, gid)
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
	index, err := strconv.Atoi(indexString)
	if err != nil {
		return errors.Join(errors.New("index is not a number"), err)
	}

	if index < 0 {
		return errors.New("index is less than zero")
	}

	homePath := path.Join("/home", username)
	sshKeyPath := path.Join(homePath, ".ssh", "authorized_keys")
	_, err = os.Stat(sshKeyPath)
	if err != nil {
		return errors.Join(errors.New("ssh file does not exist"), err)
	}

	cmd := exec.Command("sed", "-i", fmt.Sprintf("%dd", index+1), sshKeyPath)

	err = cmd.Run()
	if err != nil {
		return errors.Join(errors.New("failed to remove line from file"), err)
	}

	return nil
}
