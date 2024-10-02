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
	"bufio"
	"context"
	"encoding/base64"
	"fmt"
	"github.com/google/uuid"
	"github.com/gorilla/securecookie"
	"github.com/zitadel/oidc/v2/pkg/client/rp/cli"
	"github.com/zitadel/oidc/v2/pkg/oidc"
	"golang.org/x/crypto/ssh/knownhosts"
	"io"
	"net"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/openziti/sdk-golang/ziti"
	"github.com/pkg/errors"
	"github.com/pkg/sftp"
	"github.com/zitadel/oidc/v2/pkg/client/rp"
	httphelper "github.com/zitadel/oidc/v2/pkg/http"
	"golang.org/x/oauth2"

	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/terminal"
)

const (
	ID_RSA  = "id_rsa"
	SSH_DIR = ".ssh"
)

var (
	DefaultAuthScopes = "openid profile email"
)

func RemoteShell(client *ssh.Client, args []string) error {
	session, err := client.NewSession()
	if err != nil {
		return err
	}

	if len(args) > 0 {
		if err := session.RequestPty("xterm", 80, 40, ssh.TerminalModes{}); err != nil {
			logrus.Fatalf("Failed to request pseudo terminal: %v", err)
		}

		defer func() { _ = session.Close() }()

		stdoutPipe, err := session.StdoutPipe()
		if err != nil {
			logrus.Fatal(os.Stderr, "Failed to create stdout pipe:", err)
		}

		stderrPipe, err := session.StderrPipe()
		if err != nil {
			logrus.Fatal("Failed to create stderr pipe:", err)
		}

		cmd := strings.Join(args, " ")
		logrus.Infof("executing remote command: %v", cmd)
		if err := session.Start(cmd); err != nil {
			logrus.Fatal("Failed to start command:", err)
		}

		processOutput(stdoutPipe, stderrPipe)

		// Wait for the command to finish
		if err := session.Wait(); err != nil {
			logrus.Fatal("Command execution failed:", err)
		}

		return nil
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

	if err := session.RequestPty("xterm", termHeight, termWidth, ssh.TerminalModes{ssh.ECHO: 1}); err != nil {
		return err
	}

	err = session.Shell()
	if err != nil {
		return err
	}
	err = session.Wait()
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

// OIDCConfig represents a config for the OIDC auth flow.
type OIDCConfig struct {
	// CallbackPath is the path of the callback handler.
	CallbackPath string

	// CallbackPort is the port of the callback handler.
	CallbackPort string

	// Issuer is the URL of the OpenID Connect provider.
	Issuer string

	// HashKey is used to authenticate values using HMAC.
	HashKey []byte

	// BlockKey is used to encrypt values using AES.
	BlockKey []byte

	// IDToken is the ID token returned by the OIDC provider.
	IDToken string

	// Logger function for debug.
	Logf func(format string, args ...interface{})

	oauth2.Config
}

// GetToken starts a local HTTP server, opens the web browser to initiate the OIDC Discovery and
// Token Exchange flow, blocks until the user completes authentication and is redirected back, and returns
// the OIDC tokens.
func GetToken(ctx context.Context, config *OIDCConfig) (string, error) {
	if err := config.validateAndSetDefaults(); err != nil {
		return "", fmt.Errorf("invalid config: %w", err)
	}

	cookieHandler := httphelper.NewCookieHandler(config.HashKey, config.BlockKey, httphelper.WithUnsecure())

	options := []rp.Option{
		rp.WithCookieHandler(cookieHandler),
		rp.WithVerifierOpts(rp.WithIssuedAtOffset(5 * time.Second)),
	}
	if config.ClientSecret == "" {
		options = append(options, rp.WithPKCE(cookieHandler))
	}

	relyingParty, err := rp.NewRelyingPartyOIDC(config.Issuer, config.ClientID, config.ClientSecret, config.RedirectURL, config.Scopes, options...)
	if err != nil {
		logrus.Fatalf("error creating relyingParty %s", err.Error())
	}

	//ctx := context.Background()
	state := func() string {
		return uuid.New().String()
	}

	resultChan := make(chan *oidc.Tokens[*oidc.IDTokenClaims])

	go func() {
		tokens := cli.CodeFlow[*oidc.IDTokenClaims](ctx, relyingParty, config.CallbackPath, config.CallbackPort, state)
		resultChan <- tokens
	}()

	select {
	case tokens := <-resultChan:
		return tokens.AccessToken, nil
	case <-ctx.Done():
		return "", errors.New("Timeout: OIDC authentication took too long")
	}
}

// validateAndSetDefaults validates the config and sets default values.
func (c *OIDCConfig) validateAndSetDefaults() error {
	if c.ClientID == "" {
		return fmt.Errorf("ClientID must be set")
	}

	c.HashKey = securecookie.GenerateRandomKey(32)
	c.BlockKey = securecookie.GenerateRandomKey(32)

	if c.Logf == nil {
		c.Logf = func(string, ...interface{}) {}
	}

	c.Scopes = strings.Split(DefaultAuthScopes, " ")

	return nil
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
		HostKeyCallback: hostKeyCallback,
	}
}

func sshAuthMethodFromFile(keyPath string) (ssh.AuthMethod, error) {
	content, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("could not read zssh file [%s]: %w", keyPath, err)
	}
	_, _, _, _, pubkeyErr := ssh.ParseAuthorizedKey(content)
	if pubkeyErr == nil {
		log.Fatal("the provided key for ssh authentication is a public key, but a private key is required")
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
	localFile, err := os.ReadFile(localPath)

	if err != nil {
		return errors.Wrapf(err, "unable to read local file %v", localFile)
	}

	rmtFile, err := client.OpenFile(remotePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC)

	if err != nil {
		return errors.Wrapf(err, "unable to open remote file %v", remotePath)
	}
	defer func() { _ = rmtFile.Close() }()

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

func EstablishClient(f *SshFlags, target string, targetIdentity string) *ssh.Client {
	ctx := NewContext(f, true)
	Auth(ctx)

	_, ok := ctx.GetService(f.ServiceName)
	if !ok {
		log.Fatalf("service not found: %s", f.ServiceName)
	}
	dialOptions := &ziti.DialOptions{
		ConnectTimeout: 0,
		Identity:       targetIdentity,
		AppData:        nil,
	}
	svc, err := ctx.DialWithOptions(f.ServiceName, dialOptions)
	if err != nil {
		log.Fatalf("error when dialing service name %s. %v", f.ServiceName, err)
	}
	username := ParseUserName(target, false)
	if username == "" {
		if f.Username == "" {
			username = ParseUserName(target, true)
		} else {
			username = f.Username
		}
	}
	factory := NewSshConfigFactoryImpl(username, f.SshKeyPath)
	config := factory.Config()
	sshConn, err := Dial(config, svc)
	if err != nil {
		log.Fatalf("error dialing SSH Conn: %v", err)
	}
	return sshConn
}

func getConfig(cfgFile string) (zitiCfg *ziti.Config) {
	zitiCfg, err := ziti.NewConfigFromFile(cfgFile)
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
			log.Infof("Remote File/Directory: %s doesn't exist [%v]", remotePath, err)
		}
	}
	return remotePath
}

