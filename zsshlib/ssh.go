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

package zsshlib

import (
	"fmt"
	"github.com/openziti/sdk-golang/ziti"
	"github.com/openziti/sdk-golang/ziti/config"
	"github.com/pkg/errors"
	"github.com/pkg/sftp"
	"io"
	"io/ioutil"
	"log"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"sync"

	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/terminal"
)

const (
	ID_RSA  = "id_rsa"
	SSH_DIR = ".ssh"
)

func RemoteShell(client *ssh.Client) error {
	session, err := client.NewSession()
	if err != nil {
		return err
	}

	stdInFd := int(os.Stdin.Fd())
	stdOutFd := int(os.Stdout.Fd())

	oldState, err := terminal.MakeRaw(stdInFd)
	if err != nil {
		logrus.Fatal(err)
	}
	defer func() {
		_ = session.Close()
		_ = terminal.Restore(stdInFd, oldState)
	}()

	session.Stdout = os.Stdout
	session.Stderr = os.Stderr
	session.Stdin = os.Stdin

	termWidth, termHeight, err := terminal.GetSize(stdOutFd)
	if err != nil {
		logrus.Fatal(err)
	}

	fmt.Println("connected.")

	if err := session.RequestPty("xterm", termHeight, termWidth, ssh.TerminalModes{ssh.ECHO: 1}); err != nil {
		return err
	}

	err = session.Run("/bin/bash")
	if err != nil {
		return err
	}

	return nil
}

func Dial(config *ssh.ClientConfig, conn net.Conn) (*ssh.Client, error) {
	c, chans, reqs, err := ssh.NewClientConn(conn, "", config)
	if err != nil {
		return nil, err
	}
	return ssh.NewClient(c, chans, reqs), nil
}

type SshConfigFactory interface {
	Address() string
	Hostname() string
	Port() int
	User() string
	Config() *ssh.ClientConfig
	KeyPath() string
}

type SshConfigFactoryImpl struct {
	user            string
	host            string
	port            int
	keyPath         string
	resolveAuthOnce sync.Once
	authMethods     []ssh.AuthMethod
}

func NewSshConfigFactoryImpl(user string, keyPath string) *SshConfigFactoryImpl {
	factory := &SshConfigFactoryImpl{
		user:    user,
		host:    "",
		port:    22,
		keyPath: keyPath,
	}
	return factory
}

func (factory *SshConfigFactoryImpl) User() string {
	return factory.user
}
func (factory *SshConfigFactoryImpl) Hostname() string {
	return factory.host
}

func (factory *SshConfigFactoryImpl) Port() int {
	return factory.port
}

func (factory *SshConfigFactoryImpl) KeyPath() string {
	return factory.keyPath
}

func (factory *SshConfigFactoryImpl) Address() string {
	return factory.host + ":" + strconv.Itoa(factory.port)
}

func (factory *SshConfigFactoryImpl) Config() *ssh.ClientConfig {
	factory.resolveAuthOnce.Do(func() {
		var methods []ssh.AuthMethod

		if fileMethod, err := sshAuthMethodFromFile(factory.keyPath); err == nil {
			methods = append(methods, fileMethod)
		} else {
			logrus.Error(err)
		}

		if agentMethod := sshAuthMethodAgent(); agentMethod != nil {
			methods = append(methods, sshAuthMethodAgent())
		}

		methods = append(methods)

		factory.authMethods = methods
	})

	return &ssh.ClientConfig{
		User:            factory.user,
		Auth:            factory.authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}
}

func sshAuthMethodFromFile(keyPath string) (ssh.AuthMethod, error) {
	content, err := ioutil.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("could not read zssh file [%s]: %w", keyPath, err)
	}

	if signer, err := ssh.ParsePrivateKey(content); err == nil {
		return ssh.PublicKeys(signer), nil
	} else {
		if err.Error() == "zssh: no key found" {
			return nil, fmt.Errorf("no private key found in [%s]: %w", keyPath, err)
		} else if err.(*ssh.PassphraseMissingError) != nil {
			return nil, fmt.Errorf("file is password protected [%s] %w", keyPath, err)
		} else {
			return nil, fmt.Errorf("error parsing private key from [%s]L %w", keyPath, err)
		}
	}
}

func SendFile(client *sftp.Client, localPath string, remotePath string) error {
	localFile, err := ioutil.ReadFile(localPath)

	if err != nil {
		return errors.Wrapf(err, "unable to read local file %v", localFile)
	}

	rmtFile, err := client.OpenFile(remotePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC)

	if err != nil {
		return errors.Wrapf(err, "unable to open remote file %v", remotePath)
	}
	defer rmtFile.Close()

	_, err = rmtFile.Write(localFile)
	if err != nil {
		return err
	}

	return nil
}

func RetrieveRemoteFiles(client *sftp.Client, localPath string, remotePath string) error {

	rf, err := client.Open(remotePath)
	if err != nil {
		return fmt.Errorf("error opening remote file [%s] (%w)", remotePath, err)
	}
	defer func() { _ = rf.Close() }()

	lf, err := os.OpenFile(localPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, os.ModePerm)
	if err != nil {
		return fmt.Errorf("error opening local file [%s] (%w)", localPath, err)
	}
	defer func() { _ = lf.Close() }()

	_, err = io.Copy(lf, rf)
	if err != nil {
		return fmt.Errorf("error copying remote file to local [%s] (%w)", remotePath, err)
	}
	logrus.Infof("%s => %s", remotePath, localPath)

	return nil
}

func EstablishClient(f SshFlags, userName string, targetIdentity string) *ssh.Client {
	ctx := ziti.NewContextWithConfig(getConfig(f.ZConfig))
	_, ok := ctx.GetService(f.ServiceName)
	if !ok {
		logrus.Fatalf("service not found: %s", f.ServiceName)
	}

	dialOptions := &ziti.DialOptions{
		ConnectTimeout: 0,
		Identity:       targetIdentity,
		AppData:        nil,
	}
	svc, err := ctx.DialWithOptions(f.ServiceName, dialOptions)

	if err != nil {
		logrus.Fatalf("error when dialing service name %s. %v", f.ServiceName, err)
	}

	factory := NewSshConfigFactoryImpl(userName, f.SshKeyPath)
	config := factory.Config()
	sshConn, err := Dial(config, svc)
	if err != nil {
		logrus.Fatalf("error dialing SSH Conn: %v", err)
	}
	return sshConn
}

func (f *SshFlags) DebugLog(msg string, args ...interface{}) {
	if f.Debug {
		logrus.Infof(msg, args...)
	}
}

func getConfig(cfgFile string) (zitiCfg *config.Config) {
	zitiCfg, err := config.NewFromFile(cfgFile)
	if err != nil {
		log.Fatalf("failed to load ziti configuration file: %v", err)
	}
	return zitiCfg
}

// AppendBaseName tags file name on back of remotePath if the path is blank or a directory/*
func AppendBaseName(c *sftp.Client, remotePath string, localPath string, debug bool) string {
	localPath = filepath.Base(localPath)
	if remotePath == "" {
		remotePath = filepath.Base(localPath)
	} else {
		info, err := c.Lstat(remotePath)
		if err == nil && info.IsDir() {
			remotePath = filepath.Join(remotePath, localPath)
		} else if debug {
			logrus.Infof("Remote File/Directory: %s doesn't exist [%v]", remotePath, err)
		}
	}
	return remotePath
}
