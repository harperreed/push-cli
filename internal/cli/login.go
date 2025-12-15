// ABOUTME: Login command for authenticating with Pushover.
// ABOUTME: Handles device registration and credential storage.
package cli

import (
	"context"
	"errors"
	"fmt"

	"github.com/harper/push/internal/config"
	"github.com/harper/push/internal/pushover"
	"github.com/spf13/cobra"
)

func newLoginCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Authenticate with Pushover and store credentials",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLogin(cmd)
		},
	}
	cmd.Flags().String("device-name", "push-cli", "device name to register")

	return cmd
}

func runLogin(cmd *cobra.Command) error {
	ctx := cmd.Context()
	prom := newPrompter(cmd.OutOrStdout())

	cfg, cfgPath, err := loadConfig()
	if err != nil {
		return err
	}
	cfg = cfg.Clone()
	if cfg == nil {
		cfg = &config.Config{}
	}

	deviceName, _ := cmd.Flags().GetString("device-name")

	appToken, err := prom.Ask("Pushover app token", cfg.AppToken)
	if err != nil {
		return fmt.Errorf("reading app token: %w", err)
	}
	userKey, err := prom.Ask("Pushover user key", cfg.UserKey)
	if err != nil {
		return fmt.Errorf("reading user key: %w", err)
	}
	email, err := prom.Ask("Email", "")
	if err != nil {
		return fmt.Errorf("reading email: %w", err)
	}
	password, err := prom.AskSecret("Password")
	if err != nil {
		return fmt.Errorf("reading password: %w", err)
	}

	client := pushover.NewClient(appToken, userKey, "", "")
	loginResp, err := performLogin(ctx, prom, client, email, password)
	if err != nil {
		return err
	}

	deviceResp, err := client.RegisterDevice(ctx, loginResp.Secret, deviceName)
	if err != nil {
		return err
	}

	cfg.AppToken = appToken
	cfg.UserKey = userKey
	cfg.DeviceSecret = loginResp.Secret
	if deviceResp.ID != "" {
		cfg.DeviceID = deviceResp.ID
	} else if deviceResp.Name != "" {
		cfg.DeviceID = deviceResp.Name
	}
	if cfg.DefaultDevice == "" && deviceName != "" {
		cfg.DefaultDevice = deviceName
	}

	if err := config.Save(cfgPath, cfg); err != nil {
		return err
	}

	cmd.Printf("✓ Logged in. Device %q registered.\n", cfg.DeviceID)
	return nil
}

func performLogin(ctx context.Context, prom *prompter, client *pushover.Client, email, password string) (*pushover.LoginResponse, error) {
	loginResp, err := client.Login(ctx, email, password, "")
	if err == nil {
		return loginResp, nil
	}

	if errors.Is(err, pushover.ErrTwoFactorRequired) {
		code, promptErr := prom.Ask("2FA code", "")
		if promptErr != nil {
			return nil, promptErr
		}
		return client.Login(ctx, email, password, code)
	}

	return nil, err
}
