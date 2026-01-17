package monitor

import (
	"errors"
	"fmt"

	"github.com/grantfbarnes/ground/internal/system/execute"
)

var diskSize string

func SetupDiskSize() error {
	ds, err := execute.GetDiskSize()
	if err != nil {
		return errors.Join(errors.New("failed to get disk size"), err)
	}

	diskSize = ds
	return nil
}

func GetUptime() string {
	uptime, err := execute.GetUptime()
	if err != nil {
		return "?"
	}

	return uptime
}

func GetDirectoryDiskUsage(dirPath string) string {
	directorySize := getDirectorySize(dirPath)
	return fmt.Sprintf("%s/%s", directorySize, diskSize)
}

func getDirectorySize(dirPath string) string {
	ds, err := execute.GetDirectorySize(dirPath)
	if err != nil {
		return "?"
	}

	return ds
}
