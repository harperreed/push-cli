// ABOUTME: Cobra command for interactive push credential and storage configuration.
// ABOUTME: Launches a bubbletea TUI wizard to collect Pushover credentials and backend settings.
package cli

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/harper/push/internal/config"
	"github.com/harper/push/internal/tui"
)

func newSetupCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "setup",
		Short: "Configure push credentials and storage",
		Long:  "Interactive wizard to configure Pushover API credentials and storage backend.",
		RunE:  runSetup,
	}
}

func runSetup(cmd *cobra.Command, args []string) error {
	configPath, err := resolveConfigPath()
	if err != nil {
		return fmt.Errorf("failed to resolve config path: %w", err)
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	model := tui.NewSetupModel(cfg.AppToken, cfg.UserKey, cfg.Backend, cfg.DataDir)

	p := tea.NewProgram(model)
	result, err := p.Run()
	if err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}

	final, ok := result.(tui.SetupModel)
	if !ok {
		return fmt.Errorf("unexpected model type from TUI")
	}
	if !final.ShouldSave() {
		fmt.Println("Setup cancelled.")
		return nil
	}

	appToken, userKey, backend, dataDir := final.Result()
	cfg.AppToken = appToken
	cfg.UserKey = userKey
	cfg.Backend = backend
	cfg.DataDir = dataDir

	if err := config.Save(configPath, cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("Config saved to %s\n", configPath)
	return nil
}
