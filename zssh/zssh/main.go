/*
	Copyright NetFoundry, Inc.

	Licensed under the Apache License, Version 2.0 (the "License");
	you may not use this file except in compliance with the License.
	You may obtain a copy of the License at

	https://www.apache.org/licenses/LICENSE-2.0

	Unless required by applicable law or agreed to in writing, software
	distributed under the License is distributed on an "AS IS" BASIS,
	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	See the License for the specific language governing permissions and
	limitations under the License.
*/

package main

import (
	"fmt"
	"github.com/openziti/ziti/common/enrollment"
	"github.com/openziti/ziti/ziti/cmd/common"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"os"
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
	p := common.NewOptionsProvider(os.Stdout, os.Stderr)
	rootCmd.AddCommand(enrollment.NewEnrollCommand(p))
	e := rootCmd.Execute()
	if e != nil {
		logrus.Error(e)
	}
}
