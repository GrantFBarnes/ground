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

func GetUptime() (string, error) {
	cmd := exec.Command("uptime", "--pretty")
	outputBytes, err := cmd.Output()
	if err != nil {
		return "", errors.Join(errors.New("failed to run uptime"), err)
	}

	return string(outputBytes), nil
}

func GetDiskSize() (string, error) {
	cmd := exec.Command("df", "--human-readable", "--portability", "/home")
	outputBytes, err := cmd.Output()
	if err != nil {
		return "", errors.Join(errors.New("failed to run df"), err)
	}

	lines := strings.Split(string(outputBytes), "\n")
	if len(lines) < 2 {
		return "", errors.New("df output less than two lines")
	}

	fields := strings.Fields(lines[1])
	if len(fields) < 6 {
		return "", errors.New("df output less than six fields")
	}

	return fields[1], nil
}

func GetDirectorySize(dirPath string) (string, error) {
	dirPath = path.Clean(dirPath)

	dirInfo, err := os.Stat(dirPath)
	if err != nil {
		return "", errors.Join(errors.New("dir path not found"), err)
	}

	if !dirInfo.IsDir() {
		return "", errors.New("path is not a directory")
	}

	cmd := exec.Command("du", "--summarize", "--human-readable", dirPath)
	outputBytes, err := cmd.Output()
	if err != nil {
		return "", errors.Join(errors.New("failed to run du"), err)
	}

	fields := strings.Fields(string(outputBytes))
	if len(fields) < 2 {
		return "", errors.New("du output less than two fields")
	}

	return fields[0], nil
}

func FileSearch(filePath string, searchRegex string) error {
	cmd := exec.Command("grep", "-E", searchRegex, filePath)
	err := cmd.Run()
	if err != nil {
		return errors.Join(errors.New("failed to grep file"), err)
	}

	return nil
}

func FileLineDelete(filePath string, indexString string) error {
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

func GetGroups(username string) ([]string, error) {
	cmd := exec.Command("groups", username)
	outputBytes, err := cmd.Output()
	if err != nil {
		return nil, errors.Join(errors.New("failed get groups output"), err)
	}

	return strings.Fields(string(outputBytes)), nil
}

func GroupAdd(username string, groupname string) error {
	cmd := exec.Command("gpasswd", "-a", username, groupname)
	err := cmd.Run()
	if err != nil {
		return errors.Join(errors.New("failed to add to group"), err)
	}

	return nil
}

func GroupDelete(username string, groupname string) error {
	cmd := exec.Command("gpasswd", "-d", username, groupname)
	err := cmd.Run()
	if err != nil {
		return errors.Join(errors.New("failed to delete from group"), err)
	}

	return nil
}

func UserAdd(username string) error {
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

	err = PasswordSet(username, "password")
	if err != nil {
		return errors.Join(errors.New("failed to set user password"), err)
	}

	return nil
}

func UserDel(username string) error {
	cmd := exec.Command("userdel", "--remove", username)

	err := cmd.Run()
	if err != nil {
		return errors.Join(errors.New("failed to delete user"), err)
	}

	return nil
}

func PasswordSet(username string, password string) error {
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

func TestRunAs(username string, password string) error {
	if strings.ContainsAny(password, "\n") {
		return errors.New("password is not valid")
	}

	cmd := exec.Command("su", "-c", "su -c exit "+username, username)

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
		return errors.Join(errors.New("failed to run su"), err)
	}

	return nil
}

func Move(username string, sourcePath string, destinationPath string) error {
	sourcePath = path.Clean(sourcePath)

	if !strings.HasPrefix(sourcePath, path.Join("/home", username)) {
		return errors.New("source path is not in home directory")
	}

	_, err := os.Stat(sourcePath)
	if err != nil {
		return errors.Join(errors.New("source path not found"), err)
	}

	destinationPath = path.Clean(destinationPath)

	if !strings.HasPrefix(destinationPath, path.Join("/home", username)) {
		return errors.New("destination path is not in home directory")
	}

	_, err = os.Stat(destinationPath)
	if err == nil {
		return errors.New("destination path already exists")
	}

	destinationParentPath, _ := path.Split(destinationPath)
	err = MakeDirectory(username, destinationParentPath)
	if err != nil {
		return errors.Join(errors.New("failed to create parent directory"), err)
	}

	cmd := exec.Command("mv", sourcePath, destinationPath)

	err = executeAs(cmd, username)
	if err != nil {
		return errors.Join(errors.New("failed to set command executor"), err)
	}

	err = cmd.Run()
	if err != nil {
		return errors.Join(errors.New("failed to run mv"), err)
	}

	return nil
}

func TouchFile(username string, filePath string) error {
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
	err = MakeDirectory(username, dirPath)
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

func MakeDirectory(username string, dirPath string) error {
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

	err = MakeDirectory(username, dirPath)
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
