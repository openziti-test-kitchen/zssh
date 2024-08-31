package zsshlib

import (
	"fmt"
	"github.com/spf13/cobra"
	"os/user"
	"runtime"
	"strings"
)

type SshFlags struct {
	ZConfig     string
	SshKeyPath  string
	Debug       bool
	ServiceName string
	Username    string
	OIDC        OIDCFlags
}

type OIDCFlags struct {
	Mode          bool
	Issuer        string
	ClientID      string
	ClientSecret  string
	CallbackPort  string
	AsAscii       bool
	OIDCOnly      bool
	ControllerUrl string
}

type ScpFlags struct {
	SshFlags
	Recursive bool
}

func (f *SshFlags) GetUserAndIdentity(input string) (string, string) {
	username := ParseUserName(input, true)
	targetIdentity := ParseTargetIdentity(input)
	log.Debugf("targetIdentity set to: %s", targetIdentity)
	return username, targetIdentity
}

func ParseUserName(input string, returnDefault bool) string {
	var username string
	if strings.ContainsAny(input, "@") {
		userServiceName := strings.Split(input, "@")
		username = userServiceName[0]
	} else {
		if returnDefault {

			curUser, err := user.Current()
			if err != nil {
				log.Fatal(err)
			}
			username = curUser.Username
			if strings.Contains(username, "\\") && runtime.GOOS == "windows" {
				username = strings.Split(username, "\\")[1]
			}
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
func (f *SshFlags) OIDCFlags(cmd *cobra.Command) {
	defaults := DefaultConfig()
	cmd.Flags().StringVarP(&f.OIDC.CallbackPort, "callbackPort", "p", "", "Port for Callback. default: "+defaults.OIDC.CallbackPort)
	cmd.Flags().StringVarP(&f.OIDC.ClientID, "clientID", "n", "", "IdP ClientID. default: "+defaults.OIDC.ClientID)
	cmd.Flags().StringVarP(&f.OIDC.ClientSecret, "clientSecret", "e", "", "IdP ClientSecret. default: (empty string - use PKCE)")
	cmd.Flags().StringVarP(&f.OIDC.Issuer, "oidcIssuer", "a", "", "URL of the OpenID Connect provider. required")
	cmd.Flags().BoolVarP(&f.OIDC.Mode, "oidc", "o", false, fmt.Sprintf("toggle OIDC mode. default: %t", defaults.OIDC.Enabled))
	cmd.Flags().BoolVar(&f.OIDC.OIDCOnly, "oidcOnly", false, "toggle OIDC only mode. default: false")
	cmd.Flags().StringVar(&f.OIDC.ControllerUrl, "controllerUrl", "", "the url of the controller to use. only used with --oidcOnly")
}

func (f *SshFlags) AddCommonFlags(cmd *cobra.Command) {
	defaults := DefaultConfig()
	cmd.Flags().StringVarP(&f.ServiceName, "service", "s", "", fmt.Sprintf("service name. default: %s", defaults.Service))
	cmd.Flags().StringVarP(&f.SshKeyPath, "SshKeyPath", "i", "", "Path to ssh key. default: $HOME/.ssh/id_rsa")
	cmd.Flags().StringVarP(&f.ZConfig, "ZConfig", "c", "", fmt.Sprintf("Path to ziti config file. default: "+DefaultIdentityFile()))
	cmd.Flags().BoolVarP(&f.Debug, "debug", "d", false, "pass to enable any additional debug information")

	/*
		if f.SshKeyPath == "" {
			userHome, err := os.UserHomeDir()
			if err != nil {
				log.Fatalf("could not find UserHomeDir? %v", err)
			}
			f.SshKeyPath = filepath.Join(userHome, SSH_DIR, ID_RSA)
		}
		f.DebugLog("    flags.SshKeyPath set to: %s", f.SshKeyPath)

		if f.ZConfig == "" {
			userHome, err := os.UserHomeDir()
			if err != nil {
				log.Fatalf("could not find UserHomeDir? %v", err)
			}
			f.ZConfig = filepath.Join(userHome, ".ziti", fmt.Sprintf("%s.json", exeName))
		}
		f.DebugLog("       ZConfig set to: %s", f.ZConfig)
	*/
}

func Combine(cmd *cobra.Command, c *SshFlags, cfg *Config) {
	d := DefaultConfig()
	if c.ZConfig == "" {
		if cfg.ZConfig == "" {
			c.ZConfig = d.ZConfig
		} else {
			c.ZConfig = cfg.ZConfig
		}
	}
	if c.SshKeyPath == "" {
		if cfg.SshKeyPath == "" {
			c.SshKeyPath = d.SshKeyPath
		} else {
			c.SshKeyPath = cfg.SshKeyPath
		}
	}
	if c.ServiceName == "" {
		c.ServiceName = cfg.Service
		if cfg.Service == "" {
			c.ServiceName = d.Service
		} else {
			c.ServiceName = cfg.Service
		}
	}
	if c.Username == "" {
		c.Username = cfg.Username
		if cfg.Service == "" {
			c.Username = d.Username
		} else {
			c.Username = cfg.Username
		}
	}
	if !cmd.Flags().Changed("oidc") {
		c.OIDC.Mode = cfg.OIDC.Enabled
	}
	if c.OIDC.Mode {
		if c.OIDC.Issuer == "" {
			c.OIDC.Issuer = cfg.OIDC.Issuer
			if cfg.OIDC.Issuer == "" {
				c.OIDC.Issuer = d.OIDC.Issuer
			} else {
				c.OIDC.Issuer = cfg.OIDC.Issuer
			}
		}
		if c.OIDC.CallbackPort == "" {
			c.OIDC.CallbackPort = cfg.OIDC.CallbackPort
			if cfg.OIDC.CallbackPort == "" {
				c.OIDC.CallbackPort = d.OIDC.CallbackPort
			} else {
				c.OIDC.CallbackPort = cfg.OIDC.CallbackPort
			}
		}
		if c.OIDC.ClientID == "" {
			c.OIDC.ClientID = cfg.OIDC.ClientID
			if cfg.OIDC.ClientID == "" {
				c.OIDC.ClientID = d.OIDC.ClientID
			} else {
				c.OIDC.ClientID = cfg.OIDC.ClientID
			}
		}
		if c.OIDC.ClientSecret == "" {
			// good
		}
	}
}
