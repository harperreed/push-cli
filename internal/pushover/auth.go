// ABOUTME: Authentication operations for Pushover Open Client API.
// ABOUTME: Handles user login and device registration flows.
package pushover

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// LoginResponse carries the login secret and related metadata.
type LoginResponse struct {
	Status  int          `json:"status"`
	Request string       `json:"request"`
	Secret  string       `json:"secret"`
	Devices []DeviceInfo `json:"devices"`
}

// DeviceInfo describes a registered device returned during login.
type DeviceInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// DeviceRegistration represents the response to a device registration request.
type DeviceRegistration struct {
	Status  int    `json:"status"`
	Request string `json:"request"`
	ID      string `json:"id"`
	Secret  string `json:"secret"`
	Name    string `json:"name"`
}

// Login authenticates a Pushover user and returns the login secret.
func (c *Client) Login(ctx context.Context, email, password, twoFactorCode string) (*LoginResponse, error) {
	if email == "" || password == "" {
		return nil, fmt.Errorf("email and password are required")
	}

	values := url.Values{}
	values.Set("email", email)
	values.Set("password", password)
	if twoFactorCode != "" {
		values.Set("code", twoFactorCode)
	}
	encoded := values.Encode()

	resp, err := c.do(ctx, func() (*http.Request, error) { //nolint:bodyclose // body closed by decodeJSON/decodeAPIError
		req, err := http.NewRequest(http.MethodPost, apiBaseURL+"/users/login.json", strings.NewReader(encoded))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		return req, nil
	}, defaultRequestAttempts)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		return nil, decodeAPIError(resp)
	}

	var payload LoginResponse
	if err := decodeJSON(resp, &payload); err != nil {
		return nil, fmt.Errorf("decode login response: %w", err)
	}

	if payload.Secret == "" {
		return nil, fmt.Errorf("pushover login did not return a secret")
	}

	return &payload, nil
}

// RegisterDevice registers a device for receiving push notifications.
func (c *Client) RegisterDevice(ctx context.Context, secret, name string) (*DeviceRegistration, error) {
	if secret == "" {
		return nil, fmt.Errorf("secret is required")
	}
	if name == "" {
		return nil, fmt.Errorf("device name is required")
	}

	values := url.Values{}
	values.Set("secret", secret)
	values.Set("name", name)
	values.Set("os", "O")
	encoded := values.Encode()

	resp, err := c.do(ctx, func() (*http.Request, error) { //nolint:bodyclose // body closed by decodeJSON/decodeAPIError
		req, err := http.NewRequest(http.MethodPost, apiBaseURL+"/devices.json", strings.NewReader(encoded))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		return req, nil
	}, defaultRequestAttempts)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		return nil, decodeAPIError(resp)
	}

	var registration DeviceRegistration
	if err := decodeJSON(resp, &registration); err != nil {
		return nil, fmt.Errorf("decode device response: %w", err)
	}

	return &registration, nil
}
