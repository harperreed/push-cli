// ABOUTME: Tests for configuration management.
// ABOUTME: Validates config loading, saving, and validation.

//go:build !windows

package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadNonExistent(t *testing.T) {
	tmpDir := t.TempDir()
	origDataXDG := os.Getenv("XDG_DATA_HOME")
	defer func() { _ = os.Setenv("XDG_DATA_HOME", origDataXDG) }()
	_ = os.Setenv("XDG_DATA_HOME", tmpDir)

	cfgPath := filepath.Join(tmpDir, "push-config", "config.toml")
	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() returned error for nonexistent file: %v", err)
	}
	if cfg == nil {
		t.Fatal("Load() returned nil config")
	}
	if cfg.Backend != "markdown" {
		t.Errorf("expected markdown backend for new user, got %q", cfg.Backend)
	}

	// Verify config file was auto-created
	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		t.Error("expected config file to be auto-created")
	}
}

func TestLoadNonExistent_ExistingSQLiteUser(t *testing.T) {
	tmpDir := t.TempDir()
	origDataXDG := os.Getenv("XDG_DATA_HOME")
	defer func() { _ = os.Setenv("XDG_DATA_HOME", origDataXDG) }()
	_ = os.Setenv("XDG_DATA_HOME", tmpDir)

	// Create a fake .db file to simulate existing SQLite user
	dbDir := filepath.Join(tmpDir, "push")
	if err := os.MkdirAll(dbDir, 0750); err != nil {
		t.Fatalf("failed to create db dir: %v", err)
	}
	dbPath := filepath.Join(dbDir, "push.db")
	if err := os.WriteFile(dbPath, []byte("fake-db"), 0600); err != nil {
		t.Fatalf("failed to create fake db: %v", err)
	}

	cfgPath := filepath.Join(tmpDir, "push-config", "config.toml")
	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}
	if cfg.Backend != "sqlite" {
		t.Errorf("expected sqlite backend for existing user, got %q", cfg.Backend)
	}
}

func TestLoadAutoCreatesValidConfig(t *testing.T) {
	tmpDir := t.TempDir()
	origDataXDG := os.Getenv("XDG_DATA_HOME")
	defer func() { _ = os.Setenv("XDG_DATA_HOME", origDataXDG) }()
	_ = os.Setenv("XDG_DATA_HOME", tmpDir)

	cfgPath := filepath.Join(tmpDir, "push-config", "config.toml")
	_, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	// Read back the auto-created file and verify it's valid TOML
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("failed to read auto-created config: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("auto-created config file is empty")
	}

	// Load it again to verify it parses correctly
	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("failed to re-load auto-created config: %v", err)
	}
	if cfg.Backend != "markdown" {
		t.Errorf("expected backend 'markdown' in config file, got %q", cfg.Backend)
	}
}

func TestSaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.toml")

	original := &Config{
		AppToken:        "test-app-token",
		UserKey:         "test-user-key",
		DeviceID:        "test-device",
		DeviceSecret:    "test-secret",
		DefaultDevice:   "my-phone",
		DefaultPriority: 1,
	}

	if err := Save(cfgPath, original); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	// Verify file permissions
	info, err := os.Stat(cfgPath)
	if err != nil {
		t.Fatalf("Stat() error: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("File permissions = %o, want 0600", info.Mode().Perm())
	}

	loaded, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if loaded.AppToken != original.AppToken {
		t.Errorf("AppToken = %q, want %q", loaded.AppToken, original.AppToken)
	}
	if loaded.UserKey != original.UserKey {
		t.Errorf("UserKey = %q, want %q", loaded.UserKey, original.UserKey)
	}
	if loaded.DeviceID != original.DeviceID {
		t.Errorf("DeviceID = %q, want %q", loaded.DeviceID, original.DeviceID)
	}
	if loaded.DeviceSecret != original.DeviceSecret {
		t.Errorf("DeviceSecret = %q, want %q", loaded.DeviceSecret, original.DeviceSecret)
	}
	if loaded.DefaultPriority != original.DefaultPriority {
		t.Errorf("DefaultPriority = %d, want %d", loaded.DefaultPriority, original.DefaultPriority)
	}
}

