// ABOUTME: Configuration management for the push application.
// ABOUTME: Handles TOML config file loading, saving, and validation.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"
)

// Config describes the persisted Push settings.
type Config struct {
	AppToken        string `toml:"app_token"`
	UserKey         string `toml:"user_key"`
	DeviceID        string `toml:"device_id"`
	DeviceSecret    string `toml:"device_secret"`
	DefaultDevice   string `toml:"default_device"`
	DefaultPriority int    `toml:"default_priority"`

	// Backend selects the storage backend: "sqlite" (default) or "markdown".
	Backend string `toml:"backend,omitempty"`

	// DataDir overrides the root directory for markdown storage.
	// Supports ~ expansion for home directory.
	DataDir string `toml:"data_dir,omitempty"`
}

// defaultDBFilename is the SQLite database filename used for existing-user detection.
const defaultDBFilename = "push.db"

// GetBackend returns the configured backend, defaulting to "sqlite".
func (c *Config) GetBackend() string {
	if c == nil || c.Backend == "" {
		return "sqlite"
	}
	return c.Backend
}

// GetDataDir returns the configured data directory with ~ expanded.
// Returns empty string if not set (caller should use default).
func (c *Config) GetDataDir() string {
	if c == nil || c.DataDir == "" {
		return ""
	}
	return expandPath(c.DataDir)
}

// expandPath expands a leading ~ to the user's home directory.
func expandPath(path string) string {
	if path == "" {
		return ""
	}
	if path == "~" {
		home, _ := os.UserHomeDir()
		return home
	}
	if len(path) > 1 && path[0] == '~' && path[1] == '/' {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[2:])
	}
	return path
}

// defaultDataDir returns the default data directory for push following XDG spec.
func defaultDataDir() string {
	dataDir := os.Getenv("XDG_DATA_HOME")
	if dataDir == "" {
		home, _ := os.UserHomeDir()
		dataDir = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(dataDir, "push")
}

// defaultFirstRunConfig returns the appropriate default config for first-time runs.
// If an existing SQLite database is found, it preserves SQLite as the backend.
// Otherwise, it defaults to markdown for new users.
func defaultFirstRunConfig() *Config {
	dbPath := filepath.Join(defaultDataDir(), defaultDBFilename)
	_, err := os.Stat(dbPath)
	switch {
	case err == nil:
		return &Config{Backend: "sqlite"}
	case !os.IsNotExist(err):
		fmt.Fprintf(os.Stderr, "warning: could not check for existing database: %v\n", err)
	}
	return &Config{Backend: "markdown"}
}

// Load reads the config from disk. If the file does not exist it returns a default config.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		cfg := defaultFirstRunConfig()
		if saveErr := Save(path, cfg); saveErr != nil {
			fmt.Fprintf(os.Stderr, "warning: could not save default config: %v\n", saveErr)
		}
		return cfg, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}

	var cfg Config
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	return &cfg, nil
}

// Save writes the config atomically to disk with 0600 permissions.
// Uses a custom atomic write instead of mdstore.AtomicWrite because
// push configs contain secrets (API tokens, device credentials).
func Save(path string, cfg *Config) error {
	if cfg == nil {
		return errors.New("config is nil")
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	data, err := toml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("encoding config: %w", err)
	}

	tmpFile, err := os.CreateTemp(dir, "config-*.tmp")
	if err != nil {
		return fmt.Errorf("creating temp config file: %w", err)
	}
	tmpName := tmpFile.Name()
	if _, err := tmpFile.Write(data); err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("writing temp config file: %w", err)
	}
	if err := tmpFile.Sync(); err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("syncing temp config file: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("closing temp config file: %w", err)
	}

	if err := os.Chmod(tmpName, 0o600); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("setting config permissions: %w", err)
	}

	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("replacing config: %w", err)
	}

	return nil
}

// ValidateSend ensures the config contains the minimum fields required to send.
func (c *Config) ValidateSend() error {
	if c == nil {
		return errors.New("config is nil")
	}
	if c.AppToken == "" {
		return errors.New("app token is missing")
	}
	if c.UserKey == "" {
		return errors.New("user key is missing")
	}
	return nil
}

// ValidateReceive ensures login credentials are available for fetching messages.
func (c *Config) ValidateReceive() error {
	if err := c.ValidateSend(); err != nil {
		return err
	}
	if c.DeviceID == "" || c.DeviceSecret == "" {
		return errors.New("device credentials missing, run 'push login'")
	}
	return nil
}

// Clone returns a shallow copy of the config to avoid accidental mutation.
func (c *Config) Clone() *Config {
	if c == nil {
		return nil
	}
	copied := *c
	return &copied
}

// DeviceConfigured indicates whether receiving credentials exist.
func (c *Config) DeviceConfigured() bool {
	if c == nil {
		return false
	}
	return c.DeviceID != "" && c.DeviceSecret != ""
}
