package command

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/opsidian/awsc/awsc/sts"
	"github.com/spf13/cobra"
)

var (
	mfaAuthExpiry int64
	sessionName   string
	mfaTokenCode  string
)

var stsCmd = &cobra.Command{
	Use:   "sts command <params>",
	Short: "AWS STS commands",
}

var mfaAuthCmd = &cobra.Command{
	Use:   "auth",
	Short: "Create temporary credentials with MFA authentication",
	RunE: func(cmd *cobra.Command, args []string) error {
		config := &aws.Config{}
		if Region != "" {
			config.Region = aws.String(Region)
		}
		return sts.MFAAuth(config, cmd.OutOrStdout(), CacheDir, sessionName, mfaAuthExpiry, mfaTokenCode)
	},
	SilenceUsage:  true,
	SilenceErrors: true,
}

func init() {
	mfaAuthCmd.PersistentFlags().Int64VarP(&mfaAuthExpiry, "duration-seconds", "", 43200, "The  duration, in seconds, that the credentials should remain valid.")
	mfaAuthCmd.PersistentFlags().StringVarP(&sessionName, "session-name", "", "", "Name of the session")
	mfaAuthCmd.PersistentFlags().StringVarP(&mfaTokenCode, "token-code", "", "", "MFA token code")
	RootCmd.AddCommand(mfaAuthCmd)
}
