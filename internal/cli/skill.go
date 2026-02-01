// ABOUTME: Install Claude Code skill for push notifications.
// ABOUTME: Embeds and installs the skill definition to ~/.claude/skills/

package cli

import (
	"bufio"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

//go:embed skill/SKILL.md
var skillFS embed.FS

func newInstallSkillCmd() *cobra.Command {
	var skipConfirm bool

	cmd := &cobra.Command{
		Use:   "install-skill",
		Short: "Install Claude Code skill",
		Long: `Install the push skill for Claude Code.

This copies the skill definition to ~/.claude/skills/push/
so Claude Code can use push commands contextually.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInstallSkill(cmd, skipConfirm)
		},
	}

	cmd.Flags().BoolVarP(&skipConfirm, "yes", "y", false, "Skip confirmation prompt")
	return cmd
}

func runInstallSkill(cmd *cobra.Command, skipConfirm bool) error {
	// Determine destination
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	skillDir := filepath.Join(home, ".claude", "skills", "push")
	skillPath := filepath.Join(skillDir, "SKILL.md")

	// Show explanation
	cmd.Println("┌─────────────────────────────────────────────────────────────┐")
	cmd.Println("│              Push Skill for Claude Code                     │")
	cmd.Println("└─────────────────────────────────────────────────────────────┘")
	cmd.Println()
	cmd.Println("This will install the push skill, enabling Claude Code to:")
	cmd.Println()
	cmd.Println("  • Send push notifications via Pushover")
	cmd.Println("  • Alert you when tasks complete")
	cmd.Println("  • Check notification history")
	cmd.Println("  • Use the /push slash command")
	cmd.Println()
	cmd.Println("Destination:")
	cmd.Printf("  %s\n", skillPath)
	cmd.Println()

	// Check if already installed
	if _, err := os.Stat(skillPath); err == nil {
		cmd.Println("Note: A skill file already exists and will be overwritten.")
		cmd.Println()
	}

	// Ask for confirmation unless --yes flag is set
	if !skipConfirm {
		cmd.Print("Install the push skill? [y/N] ")
		reader := bufio.NewReader(os.Stdin)
		response, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read response: %w", err)
		}
		response = strings.TrimSpace(strings.ToLower(response))
		if response != "y" && response != "yes" {
			cmd.Println("Installation cancelled.")
			return nil
		}
		cmd.Println()
	}

	// Read embedded skill file
	content, err := skillFS.ReadFile("skill/SKILL.md")
	if err != nil {
		return fmt.Errorf("failed to read embedded skill: %w", err)
	}

	// Create directory
	if err := os.MkdirAll(skillDir, 0750); err != nil { // #nosec G301 - skill dir needs to be readable
		return fmt.Errorf("failed to create skill directory: %w", err)
	}

	// Write skill file
	if err := os.WriteFile(skillPath, content, 0600); err != nil { // #nosec G306 - skill file needs to be readable
		return fmt.Errorf("failed to write skill file: %w", err)
	}

	cmd.Println("✓ Installed push skill successfully!")
	cmd.Println()
	cmd.Println("Claude Code will now recognize /push commands.")
	cmd.Println("Try asking Claude: \"Notify me when this is done\" or \"Send a test notification\"")
	return nil
}
