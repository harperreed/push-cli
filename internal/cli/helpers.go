// ABOUTME: Helper functions shared across CLI commands.
// ABOUTME: Provides config loading, storage access, and client creation.
package cli

import (
	"fmt"
	"path/filepath"

	"github.com/harper/push/internal/config"
	"github.com/harper/push/internal/pushover"
	"github.com/harper/push/internal/storage"
)

func loadConfig() (*config.Config, string, error) {
	cfgPath, err := resolveConfigPath()
	if err != nil {
		return nil, "", err
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return nil, "", err
	}
	return cfg, cfgPath, nil
}

func resolveStorageDataDir(cfg *config.Config) (string, error) {
	// Check config-level data_dir override first
	if cfg != nil && cfg.GetDataDir() != "" {
		return cfg.GetDataDir(), nil
	}
	// Fall back to CLI flag / XDG default
	return resolveDataDir()
}

func openStorage(cfg *config.Config) (storage.Storage, string, error) {
	dataDir, err := resolveStorageDataDir(cfg)
	if err != nil {
		return nil, "", err
	}

	backend := "sqlite"
	if cfg != nil {
		backend = cfg.GetBackend()
	}

	switch backend {
	case "sqlite":
		dbPath := filepath.Join(dataDir, "push.db")
		store, err := storage.NewSqliteStore(dbPath)
		if err != nil {
			return nil, "", fmt.Errorf("open sqlite database: %w", err)
		}
		return store, dbPath, nil

	case "markdown":
		store, err := storage.NewMarkdownStore(dataDir)
		if err != nil {
			return nil, "", fmt.Errorf("open markdown store: %w", err)
		}
		return store, dataDir, nil

	default:
		return nil, "", fmt.Errorf("unknown storage backend: %q", backend)
	}
}

func newClientFromConfig(cfg *config.Config) *pushover.Client {
	if cfg == nil {
		return pushover.NewClient("", "", "", "")
	}
	return pushover.NewClient(cfg.AppToken, cfg.UserKey, cfg.DeviceID, cfg.DeviceSecret)
}
