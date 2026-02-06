// ABOUTME: Migrate command for copying data between storage backends.
// ABOUTME: Supports migration from sqlite to markdown and vice versa.
package cli

import (
	"fmt"
	"path/filepath"

	"github.com/harper/push/internal/storage"
	"github.com/spf13/cobra"
)

func newMigrateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Migrate data between storage backends",
		Long: `Migrate data from the current storage backend to a different one.

Examples:
  push migrate --to markdown --data-dir ~/push-data
  push migrate --to sqlite
  push migrate --to markdown --force`,
		RunE: runMigrate,
	}

	cmd.Flags().String("to", "", "target backend: sqlite or markdown (required)")
	cmd.Flags().String("data-dir", "", "data directory for the target backend")
	cmd.Flags().Bool("force", false, "overwrite existing data in target")

	_ = cmd.MarkFlagRequired("to")

	return cmd
}

func runMigrate(cmd *cobra.Command, args []string) error {
	targetBackend, _ := cmd.Flags().GetString("to")
	targetDataDir, _ := cmd.Flags().GetString("data-dir")
	force, _ := cmd.Flags().GetBool("force")

	if targetBackend != "sqlite" && targetBackend != "markdown" {
		return fmt.Errorf("invalid target backend %q; must be 'sqlite' or 'markdown'", targetBackend)
	}

	cfg, _, err := loadConfig()
	if err != nil {
		return err
	}

	currentBackend := cfg.GetBackend()
	if currentBackend == targetBackend && targetDataDir == "" {
		return fmt.Errorf("already using %q backend; specify --data-dir to migrate to a different location", targetBackend)
	}

	// Open source storage
	src, _, err := openStorage(cfg)
	if err != nil {
		return fmt.Errorf("open source storage: %w", err)
	}
	defer func() { _ = src.Close() }()

	// Resolve target data directory
	if targetDataDir == "" {
		resolved, err := resolveStorageDataDir(cfg)
		if err != nil {
			return fmt.Errorf("resolve target data directory: %w", err)
		}
		targetDataDir = resolved
	}

	// Open target storage
	dst, err := openTargetStorage(targetBackend, targetDataDir, force)
	if err != nil {
		return err
	}
	defer func() { _ = dst.Close() }()

	cmd.Printf("Migrating from %s to %s...\n", currentBackend, targetBackend)

	summary, err := storage.MigrateData(cmd.Context(), src, dst)
	if err != nil {
		return fmt.Errorf("migration failed: %w", err)
	}

	cmd.Printf("Migration complete: %d messages, %d sent records\n", summary.Messages, summary.Sent)
	return nil
}

// openTargetStorage validates the target directory and opens the appropriate
// storage backend. When force is false it refuses to write into a non-empty
// directory.
func openTargetStorage(backend, dataDir string, force bool) (storage.Storage, error) {
	if !force {
		nonEmpty, err := storage.IsDirNonEmpty(dataDir)
		if err != nil {
			return nil, fmt.Errorf("check target directory: %w", err)
		}
		if nonEmpty {
			return nil, fmt.Errorf("target directory %q is not empty; use --force to overwrite", dataDir)
		}
	}

	switch backend {
	case "sqlite":
		dbPath := filepath.Join(dataDir, "push.db")
		dst, err := storage.NewSqliteStore(dbPath)
		if err != nil {
			return nil, fmt.Errorf("open target sqlite store: %w", err)
		}
		return dst, nil

	case "markdown":
		dst, err := storage.NewMarkdownStore(dataDir)
		if err != nil {
			return nil, fmt.Errorf("open target markdown store: %w", err)
		}
		return dst, nil

	default:
		return nil, fmt.Errorf("unsupported target backend: %q", backend)
	}
}
