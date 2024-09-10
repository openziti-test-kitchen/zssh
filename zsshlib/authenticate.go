package zsshlib

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/openziti/edge-api/rest_model"
	edgeapis "github.com/openziti/sdk-golang/edge-apis"
	"github.com/openziti/sdk-golang/ziti"
)

func NewContext(flags *SshFlags, enableMfaListener bool) ziti.Context {
	oidcToken := ""
	var oidcErr error

	if flags.OIDC.OIDCOnly && !flags.OIDC.Mode {
		flags.OIDC.Mode = true //override Mode to true
	}

	if flags.OIDC.Mode {
		oidcToken, oidcErr = OIDCFlow(context.Background(), flags)
		if oidcErr != nil {
			log.Fatalf("error performing OIDC flow: %v", oidcErr)
		}
	}
	var ctx ziti.Context
	if !flags.OIDC.OIDCOnly {
		conf := getConfig(flags.ZConfig)
		c, err := ziti.NewContext(conf)
		if err != nil {
			log.Fatalf("error creating ziti context: %v", err)
		}
		ctx = c
		conf.Credentials.AddJWT(oidcToken)
	} else {
		ozController := flags.OIDC.ControllerUrl
		if !strings.Contains(ozController, "://") {
			ozController = "https://" + ozController
		}
		caPool, err := ziti.GetControllerWellKnownCaPool(ozController)
		if err != nil {
			log.Fatalf("error creating ziti context: %v", err)
		}

		credentials := edgeapis.NewJwtCredentials(oidcToken)
		credentials.CaPool = caPool
		cfg := &ziti.Config{
			ZtAPI:       ozController + "/edge/client/v1",
			Credentials: credentials,
		}
		credentials.AddJWT(oidcToken) // satisfy the ext-jwt-auth primary + secondary
		cfg.ConfigTypes = append(cfg.ConfigTypes, "all")

		c, ctxErr := ziti.NewContext(cfg)
		if ctxErr != nil {
			log.Fatalf("error creating ziti context: %v", ctxErr)
		}
		ctx = c
	}

	if enableMfaListener {
		ctx.Events().AddMfaTotpCodeListener(func(c ziti.Context, detail *rest_model.AuthQueryDetail, response ziti.MfaCodeResponse) {
			ok := false
			for !ok {
				fmt.Println("MFA TOTP required to fully authenticate")
				code := ReadCode(false)
				if err := response(code); err != nil {
					fmt.Println("error verifying MFA TOTP: ", err)
				} else {
					ok = true
				}
			}
		})
	}

	return ctx
}

func Auth(ctx ziti.Context) {
	if err := ctx.Authenticate(); err != nil {
		log.Errorf("error creating ziti context: %v", err)
		log.Fatalf("could not authenticate. verify your identity is correct and matches all necessary authentication conditions.")
	}
}

func ReadCode(allowEmpty bool) string {
	code := ""
	reader := bufio.NewReader(os.Stdin)
	for code == "" {
		fmt.Print("MFA TOTP code: ")
		code, _ = reader.ReadString('\n')
		code = strings.TrimSpace(code)
		if allowEmpty {
			break
		}
	}
	return code
}
