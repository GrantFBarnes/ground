package users

import (
	"errors"
	"net/http"
	"os"
	"os/user"
	"path"
	"strconv"

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

func GetUserIds(username string) (uid int, gid int, err error) {
	user, err := user.Lookup(username)
	if err != nil {
		return 0, 0, errors.Join(errors.New("failed to lookup user"), err)
	}

	uid, err = strconv.Atoi(user.Uid)
	if err != nil {
		return 0, 0, errors.Join(errors.New("failed to convert uid"), err)
	}

	gid, err = strconv.Atoi(user.Gid)
	if err != nil {
		return 0, 0, errors.Join(errors.New("failed to convert gid"), err)
	}

	return uid, gid, nil
}

func GetRequestor(r *http.Request) string {
	return r.Context().Value("requestor").(string)
}
