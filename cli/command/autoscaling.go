package command

import (
	"errors"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/opsidian/awsc/awsc/autoscaling"
	"github.com/spf13/cobra"
)

var autoScalingCmd = &cobra.Command{
	Use:   "autoscaling command <params>",
	Short: "AWS auto scaling commands",
}

var migrateCmd = &cobra.Command{
	Use:        "migrate <auto scaling group name>",
	Short:      "Migrate",
	ArgAliases: []string{"name"},
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) < 1 {
			return errors.New("auto scaling group name is missing")
		}
		if len(args) > 1 {
			return errors.New("too many arguments")
		}
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		config := &aws.Config{}
		if Region != "" {
			config.Region = aws.String(Region)
		}
		return autoscaling.MigrateInstances(config, cmd.OutOrStdout(), args[0])
	},
	SilenceUsage:  true,
	SilenceErrors: true,
}

func init() {
	autoScalingCmd.AddCommand(migrateCmd)
	RootCmd.AddCommand(autoScalingCmd)
}
