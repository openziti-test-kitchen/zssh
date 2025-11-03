package zsshlib

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/zitadel/oidc/v3/pkg/client/rp"
	"github.com/zitadel/oidc/v3/pkg/client/rp/cli"
	httphelper "github.com/zitadel/oidc/v3/pkg/http"
	"github.com/zitadel/oidc/v3/pkg/oidc"
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

	log.Infof("OIDC auth flow succeeded")

	return token, nil
}

func zsshCodeFlow[C oidc.IDClaims](ctx context.Context, relyingParty rp.RelyingParty, config *OIDCConfig) *oidc.Tokens[C] {

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

	mux := http.NewServeMux()
	mux.Handle("/login", authHandlerWithQueryState(relyingParty))
	mux.Handle(config.CallbackPath, rp.CodeExchangeHandler(callback, relyingParty))
	server := &http.Server{
		Addr:    ":" + config.CallbackPort,
		Handler: mux,
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Errorf("ListenAndServe error: %v", err)
		}
	}()

	// Shutdown on context cancellation
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Debugf("Server shutdown warning. Took too long to shutDown: %v", err)
		}
	}()

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

	relyingParty, err := rp.NewRelyingPartyOIDC(ctx, config.Issuer, config.ClientID, config.ClientSecret, config.RedirectURL, config.Scopes, options...)
	if err != nil {
		logrus.Fatalf("error creating relyingParty %s", err.Error())
	}

	resultChan := make(chan *oidc.Tokens[*oidc.IDTokenClaims])

	go func() {
		codeflowCtx, cancel := context.WithCancel(ctx)
		defer cancel()
		tokens := zsshCodeFlow[*oidc.IDTokenClaims](codeflowCtx, relyingParty, config)
		select {
		case resultChan <- tokens:
		case <-codeflowCtx.Done():
			// Context cancelled, exit without blocking
		}
	}()

	select {
	case tokens := <-resultChan:
		log.Debugf("-- Refresh token: %s", tokens.RefreshToken)
		PrintDecodedToken(tokens.RefreshToken)
		log.Debugf("-- ID token: %s", tokens.IDToken)
		PrintDecodedToken(tokens.IDToken)
		log.Debugf("-- Access token: %s", tokens.AccessToken)
		PrintDecodedToken(tokens.AccessToken)
		return tokens.AccessToken, nil
	case <-ctx.Done():
		return "", errors.New("timeout: OIDC authentication took too long")
	}
}

func PrintDecodedToken(token string) {
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		log.Debugf("invalid token: %s", token)
		return
	}

	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		log.Debugf("decode error: %v", err)
		return
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		log.Debugf("json unmarshal error: %v", err)
		log.Debugf("raw payload: %s", string(payloadBytes))
		return
	}

	out, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		log.Errorf("json marshal error: %v", err)
		return
	}

	log.Debugf("Decoded token:\n%s", string(out))
}
