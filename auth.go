package main

import (
	"io"
	"os/exec"
)

func loginIsValid(username string, password string) (bool, error) {
	cmd := exec.Command("su", "-c", "exit", username)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return false, err
	}

	go func() {
		defer stdin.Close()
		io.WriteString(stdin, password+"\n")
	}()

	err = cmd.Start()
	if err != nil {
		return false, err
	}

	err = cmd.Wait()
	if err != nil {
		return false, err
	}

	return true, nil
}
