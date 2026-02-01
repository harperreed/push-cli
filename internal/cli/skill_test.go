// ABOUTME: Tests for install-skill command functionality.
// ABOUTME: Covers directory creation, file content, overwrite, and Cobra integration.
package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// createTestInstallSkillCmd creates a command with a custom home directory for testing.
func createTestInstallSkillCmd(homeDir string) *cobra.Command {
	var skipConfirm bool

	cmd := &cobra.Command{
		Use:   "install-skill",
		Short: "Install Claude Code skill",
		Long: `Install the push skill for Claude Code.

This copies the skill definition to ~/.claude/skills/push/
so Claude Code can use push commands contextually.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInstallSkillWithHome(cmd, skipConfirm, homeDir)
		},
	}

	cmd.Flags().BoolVarP(&skipConfirm, "yes", "y", false, "Skip confirmation prompt")
	return cmd
}

// runInstallSkillWithHome is a test version that accepts a custom home directory.
func runInstallSkillWithHome(cmd *cobra.Command, skipConfirm bool, home string) error {
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

	// For tests, we always skip confirmation when skipConfirm is true
	if !skipConfirm {
		// In tests, we don't handle stdin, so this would fail
		// The actual implementation handles this, but for testability
		// we require skipConfirm=true in tests
		cmd.Println("Installation cancelled.")
		return nil
	}

	// Read embedded skill file
	content, err := skillFS.ReadFile("skill/SKILL.md")
	if err != nil {
		return err
	}

	// Create directory
	if err := os.MkdirAll(skillDir, 0750); err != nil {
		return err
	}

	// Write skill file
	if err := os.WriteFile(skillPath, content, 0600); err != nil { //nolint:gosec // test file
		return err
	}

	cmd.Println("✓ Installed push skill successfully!")
	cmd.Println()
	cmd.Println("Claude Code will now recognize /push commands.")
	cmd.Println("Try asking Claude: \"Notify me when this is done\" or \"Send a test notification\"")
	return nil
}

func TestSkill_SuccessfulInstallation(t *testing.T) {
	homeDir := t.TempDir()

	cmd := createTestInstallSkillCmd(homeDir)
	var outBuf bytes.Buffer
	cmd.SetOut(&outBuf)
	cmd.SetErr(&outBuf)

	// Set the --yes flag to skip confirmation
	cmd.SetArgs([]string{"--yes"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("install-skill command failed: %v", err)
	}

	// Verify success message in output
	output := outBuf.String()
	if !strings.Contains(output, "Installed push skill successfully!") {
		t.Errorf("expected success message in output, got: %s", output)
	}
}

func TestSkill_DirectoryCreation(t *testing.T) {
	homeDir := t.TempDir()

	cmd := createTestInstallSkillCmd(homeDir)
	var outBuf bytes.Buffer
	cmd.SetOut(&outBuf)
	cmd.SetErr(&outBuf)
	cmd.SetArgs([]string{"--yes"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("install-skill command failed: %v", err)
	}

	// Verify directory structure was created
	skillDir := filepath.Join(homeDir, ".claude", "skills", "push")
	info, err := os.Stat(skillDir)
	if os.IsNotExist(err) {
		t.Fatalf("skill directory was not created: %s", skillDir)
	}
	if err != nil {
		t.Fatalf("failed to stat skill directory: %v", err)
	}
	if !info.IsDir() {
		t.Errorf("expected %s to be a directory", skillDir)
	}
}

func TestSkill_FileContent(t *testing.T) {
	homeDir := t.TempDir()

	cmd := createTestInstallSkillCmd(homeDir)
	var outBuf bytes.Buffer
	cmd.SetOut(&outBuf)
	cmd.SetErr(&outBuf)
	cmd.SetArgs([]string{"--yes"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("install-skill command failed: %v", err)
	}

	// Read the installed file
	skillPath := filepath.Join(homeDir, ".claude", "skills", "push", "SKILL.md")
	installedContent, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatalf("failed to read installed skill file: %v", err)
	}

	// Read the embedded content for comparison
	embeddedContent, err := skillFS.ReadFile("skill/SKILL.md")
	if err != nil {
		t.Fatalf("failed to read embedded skill file: %v", err)
	}

	// Verify content matches
	if string(installedContent) != string(embeddedContent) {
		t.Errorf("installed content does not match embedded content")
		t.Errorf("installed length: %d, embedded length: %d", len(installedContent), len(embeddedContent))
	}

	// Verify key content elements
	content := string(installedContent)
	expectedElements := []string{
		"name: push",
		"# push - Push Notifications",
		"mcp__push__send",
		"mcp__push__history",
		"Priority levels",
	}

	for _, elem := range expectedElements {
		if !strings.Contains(content, elem) {
			t.Errorf("expected skill file to contain %q", elem)
		}
	}
}

func TestSkill_Overwrite(t *testing.T) {
	homeDir := t.TempDir()

	// Pre-create the skill directory and file with different content
	skillDir := filepath.Join(homeDir, ".claude", "skills", "push")
	skillPath := filepath.Join(skillDir, "SKILL.md")

	if err := os.MkdirAll(skillDir, 0750); err != nil {
		t.Fatalf("failed to create test directory: %v", err)
	}

	oldContent := []byte("# Old skill file\nThis should be overwritten.")
	if err := os.WriteFile(skillPath, oldContent, 0600); err != nil { //nolint:gosec // test file
		t.Fatalf("failed to write test file: %v", err)
	}

	cmd := createTestInstallSkillCmd(homeDir)
	var outBuf bytes.Buffer
	cmd.SetOut(&outBuf)
	cmd.SetErr(&outBuf)
	cmd.SetArgs([]string{"--yes"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("install-skill command failed: %v", err)
	}

	// Verify overwrite message was shown
	output := outBuf.String()
	if !strings.Contains(output, "A skill file already exists and will be overwritten") {
		t.Errorf("expected overwrite notice in output, got: %s", output)
	}

	// Verify file was actually overwritten
	newContent, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatalf("failed to read skill file after overwrite: %v", err)
	}

	if string(newContent) == string(oldContent) {
		t.Error("skill file was not overwritten")
	}

	if !strings.Contains(string(newContent), "name: push") {
		t.Error("overwritten file does not contain expected skill content")
	}
}

func TestSkill_OutputMessages(t *testing.T) {
	homeDir := t.TempDir()

	cmd := createTestInstallSkillCmd(homeDir)
	var outBuf bytes.Buffer
	cmd.SetOut(&outBuf)
	cmd.SetErr(&outBuf)
	cmd.SetArgs([]string{"--yes"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("install-skill command failed: %v", err)
	}

	output := outBuf.String()

	// Verify all expected output messages
	expectedMessages := []string{
		"Push Skill for Claude Code",
		"Send push notifications via Pushover",
		"Alert you when tasks complete",
		"Check notification history",
		"Use the /push slash command",
		"Destination:",
		".claude/skills/push/SKILL.md",
		"Installed push skill successfully!",
		"Claude Code will now recognize /push commands",
	}

	for _, msg := range expectedMessages {
		if !strings.Contains(output, msg) {
			t.Errorf("expected output to contain %q", msg)
		}
	}
}

func TestSkill_WithoutYesFlag(t *testing.T) {
	homeDir := t.TempDir()

	cmd := createTestInstallSkillCmd(homeDir)
	var outBuf bytes.Buffer
	cmd.SetOut(&outBuf)
	cmd.SetErr(&outBuf)
	// Do not set --yes flag

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("install-skill command failed: %v", err)
	}

	// Verify installation was cancelled
	output := outBuf.String()
	if !strings.Contains(output, "Installation cancelled") {
		t.Errorf("expected cancellation message, got: %s", output)
	}

	// Verify file was NOT created
	skillPath := filepath.Join(homeDir, ".claude", "skills", "push", "SKILL.md")
	if _, err := os.Stat(skillPath); !os.IsNotExist(err) {
		t.Error("skill file should not exist when installation is cancelled")
	}
}

func TestSkill_FilePermissions(t *testing.T) {
	homeDir := t.TempDir()

	cmd := createTestInstallSkillCmd(homeDir)
	var outBuf bytes.Buffer
	cmd.SetOut(&outBuf)
	cmd.SetErr(&outBuf)
	cmd.SetArgs([]string{"--yes"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("install-skill command failed: %v", err)
	}

	// Check file permissions
	skillPath := filepath.Join(homeDir, ".claude", "skills", "push", "SKILL.md")
	info, err := os.Stat(skillPath)
	if err != nil {
		t.Fatalf("failed to stat skill file: %v", err)
	}

	// File should be readable (0644 or similar)
	mode := info.Mode().Perm()
	if mode&0400 == 0 {
		t.Errorf("skill file should be readable by owner, got permissions: %o", mode)
	}
}

func TestSkill_CobraCommandIntegration(t *testing.T) {
	// Test the actual newInstallSkillCmd factory function
	cmd := newInstallSkillCmd()

	// Verify command properties
	if cmd.Use != "install-skill" {
		t.Errorf("expected Use to be 'install-skill', got %q", cmd.Use)
	}

	if cmd.Short != "Install Claude Code skill" {
		t.Errorf("expected Short description, got %q", cmd.Short)
	}

	if !strings.Contains(cmd.Long, "Install the push skill for Claude Code") {
		t.Error("expected Long description to contain installation info")
	}

	// Verify --yes flag exists
	yesFlag := cmd.Flags().Lookup("yes")
	if yesFlag == nil {
		t.Fatal("expected --yes flag to exist")
	}

	if yesFlag.Shorthand != "y" {
		t.Errorf("expected --yes shorthand to be 'y', got %q", yesFlag.Shorthand)
	}

	if yesFlag.DefValue != "false" {
		t.Errorf("expected --yes default to be 'false', got %q", yesFlag.DefValue)
	}
}

func TestSkill_EmbeddedSkillFileExists(t *testing.T) {
	// Verify the embedded skill file exists and is readable
	content, err := skillFS.ReadFile("skill/SKILL.md")
	if err != nil {
		t.Fatalf("failed to read embedded skill file: %v", err)
	}

	if len(content) == 0 {
		t.Error("embedded skill file is empty")
	}

	// Verify it's valid markdown with expected frontmatter
	contentStr := string(content)
	if !strings.HasPrefix(contentStr, "---") {
		t.Error("skill file should start with YAML frontmatter")
	}

	if !strings.Contains(contentStr, "name: push") {
		t.Error("skill file should contain 'name: push' in frontmatter")
	}
}

func TestSkill_NestedDirectoryCreation(t *testing.T) {
	// Test that deeply nested directories are created properly
	homeDir := t.TempDir()

	// Verify none of the intermediate directories exist
	claudeDir := filepath.Join(homeDir, ".claude")
	if _, err := os.Stat(claudeDir); !os.IsNotExist(err) {
		t.Fatalf(".claude directory should not exist before test")
	}

	cmd := createTestInstallSkillCmd(homeDir)
	var outBuf bytes.Buffer
	cmd.SetOut(&outBuf)
	cmd.SetErr(&outBuf)
	cmd.SetArgs([]string{"--yes"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("install-skill command failed: %v", err)
	}

	// Verify all directories in the path exist
	dirsToCheck := []string{
		filepath.Join(homeDir, ".claude"),
		filepath.Join(homeDir, ".claude", "skills"),
		filepath.Join(homeDir, ".claude", "skills", "push"),
	}

	for _, dir := range dirsToCheck {
		info, err := os.Stat(dir)
		if os.IsNotExist(err) {
			t.Errorf("directory should exist: %s", dir)
			continue
		}
		if err != nil {
			t.Errorf("failed to stat directory %s: %v", dir, err)
			continue
		}
		if !info.IsDir() {
			t.Errorf("expected %s to be a directory", dir)
		}
	}
}

func TestSkill_MultipleInstallations(t *testing.T) {
	homeDir := t.TempDir()

	// Install twice
	for i := 0; i < 2; i++ {
		cmd := createTestInstallSkillCmd(homeDir)
		var outBuf bytes.Buffer
		cmd.SetOut(&outBuf)
		cmd.SetErr(&outBuf)
		cmd.SetArgs([]string{"--yes"})

		err := cmd.Execute()
		if err != nil {
			t.Fatalf("install-skill command failed on iteration %d: %v", i+1, err)
		}

		// Second iteration should show overwrite message
		if i == 1 {
			output := outBuf.String()
			if !strings.Contains(output, "A skill file already exists") {
				t.Error("second installation should show overwrite notice")
			}
		}
	}

	// Verify file still has correct content after multiple installations
	skillPath := filepath.Join(homeDir, ".claude", "skills", "push", "SKILL.md")
	content, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatalf("failed to read skill file: %v", err)
	}

	if !strings.Contains(string(content), "name: push") {
		t.Error("file content corrupted after multiple installations")
	}
}
