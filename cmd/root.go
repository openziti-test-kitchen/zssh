package cmd

import (
	"fmt"
	"github.com/michaelquigley/pfxlog"
	"github.com/openziti/sdk-golang/ziti"
	"github.com/openziti/sdk-golang/ziti/config"
	"github.com/spf13/cobra"
	"os"
	"path/filepath"
	"strings"
	"zssh/zsshlib"
)


var (
	log = pfxlog.Logger()
	ZConfig string
	SshKeyPath string

	rootCmd = &cobra.Command{
		Use: "zssh <remoteUsername>@<targetIdentity>",
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
				ZConfig = filepath.Join(userHome,".ziti","zssh.json")
			}

			ctx := ziti.NewContextWithConfig(getConfig(ZConfig))

			foundSvc, ok := ctx.GetService("ssh-linux-aws")
			if !ok {
				panic("error when retrieving all the services for the provided config")
			}
			log.Info("found service named: zssh")


			clientConfig := &ServiceConfig{}
			found, err := foundSvc.GetConfigOfType("zssh", clientConfig)
			if err != nil {
				panic(fmt.Sprintf("error when getting configs for service named %s. %v", "zssh", err))
			}
			if !found {
				log.Warn("no config of type ziti-tunneler-client.v1 was found")
			}
			userServiceName := strings.Split(args[0], "@")
			username := userServiceName[0]
			targetIdentity := userServiceName[1]



			dialOptions := &ziti.DialOptions{
				ConnectTimeout: 0,
				Identity:       targetIdentity,
				AppData:        nil,
			}
			svc, err := ctx.DialWithOptions("zssh", dialOptions)
				if err != nil {
					panic(fmt.Sprintf("error when dialing service name %s. %v", "zssh", err))

					factory := zsshlib.NewSshConfigFactoryImpl(username, SshKeyPath)
					zclient, err := zsshlib.Dial(factory.Config(), svc)
					if err != nil {
						panic(err)
					}
					zsshlib.RemoteShell(factory, zclient)
				}
		},

	}
)

func init() {
	rootCmd.Flags().StringVarP(&ZConfig, "ZConfig", "c", "", "Path to ziti config file")
	rootCmd.Flags().StringVarP(&SshKeyPath, "SshKeyPath", "i", "", "Path to ssh key")
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
	zitiCfg.ConfigTypes = []string{
		"ziti-tunneler-client.v1",
	}
	return zitiCfg
}