// processOutput processes the stdout and stderr streams concurrently
func processOutput(stdout io.Reader, stderr io.Reader) {
	var wg sync.WaitGroup
	wg.Add(2)

	// Goroutine to process stdout
	go func() {
		defer wg.Done()
		if _, err := io.Copy(os.Stdout, stdout); err != nil {
			log.Fatalf("Error copying stdout: %v", err)
		}
	}()

	// Goroutine to process stderr
	go func() {
		defer wg.Done()
		if _, err := io.Copy(os.Stderr, stderr); err != nil {
			log.Fatalf("Error copying stderr: %v", err)
		}
	}()

	// Wait for both goroutines to finish
	wg.Wait()
}

type zitiEdgeConnAdapter struct {
	orig net.Addr
}

func (a zitiEdgeConnAdapter) Network() string {
	return ""
}
func (a zitiEdgeConnAdapter) String() string {
	// ziti connections will have the format: "ziti-edge-router connId=%v, logical=%v", e.MsgCh.Id(), e.MsgCh.LogicalName()
	// see ziti/edge/addr.go in github.com/openziti/sdk-golang if it changes
	// example: ziti-edge-router connId=1, logical=ziti-sdk[router=tls:ec2-3-18-113-172.us-east-2.compute.amazonaws.com:8442]
	parts := strings.Split(a.orig.String(), ":")
	answer := strings.Join(parts[len(parts)-2:], ":")
	answer = strings.ReplaceAll(answer, "]", "")
	return answer
}

