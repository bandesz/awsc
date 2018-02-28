package command

import (
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/opsidian/awsc/awsc/sts"
	"github.com/spf13/cobra"
)

var (
	awsProfile    string
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

		return sts.MFAAuth(config, cmd.OutOrStdout(), CacheDir,
			awsProfile, sessionName, mfaAuthExpiry, mfaTokenCode)
	},
	SilenceUsage:  true,
	SilenceErrors: true,
}

func init() {
	mfaAuthCmd.PersistentFlags().StringVarP(&awsProfile, "aws-profile", "", "default", "The AWS profile name")
	mfaAuthCmd.PersistentFlags().Int64VarP(&mfaAuthExpiry, "duration-seconds", "", 43200, "The duration, in seconds, that the credentials should remain valid.")
	mfaAuthCmd.PersistentFlags().StringVarP(&sessionName, "session-name", "", "", "Name of the session (defaults to the AWS profile name)")
	mfaAuthCmd.PersistentFlags().StringVarP(&mfaTokenCode, "token-code", "", "", "MFA token code")
	RootCmd.AddCommand(mfaAuthCmd)

	envs := map[string]string{
		"AWS_PROFILE":        "aws-profile",
		"AWS_MFA_TOKEN_CODE": "token-code",
	}

	for env, flag := range envs {
		flag := mfaAuthCmd.PersistentFlags().Lookup(flag)
		flag.Usage = fmt.Sprintf("%v [$%v]", flag.Usage, env)
		if value := os.Getenv(env); value != "" {
			flag.Value.Set(value)
		}
	}
}
