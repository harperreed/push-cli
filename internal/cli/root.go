// ABOUTME: Root command and CLI setup for the push application.
// ABOUTME: Configures Cobra commands and resolves config/data paths.
package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

// appOptions carries CLI-wide path overrides.
type appOptions struct {
	configPath string
	dataDir    string
}

var opts = appOptions{}

// Execute runs the Cobra root command.
func Execute() error {
	cmd := newRootCmd()
	return cmd.Execute()
}

func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "push",
		Short: "Push bridges the Pushover API with a CLI and MCP server",
		Long:  "Push sends, receives, and persists Pushover messages for both human and AI assistant workflows.",
	}
	cmd.SilenceUsage = true

	cmd.PersistentFlags().StringVar(&opts.configPath, "config", "", "config file (default ~/.config/push/config.toml)")
	cmd.PersistentFlags().StringVar(&opts.dataDir, "data", "", "data directory (default ~/.local/share/push)")

	cmd.AddCommand(
		newLoginCmd(),
		newLogoutCmd(),
		newSendCmd(),
		newMessagesCmd(),
		newHistoryCmd(),
		newConfigCmd(),
		newMCPCmd(),
	)

	return cmd
}

func resolveConfigPath() (string, error) {
	if opts.configPath != "" {
		return opts.configPath, nil
	}

	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("locating config directory: %w", err)
	}
	return filepath.Join(configDir, "push", "config.toml"), nil
}

func resolveDataDir() (string, error) {
	if opts.dataDir != "" {
		return opts.dataDir, nil
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("locating home directory: %w", err)
	}
	return filepath.Join(homeDir, ".local", "share", "push"), nil
}