func keyToString(k ssh.PublicKey) string {
	return k.Type() + " " + base64.StdEncoding.EncodeToString(k.Marshal())
}

func hostKeyCallback(hostname string, remote net.Addr, key ssh.PublicKey) error {
	var keyErr *knownhosts.KeyError
	remoteCopy := zitiEdgeConnAdapter{
		orig: remote,
	}

	if err := ensureKnownHosts(); err != nil {
		return err
	}

	knownHosts := knownHostsFile()

	cb, err := knownhosts.New(knownHosts)
	if err != nil {
		return err
	}

	err = cb(hostname, remoteCopy, key)
	if err != nil {
		if err.Error() == "knownhosts: key is unknown" {
			log.Warnf("key is not known: %s", keyToString(key))
			time.Sleep(50 * time.Millisecond)
			fmt.Print("do you want to add this key to your known_hosts file? (N/y): ")

			reader := bufio.NewReader(os.Stdin)
			answer, readerr := reader.ReadString('\n')
			if readerr != nil {
				log.Fatalf("error reading line: %v", readerr)
			}

			if strings.ToLower(answer)[:1] == "y" {
				adderr := addKnownHostUnhashed(remoteCopy.String(), key)
				if adderr != nil {
					log.Fatalf("error adding key to known_hosts: %v", adderr)
				}
				log.Infof("added key to known_hosts: %s", keyToString(key))

				cb, err = knownhosts.New(knownHosts)
				if err != nil {
					return err
				}
				err = cb(hostname, remoteCopy, key)
			} else {
				os.Exit(1)
			}
		}
	}

	// Make sure that the error returned from the callback is host not in file error.
	// If keyErr.Want is greater than 0 length, that means host is in file with different key.
	if errors.As(err, &keyErr) && len(keyErr.Want) > 0 {
		return keyErr
	}

	if err != nil {
		return err
	}

	return nil
}

func ensureKnownHosts() error {
	filePath := knownHostsFile()
	_, err := os.Stat(filePath)
	if os.IsNotExist(err) {
		// Create the directories if they don't exist
		dir := filepath.Dir(filePath)
		if err := os.MkdirAll(dir, 0700); err != nil {
			return fmt.Errorf("failed to create directories: %w", err)
		}

		// Create the file with 0600 permissions
		file, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY, 0600)
		if err != nil {
			return fmt.Errorf("failed to create file: %w", err)
		}
		defer file.Close()
	} else if err != nil {
		return fmt.Errorf("error checking file: %w", err)
	}

	return nil
}

func knownHostsFile() string {
	home, err := os.UserHomeDir()
	if err != nil {
		log.Fatalf("unable to determine home directory - cannot find known_hosts file: %v", err)
	}
	return path.Join(home, ".ssh", "known_hosts")
}

// couldn't get the openssh hashing to work yet. unhashed works and it's good enoguh for now.
func addKnownHostUnhashed(hostname string, key ssh.PublicKey) error {
	knownHosts := knownHostsFile()
	f, err := os.OpenFile(knownHosts, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	keyBytes := key.Marshal()
	keyString := base64.StdEncoding.EncodeToString(keyBytes)
	entry := fmt.Sprintf("%s %s %s\n", knownhosts.Normalize(hostname), key.Type(), keyString)

	if _, err := f.WriteString(entry); err != nil {
		return fmt.Errorf("failed to write to known_hosts file: %v", err)
	}

	return err
}
