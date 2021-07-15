package main

import (
	"fmt"
	"github.com/openziti/sdk-golang/ziti"
	"github.com/openziti/sdk-golang/ziti/config"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"log"
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
		username := zsshlib.ParseUserName(args[0])
		targetIdentity := zsshlib.ParseTargetIdentity(args[0])

		flags.DebugLog("      username set to: %s", username)
		flags.DebugLog("targetIdentity set to: %s", targetIdentity)

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
	flags.InitFlags(rootCmd, ExpectedServiceAndExeName)
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
