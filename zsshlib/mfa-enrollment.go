package zsshlib

import (
	"bufio"
	"context"
	"fmt"
	"github.com/openziti/sdk-golang/ziti"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"net/url"
	"os"
	"strings"
	"zssh/config"
)

func NewMfaCmd(flags *SshFlags) *cobra.Command {
	var mfaCmd = &cobra.Command{
		Use:   "mfa",
		Short: "Manage MFA for the provided identity",
	}

	mfaCmd.AddCommand(NewEnableCmd(flags))
	return mfaCmd
}

func NewEnableCmd(flags *SshFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "enable",
		Short: "Enable MFA. Enables MFA TOTP for the provided identity",
		Run: func(cmd *cobra.Command, args []string) {
			cfg := config.DefaultConfig()
			Combine(cmd, flags, cfg)

			oidcToken := ""
			var err error
			if flags.OIDC.Mode {
				oidcToken, err = OIDCFlow(context.Background(), flags)
				if err != nil {
					logrus.Fatalf("error performing OIDC flow: %v", err)
				}
			}
			EnableMFA(flags, oidcToken)
		},
	}

	flags.OIDCFlags(cmd)
	return cmd
}

func EnableMFA(flags *SshFlags, oidcToken string) {
	conf := getConfig(flags.ZConfig)
	ctx, err := ziti.NewContext(conf)
	conf.Credentials.AddJWT(oidcToken)
	if err != nil {
		logrus.Fatalf("error creating ziti context: %v", err)
	}

	if err = ctx.Authenticate(); err != nil {
		logrus.Errorf("error creating ziti context: %v", err)
		logrus.Fatalf("could not authenticate. verify your identity is correct and matches all necessary authentication conditions.")
	}
	
	if deet, err := ctx.EnrollZitiMfa(); err != nil {
		logrus.Fatalf("error enrolling ziti context: %v", err)
	} else {
		parsedURL, err := url.Parse(deet.ProvisioningURL)
		if err != nil {
			panic(err)
		}

		params := parsedURL.Query()
		secret := params.Get("secret")
		fmt.Println()
		fmt.Println("Generate and enter the correct code to continue.")
		fmt.Println("Add this secret to your TOTP generator and verify the code.")
		fmt.Println()
		fmt.Println("  MFA TOTP Secret: ", secret)
		fmt.Println()
		reader := bufio.NewReader(os.Stdin)
		code := ""
		for code == "" {
			fmt.Print("Enter MFA: ")
			code, _ = reader.ReadString('\n')
			code = strings.TrimSpace(code)
			fmt.Println("You entered: " + code + " - verifying")
		}
		if err := ctx.VerifyZitiMfa(code); err != nil {
			logrus.Fatalf("error verifying ziti context: %v", err)
		}
		fmt.Println()
		fmt.Println("Code verified. These are your recovery codes. Save these codes somewhere safe.")
		fmt.Println("If you lose your TOTP generator, these codes can be used to verify")
		fmt.Println("your MFA TOTP to generate a new code.")
		fmt.Println()
		recoveryCodes := deet.RecoveryCodes

		fmt.Println("┌────────┬────────┬────────┬────────┬────────┐")

		for i := 0; i < len(recoveryCodes); i += 5 {
			for j := 0; j < 5 && i+j < len(recoveryCodes); j++ {
				fmt.Printf("│ %6s ", recoveryCodes[i+j])
			}
			fmt.Println("│")
			if i+5 < len(recoveryCodes) {
				fmt.Println("├────────┼────────┼────────┼────────┼────────┤")
			}
		}

		fmt.Println("└────────┴────────┴────────┴────────┴────────┘")
	}
}
