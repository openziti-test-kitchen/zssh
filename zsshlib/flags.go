package zsshlib

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

type SshFlags struct {
	CallbackPort string
	ClientID     string
	ClientSecret string
	OIDCIssuer   string
	ZConfig      string
	SshKeyPath   string
	Debug        bool
	OIDCMode     bool
	ServiceName  string
}

type ScpFlags struct {
	SshFlags
	Recursive bool
}

func (f *SshFlags) GetUserAndIdentity(input string) (string, string) {
	username := ParseUserName(input)
	f.DebugLog("      username set to: %s", username)
	targetIdentity := ParseTargetIdentity(input)
	f.DebugLog("targetIdentity set to: %s", targetIdentity)
	return username, targetIdentity
}

func ParseUserName(input string) string {
	var username string
	if strings.ContainsAny(input, "@") {
		userServiceName := strings.Split(input, "@")
		username = userServiceName[0]
	} else {
		curUser, err := user.Current()
		if err != nil {
			logrus.Fatal(err)
		}
		username = curUser.Username
		if strings.Contains(username, "\\") && runtime.GOOS == "windows" {
			username = strings.Split(username, "\\")[1]
		}
	}
	return username
}

func ParseTargetIdentity(input string) string {
	var targetIdentity string
	if strings.ContainsAny(input, "@") {
		targetIdentity = strings.Split(input, "@")[1]
	} else {
		targetIdentity = input
	}

	if strings.Contains(targetIdentity, ":") {
		return strings.Split(targetIdentity, ":")[0]
	}
	return targetIdentity
}

func ParseFilePath(input string) string {
	if strings.Contains(input, ":") {
		colPos := strings.Index(input, ":") + 1
		return input[colPos:]
	}
	return input
}

// TODO: Add config file support
func (f *SshFlags) OIDCFlags(cmd *cobra.Command, exeName string) {
	cmd.Flags().StringVarP(&f.CallbackPort, "CallbackPort", "p", "63275", "Port for Callback. default: 63275")
	cmd.Flags().StringVarP(&f.ClientID, "ClientID", "n", "12345678", "IdP ClientID. default: 12345678")
	cmd.Flags().StringVarP(&f.ClientSecret, "ClientSecret", "e", "", "IdP ClientSecret. default: (empty string - use PKCE)")
	cmd.Flags().StringVarP(&f.OIDCIssuer, "OIDCIssuer", "a", "https://dev-yourid.okta.com", "URL of the OpenID Connect provider. default: https://dev-yourid.okta.com")
	cmd.Flags().BoolVarP(&f.OIDCMode, "oidc", "o", false, "toggle OIDC mode. default: false")
}

func (f *SshFlags) InitFlags(cmd *cobra.Command, exeName string) {
	cmd.Flags().StringVarP(&f.ServiceName, "service", "s", exeName, fmt.Sprintf("service name. default: %s", exeName))
	cmd.Flags().StringVarP(&f.ZConfig, "ZConfig", "c", "", fmt.Sprintf("Path to ziti config file. default: $HOME/.ziti/%s.json", f.ServiceName))
	cmd.Flags().StringVarP(&f.SshKeyPath, "SshKeyPath", "i", "", "Path to ssh key. default: $HOME/.ssh/id_rsa")
	cmd.Flags().BoolVarP(&f.Debug, "debug", "d", false, "pass to enable additional debug information")

	if f.SshKeyPath == "" {
		userHome, err := os.UserHomeDir()
		if err != nil {
			logrus.Fatalf("could not find UserHomeDir? %v", err)
		}
		f.SshKeyPath = filepath.Join(userHome, SSH_DIR, ID_RSA)
	}
	f.DebugLog("    flags.SshKeyPath set to: %s", f.SshKeyPath)

	if f.ZConfig == "" {
		userHome, err := os.UserHomeDir()
		if err != nil {
			logrus.Fatalf("could not find UserHomeDir? %v", err)
		}
		f.ZConfig = filepath.Join(userHome, ".ziti", fmt.Sprintf("%s.json", exeName))
	}
	f.DebugLog("       ZConfig set to: %s", f.ZConfig)
}
