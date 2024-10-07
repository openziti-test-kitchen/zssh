package zsshlib

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/zitadel/oidc/v2/pkg/client/rp"
	"github.com/zitadel/oidc/v2/pkg/client/rp/cli"
	httphelper "github.com/zitadel/oidc/v2/pkg/http"
	"github.com/zitadel/oidc/v2/pkg/oidc"
	"golang.org/x/oauth2"
)

func OIDCFlow(initialContext context.Context, flags *SshFlags) (string, error) {
	callbackPath := "/auth/callback"
	cfg := &OIDCConfig{
		Config: oauth2.Config{
			ClientID:     flags.OIDC.ClientID,
			ClientSecret: flags.OIDC.ClientSecret,
			RedirectURL:  fmt.Sprintf("http://localhost:%v%v", flags.OIDC.CallbackPort, callbackPath),
		},
		CallbackPath:          callbackPath,
		CallbackPort:          flags.OIDC.CallbackPort,
		Issuer:                flags.OIDC.Issuer,
		Logf:                  log.Debugf,
		AdditionalLoginParams: flags.OIDC.AdditionalLoginParams,
	}
	waitFor := 30 * time.Second
	ctx, cancel := context.WithTimeout(initialContext, waitFor)
	defer cancel() // Ensure the cancel function is called to release resources

	log.Infof("OIDC requested. If the CLI appears to be hung, check your browser for a login prompt. Waiting up to %v", waitFor)
	token, err := GetToken(ctx, cfg)
	if err != nil {
		return "", err
	}

	log.Debugf("ID token: %s", token)
	log.Infof("OIDC auth flow succeeded")

	return token, nil
}

func zsshCodeFlow[C oidc.IDClaims](ctx context.Context, relyingParty rp.RelyingParty, config *OIDCConfig) *oidc.Tokens[C] {
	codeflowCtx, codeflowCancel := context.WithCancel(ctx)
	defer codeflowCancel()

	tokenChan := make(chan *oidc.Tokens[C], 1)

	callback := func(w http.ResponseWriter, r *http.Request, tokens *oidc.Tokens[C], state string, rp rp.RelyingParty) {
		tokenChan <- tokens
		msg := "<script type=\"text/javascript\">window.close()</script><body onload=\"window.close()\">You may close this window</body><p><strong>Success!</strong></p>"
		msg = msg + "<p>You are authenticated and can now return to the CLI.</p>"
		w.Write([]byte(msg))
	}

	authHandlerWithQueryState := func(party rp.RelyingParty) http.HandlerFunc {
		var urlParamOpts rp.URLParamOpt
		for _, v := range config.AdditionalLoginParams {
			parts := strings.Split(v, "=")
			urlParamOpts = rp.WithURLParam(parts[0], parts[1])
		}
		if urlParamOpts == nil {
			urlParamOpts = func() []oauth2.AuthCodeOption {
				return []oauth2.AuthCodeOption{}
			}
		}
		return func(w http.ResponseWriter, r *http.Request) {
			rp.AuthURLHandler(func() string {
				return uuid.New().String()
			}, party, urlParamOpts /*rp.WithURLParam("audience", "openziti2")*/)(w, r)
		}
	}

	http.Handle("/login", authHandlerWithQueryState(relyingParty))
	http.Handle(config.CallbackPath, rp.CodeExchangeHandler(callback, relyingParty))

	httphelper.StartServer(codeflowCtx, ":"+config.CallbackPort)

	cli.OpenBrowser("http://localhost:" + config.CallbackPort + "/login")

	return <-tokenChan
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

	// Additional params to add to the login request
	AdditionalLoginParams []string

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

	resultChan := make(chan *oidc.Tokens[*oidc.IDTokenClaims])

	go func() {
		tokens := zsshCodeFlow[*oidc.IDTokenClaims](ctx, relyingParty, config)
		resultChan <- tokens
	}()

	select {
	case tokens := <-resultChan:
		Logger().Debugf("Refresh token: %s", tokens.RefreshToken)
		return tokens.AccessToken, nil
	case <-ctx.Done():
		return "", errors.New("timeout: OIDC authentication took too long")
	}
}
