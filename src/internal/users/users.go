package users

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"os/user"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/grantfbarnes/ground/internal/system"
)

var usernameRegex *regexp.Regexp
var adminGroup string

type UserListItem struct {
	Username  string
	DiskUsage string
	IsAdmin   bool
}

func SetupUsernameRegex() error {
	// contains only letters, numbers, or ._-
	// cannot start with -
	// cannot be exclusively numbers
	// can end with $
	// length between 1 and 256
	re, err := regexp.Compile(`^([a-zA-Z0-9._]*[a-zA-Z._][a-zA-Z0-9._-]*[$]?){1,256}$`)
	if err != nil {
		return errors.Join(errors.New("regex compile failed"), err)
	}
	usernameRegex = re
	return nil
}

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
	return err
}

func GetUserListItems() ([]UserListItem, error) {
	homeEntries, err := os.ReadDir("/home")
	if err != nil {
		return nil, errors.New("Failed to read home directory.")
	}

	listItems := []UserListItem{}
	for _, e := range homeEntries {
		if e.IsDir() {
			listItems = append(listItems, UserListItem{
				Username:  e.Name(),
				DiskUsage: system.GetDirectoryDiskUsage(path.Join("/home", e.Name())),
				IsAdmin:   IsAdmin(e.Name()),
			})
		}
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
	err := Validate(username)
	if err != nil {
		return err
	}

	if !credentialsAreValid(username, password) {
		return errors.New("Invalid credentials provided.")
	}

	return nil
}

func Validate(username string) error {
	if !usernameRegex.MatchString(username) {
		return errors.New("Username is not valid.")
	}

	_, err := user.Lookup(username)
	if err != nil {
		return errors.New("User does not exist.")
	}

	homePath := path.Join("/home", username)
	_, err = os.Stat(homePath)
	if err != nil {
		return errors.New("User has no home.")
	}

	return nil
}

func credentialsAreValid(username string, password string) bool {
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
