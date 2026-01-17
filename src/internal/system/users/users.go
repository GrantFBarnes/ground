package users

import (
	"errors"
	"os"
	"path"

	"github.com/grantfbarnes/ground/internal/system/monitor"
)

type UserListItem struct {
	Username  string
	DiskUsage string
	IsAdmin   bool
}

func GetUserListItems() ([]UserListItem, error) {
	homeEntries, err := os.ReadDir("/home")
	if err != nil {
		return nil, errors.Join(errors.New("failed to read directory"), err)
	}

	listItems := []UserListItem{}
	for _, e := range homeEntries {
		if e.IsDir() {
			listItems = append(listItems, UserListItem{
				Username:  e.Name(),
				DiskUsage: monitor.GetDirectoryDiskUsage(path.Join("/home", e.Name())),
				IsAdmin:   IsAdmin(e.Name()),
			})
		}
	}

	return listItems, nil
}
