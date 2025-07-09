package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

var secret []byte = []byte(os.Getenv("GROUND_SECRET"))

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

func getTokenFromUsername(username string) (string, time.Time) {
	expiry := time.Now().Add(12 * time.Hour)
	value := fmt.Sprintf("%s %d", username, expiry.Unix())
	valueBytes := []byte(value)
	valueBytesEncoded := base64.URLEncoding.EncodeToString(valueBytes)
	valueBytesHashed := getHashedBytes(valueBytes)
	valueBytesHashedEncoded := base64.URLEncoding.EncodeToString(valueBytesHashed)
	token := fmt.Sprintf("%s|%s", valueBytesEncoded, valueBytesHashedEncoded)
	return token, expiry
}

func getUsernameFromToken(token string) (string, error) {
	split := strings.Split(token, "|")
	if len(split) != 2 {
		return "", errors.New("invalid token")
	}

	valueBytesHashedEncoded := split[1]
	valueBytesHashed, err := base64.URLEncoding.DecodeString(valueBytesHashedEncoded)
	if err != nil {
		return "", errors.New("failed to decode token")
	}

	valueBytesEncoded := split[0]
	valueBytes, err := base64.URLEncoding.DecodeString(valueBytesEncoded)
	if err != nil {
		return "", errors.New("failed to decode token")
	}

	if !hmac.Equal(getHashedBytes(valueBytes), valueBytesHashed) {
		return "", errors.New("hash invalid")
	}

	value := string(valueBytes)
	split = strings.Split(value, " ")
	if len(split) != 2 {
		return "", errors.New("invalid token")
	}

	expiry, err := strconv.ParseInt(split[1], 10, 64)
	if err != nil || time.Now().Unix() > expiry {
		return "", errors.New("token expired")
	}

	return split[0], nil
}

func getHashedBytes(bytes []byte) []byte {
	hash := hmac.New(sha256.New, secret)
	hash.Write(bytes)
	return hash.Sum(nil)
}
