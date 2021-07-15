package zsshlib

import (
	"os/user"
	"runtime"
	"strings"
	"testing"
)
import "github.com/stretchr/testify/assert"

func TestParseTargetIdentity(t *testing.T) {
	result := ParseTargetIdentity("user@hostname:port")
	assert.Equal(t, result, "hostname", "user not correct")

	result = ParseTargetIdentity("hostname:port")
	assert.Equal(t, result, "hostname", "user not correct")

	result = ParseTargetIdentity("hostname")
	assert.Equal(t, result, "hostname", "user not correct")

	result = ParseTargetIdentity("user@hostname")
	assert.Equal(t, result, "hostname", "user not correct")
}

func getOsUser() string {
	u, _ := user.Current()
	return u.Username
}
func TestParseUserName(t *testing.T) {
	result := ParseUserName("user@hostname:port")
	assert.Equal(t, result, "user", "user not correct")

	var osUser string
	if runtime.GOOS == "windows" {
		osUser = strings.Split(getOsUser(), `\`)[1]
	} else {
		osUser = getOsUser()
	}

	result = ParseUserName("hostname:port")
	assert.Equal(t, result, osUser, "user not correct")

	result = ParseUserName("hostname")
	assert.Equal(t, result, osUser, "user not correct")

	result = ParseUserName("user@hostname")
	assert.Equal(t, result, "user", "user not correct")
}
func TestParseFilePath(t *testing.T) {
	result := ParseFilePath("user@hostname:/*/bob")
	assert.Equal(t, result, "/*/bob", "user not correct")

	result = ParseFilePath("user@hostname:/bob")
	assert.Equal(t, result, "/bob", "user not correct")

	result = ParseFilePath("user@hostname:")
	assert.Equal(t, result, "", "user not correct")

	result = ParseFilePath("user@hostname:/haha://two:colons")
	assert.Equal(t, result, ".", "user not correct")
}