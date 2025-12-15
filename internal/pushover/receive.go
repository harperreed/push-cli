// ABOUTME: Receive operations for Pushover Open Client API.
// ABOUTME: Fetches and acknowledges messages from Pushover servers.
package pushover

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// ReceivedMessage describes a message returned by the Open Client API.
type ReceivedMessage struct {
	PushoverID int64  `json:"id"`
	UMID       string `json:"umid"`
	Title      string `json:"title"`
	Message    string `json:"message"`
	App        string `json:"app"`
	AID        string `json:"aid"`
	Icon       string `json:"icon"`
	Priority   int    `json:"priority"`
	URL        string `json:"url"`
	URLTitle   string `json:"url_title"`
	Acked      bool   `json:"acked"`
	HTML       bool   `json:"html"`
	Timestamp  int64  `json:"timestamp"`
}

// FetchResult bundles a set of received messages and cursor metadata.
type FetchResult struct {
	Messages      []ReceivedMessage
	LastMessageID int64
	RequestID     string
}

// FetchMessages retrieves unread messages via the Open Client API.
func (c *Client) FetchMessages(ctx context.Context) (*FetchResult, error) {
	if err := c.ensureReceiveCredentials(); err != nil {
		return nil, err
	}

	params := url.Values{}
	params.Set("secret", c.DeviceSecret)
	params.Set("device_id", c.DeviceID)

	resp, err := c.do(ctx, func() (*http.Request, error) { //nolint:bodyclose // body closed by decodeJSON/decodeAPIError
		req, err := http.NewRequest(http.MethodGet, apiBaseURL+"/messages.json?"+params.Encode(), nil)
		if err != nil {
			return nil, err
		}
		return req, nil
	}, defaultRequestAttempts)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		return nil, decodeAPIError(resp)
	}

	var payload struct {
		Status   int               `json:"status"`
		Request  string            `json:"request"`
		Last     int64             `json:"last"`
		Messages []ReceivedMessage `json:"messages"`
	}

	if err := decodeJSON(resp, &payload); err != nil {
		return nil, fmt.Errorf("decode fetch response: %w", err)
	}

	return &FetchResult{Messages: payload.Messages, LastMessageID: payload.Last, RequestID: payload.Request}, nil
}

// DeleteMessages acknowledges messages up to the supplied ID.
func (c *Client) DeleteMessages(ctx context.Context, upToID int64) error {
	if err := c.ensureReceiveCredentials(); err != nil {
		return err
	}
	if upToID <= 0 {
		return fmt.Errorf("message id must be positive")
	}

	values := url.Values{}
	values.Set("secret", c.DeviceSecret)
	values.Set("message", strconv.FormatInt(upToID, 10))
	encoded := values.Encode()

	endpoint := fmt.Sprintf("%s/devices/%s/update_highest_message.json", apiBaseURL, url.PathEscape(c.DeviceID))
	resp, err := c.do(ctx, func() (*http.Request, error) {
		req, err := http.NewRequest(http.MethodPost, endpoint, strings.NewReader(encoded))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		return req, nil
	}, defaultRequestAttempts)
	if err != nil {
		return err
	}

	if resp.StatusCode >= 400 {
		return decodeAPIError(resp)
	}

	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
	return nil
}
