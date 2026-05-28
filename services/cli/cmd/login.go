package cmd

import (
	"fmt"

	"github.com/abskrj/velane/services/cli/internal/keyring"
	"github.com/spf13/cobra"
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Store API key in the system keychain",
	RunE: func(cmd *cobra.Command, args []string) error {
		key, _ := cmd.Flags().GetString("key")
		if key == "" {
			return fmt.Errorf("--key is required")
		}
		if err := keyring.SaveAPIKey(key); err != nil {
			return fmt.Errorf("save api key: %w", err)
		}
		fmt.Println("API key saved to keychain.")
		return nil
	},
}

func init() {
	loginCmd.Flags().String("key", "", "API key (vl_xxxx)")
	_ = loginCmd.MarkFlagRequired("key")
}
