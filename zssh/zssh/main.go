package main

import (
	"fmt"
	"github.com/openziti/sdk-golang/ziti"
	"github.com/openziti/sdk-golang/ziti/config"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"log"
	"os"
	"path/filepath"
	"zssh/zsshlib"
)

const ExpectedServiceAndExeName = "zssh"

var flags = &zsshlib.SshFlags{}

var rootCmd = &cobra.Command{
	Use:   fmt.Sprintf("%s <remoteUsername>@<targetIdentity>", flags.ServiceName),
	Short: "Z(iti)ssh, Carb-loaded ssh performs faster and stronger than ssh",
	Long:  "Z(iti)ssh is a version of ssh that utilizes a ziti network to provide a faster and more secure remote connection. A ziti connection must be established before use",
	Args:  cobra.ExactValidArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if flags.SshKeyPath == "" {
			userHome, err := os.UserHomeDir()
			if err != nil {
				logrus.Fatalf("could not find UserHomeDir? %v", err)
			}
			flags.SshKeyPath = filepath.Join(userHome, zsshlib.SSH_DIR, zsshlib.ID_RSA)
		}
		if flags.Debug {
			logrus.Infof("    flags.SshKeyPath set to: %s", flags.SshKeyPath)
		}

		if flags.ZConfig == "" {
			userHome, err := os.UserHomeDir()
			if err != nil {
				logrus.Fatalf("could not find UserHomeDir? %v", err)
			}
			flags.ZConfig = filepath.Join(userHome, ".ziti", fmt.Sprintf("%s.json", ExpectedServiceAndExeName))
		}
		if flags.Debug {
			logrus.Infof("       ZConfig set to: %s", flags.ZConfig)
		}

		username := zsshlib.ParseUserName(args[0])
		targetIdentity := zsshlib.ParseTargetIdentity(args[1])

		if flags.Debug {
			logrus.Infof("      username set to: %s", username)
			logrus.Infof("targetIdentity set to: %s", targetIdentity)
		}

		ctx := ziti.NewContextWithConfig(getConfig(flags.ZConfig))

		_, ok := ctx.GetService(flags.ServiceName)
		if !ok {
			logrus.Fatalf("could not find service: %s", flags.ServiceName)
		}

		dialOptions := &ziti.DialOptions{
			ConnectTimeout: 0,
			Identity:       targetIdentity,
			AppData:        nil,
		}
		svc, err := ctx.DialWithOptions(flags.ServiceName, dialOptions)
		if err != nil {
			logrus.Fatalf("error when dialing service name %s. %v", flags.ServiceName, err)
		}
		factory := zsshlib.NewSshConfigFactoryImpl(username, flags.SshKeyPath)
		zclient, err := zsshlib.Dial(factory.Config(), svc)
		if err != nil {
			logrus.Fatal(err)
		}
		err = zsshlib.RemoteShell(zclient)
	},
}

func init() {
	rootCmd.Flags().StringVarP(&flags.ServiceName, "service", "s", ExpectedServiceAndExeName, fmt.Sprintf("service name. default: %s", ExpectedServiceAndExeName))
	rootCmd.Flags().StringVarP(&flags.ZConfig, "ZConfig", "c", "", fmt.Sprintf("Path to ziti config file. default: $HOME/.ziti/%s.json", flags.ServiceName))
	rootCmd.Flags().StringVarP(&flags.SshKeyPath, "SshKeyPath", "i", "", "Path to ssh key. default: $HOME/.ssh/id_rsa")
	rootCmd.Flags().BoolVarP(&flags.Debug, "debug", "d", false, "pass to enable additional debug information")
}

func getConfig(cfgFile string) (zitiCfg *config.Config) {
	zitiCfg, err := config.NewFromFile(cfgFile)
	if err != nil {
		log.Fatalf("failed to load ziti configuration file: %v", err)
	}
	return zitiCfg
}

func main() {
	_ = rootCmd.Execute()
}
