package users

import (
	"errors"
	"os"
	"os/user"
	"path"
	"strconv"

	"github.com/grantfbarnes/ground/internal/system"
)

type UserListItem struct {
	Username  string
	DiskUsage string
	IsAdmin   bool
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
