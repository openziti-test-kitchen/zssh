package main

import (
	"fmt"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"zssh/zsshlib"
)

const ExpectedServiceAndExeName = "zssh"

var flags = zsshlib.SshFlags{}

var rootCmd = &cobra.Command{
	Use:   fmt.Sprintf("%s <remoteUsername>@<targetIdentity>", flags.ServiceName),
	Short: "Z(iti)ssh, Carb-loaded ssh performs faster and stronger than ssh",
	Long:  "Z(iti)ssh is a version of ssh that utilizes a ziti network to provide a faster and more secure remote connection. A ziti connection must be established before use",
	Args:  cobra.ExactValidArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		username, targetIdentity := flags.GetUserAndIdentity(args[0])

		sshConn := zsshlib.EstablishClient(flags, username, targetIdentity)
		defer func() { _ = sshConn.Close() }()
		err := zsshlib.RemoteShell(sshConn)
		if err != nil {
			logrus.Fatalf("error opening remote shell: %v", err)
		}
	},
}

func init() {
	flags.InitFlags(rootCmd, ExpectedServiceAndExeName)
}

func main() {
	_ = rootCmd.Execute()
}
