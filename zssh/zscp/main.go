package main

import (
	"fmt"
	"github.com/pkg/sftp"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"zssh/zsshlib"
)

const ExpectedServiceAndExeName = "zssh"

var flags = &zsshlib.ScpFlags{}
var rootCmd = &cobra.Command{
	Use: "Remote to Local: zscp <remoteUsername>@<targetIdentity>:[Remote Path] [Local Path]\n" +
		"Local to Remote: zscp [Local Path] <remoteUsername>@<targetIdentity>:[Remote Path]",
	Short: "Z(iti)scp, Carb-loaded ssh performs faster and stronger than ssh",
	Long:  "Z(iti)scp is a version of ssh that utilizes a ziti network to provide a faster and more secure remote connection. A ziti connection must be established before use",
	Args:  cobra.ExactValidArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		var remoteFilePath string
		var localFilePath string
		var isCopyToRemote bool

		if strings.ContainsAny(args[0], ":") {
			remoteFilePath = args[0]
			localFilePath = args[1]
			isCopyToRemote = false

		} else if strings.ContainsAny(args[1], ":") {
			remoteFilePath = args[1]
			localFilePath = args[0]
			isCopyToRemote = true
		} else {
			logrus.Fatal(`cannot determine remote file PATH use ":" for remote path`)
		}

		localFilePath, err := filepath.Abs(localFilePath)
		if err != nil {
			logrus.Fatalf("cannot determine absolute local file path, unrecognized file name: %s", localFilePath)
		}
		if _, err := os.Stat(localFilePath); err != nil {
			logrus.Fatal(err)
		}

		flags.DebugLog("           local path: %s", localFilePath)

		username, targetIdentity := flags.GetUserAndIdentity(remoteFilePath)
		remoteFilePath = zsshlib.ParseFilePath(remoteFilePath)

		sshConn := zsshlib.EstablishClient(flags.SshFlags, username, targetIdentity)
		defer func() { _ = sshConn.Close() }()

		client, err := sftp.NewClient(sshConn)
		if err != nil {
			logrus.Fatalf("error creating sftp client: %v", err)
		}
		defer func() { _ = client.Close() }()

		if isCopyToRemote {
			if flags.Recursive {
				baseDir := filepath.Base(localFilePath)
				err := filepath.WalkDir(localFilePath, func(path string, info fs.DirEntry, err error) error {
					remotePath := filepath.Join(remoteFilePath, baseDir, after(path, baseDir))
					if info.IsDir() {
						err = client.Mkdir(remotePath)
						if err != nil {
							flags.DebugLog("%s", err) //occurs when directories exist already. Is not fatal. Only logs when debug flag is set.
						} else {
							flags.DebugLog("made directory: %s", remotePath)
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
				err = zsshlib.SendFile(client, localFilePath, remoteFilePath)
				if err != nil {
					logrus.Errorf("could not send file: %s [%v]", localFilePath, err)
				} else {
					logrus.Infof("sent file: %s ==> %s", localFilePath, remoteFilePath)
				}
			}
		} else {
			if flags.Recursive {
				baseDir := filepath.Base(remoteFilePath)
				walker := client.Walk(remoteFilePath)
				for walker.Step() {
					localPath := filepath.Join(localFilePath, baseDir, after(walker.Path(), baseDir))
					if walker.Stat().IsDir() {
						err = os.Mkdir(localPath, os.ModePerm)
						if err != nil {
							flags.DebugLog("failed to make directory: %s [%v]", localPath, err) //occurs when directories exist already. Is not fatal. Only logs when debug flag is set.
						} else {
							flags.DebugLog("made directory: %s", localPath)
						}
					} else {
						err = zsshlib.RetrieveRemoteFiles(client, localPath, walker.Path())
						if err != nil {
							logrus.Fatalf("failed to retrieve file: %s [%v]", walker.Path(), err)
						}
					}
				}
			} else {
				err = zsshlib.RetrieveRemoteFiles(client, localFilePath, remoteFilePath)
				if err != nil {
					logrus.Fatalf("failed to retrieve file: %s [%v]", remoteFilePath, err)
				}
			}
		}
	},
}

func init() {
	flags.InitFlags(rootCmd, ExpectedServiceAndExeName)
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
	return value[adjustedPos:len(value)]
}

func main() {
	_ = rootCmd.Execute()
}
