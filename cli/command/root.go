package command

import (
	"fmt"
	"os"
	"path"

	homedir "github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
)

// Global flags and options
var (
	Region   string
	CacheDir string
)

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:   "awsc <command> <subcommand> [args]",
	Short: "AWS companion app",
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := RootCmd.Execute(); err != nil {
		fmt.Fprintln(RootCmd.OutOrStdout(), err)
		os.Exit(1)
	}
}

func init() {
	homeDir, err := homedir.Dir()
	if err != nil {
		homeDir = "/tmp"
	}
	defaultCacheDir := path.Join(homeDir, ".awsc")
	RootCmd.PersistentFlags().StringVarP(&Region, "region", "r", "", "The region to use, overrides the value from the shared AWS credential files")
	RootCmd.PersistentFlags().StringVarP(&CacheDir, "cache-dir", "c", defaultCacheDir, "Cache directory")

	envs := map[string]string{
		"AWS_REGION": "region",
	}

	for env, flag := range envs {
		flag := RootCmd.PersistentFlags().Lookup(flag)
		flag.Usage = fmt.Sprintf("%v [$%v]", flag.Usage, env)
		if value := os.Getenv(env); value != "" {
			flag.Value.Set(value)
		}
	}
}
