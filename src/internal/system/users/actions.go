package users

import (
	"errors"

	"github.com/grantfbarnes/ground/internal/system/execute"
)

func CreateUser(username string) error {
	err := execute.UserAdd(username)
	if err != nil {
		return errors.Join(errors.New("failed to add user"), err)
	}

	return nil
}

func DeleteUser(username string) error {
	err := execute.UserDel(username)
	if err != nil {
		return errors.Join(errors.New("failed to delete user"), err)
	}

	return nil
}

func ResetUserPassword(username string) error {
	err := SetUserPassword(username, "password")
	if err != nil {
		return errors.Join(errors.New("failed to set user password"), err)
	}

	return nil
}

func SetUserPassword(username string, password string) error {
	err := execute.PasswordSet(username, password)
	if err != nil {
		return errors.Join(errors.New("failed to change password"), err)
	}

	return nil
}