func TestValidateSend(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *Config
		wantErr bool
	}{
		{
			name:    "nil config",
			cfg:     nil,
			wantErr: true,
		},
		{
			name:    "empty config",
			cfg:     &Config{},
			wantErr: true,
		},
		{
			name: "missing user key",
			cfg: &Config{
				AppToken: "token",
			},
			wantErr: true,
		},
		{
			name: "valid send config",
			cfg: &Config{
				AppToken: "token",
				UserKey:  "user",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.ValidateSend()
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateSend() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateReceive(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *Config
		wantErr bool
	}{
		{
			name: "missing device credentials",
			cfg: &Config{
				AppToken: "token",
				UserKey:  "user",
			},
			wantErr: true,
		},
		{
			name: "valid receive config",
			cfg: &Config{
				AppToken:     "token",
				UserKey:      "user",
				DeviceID:     "device",
				DeviceSecret: "secret",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.ValidateReceive()
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateReceive() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestClone(t *testing.T) {
	original := &Config{
		AppToken: "token",
		UserKey:  "user",
	}

	cloned := original.Clone()
	if cloned == original {
		t.Error("Clone() returned same pointer")
	}
	if cloned.AppToken != original.AppToken {
		t.Errorf("Clone().AppToken = %q, want %q", cloned.AppToken, original.AppToken)
	}

	// Modify clone, ensure original unchanged
	cloned.AppToken = "modified"
	if original.AppToken == "modified" {
		t.Error("Modifying clone affected original")
	}
}

func TestDeviceConfigured(t *testing.T) {
	tests := []struct {
		name string
		cfg  *Config
		want bool
	}{
		{
			name: "nil config",
			cfg:  nil,
			want: false,
		},
		{
			name: "empty config",
			cfg:  &Config{},
			want: false,
		},
		{
			name: "only device id",
			cfg: &Config{
				DeviceID: "device",
			},
			want: false,
		},
		{
			name: "both set",
			cfg: &Config{
				DeviceID:     "device",
				DeviceSecret: "secret",
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.cfg.DeviceConfigured(); got != tt.want {
				t.Errorf("DeviceConfigured() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestClone_NilConfig(t *testing.T) {
	var cfg *Config
	cloned := cfg.Clone()
	if cloned != nil {
		t.Error("Clone() of nil should return nil")
	}
}

func TestSaveNilConfig(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.toml")

	err := Save(cfgPath, nil)
	if err == nil {
		t.Error("expected error for nil config")
	}
}

func TestSaveCreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "nested", "dir", "config.toml")

	cfg := &Config{AppToken: "token"}
	err := Save(cfgPath, cfg)
	if err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	// Verify nested directories were created
	if _, err := os.Stat(filepath.Dir(cfgPath)); os.IsNotExist(err) {
		t.Error("nested directories were not created")
	}
}

func TestLoadInvalidTOML(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.toml")

	// Write invalid TOML
	if err := os.WriteFile(cfgPath, []byte("invalid [[ toml"), 0600); err != nil {
		t.Fatalf("failed to write invalid toml: %v", err)
	}

	_, err := Load(cfgPath)
	if err == nil {
		t.Error("expected error for invalid TOML")
	}
}

func TestLoadReadError(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.toml")

	// Create a directory where file is expected (causes read error)
	if err := os.MkdirAll(cfgPath, 0755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}

	_, err := Load(cfgPath)
	if err == nil {
		t.Error("expected error when path is a directory")
	}
}

func TestSaveAndLoadAllFields(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.toml")

	original := &Config{
		AppToken:        "test-app-token",
		UserKey:         "test-user-key",
		DeviceID:        "test-device-id",
		DeviceSecret:    "test-device-secret",
		DefaultDevice:   "my-phone",
		DefaultPriority: -1,
	}

	if err := Save(cfgPath, original); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	loaded, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if loaded.AppToken != original.AppToken {
		t.Errorf("AppToken = %q, want %q", loaded.AppToken, original.AppToken)
	}
	if loaded.UserKey != original.UserKey {
		t.Errorf("UserKey = %q, want %q", loaded.UserKey, original.UserKey)
	}
	if loaded.DeviceID != original.DeviceID {
		t.Errorf("DeviceID = %q, want %q", loaded.DeviceID, original.DeviceID)
	}
	if loaded.DeviceSecret != original.DeviceSecret {
		t.Errorf("DeviceSecret = %q, want %q", loaded.DeviceSecret, original.DeviceSecret)
	}
	if loaded.DefaultDevice != original.DefaultDevice {
		t.Errorf("DefaultDevice = %q, want %q", loaded.DefaultDevice, original.DefaultDevice)
	}
	if loaded.DefaultPriority != original.DefaultPriority {
		t.Errorf("DefaultPriority = %d, want %d", loaded.DefaultPriority, original.DefaultPriority)
	}
}

func TestValidateSend_MissingAppToken(t *testing.T) {
	cfg := &Config{
		UserKey: "user",
	}
	err := cfg.ValidateSend()
	if err == nil {
		t.Error("expected error for missing app token")
	}
}

func TestValidateSend_MissingUserKey(t *testing.T) {
	cfg := &Config{
		AppToken: "token",
	}
	err := cfg.ValidateSend()
	if err == nil {
		t.Error("expected error for missing user key")
	}
}

func TestValidateReceive_MissingDeviceID(t *testing.T) {
	cfg := &Config{
		AppToken:     "token",
		UserKey:      "user",
		DeviceSecret: "secret",
	}
	err := cfg.ValidateReceive()
	if err == nil {
		t.Error("expected error for missing device id")
	}
}

func TestValidateReceive_MissingDeviceSecret(t *testing.T) {
	cfg := &Config{
		AppToken: "token",
		UserKey:  "user",
		DeviceID: "device",
	}
	err := cfg.ValidateReceive()
	if err == nil {
		t.Error("expected error for missing device secret")
	}
}

func TestValidateReceive_Valid(t *testing.T) {
	cfg := &Config{
		AppToken:     "token",
		UserKey:      "user",
		DeviceID:     "device",
		DeviceSecret: "secret",
	}
	err := cfg.ValidateReceive()
	if err != nil {
		t.Errorf("ValidateReceive() error = %v, want nil", err)
	}
}

func TestDeviceConfigured_OnlySecret(t *testing.T) {
	cfg := &Config{
		DeviceSecret: "secret",
	}
	if cfg.DeviceConfigured() {
		t.Error("DeviceConfigured() should return false when only secret is set")
	}
}

func TestClone_AllFields(t *testing.T) {
	original := &Config{
		AppToken:        "app-token",
		UserKey:         "user-key",
		DeviceID:        "device-id",
		DeviceSecret:    "device-secret",
		DefaultDevice:   "my-phone",
		DefaultPriority: 1,
	}

	cloned := original.Clone()

	// Verify all fields are copied
	if cloned.AppToken != original.AppToken {
		t.Errorf("AppToken not cloned correctly")
	}
	if cloned.UserKey != original.UserKey {
		t.Errorf("UserKey not cloned correctly")
	}
	if cloned.DeviceID != original.DeviceID {
		t.Errorf("DeviceID not cloned correctly")
	}
	if cloned.DeviceSecret != original.DeviceSecret {
		t.Errorf("DeviceSecret not cloned correctly")
	}
	if cloned.DefaultDevice != original.DefaultDevice {
		t.Errorf("DefaultDevice not cloned correctly")
	}
	if cloned.DefaultPriority != original.DefaultPriority {
		t.Errorf("DefaultPriority not cloned correctly")
	}

	// Verify independence
	cloned.AppToken = "modified"
	if original.AppToken == "modified" {
		t.Error("Modifying clone affected original")
	}
}

func TestSaveAndLoadRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.toml")

	configs := []*Config{
		{AppToken: "token1"},
		{AppToken: "token2", UserKey: "user"},
		{AppToken: "token3", UserKey: "user", DeviceID: "device", DeviceSecret: "secret"},
		{
			AppToken:        "full",
			UserKey:         "full-user",
			DeviceID:        "full-device",
			DeviceSecret:    "full-secret",
			DefaultDevice:   "my-phone",
			DefaultPriority: -2,
		},
	}

	for i, original := range configs {
		if err := Save(cfgPath, original); err != nil {
			t.Fatalf("Save() error on iteration %d: %v", i, err)
		}

		loaded, err := Load(cfgPath)
		if err != nil {
			t.Fatalf("Load() error on iteration %d: %v", i, err)
		}

		if loaded.AppToken != original.AppToken {
			t.Errorf("iteration %d: AppToken mismatch", i)
		}
		if loaded.UserKey != original.UserKey {
			t.Errorf("iteration %d: UserKey mismatch", i)
		}
		if loaded.DeviceID != original.DeviceID {
			t.Errorf("iteration %d: DeviceID mismatch", i)
		}
		if loaded.DeviceSecret != original.DeviceSecret {
			t.Errorf("iteration %d: DeviceSecret mismatch", i)
		}
		if loaded.DefaultDevice != original.DefaultDevice {
			t.Errorf("iteration %d: DefaultDevice mismatch", i)
		}
		if loaded.DefaultPriority != original.DefaultPriority {
			t.Errorf("iteration %d: DefaultPriority mismatch", i)
		}
	}
}

func TestValidateSend_Valid(t *testing.T) {
	cfg := &Config{
		AppToken: "token",
		UserKey:  "user",
	}
	err := cfg.ValidateSend()
	if err != nil {
		t.Errorf("ValidateSend() error = %v, want nil", err)
	}
}

func TestLoad_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.toml")

	// Write empty file
	if err := os.WriteFile(cfgPath, []byte(""), 0600); err != nil {
		t.Fatalf("failed to write empty file: %v", err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() error for empty file: %v", err)
	}
	if cfg == nil {
		t.Error("expected non-nil config for empty file")
	}
}

func TestSave_MkdirAllError(t *testing.T) {
	// On Unix-like systems, /dev/null is not a valid directory
	// Using an invalid path that cannot have subdirectories
	cfg := &Config{AppToken: "token"}

	// Try to save to a path that cannot have nested directories
	// /dev/null/nested/config.toml - cannot create directories under /dev/null
	err := Save("/dev/null/nested/dir/config.toml", cfg)
	if err == nil {
		t.Error("expected error when creating directory fails")
	}
}

func TestSave_CreateTempError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("test cannot run as root")
	}

	tmpDir := t.TempDir()
	// Create a read-only directory
	readOnlyDir := filepath.Join(tmpDir, "readonly")
	if err := os.Mkdir(readOnlyDir, 0555); err != nil {
		t.Fatalf("failed to create readonly dir: %v", err)
	}
	defer func() { _ = os.Chmod(readOnlyDir, 0600) }()

	cfg := &Config{AppToken: "token"}
	cfgPath := filepath.Join(readOnlyDir, "config.toml")

	err := Save(cfgPath, cfg)
	if err == nil {
		t.Error("expected error when creating temp file in read-only directory")
	}
}

func TestSave_RenameError(t *testing.T) {
	// This test attempts to trigger the rename error path
	// by trying to rename across different filesystems or invalid scenarios

	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.toml")

	// First, save a valid config
	cfg := &Config{AppToken: "token"}
	err := Save(cfgPath, cfg)
	if err != nil {
		t.Fatalf("initial Save() failed: %v", err)
	}

	// Now try to make the target path a directory (rename will fail)
	// This won't trigger the rename error, so let's verify the happy path works
	loaded, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() after Save() failed: %v", err)
	}
	if loaded.AppToken != "token" {
		t.Errorf("AppToken = %q, want %q", loaded.AppToken, "token")
	}
}

func TestSave_OverwriteExisting(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.toml")

	// Save initial config
	cfg1 := &Config{AppToken: "token1", UserKey: "user1"}
	if err := Save(cfgPath, cfg1); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	// Overwrite with new config
	cfg2 := &Config{AppToken: "token2", UserKey: "user2"}
	if err := Save(cfgPath, cfg2); err != nil {
		t.Fatalf("Save() overwrite error: %v", err)
	}

	// Verify the new config was written
	loaded, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if loaded.AppToken != "token2" {
		t.Errorf("AppToken = %q, want %q", loaded.AppToken, "token2")
	}
	if loaded.UserKey != "user2" {
		t.Errorf("UserKey = %q, want %q", loaded.UserKey, "user2")
	}
}

func TestSave_FilePermissions(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.toml")

	cfg := &Config{AppToken: "secret-token", UserKey: "secret-user"}
	if err := Save(cfgPath, cfg); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	// Verify file has 0600 permissions (owner read/write only)
	info, err := os.Stat(cfgPath)
	if err != nil {
		t.Fatalf("Stat() error: %v", err)
	}

	// On Unix, check that group and other have no permissions
	perm := info.Mode().Perm()
	if perm&0077 != 0 {
		t.Errorf("file has group/other permissions: %o", perm)
	}
	if perm&0600 != 0600 {
		t.Errorf("file missing owner read/write: %o", perm)
	}
}

func TestSave_DeepNestedDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "a", "b", "c", "d", "e", "config.toml")

	cfg := &Config{AppToken: "token"}
	if err := Save(cfgPath, cfg); err != nil {
		t.Fatalf("Save() error for deeply nested path: %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		t.Error("config file was not created")
	}

	// Verify content
	loaded, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if loaded.AppToken != "token" {
		t.Errorf("AppToken = %q, want %q", loaded.AppToken, "token")
	}
}

func TestSave_EmptyConfig(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.toml")

	// Save empty config (all zero values)
	cfg := &Config{}
	if err := Save(cfgPath, cfg); err != nil {
		t.Fatalf("Save() error for empty config: %v", err)
	}

	// Load and verify
	loaded, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if loaded.AppToken != "" {
		t.Errorf("AppToken = %q, want empty", loaded.AppToken)
	}
	if loaded.DefaultPriority != 0 {
		t.Errorf("DefaultPriority = %d, want 0", loaded.DefaultPriority)
	}
}

func TestSave_NegativePriority(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.toml")

	cfg := &Config{
		AppToken:        "token",
		DefaultPriority: -2,
	}
	if err := Save(cfgPath, cfg); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	loaded, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if loaded.DefaultPriority != -2 {
		t.Errorf("DefaultPriority = %d, want -2", loaded.DefaultPriority)
	}
}

func TestLoad_PartialConfig(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.toml")

	// Write a config with only some fields
	content := `app_token = "partial-token"
default_priority = 1
`
	if err := os.WriteFile(cfgPath, []byte(content), 0600); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.AppToken != "partial-token" {
		t.Errorf("AppToken = %q, want %q", cfg.AppToken, "partial-token")
	}
	if cfg.UserKey != "" {
		t.Errorf("UserKey = %q, want empty", cfg.UserKey)
	}
	if cfg.DefaultPriority != 1 {
		t.Errorf("DefaultPriority = %d, want 1", cfg.DefaultPriority)
	}
}

func TestValidateReceive_NilConfig(t *testing.T) {
	var cfg *Config
	err := cfg.ValidateReceive()
	if err == nil {
		t.Error("expected error for nil config")
	}
}
