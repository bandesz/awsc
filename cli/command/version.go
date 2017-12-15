package command

import (
	"fmt"

	"github.com/opsidian/awsc/awsc"
	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Fprintf(cmd.OutOrStdout(), "awsc %s\n", awsc.Version)
	},
}

func init() {
	RootCmd.AddCommand(versionCmd)
}
