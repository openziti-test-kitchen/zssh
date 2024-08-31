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
	gendoc "github.com/openziti/cobra-to-md"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"zssh/zsshlib"

	"github.com/pkg/sftp"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/openziti/ziti/common/enrollment"
	"github.com/openziti/ziti/ziti/cmd/common"
)

var flags = &zsshlib.ScpFlags{}
var rootCmd = &cobra.Command{
	Use: "zscp <remoteUsername>@<targetIdentity>:[Remote Path] [Local Path] or " +
		"zscp [Local Path] <remoteUsername>@<targetIdentity>:[Remote Path]",
	Short: "Z(iti)scp, Carb-loaded ssh performs faster and stronger than ssh",
	Long:  "Z(iti)scp is a version of ssh that utilizes a ziti network to provide a faster and more secure remote connection. A ziti connection must be established before use",
	Args:  cobra.MinimumNArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		var remoteFilePath string
		var localFilePaths []string
		var isCopyToRemote bool

		if strings.ContainsAny(args[0], ":") {
			remoteFilePath = args[0]
			localFilePaths = args[1:]
			if len(localFilePaths) > 1 {
				logrus.Fatalf("remote to local cannot have more than two arguments")
			}
			isCopyToRemote = false

		} else if strings.ContainsAny(args[len(args)-1], ":") {
			remoteFilePath = args[len(args)-1]
			localFilePaths = args[0 : len(args)-1]
			isCopyToRemote = true
		} else {
			logrus.Fatal(`cannot determine remote file PATH use ":" for remote path`)
		}
		var err error
		for i, path := range localFilePaths {
			if localFilePaths[i], err = filepath.Abs(path); err != nil {
				logrus.Fatalf("cannot determine absolute local file path, unrecognized file name: %s", path)
			}
			if _, err := os.Stat(localFilePaths[i]); err != nil {
				logrus.Fatal(err)
			}
			zsshlib.Logger().Debugf("           local path: %s", localFilePaths[i])
		}

		targetIdentity := zsshlib.ParseTargetIdentity(remoteFilePath)
		cfg := zsshlib.FindConfigByKey(targetIdentity)
		zsshlib.Combine(cmd, &flags.SshFlags, cfg)

		remoteFilePath = zsshlib.ParseFilePath(remoteFilePath)

		sshConn := zsshlib.EstablishClient(&flags.SshFlags, remoteFilePath, targetIdentity)
		defer func() { _ = sshConn.Close() }()

		client, err := sftp.NewClient(sshConn)
		if err != nil {
			logrus.Fatalf("error creating sftp client: %v", err)
		}
		defer func() { _ = client.Close() }()

		if remoteFilePath == "~" {
			remoteFilePath = ""
		} else if len(remoteFilePath) > 1 && remoteFilePath[0:1] == "~" {
			remoteFilePath = remoteFilePath[2:]
		}

		remoteFilePath, err = client.RealPath(remoteFilePath)
		if err != nil {
			logrus.Fatalf("cannot find remote file path: %s [%v]", remoteFilePath, err)
		}

		remoteGlob, err := client.Glob(remoteFilePath)
		if err != nil {
			logrus.Fatalf("file pattern [%s] not recognized [%v]", remoteFilePath, err)
		} else if remoteGlob == nil {
			remoteGlob = append(remoteGlob, remoteFilePath)
		}

		if isCopyToRemote { //local to remote
			for i, localFilePath := range localFilePaths {
				if flags.Recursive {
					baseDir := filepath.Base(localFilePath)
					err := filepath.WalkDir(localFilePath, func(path string, info fs.DirEntry, err error) error {
						remotePath := filepath.Join(remoteFilePath, baseDir, after(path, baseDir))
						remotePath = strings.ReplaceAll(remotePath, `\`, `/`)
						if info.IsDir() {
							err = client.Mkdir(remotePath)
							if err != nil {
								zsshlib.Logger().Debugf("%s", err) //occurs when directories exist already. Is not fatal. Only logs when debug flag is set.
							} else {
								zsshlib.Logger().Debugf("made directory: %s", remotePath)
							}
						} else {
							err = zsshlib.SendFile(client, path, remotePath)
							if err != nil {
								return fmt.Errorf("could not send file: %s [%v]", path, err)
							} else {
								logrus.Infof("sent file: %s ==> %s", path, remotePath)
							}
						}
						return nil
					})
					if err != nil {
						logrus.Fatal(err)
					}
				} else {
					if i > 0 {
						remoteFilePath = filepath.Join(filepath.Dir(remoteFilePath), filepath.Base(localFilePath))
					}
					remoteFilePath = zsshlib.AppendBaseName(client, remoteFilePath, localFilePath, flags.Debug)
					remoteFilePath = strings.ReplaceAll(remoteFilePath, `\`, `/`)
					err = zsshlib.SendFile(client, localFilePath, remoteFilePath)
					if err != nil {
						logrus.Errorf("could not send file: %s [%v]", localFilePath, err)
					} else {
						logrus.Infof("sent file: %s ==> %s", localFilePath, remoteFilePath)
					}
				}
			}
		} else { //remote to local
			localFilePath := localFilePaths[0]
			for _, remoteFilePath = range remoteGlob {
				if flags.Recursive {
					baseDir := filepath.Base(remoteFilePath)
					walker := client.Walk(remoteFilePath)
					for walker.Step() {
						localPath := filepath.Join(localFilePath, baseDir, after(walker.Path(), baseDir)) //saves base directory to cut remote directory after it to append to localpath
						if walker.Stat().IsDir() {
							err = os.Mkdir(localPath, os.ModePerm)
							if err != nil {
								zsshlib.Logger().Debugf("failed to make directory: %s [%v]", localPath, err) //occurs when directories exist already. Is not fatal. Only logs when debug flag is set.
							} else {
								zsshlib.Logger().Debugf("made directory: %s", localPath)
							}
						} else {
							err = zsshlib.RetrieveRemoteFiles(client, localPath, walker.Path())
							if err != nil {
								logrus.Fatalf("failed to retrieve file: %s [%v]", walker.Path(), err)
							}
						}
					}
				} else {
					if info, _ := os.Lstat(localFilePaths[0]); info.IsDir() {
						localFilePath = filepath.Join(localFilePaths[0], filepath.Base(remoteFilePath))
					}
					err = zsshlib.RetrieveRemoteFiles(client, localFilePath, remoteFilePath)
					if err != nil {
						logrus.Fatalf("failed to retrieve file: %s [%v]", remoteFilePath, err)
					}
				}
			}
		}
	},
}

func init() {
	flags.OIDCFlags(rootCmd)
	rootCmd.Flags().BoolVarP(&flags.Recursive, "recursive", "r", false, "pass to enable recursive file transfer")
}

func after(value string, a string) string {
	// Get substring after a string.
	pos := strings.LastIndex(value, a)
	if pos == -1 {
		return ""
	}
	adjustedPos := pos + len(a)
	if adjustedPos >= len(value) {
		return ""
	}
	return value[adjustedPos:]
}

func main() {
	p := common.NewOptionsProvider(os.Stdout, os.Stderr)
	flags.AddCommonFlags(rootCmd)
	rootCmd.AddCommand(enrollment.NewEnrollCommand(p))
	rootCmd.AddCommand(zsshlib.NewMfaCmd(&flags.SshFlags))
	rootCmd.AddCommand(gendoc.NewGendocCmd(rootCmd))
	e := rootCmd.Execute()
	if e != nil {
		logrus.Error(e)
	}
}
