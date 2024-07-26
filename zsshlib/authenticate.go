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

func Auth(flags *SshFlags) ziti.Context {
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

	ctx.Events().AddMfaTotpCodeListener(func(c ziti.Context, detail *rest_model.AuthQueryDetail, response ziti.MfaCodeResponse) {
		reader := bufio.NewReader(os.Stdin)
		codeok := false
		for !codeok {
			fmt.Print("Enter MFA: ")
			code, _ := reader.ReadString('\n')
			code = strings.TrimSpace(code)
			fmt.Println("You entered:" + code + " - verifying")
			if err := response(code); err != nil {
				fmt.Println("error verifying MFA TOTP: ", err)
			} else {
				codeok = true
			}
		}
	})

	if err = ctx.Authenticate(); err != nil {
		logrus.Errorf("error creating ziti context: %v", err)
		logrus.Fatalf("could not authenticate. verify your identity is correct and matches all necessary authentication conditions.")
	}

	return ctx
}

func ReadCode() string {
	code := ""
	reader := bufio.NewReader(os.Stdin)
	for code == "" {
		fmt.Print("Enter MFA: ")
		code, _ = reader.ReadString('\n')
		code = strings.TrimSpace(code)
	}
	return code
}
