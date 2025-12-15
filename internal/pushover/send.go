// ABOUTME: Send operations for Pushover Message API.
// ABOUTME: Dispatches push notifications with various options.
package pushover

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// SendParams captures the fields for the Message API.
type SendParams struct {
	Message   string
	Title     string
	Device    string
	Priority  int
	URL       string
	URLTitle  string
	Sound     string
	Timestamp time.Time
	HTML      bool
	Monospace bool
}

// SendResponse mirrors the API response to a send request.
type SendResponse struct {
	Status  int      `json:"status"`
	Request string   `json:"request"`
	Receipt string   `json:"receipt"`
	Errors  []string `json:"errors"`
}

// Send dispatches a push notification via the Message API.
func (c *Client) Send(ctx context.Context, params SendParams) (*SendResponse, error) {
	if err := c.ensureSendCredentials(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(params.Message) == "" {
		return nil, fmt.Errorf("message cannot be empty")
	}

	values := url.Values{}
	values.Set("token", c.AppToken)
	values.Set("user", c.UserKey)
	values.Set("message", params.Message)

	if params.Title != "" {
		values.Set("title", params.Title)
	}
	if params.Device != "" {
		values.Set("device", params.Device)
	}
	if params.Priority != 0 {
		values.Set("priority", strconv.Itoa(params.Priority))
	}
	if params.URL != "" {
		values.Set("url", params.URL)
	}
	if params.URLTitle != "" {
		values.Set("url_title", params.URLTitle)
	}
	if params.Sound != "" {
		values.Set("sound", params.Sound)
	}
	if !params.Timestamp.IsZero() {
		values.Set("timestamp", strconv.FormatInt(params.Timestamp.Unix(), 10))
	}
	if params.HTML {
		values.Set("html", "1")
	}
	if params.Monospace {
		values.Set("monospace", "1")
	}

	encoded := values.Encode()

	resp, err := c.do(ctx, func() (*http.Request, error) { //nolint:bodyclose // body closed by decodeJSON/decodeAPIError
		req, err := http.NewRequest(http.MethodPost, apiBaseURL+"/messages.json", strings.NewReader(encoded))
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

	var payload SendResponse
	if err := decodeJSON(resp, &payload); err != nil {
		return nil, fmt.Errorf("decode send response: %w", err)
	}

	return &payload, nil
}
