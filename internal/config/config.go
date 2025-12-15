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
}

// Load reads the config from disk. If the file does not exist it returns a default config.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return &Config{}, nil
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

// Save writes the config atomically to disk.
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
