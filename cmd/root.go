package cmd

import (
	"fmt"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

)

var (
	cfgFile string
	bindInterface string






	rootCmd = &cobra.Command{
		Use: "zzsh",
		Short: "Z(iti)ssh, Carb-loaded ssh performs faster and stronger than ssh",
		Long: "Z(iti)ssh is a version of ssh that utilizes a ziti network to provide a faster and more secure remote connection. A ziti connection must be established before use",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return errors.New("No Hostname provided")
			}
			return nil
		},
	}
)

func Execute() error{
	return rootCmd.Execute()
}

func init(){
	cobra.OnInitialize(initConfig)
}

func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		//home, err := homedir.Dir()
		//cobra.CheckErr(err)

		// Search config in home directory with name ".cobra" (without extension).
		//viper.AddConfigPath(home)
		//viper.SetConfigName(".cobra")
	}

	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err == nil {
		fmt.Println("Using config file:", viper.ConfigFileUsed())
	}
}