package users

import (
	"errors"
	"os/exec"
	"slices"
	"strings"
)

var adminGroup string

func SetupAdminGroup() error {
	groups := []string{"sudo", "wheel"}
	for _, group := range groups {
		cmd := exec.Command("grep", "-E", "^%"+group+".*ALL", "/etc/sudoers")
		if cmd.Run() == nil {
			adminGroup = group
			return nil
		}
	}
	return errors.New("no admin group found")
}

func IsAdmin(username string) bool {
	cmd := exec.Command("groups", username)
	outputBytes, err := cmd.Output()
	if err != nil {
		return false
	}

	userGroups := strings.Fields(string(outputBytes))
	return slices.Contains(userGroups, adminGroup)
}

func ToggleAdmin(username string) (err error) {
	if IsAdmin(username) {
		cmd := exec.Command("gpasswd", "-d", username, adminGroup)
		err = cmd.Run()
	} else {
		cmd := exec.Command("gpasswd", "-a", username, adminGroup)
		err = cmd.Run()
	}
	return errors.Join(errors.New("failed to run gpasswd"), err)
}
