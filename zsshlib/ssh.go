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
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/gorilla/securecookie"
	"github.com/zitadel/oidc/v2/pkg/oidc"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/openziti/sdk-golang/ziti"
	"github.com/openziti/sdk-golang/ziti/config"
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
	// OktaAuthEndpoint is the Okta authentication endpoint.
	OktaAuthEndpoint = oauth2.Endpoint{
		AuthURL:   "https://dev-82868739.okta.com/oauth2/v1/authorize",
		TokenURL:  "https://dev-82868739.okta.com/oauth2/v1/token",
		AuthStyle: oauth2.AuthStyleInParams,
	}

	// OktaAuthScope is the Okta authorization scope(s).
	//OktaAuthScope = "okta.users.read.self"
	OktaAuthScope = "okta.users.read.self openid profile"

	ErrTokenIsNil = errors.New("OAuth 2.0 token is nil")
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

	err = session.Shell()
	if err != nil {
		return err
	}
	session.Wait()
	return nil
}

func Dial(config *ssh.ClientConfig, conn net.Conn) (*ssh.Client, error) {
	c, chans, reqs, err := ssh.NewClientConn(conn, "", config)
	if err != nil {
		return nil, err
	}
	return ssh.NewClient(c, chans, reqs), nil
}

// Config represents a config for the OAuth 2.0 flow.
type Config struct {
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

	// Logger function for debug.
	Logf func(format string, args ...interface{})

	oauth2.Config
}

// GetToken create a new OAuth2 token
func GetToken(ctx context.Context, config *Config) (*oauth2.Token, error) {
	if err := config.validateAndSetDefaults(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return config.getTokenFromWeb(ctx)
}

// validateAndSetDefaults validates the config and sets default values.
func (c *Config) validateAndSetDefaults() error {
	if c.ClientID == "" {
		return fmt.Errorf("ClientID must be set")
	}

	//if c.ClientID == "" || c.ClientSecret == "" {
	//	return fmt.Errorf("both ClientID and ClientSecret must be set")
	//}

	c.HashKey = securecookie.GenerateRandomKey(32)
	c.BlockKey = securecookie.GenerateRandomKey(32)

	if c.Logf == nil {
		c.Logf = func(string, ...interface{}) {}
	}

	c.Scopes = strings.Split(OktaAuthScope, " ")
	c.Endpoint = OktaAuthEndpoint

	return nil
}

// getTokenFromWeb starts a local HTTP server, opens the web browser to initiate the OAuth 2.0 flow,
// blocks until the user completes authorization and is redirected back, and returns the access token.
func (c *Config) getTokenFromWeb(ctx context.Context) (*oauth2.Token, error) {
	cookieHandler := httphelper.NewCookieHandler(c.HashKey, c.BlockKey, httphelper.WithUnsecure())

	options := []rp.Option{
		rp.WithCookieHandler(cookieHandler),
		rp.WithVerifierOpts(rp.WithIssuedAtOffset(5 * time.Second)),
	}
	if c.ClientSecret == "" {
		options = append(options, rp.WithPKCE(cookieHandler))
	}

	provider, err := rp.NewRelyingPartyOIDC(c.Issuer, c.ClientID, c.ClientSecret, c.RedirectURL, c.Scopes, options...)
	if err != nil {
		logrus.Fatalf("error creating provider %s", err.Error())
	}

	state := func() string {
		return uuid.New().String()
	}

	http.Handle("/", rp.AuthURLHandler(state, provider, rp.WithPromptURLParam("Welcome back!")))

	//marshalUserinfo := func(w http.ResponseWriter, r *http.Request, tokens *oidc.Tokens[*oidc.IDTokenClaims], state string, rp rp.RelyingParty, info *oidc.UserInfo) {
	//	data, err := json.Marshal(info)
	//	if err != nil {
	//		http.Error(w, err.Error(), http.StatusInternalServerError)
	//		return
	//	}
	//	w.Write(data)
	//}

	marshalToken := func(w http.ResponseWriter, r *http.Request, tokens *oidc.Tokens[*oidc.IDTokenClaims], state string, rp rp.RelyingParty) {
		data, err := json.Marshal(tokens)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Write(data)
	}

	// register the CodeExchangeHandler at the callbackPath
	// the CodeExchangeHandler handles the auth response, creates the token request and calls the callback function
	// with the returned tokens from the token endpoint
	// in this example the callback function itself is wrapped by the UserinfoCallback which
	// will call the Userinfo endpoint, check the sub and pass the info into the callback function
	//http.Handle(c.CallbackPath, rp.CodeExchangeHandler(rp.UserinfoCallback(marshalUserinfo), provider))

	// if you would use the callback without calling the userinfo endpoint, simply switch the callback handler for:
	//
	http.Handle(c.CallbackPath, rp.CodeExchangeHandler(marshalToken, provider))

	lis := fmt.Sprintf("127.0.0.1:%s", c.CallbackPort)
	logrus.Infof("listening on http://%s/", lis)
	logrus.Info("press ctrl+c to stop")
	logrus.Fatal(http.ListenAndServe(lis, nil))

	//ready := make(chan string, 1)
	//defer close(ready)
	//cfg := oauth2cli.Config{
	//	OAuth2Config:           *c.oAuth2Config,
	//	LocalServerReadyChan:   ready,
	//	LocalServerBindAddress: []string{"localhost:63275"},
	//	Logf:                   c.Logf,
	//}
	//
	//var token *oauth2.Token
	//eg, ctx := errgroup.WithContext(ctx)
	//eg.Go(func() error {
	//	select {
	//	case url := <-ready:
	//		log.Printf("Open %s", url)
	//		if err := browser.OpenURL(url); err != nil {
	//			log.Printf("could not open browser: %v", err)
	//		}
	//		return nil
	//	case <-ctx.Done():
	//		return fmt.Errorf("context done while waiting for authorization: %w", ctx.Err())
	//	}
	//})
	//eg.Go(func() error {
	//	token, err := oauth2cli.GetToken(ctx, cfg)
	//	if err != nil {
	//		return fmt.Errorf("could not get token: %w", err)
	//	}
	//	log.Printf("You got a valid token until %s", token.Expiry)
	//	return nil
	//})
	//// if err := eg.Wait(); err != nil {
	//// 	log.Fatalf("authorization error: %s", err)
	//// }

	return nil, nil
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
