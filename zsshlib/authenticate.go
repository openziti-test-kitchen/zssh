package zsshlib

import (
	"bufio"
	"context"
	"fmt"
	"github.com/openziti/edge-api/rest_model"
	"github.com/openziti/sdk-golang/ziti"
	"github.com/sirupsen/logrus"
	"os"
	"strings"
)

func NewContext(flags *SshFlags, enableMfaListener bool) ziti.Context {
	oidcToken := ""
	var oidcErr error
	if flags.OIDC.Mode {
		oidcToken, oidcErr = OIDCFlow(context.Background(), flags)
		if oidcErr != nil {
			logrus.Fatalf("error performing OIDC flow: %v", oidcErr)
		}
	}

	conf := getConfig(flags.ZConfig)
	ctx, err := ziti.NewContext(conf)
	conf.Credentials.AddJWT(oidcToken)
	if err != nil {
		logrus.Fatalf("error creating ziti context: %v", err)
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
