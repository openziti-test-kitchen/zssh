package zsshlib

import (
	"bufio"
	"context"
	"fmt"
	"github.com/sirupsen/logrus"
	"os"
	"strings"

	"github.com/openziti/edge-api/rest_model"
	edgeapis "github.com/openziti/sdk-golang/edge-apis"
	"github.com/openziti/sdk-golang/ziti"
)

func NewContext(flags *SshFlags, enableMfaListener bool) ziti.Context {
	oidcToken := ""
	var oidcErr error
	if flags.OIDC.Mode || flags.OIDC.OIDCOnly {
		oidcToken, oidcErr = OIDCFlow(context.Background(), flags)
		if oidcErr != nil {
			logrus.Fatalf("error performing OIDC flow: %v", oidcErr)
		}
	}
	var ctx ziti.Context
	if !flags.OIDC.OIDCOnly {
		conf := getConfig(flags.ZConfig)
		c, err := ziti.NewContext(conf)
		if err != nil {
			logrus.Fatalf("error creating ziti context: %v", err)
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
			logrus.Fatalf("error creating ziti context: %v", err)
		}

		credentials := edgeapis.NewJwtCredentials(oidcToken)
		credentials.CaPool = caPool
		cfg := &ziti.Config{
			ZtAPI:       ozController + "/edge/client/v1",
			Credentials: credentials,
		}
		cfg.ConfigTypes = append(cfg.ConfigTypes, "all")

		c, ctxErr := ziti.NewContext(cfg)
		if ctxErr != nil {
			logrus.Fatalf("error creating ziti context: %v", ctxErr)
		}
		ctx = c
	}

	if enableMfaListener {
		ctx.Events().AddMfaTotpCodeListener(func(c ziti.Context, detail *rest_model.AuthQueryDetail, response ziti.MfaCodeResponse) {
			ok := false
			for !ok {
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
		logrus.Errorf("error creating ziti context: %v", err)
		logrus.Fatalf("could not authenticate. verify your identity is correct and matches all necessary authentication conditions.")
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
