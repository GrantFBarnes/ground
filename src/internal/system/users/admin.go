package users

import (
	"errors"
	"slices"

	"github.com/grantfbarnes/ground/internal/system/execute"
)

var adminGroup string

func SetupAdminGroup() error {
	groups := []string{"sudo", "wheel"}
	for _, group := range groups {
		if execute.FileSearch("/etc/sudoers", "^%"+group+".*ALL") == nil {
			adminGroup = group
			return nil
		}
	}
	return errors.New("no admin group found")
}

func IsAdmin(username string) bool {
	userGroups, err := execute.GetGroups(username)
	if err != nil {
		return false
	}
	return slices.Contains(userGroups, adminGroup)
}

func ToggleAdmin(username string) (err error) {
	if IsAdmin(username) {
		err = execute.GroupDelete(username, adminGroup)
	} else {
		err = execute.GroupAdd(username, adminGroup)
	}

	if err != nil {
		return errors.Join(errors.New("failed to modify group"), err)
	}

	return nil
}
