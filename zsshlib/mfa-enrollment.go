package zsshlib

import (
	"fmt"
	"github.com/sirupsen/logrus"
	"github.com/skip2/go-qrcode"
	"github.com/spf13/cobra"
	"net/url"
	"zssh/config"
)

func NewMfaCmd(flags *SshFlags) *cobra.Command {
	var cmd = &cobra.Command{
		Use:   "mfa",
		Short: "Manage MFA for the provided identity",
	}

	cmd.AddCommand(NewEnableCmd(flags), NewRemoveMfaCmd(flags))
	return cmd
}

func NewEnableCmd(flags *SshFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "enable",
		Short: "Enable MFA. Enables MFA TOTP for the provided identity",
		Run: func(cmd *cobra.Command, args []string) {
			cfg := config.DefaultConfig()
			Combine(cmd, flags, cfg)
			EnableMFA(flags)
		},
	}

	flags.OIDCFlags(cmd)
	cmd.Flags().BoolVarP(&flags.OIDC.AsAscii, "qr-code", "q", false, fmt.Sprintf("display MFA secret as ascii QR code: %t", false))
	return cmd
}

func NewRemoveMfaCmd(flags *SshFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remove",
		Short: "Remove MFA. Removes the MFA TOTP enablement for the provided identity",
		Run: func(cmd *cobra.Command, args []string) {
			cfg := config.DefaultConfig()
			Combine(cmd, flags, cfg)
			RemoveMfa(flags)
		},
	}

	flags.OIDCFlags(cmd)
	return cmd
}

func EnableMFA(flags *SshFlags) {
	ctx := NewContext(flags, true)
	Auth(ctx)

	if deet, err := ctx.EnrollZitiMfa(); err != nil {
		logrus.Error("Attempting to enroll for MFA TOTP failed.")
		logrus.Error("This identity is likely already enrolled or is in the process of being enrolled.")
		logrus.Error("To continue the MFA TOTP enrollment process you must \"remove\" MFA TOTP first.")
		logrus.Fatalf("Run \"mfa remove\" to clear the current state, then try again.")
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

		var q *qrcode.QRCode
		q, err = qrcode.New(fmt.Sprintf("otpauth://totp/zsshlabel?secret=%s&issuer=zssh", secret), qrcode.Highest)
		art := q.ToString(false)
		fmt.Println(art)

		fmt.Println()

		code := ReadCode(false)

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

func RemoveMfa(flags *SshFlags) {
	ctx := NewContext(flags, false)
	Auth(ctx)

	fmt.Println()
	fmt.Println("If MFA TOTP has been successfully enrolled, you must enter a valid code or a valid recovery code,")
	fmt.Println("otherwise, enter any value to continue.")
	fmt.Println()
	code := ReadCode(true)
	if err := ctx.RemoveZitiMfa(code); err != nil {
		logrus.Fatalf("error removing MFA TOTP: %v", err)
	}
}
