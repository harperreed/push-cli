// ABOUTME: HTTP validation for Pushover API credentials.
// ABOUTME: Tests app token and user key by calling the Pushover validate endpoint.
package tui

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ValidateCredentials tests the Pushover API credentials by calling the validate endpoint.
// The context allows cancellation when the user quits during validation.
func ValidateCredentials(ctx context.Context, appToken, userKey string) error {
	client := &http.Client{Timeout: 10 * time.Second}

	data := url.Values{}
	data.Set("token", appToken)
	data.Set("user", userKey)

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.pushover.net/1/users/validate.json", strings.NewReader(data.Encode()))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("connection failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		return fmt.Errorf("API returned %d: %s", resp.StatusCode, string(body))
	}

	return nil
}
