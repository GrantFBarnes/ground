package users

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"os/user"
	"path"
	"strconv"

	"github.com/grantfbarnes/ground/internal/auth"
	"github.com/grantfbarnes/ground/internal/system"
)

type UserListItem struct {
	Username  string
	DiskUsage string
}

func GetUserListItems() ([]UserListItem, error) {
	homeEntries, err := os.ReadDir("/home")
	if err != nil {
		return nil, errors.New("Failed to read home directory.")
	}

	listItems := []UserListItem{}
	for _, e := range homeEntries {
		if !e.IsDir() {
			continue
		}

		listItem := UserListItem{
			Username: e.Name(),
		}

		listItem.DiskUsage, err = system.GetDirectoryDiskUsage(path.Join("/home", listItem.Username))
		if err != nil {
			return nil, errors.New("Failed to get user disk usage.")
		}

		listItems = append(listItems, listItem)
	}

	return listItems, nil
}

func GetUserIds(username string) (uid int, gid int, err error) {
	user, err := user.Lookup(username)
	if err != nil {
		return 0, 0, errors.New("Failed to lookup user.")
	}

	uid, err = strconv.Atoi(user.Uid)
	if err != nil {
		return 0, 0, errors.New("Uid is invalid.")
	}

	gid, err = strconv.Atoi(user.Gid)
	if err != nil {
		return 0, 0, errors.New("Gid is invalid.")
	}

	return uid, gid, nil
}

func Login(username string, password string) error {
	_, err := user.Lookup(username)
	if err != nil {
		return errors.New("User does not exist.")
	}

	homePath := path.Join("/home", username)
	_, err = os.Stat(homePath)
	if err != nil {
		return errors.New("User has no home.")
	}

	if !auth.CredentialsAreValid(username, password) {
		return errors.New("Invalid credentials provided.")
	}

	return nil
}

func CreateUser(username string, newUsername string) error {
	if !auth.IsAdmin(username) {
		return errors.New("Must be admin to create new users.")
	}

	homePath := path.Join("/home", newUsername)
	_, err := os.Stat(homePath)
	if err == nil {
		return errors.New("User already exists.")
	}

	cmd := exec.Command("useradd", "--create-home", newUsername)

	err = cmd.Start()
	if err != nil {
		return errors.New("Failed to create user.")
	}

	err = cmd.Wait()
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
	if !auth.IsAdmin(username) {
		return errors.New("Must be admin to delete users.")
	}

	cmd := exec.Command("userdel", "--remove", targetUsername)

	err := cmd.Start()
	if err != nil {
		return errors.New("Failed to delete user.")
	}

	err = cmd.Wait()
	if err != nil {
		return errors.New("Failed to delete user.")
	}

	return nil
}

func ResetUserPassword(username string, targetUsername string) error {
	if !auth.IsAdmin(username) {
		return errors.New("Must be admin to reset user passwords.")
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

	err = cmd.Start()
	if err != nil {
		return err
	}

	err = cmd.Wait()
	if err != nil {
		return err
	}

	return nil
}
