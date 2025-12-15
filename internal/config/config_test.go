// ABOUTME: Tests for configuration management.
// ABOUTME: Validates config loading, saving, and validation.
package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadNonExistent(t *testing.T) {
	cfg, err := Load("/nonexistent/path/config.toml")
	if err != nil {
		t.Fatalf("Load() returned error for nonexistent file: %v", err)
	}
	if cfg == nil {
		t.Fatal("Load() returned nil config")
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
