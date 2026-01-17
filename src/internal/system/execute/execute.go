package execute

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/user"
	"path"
	"strconv"
	"strings"
	"syscall"
)

func Reboot() error {
	cmd := exec.Command("systemctl", "reboot")
	err := cmd.Run()
	if err != nil {
		return errors.Join(errors.New("failed to call reboot"), err)
	}

	return nil
}

func Poweroff() error {
	cmd := exec.Command("systemctl", "poweroff")
	err := cmd.Run()
	if err != nil {
		return errors.Join(errors.New("failed to call poweroff"), err)
	}

	return nil
}

func SedDeleteLine(filePath string, indexString string) error {
	filePath = path.Clean(filePath)
	_, err := os.Stat(filePath)
	if err != nil {
		return errors.Join(errors.New("file path not found"), err)
	}

	index, err := strconv.Atoi(indexString)
	if err != nil {
		return errors.Join(errors.New("index is not a number"), err)
	}

	if index < 0 {
		return errors.New("index is less than zero")
	}

	cmd := exec.Command("sed", "--in-place", fmt.Sprintf("%dd", index+1), filePath)

	err = cmd.Run()
	if err != nil {
		return errors.Join(errors.New("failed run sed"), err)
	}

	return nil
}

func Useradd(username string) error {
	homePath := path.Join("/home", username)
	_, err := os.Stat(homePath)
	if err == nil {
		return errors.New("user already exists")
	}

	cmd := exec.Command("useradd", "--create-home", username)

	err = cmd.Run()
	if err != nil {
		return errors.Join(errors.New("failed to create user"), err)
	}

	err = Passwd(username, "password")
	if err != nil {
		return errors.Join(errors.New("failed to set user password"), err)
	}

	return nil
}

func Userdel(username string) error {
	cmd := exec.Command("userdel", "--remove", username)

	err := cmd.Run()
	if err != nil {
		return errors.Join(errors.New("failed to delete user"), err)
	}

	return nil
}

func Passwd(username string, password string) error {
	if strings.ContainsAny(password, "\n") {
		return errors.New("password is not valid")
	}

	cmd := exec.Command("passwd", "--stdin", username)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return errors.Join(errors.New("failed to read stdin"), err)
	}

	go func() {
		defer stdin.Close()
		io.WriteString(stdin, password+"\n")
	}()

	err = cmd.Run()
	if err != nil {
		return errors.Join(errors.New("failed to run passwd"), err)
	}

	return nil
}

func Touch(username string, filePath string) error {
	filePath = path.Clean(filePath)

	if !strings.HasPrefix(filePath, path.Join("/home", username)) {
		return errors.New("file path is not in home directory")
	}

	_, err := os.Stat(filePath)
	if err == nil {
		// file already exists
		return nil
	}

	dirPath, _ := path.Split(filePath)
	err = Mkdir(username, dirPath)
	if err != nil {
		return errors.Join(errors.New("failed to create parent directory"), err)
	}

	cmd := exec.Command("touch", filePath)

	err = executeAs(cmd, username)
	if err != nil {
		return errors.Join(errors.New("failed to set command executor"), err)
	}

	err = cmd.Run()
	if err != nil {
		return errors.Join(errors.New("failed to create file"), err)
	}

	return nil
}

func Mkdir(username string, dirPath string) error {
	dirPath = path.Clean(dirPath)

	if !strings.HasPrefix(dirPath, path.Join("/home", username)) {
		return errors.New("dir path is not in home directory")
	}

	cmd := exec.Command("mkdir", "-p", dirPath)

	err := executeAs(cmd, username)
	if err != nil {
		return errors.Join(errors.New("failed to set command executor"), err)
	}

	err = cmd.Run()
	if err != nil {
		return errors.Join(errors.New("failed to create directory"), err)
	}

	return nil
}

func TarCompressDirectory(username string, dirPath string, filePath string) error {
	dirPath = path.Clean(dirPath)

	if !strings.HasPrefix(dirPath, path.Join("/home", username)) {
		return errors.New("dir path is not in home directory")
	}

	dirInfo, err := os.Stat(dirPath)
	if err != nil {
		return errors.Join(errors.New("dir path not found"), err)
	}

	if !dirInfo.IsDir() {
		return errors.New("dir path is not a directory")
	}

	filePath = path.Clean(filePath)

	if !strings.HasPrefix(filePath, path.Join("/home", username)) {
		return errors.New("file path is not in home directory")
	}

	if !strings.HasSuffix(filePath, ".tar.gz") {
		return errors.New("file path is not a compressed file")
	}

	_, err = os.Stat(filePath)
	if err == nil {
		return errors.Join(errors.New("file path already exists"), err)
	}

	cmd := exec.Command("tar", "-zchf", filePath, "--directory", dirPath, ".")

	err = executeAs(cmd, username)
	if err != nil {
		return errors.Join(errors.New("failed to set command executor"), err)
	}

	err = cmd.Run()
	if err != nil {
		return errors.Join(errors.New("failed to compress directory"), err)
	}

	return nil
}

func TarExtractFile(username string, filePath string, dirPath string) error {
	filePath = path.Clean(filePath)

	if !strings.HasPrefix(filePath, path.Join("/home", username)) {
		return errors.New("file path is not in home directory")
	}

	if !strings.HasSuffix(filePath, ".tar.gz") {
		return errors.New("file path is not a compressed file")
	}

	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return errors.Join(errors.New("file path not found"), err)
	}

	if fileInfo.IsDir() {
		return errors.New("file path is a directory")
	}

	err = Mkdir(username, dirPath)
	if err != nil {
		return errors.Join(errors.New("failed to create extract directory"), err)
	}

	cmd := exec.Command("tar", "-xzf", filePath, "--directory", dirPath)

	err = executeAs(cmd, username)
	if err != nil {
		return errors.Join(errors.New("failed to set command executor"), err)
	}

	err = cmd.Run()
	if err != nil {
		return errors.Join(errors.New("failed to extract file"), err)
	}

	return nil
}

func executeAs(cmd *exec.Cmd, username string) error {
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
