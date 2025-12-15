// ABOUTME: Config command for displaying current configuration.
// ABOUTME: Shows config file path and contents in TOML format.
package cli

import (
	"fmt"

	"github.com/pelletier/go-toml/v2"
	"github.com/spf13/cobra"
)

func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Show current configuration",
		RunE:  runConfig,
	}

	cmd.Flags().Bool("path", false, "print the config file path only")

	return cmd
}

func runConfig(cmd *cobra.Command, args []string) error {
	showPathOnly, _ := cmd.Flags().GetBool("path")
	cfg, cfgPath, err := loadConfig()
	if err != nil {
		return err
	}

	if showPathOnly {
		cmd.Println(cfgPath)
		return nil
	}

	data, err := toml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}

	cmd.Printf("# %s\n%s", cfgPath, string(data))
	if len(data) == 0 || data[len(data)-1] != '\n' {
		cmd.Println()
	}
	return nil
}
