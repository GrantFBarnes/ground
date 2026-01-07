package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"time"
)

const cookieNameUserToken string = "GROUND-USER-TOKEN"
const cookieNameRedirectURL string = "GROUND-REDIRECT-URL"

var hashSecret []byte
var adminGroup string

func SetupHashSecret() error {
	bytes := make([]byte, 32)
	_, err := rand.Read(bytes)
	if err != nil {
		return errors.Join(errors.New("rand read failed"), err)
	}
	hashSecret = bytes
	return nil
}

func SetupAdminGroup() error {
	groups := []string{"sudo", "wheel"}
	for _, group := range groups {
		cmd := exec.Command("grep", "-E", "^%"+group+".*ALL", "/etc/sudoers")
		if cmd.Run() == nil {
			adminGroup = group
			return nil
		}
	}
	return errors.New("no admin group found")
}

func CredentialsAreValid(username string, password string) bool {
	// since program is run as root, standard su doesn't require password
	// use su to run su as that user checking for password
	cmd := exec.Command("su", "-c", "su -c exit "+username, username)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return false
	}

	go func() {
		defer stdin.Close()
		io.WriteString(stdin, password+"\n")
	}()

	return cmd.Run() == nil
}

func IsAdmin(username string) bool {
	cmd := exec.Command("groups", username)
	outputBytes, err := cmd.Output()
	if err != nil {
		return false
	}

	userGroups := strings.Fields(string(outputBytes))
	return slices.Contains(userGroups, adminGroup)
}

func ToggleAdmin(username string) (err error) {
	if IsAdmin(username) {
		cmd := exec.Command("gpasswd", "-d", username, adminGroup)
		err = cmd.Run()
	} else {
		cmd := exec.Command("gpasswd", "-a", username, adminGroup)
		err = cmd.Run()
	}
	return err
}

func GetUsername(r *http.Request) (string, error) {
	token, err := getCookieValue(r, cookieNameUserToken)
	if err != nil {
		return "", errors.New("cookie not found")
	}

	username, err := getUsernameFromToken(token)
	if err != nil {
		return "", errors.New("user not logged in")
	}

	return username, nil
}

func SetUsername(w http.ResponseWriter, username string) {
	token, expiry := getTokenFromUsername(username)
	http.SetCookie(w, &http.Cookie{
		Name:    cookieNameUserToken,
		Value:   token,
		Path:    "/",
		Expires: expiry,
	})
}

func RemoveUsername(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:    cookieNameUserToken,
		Value:   "",
		Path:    "/",
		Expires: time.Unix(0, 0),
	})
}

func GetRedirectUrl(r *http.Request) string {
	redirectPath, err := getCookieValue(r, cookieNameRedirectURL)
	if err != nil {
		return "/"
	}
	return redirectPath
}

func SetRedirectUrl(w http.ResponseWriter, url string) {
	http.SetCookie(w, &http.Cookie{
		Name:    cookieNameRedirectURL,
		Value:   url,
		Path:    "/",
		Expires: getExpiry(),
	})
}

func getCookieValue(r *http.Request, cookieName string) (string, error) {
	cookieFound := false
	cookieValue := ""
	for _, c := range r.Cookies() {
		if c.Name != cookieName {
			continue
		}
		cookieFound = true
		cookieValue = c.Value
		break
	}

	if !cookieFound {
		return "", errors.New("cookie not found")
	}

	return cookieValue, nil
}

func getTokenFromUsername(username string) (string, time.Time) {
	expiry := getExpiry()
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
	hash := hmac.New(sha256.New, hashSecret)
	hash.Write(bytes)
	return hash.Sum(nil)
}

func getExpiry() time.Time {
	return time.Now().Add(12 * time.Hour)
}
