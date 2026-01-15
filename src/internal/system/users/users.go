package users

import (
	"errors"
	"os"
	"os/exec"
	"os/user"
	"path"
	"strconv"
	"syscall"

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

func ExecuteAs(cmd *exec.Cmd, username string) error {
	user, err := user.Lookup(username)
	if err != nil {
		return errors.Join(errors.New("failed to lookup user"), err)
	}

	uid64, err := strconv.ParseUint(user.Uid, 10, 32)
	if err != nil {
		return errors.Join(errors.New("failed to parse uid"), err)
	}

	gid64, err := strconv.ParseUint(user.Gid, 10, 32)
	if err != nil {
		return errors.Join(errors.New("failed to parse gid"), err)
	}

	cmd.SysProcAttr = &syscall.SysProcAttr{
		Credential: &syscall.Credential{
			Uid: uint32(uid64),
			Gid: uint32(gid64),
		},
	}

	return nil
}
