// ABOUTME: HTTP client wrapper for Pushover API communication.
// ABOUTME: Handles request building, retries, and rate limiting.
package pushover

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"strings"
	"time"
)

const (
	apiBaseURL             = "https://api.pushover.net/1"
	retryDelay             = 5 * time.Second
	maxConcurrentRequests  = 2
	defaultRequestAttempts = 2
)

// Client wraps HTTP access to the Pushover API.
type Client struct {
	AppToken     string
	UserKey      string
	DeviceID     string
	DeviceSecret string

	httpClient *http.Client
	limiter    chan struct{}
	userAgent  string
}

// NewClient returns a configured client with sane defaults.
func NewClient(appToken, userKey, deviceID, deviceSecret string) *Client {
	return &Client{
		AppToken:     appToken,
		UserKey:      userKey,
		DeviceID:     deviceID,
		DeviceSecret: deviceSecret,
		httpClient:   &http.Client{Timeout: 15 * time.Second},
		limiter:      make(chan struct{}, maxConcurrentRequests),
		userAgent:    fmt.Sprintf("push-cli/1.0 (%s)", runtime.GOOS),
	}
}

// SetHTTPClient overrides the default HTTP client.
func (c *Client) SetHTTPClient(client *http.Client) {
	if client != nil {
		c.httpClient = client
	}
}

type requestBuilder func() (*http.Request, error)

func (c *Client) do(ctx context.Context, build requestBuilder, retries int) (*http.Response, error) { //nolint:unparam // retries kept for flexibility
	attempts := retries
	if attempts <= 0 {
		attempts = 1
	}

	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		req, err := build()
		if err != nil {
			return nil, err
		}
		req = req.WithContext(ctx)
		req.Header.Set("User-Agent", c.userAgent)

		resp, err := c.doOnce(req)
		if err == nil {
			return resp, nil
		}

		lastErr = err
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		if attempt < attempts {
			if err := waitRetry(ctx); err != nil {
				return nil, err
			}
		}
	}

	if lastErr != nil {
		return nil, lastErr
	}
	return nil, errors.New("pushover: request failed")
}

func (c *Client) doOnce(req *http.Request) (*http.Response, error) {
	limiter := c.limiter
	if limiter != nil {
		select {
		case limiter <- struct{}{}:
		case <-req.Context().Done():
			return nil, req.Context().Err()
		}
		defer func() { <-limiter }()
	}

	client := c.httpClient
	if client == nil {
		client = http.DefaultClient
	}

	return client.Do(req)
}

func waitRetry(ctx context.Context) error {
	timer := time.NewTimer(retryDelay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// APIError captures error responses from the Pushover API.
type APIError struct {
	Status    int
	RequestID string
	Messages  []string
}

func (e *APIError) Error() string {
	if e == nil {
		return "pushover API error"
	}

	if len(e.Messages) == 0 {
		return fmt.Sprintf("pushover API error (status=%d, request=%s)", e.Status, e.RequestID)
	}

	return fmt.Sprintf("pushover API error: %s", strings.Join(e.Messages, "; "))
}

var ErrTwoFactorRequired = errors.New("pushover: two-factor authentication required")

func decodeAPIError(resp *http.Response) error {
	if resp == nil {
		return errors.New("pushover API error: nil response")
	}
	defer func() { _, _ = io.Copy(io.Discard, resp.Body); _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusPreconditionFailed {
		return ErrTwoFactorRequired
	}

	var payload struct {
		Status  int      `json:"status"`
		Request string   `json:"request"`
		Errors  []string `json:"errors"`
	}

	body, _ := io.ReadAll(resp.Body)
	if len(body) > 0 {
		_ = json.Unmarshal(body, &payload)
	}

	if len(payload.Errors) == 0 && len(body) > 0 {
		payload.Errors = []string{strings.TrimSpace(string(body))}
	}

	if payload.Status == 0 {
		payload.Status = resp.StatusCode
	}

	return &APIError{Status: payload.Status, RequestID: payload.Request, Messages: payload.Errors}
}

func decodeJSON(resp *http.Response, target interface{}) error {
	defer func() { _ = resp.Body.Close() }()
	dec := json.NewDecoder(resp.Body)
	return dec.Decode(target)
}

func (c *Client) ensureSendCredentials() error {
	if strings.TrimSpace(c.AppToken) == "" {
		return errors.New("pushover: app token not configured")
	}
	if strings.TrimSpace(c.UserKey) == "" {
		return errors.New("pushover: user key not configured")
	}
	return nil
}

func (c *Client) ensureReceiveCredentials() error {
	if err := c.ensureSendCredentials(); err != nil {
		return err
	}
	if strings.TrimSpace(c.DeviceID) == "" || strings.TrimSpace(c.DeviceSecret) == "" {
		return errors.New("pushover: device credentials missing")
	}
	return nil
}
