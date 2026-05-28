package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var (
	apiURL    string
	tenantSlug string
)

var rootCmd = &cobra.Command{
	Use:   "velane",
	Short: "Velane CLI — manage and invoke AI agent snippets",
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&apiURL, "api-url", "https://api.velane.io", "Velane control plane URL")
	rootCmd.PersistentFlags().StringVar(&tenantSlug, "tenant", "", "Tenant slug")

	rootCmd.AddCommand(loginCmd)
	rootCmd.AddCommand(snippetsCmd)
	rootCmd.AddCommand(invokeCmd)
	rootCmd.AddCommand(logsCmd)
	rootCmd.AddCommand(versionsCmd)
}
