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
		return errors.Join(errors.New("regex compile failed"), err)
	}
	sshKeyRegex = re
	return nil
}

func GetUserSshKeys(username string) []string {
	sshKeyPath := path.Join("/home", username, ".ssh", "authorized_keys")
	sshKeys, _ := getFileLines(sshKeyPath)
	return sshKeys
}

func AddUserSshKey(username string, targetUsername string, sshKey string) error {
	if username != targetUsername && !users.IsAdmin(username) {
		return errors.New("Must be admin to add other user SSH Keys.")
	}

	if !sshKeyRegex.MatchString(sshKey) {
		return errors.New("SSH Key is not valid.")
	}

	homePath := path.Join("/home", targetUsername)
	sshKeyPath := path.Join(homePath, ".ssh", "authorized_keys")
	_, err := os.Stat(sshKeyPath)
	if err != nil {
		uid, gid, err := users.GetUserIds(targetUsername)
		if err != nil {
			return err
		}

		err = CreateMissingDirectories(homePath, ".ssh", uid, gid)
		if err != nil {
			return errors.New("Failed to create SSH folder.")
		}

		err = createMissingFile(sshKeyPath, uid, gid)
		if err != nil {
			return errors.New("Failed to create SSH file.")
		}
	}

	sshKeyFile, err := os.OpenFile(sshKeyPath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return errors.New("Failed to open SSH file.")
	}
	defer sshKeyFile.Close()

	_, err = sshKeyFile.WriteString(sshKey + "\n")
	if err != nil {
		return errors.New("Failed to write to SSH file.")
	}

	return nil
}

func DeleteUserSshKey(username string, targetUsername string, indexString string) error {
	if username != targetUsername && !users.IsAdmin(username) {
		return errors.New("Must be admin to delete other user SSH Keys.")
	}

	index, err := strconv.Atoi(indexString)
	if err != nil {
		return errors.New("Index is not a number.")
	}

	if index < 0 {
		return errors.New("Index is not valid.")
	}

	homePath := path.Join("/home", targetUsername)
	sshKeyPath := path.Join(homePath, ".ssh", "authorized_keys")
	_, err = os.Stat(sshKeyPath)
	if err != nil {
		return errors.New("SSH file does not exist.")
	}

	cmd := exec.Command("sed", "-i", fmt.Sprintf("%dd", index+1), sshKeyPath)

	err = cmd.Run()
	if err != nil {
		return errors.New("Failed to delete SSH Key.")
	}

	return nil
}
