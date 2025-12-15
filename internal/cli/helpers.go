// ABOUTME: Helper functions shared across CLI commands.
// ABOUTME: Provides config loading, database access, and client creation.
package cli

import (
	"fmt"
	"path/filepath"

	"github.com/harper/push/internal/config"
	"github.com/harper/push/internal/db"
	"github.com/harper/push/internal/pushover"
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

func databasePath() (string, error) {
	dataDir, err := resolveDataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dataDir, "push.db"), nil
}

func openStore() (*db.Store, string, error) {
	path, err := databasePath()
	if err != nil {
		return nil, "", err
	}
	store, err := db.Open(path)
	if err != nil {
		return nil, "", fmt.Errorf("open database: %w", err)
	}
	return store, path, nil
}

func newClientFromConfig(cfg *config.Config) *pushover.Client {
	if cfg == nil {
		return pushover.NewClient("", "", "", "")
	}
	return pushover.NewClient(cfg.AppToken, cfg.UserKey, cfg.DeviceID, cfg.DeviceSecret)
}
