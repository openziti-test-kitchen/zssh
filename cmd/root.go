package cmd

import (
	"fmt"
	"github.com/michaelquigley/pfxlog"
	"github.com/openziti/sdk-golang/ziti"
	"github.com/openziti/sdk-golang/ziti/config"
	"github.com/spf13/cobra"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"zssh/zsshlib"
)

const ExpectedServiceAndExeName = "zssh"

var (
	log = pfxlog.Logger()
	ZConfig string
	SshKeyPath string

	rootCmd = &cobra.Command{
		Use: fmt.Sprintf("%s <remoteUsername>@<targetIdentity>", ExpectedServiceAndExeName),
		Short: "Z(iti)ssh, Carb-loaded ssh performs faster and stronger than ssh",
		Long: "Z(iti)ssh is a version of ssh that utilizes a ziti network to provide a faster and more secure remote connection. A ziti connection must be established before use",
		Args: cobra.ExactValidArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			if SshKeyPath == "" {
				userHome, err := os.UserHomeDir()
				if err != nil {
					panic(err)
				}
				SshKeyPath = filepath.Join(userHome,".ssh","id_rsa")
			}

			if ZConfig == "" {
				userHome, err := os.UserHomeDir()
				if err != nil {
					panic(err)
				}
				ZConfig = filepath.Join(userHome,".ziti", fmt.Sprintf("%s.json", ExpectedServiceAndExeName))
			}

			var username string
			var targetIdentity string
			if strings.ContainsAny(args[0], "@") {
				userServiceName := strings.Split(args[0], "@")
				username = userServiceName[0]
				targetIdentity = userServiceName[1]
			} else {
				curUser, err := user.Current()
				if err != nil {
					panic(err)
				}
				username = curUser.Username
				targetIdentity = args[0]
			}

			ctx := ziti.NewContextWithConfig(getConfig(ZConfig))

			_, ok := ctx.GetService(ExpectedServiceAndExeName)
			if !ok {
				panic("error when retrieving all the services for the provided config")
			}
			log.Infof("found service named: %s", ExpectedServiceAndExeName)

			dialOptions := &ziti.DialOptions{
				ConnectTimeout: 0,
				Identity:       targetIdentity,
				AppData:        nil,
			}
			svc, err := ctx.DialWithOptions(ExpectedServiceAndExeName, dialOptions)
			if err != nil {
				panic(fmt.Sprintf("error when dialing service name %s. %v", ExpectedServiceAndExeName, err))
			}
			factory := zsshlib.NewSshConfigFactoryImpl(username, SshKeyPath)
			zclient, err := zsshlib.Dial(factory.Config(), svc)
			if err != nil {
				panic(err)
			}
			zsshlib.RemoteShell(factory, zclient)
		},

	}
)

func init() {
	rootCmd.Flags().StringVarP(&ZConfig, "ZConfig", "c", "", fmt.Sprintf("Path to ziti config file. default: $HOME/.ziti/%s.json", ExpectedServiceAndExeName))
	rootCmd.Flags().StringVarP(&SshKeyPath, "SshKeyPath", "i", "", "Path to ssh key. default: $HOME/.ssh/id_rsa")
}

type ServiceConfig struct {
	Protocol string
	Hostname string
	Port     int
}

func Execute() error{
	return rootCmd.Execute()
}

func getConfig(cfgFile string) (zitiCfg *config.Config) {
	zitiCfg, err := config.NewFromFile(cfgFile)
	if err != nil {
		log.Fatalf("failed to load ziti configuration file: %v", err)
	}
	return zitiCfg
}



