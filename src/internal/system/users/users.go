package users

import (
	"errors"
	"os"
)

type UserListItem struct {
	Username string
	IsAdmin  bool
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
				Username: e.Name(),
				IsAdmin:  IsAdmin(e.Name()),
			})
		}
	}

	return listItems, nil
}
