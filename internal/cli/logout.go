// ABOUTME: Logout command for removing device credentials.
// ABOUTME: Clears stored Pushover device authentication.
package cli

import (
	"fmt"

	"github.com/harper/push/internal/config"
	"github.com/spf13/cobra"
)

func newLogoutCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "logout",
		Short: "Remove stored device credentials",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLogout(cmd)
		},
	}
	return cmd
}

func runLogout(cmd *cobra.Command) error {
	cfg, cfgPath, err := loadConfig()
	if err != nil {
		return err
	}
	if cfg.DeviceID == "" && cfg.DeviceSecret == "" {
		cmd.Println("No device credentials were stored.")
		return nil
	}

	cfg.DeviceID = ""
	cfg.DeviceSecret = ""

	if err := config.Save(cfgPath, cfg); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}

	cmd.Println("✓ Device credentials removed.")
	return nil
}
