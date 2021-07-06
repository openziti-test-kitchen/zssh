package cmd

import (
	"fmt"
	"github.com/michaelquigley/pfxlog"
	"github.com/openziti/sdk-golang/ziti"
	"github.com/openziti/sdk-golang/ziti/config"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"zssh/zsshlib"
)


var (
	log = pfxlog.Logger()


	rootCmd = &cobra.Command{
		Use: "zzh",
		Short: "Z(iti)zh, Carb-loaded ssh performs faster and stronger than ssh",
		Long: "Z(iti)zh is a version of ssh that utilizes a ziti network to provide a faster and more secure remote connection. A ziti connection must be established before use",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return errors.New("No Config File provided")
			}
			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
			ctx := ziti.NewContextWithConfig(getConfig(args[0]))

			foundSvc, ok := ctx.GetService("ssh-linux-aws")
			if !ok {
				panic("error when retrieving all the services for the provided config")
			}
			log.Info("found service named: zzh")


			clientConfig := &ServiceConfig{}
			found, err := foundSvc.GetConfigOfType("zssh", clientConfig)
			if err != nil {
				panic(fmt.Sprintf("error when getting configs for service named %s. %v", "zzh", err))
			}
			if !found {
				log.Warn("no config of type ziti-tunneler-client.v1 was found")
			}

			dialOptions := &ziti.DialOptions{
				ConnectTimeout: 0,
				Identity:       "ziti-tunnel-aws",
				AppData:        nil,
			}
			svc, err := ctx.DialWithOptions("zssh", dialOptions)
				if err != nil {
					panic(fmt.Sprintf("error when dialing service name %s. %v", "zzh", err))
				}

				factory := zsshlib.NewSshConfigFactoryImpl("ubuntu","/Users/jkochanik/.ssh/id_rsa")
				zclient, err := zsshlib.Dial(factory.Config(), svc)
				if err != nil {
					panic(err)
				}
				zsshlib.RemoteShell(factory, zclient)









		},
	}
)

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



