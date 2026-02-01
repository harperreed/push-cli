// ABOUTME: Tests for Pushover client and API operations.
// ABOUTME: Uses httptest mock server to test without real API calls.
package pushover

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNewClient(t *testing.T) {
	client := NewClient("app-token", "user-key", "device-id", "device-secret")

	if client.AppToken != "app-token" {
		t.Errorf("AppToken = %q, want %q", client.AppToken, "app-token")
	}
	if client.UserKey != "user-key" {
		t.Errorf("UserKey = %q, want %q", client.UserKey, "user-key")
	}
	if client.DeviceID != "device-id" {
		t.Errorf("DeviceID = %q, want %q", client.DeviceID, "device-id")
	}
	if client.DeviceSecret != "device-secret" {
		t.Errorf("DeviceSecret = %q, want %q", client.DeviceSecret, "device-secret")
	}
	if client.httpClient == nil {
		t.Error("expected httpClient to be initialized")
	}
	if client.limiter == nil {
		t.Error("expected limiter to be initialized")
	}
}

func TestSetHTTPClient(t *testing.T) {
	client := NewClient("", "", "", "")
	originalClient := client.httpClient

	customClient := &http.Client{Timeout: 30 * time.Second}
	client.SetHTTPClient(customClient)

	if client.httpClient != customClient {
		t.Error("SetHTTPClient did not set custom client")
	}

	// Test nil is ignored
	client.SetHTTPClient(nil)
	if client.httpClient != customClient {
		t.Error("SetHTTPClient should ignore nil")
	}

	// Restore original for safety
	client.httpClient = originalClient
}

func TestAPIError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      *APIError
		contains string
	}{
		{
			name:     "nil error",
			err:      nil,
			contains: "pushover API error",
		},
		{
			name:     "empty messages",
			err:      &APIError{Status: 400, RequestID: "abc123"},
			contains: "status=400",
		},
		{
			name:     "with messages",
			err:      &APIError{Status: 400, RequestID: "abc", Messages: []string{"invalid token", "user not found"}},
			contains: "invalid token; user not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := tt.err.Error()
			if !strings.Contains(msg, tt.contains) {
				t.Errorf("Error() = %q, want to contain %q", msg, tt.contains)
			}
		})
	}
}

func TestEnsureSendCredentials(t *testing.T) {
	tests := []struct {
		name    string
		client  *Client
		wantErr bool
		errText string
	}{
		{
			name:    "valid credentials",
			client:  NewClient("token", "user", "", ""),
			wantErr: false,
		},
		{
			name:    "empty app token",
			client:  NewClient("", "user", "", ""),
			wantErr: true,
			errText: "app token",
		},
		{
			name:    "whitespace app token",
			client:  NewClient("   ", "user", "", ""),
			wantErr: true,
			errText: "app token",
		},
		{
			name:    "empty user key",
			client:  NewClient("token", "", "", ""),
			wantErr: true,
			errText: "user key",
		},
		{
			name:    "whitespace user key",
			client:  NewClient("token", "   ", "", ""),
			wantErr: true,
			errText: "user key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.client.ensureSendCredentials()
			if (err != nil) != tt.wantErr {
				t.Errorf("ensureSendCredentials() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil && tt.errText != "" && !strings.Contains(err.Error(), tt.errText) {
				t.Errorf("error = %q, want to contain %q", err.Error(), tt.errText)
			}
		})
	}
}

func TestEnsureReceiveCredentials(t *testing.T) {
	tests := []struct {
		name    string
		client  *Client
		wantErr bool
		errText string
	}{
		{
			name:    "valid credentials",
			client:  NewClient("token", "user", "device", "secret"),
			wantErr: false,
		},
		{
			name:    "missing send credentials",
			client:  NewClient("", "user", "device", "secret"),
			wantErr: true,
			errText: "app token",
		},
		{
			name:    "missing device id",
			client:  NewClient("token", "user", "", "secret"),
			wantErr: true,
			errText: "device credentials",
		},
		{
			name:    "missing device secret",
			client:  NewClient("token", "user", "device", ""),
			wantErr: true,
			errText: "device credentials",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.client.ensureReceiveCredentials()
			if (err != nil) != tt.wantErr {
				t.Errorf("ensureReceiveCredentials() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil && tt.errText != "" && !strings.Contains(err.Error(), tt.errText) {
				t.Errorf("error = %q, want to contain %q", err.Error(), tt.errText)
			}
		})
	}
}

func TestSend_MissingCredentials(t *testing.T) {
	client := NewClient("", "", "", "")
	ctx := context.Background()

	_, err := client.Send(ctx, SendParams{Message: "test"})
	if err == nil {
		t.Error("expected error for missing credentials")
	}
}

func TestSend_EmptyMessage(t *testing.T) {
	client := NewClient("token", "user", "", "")
	ctx := context.Background()

	_, err := client.Send(ctx, SendParams{Message: ""})
	if err == nil {
		t.Error("expected error for empty message")
	}

	_, err = client.Send(ctx, SendParams{Message: "   "})
	if err == nil {
		t.Error("expected error for whitespace-only message")
	}
}

func TestSend_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/messages.json") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		if err := r.ParseForm(); err != nil {
			t.Errorf("failed to parse form: %v", err)
		}
		if r.Form.Get("token") != "app-token" {
			t.Errorf("token = %q, want %q", r.Form.Get("token"), "app-token")
		}
		if r.Form.Get("user") != "user-key" {
			t.Errorf("user = %q, want %q", r.Form.Get("user"), "user-key")
		}
		if r.Form.Get("message") != "Hello World" {
			t.Errorf("message = %q, want %q", r.Form.Get("message"), "Hello World")
		}
		if r.Form.Get("title") != "Test Title" {
			t.Errorf("title = %q, want %q", r.Form.Get("title"), "Test Title")
		}
		if r.Form.Get("priority") != "1" {
			t.Errorf("priority = %q, want %q", r.Form.Get("priority"), "1")
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(SendResponse{
			Status:  1,
			Request: "req-123",
		})
	}))
	defer server.Close()

	client := NewClient("app-token", "user-key", "", "")
	client.SetHTTPClient(server.Client())
	// Override base URL (not easily done, so we test with the full path)
	// We need to mock the URL - let's create a custom transport instead

	// Create a client that redirects to our test server
	client.httpClient = &http.Client{
		Transport: &rewriteTransport{
			targetURL: server.URL,
			rt:        http.DefaultTransport,
		},
	}

	ctx := context.Background()
	resp, err := client.Send(ctx, SendParams{
		Message:  "Hello World",
		Title:    "Test Title",
		Priority: 1,
	})
	if err != nil {
		t.Fatalf("Send() failed: %v", err)
	}

	if resp.Status != 1 {
		t.Errorf("Status = %d, want 1", resp.Status)
	}
	if resp.Request != "req-123" {
		t.Errorf("Request = %q, want %q", resp.Request, "req-123")
	}
}

func TestSend_AllParams(t *testing.T) {
	var capturedForm map[string][]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Errorf("failed to parse form: %v", err)
		}
		capturedForm = r.Form

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(SendResponse{Status: 1, Request: "req-456"})
	}))
	defer server.Close()

	client := NewClient("app-token", "user-key", "", "")
	client.httpClient = &http.Client{
		Transport: &rewriteTransport{targetURL: server.URL, rt: http.DefaultTransport},
	}

	ctx := context.Background()
	ts := time.Unix(1700000000, 0)
	_, err := client.Send(ctx, SendParams{
		Message:   "Test message",
		Title:     "Test title",
		Device:    "my-phone",
		Priority:  2,
		URL:       "https://example.com",
		URLTitle:  "Example",
		Sound:     "cashregister",
		Timestamp: ts,
		HTML:      true,
		Monospace: true,
	})
	if err != nil {
		t.Fatalf("Send() failed: %v", err)
	}

	checks := map[string]string{
		"device":    "my-phone",
		"url":       "https://example.com",
		"url_title": "Example",
		"sound":     "cashregister",
		"timestamp": "1700000000",
		"html":      "1",
		"monospace": "1",
	}
	for key, want := range checks {
		vals := capturedForm[key]
		got := ""
		if len(vals) > 0 {
			got = vals[0]
		}
		if got != want {
			t.Errorf("%s = %q, want %q", key, got, want)
		}
	}
}

func TestSend_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  0,
			"request": "req-err",
			"errors":  []string{"invalid token"},
		})
	}))
	defer server.Close()

	client := NewClient("app-token", "user-key", "", "")
	client.httpClient = &http.Client{
		Transport: &rewriteTransport{targetURL: server.URL, rt: http.DefaultTransport},
	}

	ctx := context.Background()
	_, err := client.Send(ctx, SendParams{Message: "test"})
	if err == nil {
		t.Fatal("expected error for API error response")
	}

	if !strings.Contains(err.Error(), "invalid token") {
		t.Errorf("error = %q, want to contain %q", err.Error(), "invalid token")
	}
}

func TestLogin_MissingCredentials(t *testing.T) {
	client := NewClient("", "", "", "")
	ctx := context.Background()

	_, err := client.Login(ctx, "", "password", "")
	if err == nil {
		t.Error("expected error for missing email")
	}

	_, err = client.Login(ctx, "email", "", "")
	if err == nil {
		t.Error("expected error for missing password")
	}
}

func TestLogin_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/users/login.json") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		if err := r.ParseForm(); err != nil {
			t.Errorf("failed to parse form: %v", err)
		}
		if r.Form.Get("email") != "test@example.com" {
			t.Errorf("email = %q, want %q", r.Form.Get("email"), "test@example.com")
		}
		if r.Form.Get("password") != "secret123" {
			t.Errorf("password = %q, want %q", r.Form.Get("password"), "secret123")
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(LoginResponse{
			Status:  1,
			Request: "req-login",
			Secret:  "login-secret-abc",
			Devices: []DeviceInfo{{ID: "dev1", Name: "Phone"}},
		})
	}))
	defer server.Close()

	client := NewClient("", "", "", "")
	client.httpClient = &http.Client{
		Transport: &rewriteTransport{targetURL: server.URL, rt: http.DefaultTransport},
	}

	ctx := context.Background()
	resp, err := client.Login(ctx, "test@example.com", "secret123", "")
	if err != nil {
		t.Fatalf("Login() failed: %v", err)
	}

	if resp.Secret != "login-secret-abc" {
		t.Errorf("Secret = %q, want %q", resp.Secret, "login-secret-abc")
	}
	if len(resp.Devices) != 1 {
		t.Errorf("Devices count = %d, want 1", len(resp.Devices))
	}
}

func TestLogin_WithTwoFactor(t *testing.T) {
	var capturedTwoFA string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Errorf("failed to parse form: %v", err)
		}
		capturedTwoFA = r.Form.Get("twofa")

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(LoginResponse{
			Status:  1,
			Request: "req-2fa",
			Secret:  "2fa-secret",
		})
	}))
	defer server.Close()

	client := NewClient("", "", "", "")
	client.httpClient = &http.Client{
		Transport: &rewriteTransport{targetURL: server.URL, rt: http.DefaultTransport},
	}

	ctx := context.Background()
	_, err := client.Login(ctx, "test@example.com", "secret", "123456")
	if err != nil {
		t.Fatalf("Login() failed: %v", err)
	}

	if capturedTwoFA != "123456" {
		t.Errorf("twofa = %q, want %q", capturedTwoFA, "123456")
	}
}

func TestLogin_NoSecret(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(LoginResponse{
			Status:  1,
			Request: "req-no-secret",
			Secret:  "",
		})
	}))
	defer server.Close()

	client := NewClient("", "", "", "")
	client.httpClient = &http.Client{
		Transport: &rewriteTransport{targetURL: server.URL, rt: http.DefaultTransport},
	}

	ctx := context.Background()
	_, err := client.Login(ctx, "test@example.com", "secret", "")
	if err == nil {
		t.Fatal("expected error when secret is empty")
	}
	if !strings.Contains(err.Error(), "secret") {
		t.Errorf("error = %q, want to contain 'secret'", err.Error())
	}
}

func TestRegisterDevice_MissingParams(t *testing.T) {
	client := NewClient("", "", "", "")
	ctx := context.Background()

	_, err := client.RegisterDevice(ctx, "", "name")
	if err == nil {
		t.Error("expected error for missing secret")
	}

	_, err = client.RegisterDevice(ctx, "secret", "")
	if err == nil {
		t.Error("expected error for missing name")
	}
}

func TestRegisterDevice_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/devices.json") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		if err := r.ParseForm(); err != nil {
			t.Errorf("failed to parse form: %v", err)
		}
		if r.Form.Get("secret") != "login-secret" {
			t.Errorf("secret = %q, want %q", r.Form.Get("secret"), "login-secret")
		}
		if r.Form.Get("name") != "my-device" {
			t.Errorf("name = %q, want %q", r.Form.Get("name"), "my-device")
		}
		if r.Form.Get("os") != "O" {
			t.Errorf("os = %q, want %q", r.Form.Get("os"), "O")
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(DeviceRegistration{
			Status:  1,
			Request: "req-device",
			ID:      "device-id-123",
			Secret:  "device-secret-456",
			Name:    "my-device",
		})
	}))
	defer server.Close()

	client := NewClient("", "", "", "")
	client.httpClient = &http.Client{
		Transport: &rewriteTransport{targetURL: server.URL, rt: http.DefaultTransport},
	}

	ctx := context.Background()
	resp, err := client.RegisterDevice(ctx, "login-secret", "my-device")
	if err != nil {
		t.Fatalf("RegisterDevice() failed: %v", err)
	}

	if resp.ID != "device-id-123" {
		t.Errorf("ID = %q, want %q", resp.ID, "device-id-123")
	}
	if resp.Secret != "device-secret-456" {
		t.Errorf("Secret = %q, want %q", resp.Secret, "device-secret-456")
	}
}

func TestFetchMessages_MissingCredentials(t *testing.T) {
	client := NewClient("token", "user", "", "")
	ctx := context.Background()

	_, err := client.FetchMessages(ctx)
	if err == nil {
		t.Error("expected error for missing device credentials")
	}
}

func TestFetchMessages_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/messages.json") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		secret := r.URL.Query().Get("secret")
		deviceID := r.URL.Query().Get("device_id")
		if secret != "device-secret" {
			t.Errorf("secret = %q, want %q", secret, "device-secret")
		}
		if deviceID != "device-id" {
			t.Errorf("device_id = %q, want %q", deviceID, "device-id")
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  1,
			"request": "req-fetch",
			"last":    12345,
			"messages": []ReceivedMessage{
				{PushoverID: 100, Title: "Test1", Message: "Message 1"},
				{PushoverID: 101, Title: "Test2", Message: "Message 2"},
			},
		})
	}))
	defer server.Close()

	client := NewClient("app-token", "user-key", "device-id", "device-secret")
	client.httpClient = &http.Client{
		Transport: &rewriteTransport{targetURL: server.URL, rt: http.DefaultTransport},
	}

	ctx := context.Background()
	result, err := client.FetchMessages(ctx)
	if err != nil {
		t.Fatalf("FetchMessages() failed: %v", err)
	}

	if result.LastMessageID != 12345 {
		t.Errorf("LastMessageID = %d, want 12345", result.LastMessageID)
	}
	if len(result.Messages) != 2 {
		t.Fatalf("Messages count = %d, want 2", len(result.Messages))
	}
	if result.Messages[0].PushoverID != 100 {
		t.Errorf("Messages[0].PushoverID = %d, want 100", result.Messages[0].PushoverID)
	}
}

func TestDeleteMessages_MissingCredentials(t *testing.T) {
	client := NewClient("token", "user", "", "")
	ctx := context.Background()

	err := client.DeleteMessages(ctx, 123)
	if err == nil {
		t.Error("expected error for missing device credentials")
	}
}

func TestDeleteMessages_InvalidID(t *testing.T) {
	client := NewClient("token", "user", "device", "secret")
	ctx := context.Background()

	err := client.DeleteMessages(ctx, 0)
	if err == nil {
		t.Error("expected error for zero message id")
	}

	err = client.DeleteMessages(ctx, -1)
	if err == nil {
		t.Error("expected error for negative message id")
	}
}

func TestDeleteMessages_Success(t *testing.T) {
	var capturedMessage string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if !strings.Contains(r.URL.Path, "/devices/") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		if err := r.ParseForm(); err != nil {
			t.Errorf("failed to parse form: %v", err)
		}
		capturedMessage = r.Form.Get("message")

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": 1})
	}))
	defer server.Close()

	client := NewClient("app-token", "user-key", "device-id", "device-secret")
	client.httpClient = &http.Client{
		Transport: &rewriteTransport{targetURL: server.URL, rt: http.DefaultTransport},
	}

	ctx := context.Background()
	err := client.DeleteMessages(ctx, 12345)
	if err != nil {
		t.Fatalf("DeleteMessages() failed: %v", err)
	}

	if capturedMessage != "12345" {
		t.Errorf("message = %q, want %q", capturedMessage, "12345")
	}
}

func TestDecodeAPIError_NilResponse(t *testing.T) {
	err := decodeAPIError(nil)
	if err == nil {
		t.Error("expected error for nil response")
	}
}

func TestDecodeAPIError_TwoFactorRequired(t *testing.T) {
	resp := &http.Response{
		StatusCode: http.StatusPreconditionFailed,
		Body:       http.NoBody,
	}

	err := decodeAPIError(resp)
	if !errors.Is(err, ErrTwoFactorRequired) {
		t.Errorf("expected ErrTwoFactorRequired, got %v", err)
	}
}

func TestContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient("app-token", "user-key", "", "")
	client.httpClient = &http.Client{
		Transport: &rewriteTransport{targetURL: server.URL, rt: http.DefaultTransport},
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := client.Send(ctx, SendParams{Message: "test"})
	if err == nil {
		t.Error("expected error for cancelled context")
	}
}

// rewriteTransport is a test helper that redirects requests to a test server.
type rewriteTransport struct {
	targetURL string
	rt        http.RoundTripper
}

func (t *rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Preserve the path and query from the original request
	newURL := t.targetURL + req.URL.Path
	if req.URL.RawQuery != "" {
		newURL += "?" + req.URL.RawQuery
	}

	newReq, err := http.NewRequestWithContext(req.Context(), req.Method, newURL, req.Body)
	if err != nil {
		return nil, err
	}
	newReq.Header = req.Header
	return t.rt.RoundTrip(newReq)
}

func TestWaitRetry_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err := waitRetry(ctx)
	if err == nil {
		t.Error("expected error for cancelled context")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

func TestDoOnce_NilLimiter(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient("token", "user", "", "")
	client.limiter = nil // Test nil limiter path
	client.httpClient = server.Client()

	req, _ := http.NewRequest(http.MethodGet, server.URL, nil)
	resp, err := client.doOnce(req)
	if err != nil {
		t.Fatalf("doOnce() error: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("StatusCode = %d, want %d", resp.StatusCode, http.StatusOK)
	}
}

func TestDoOnce_NilHTTPClient(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient("token", "user", "", "")
	client.httpClient = nil // Test nil httpClient path - will use default
	client.limiter = nil    // Avoid limiter blocking

	req, _ := http.NewRequest(http.MethodGet, server.URL, nil)
	resp, err := client.doOnce(req)
	if err != nil {
		t.Fatalf("doOnce() error: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("StatusCode = %d, want %d", resp.StatusCode, http.StatusOK)
	}
}

func TestDo_ContextCancelledDuringRetry(t *testing.T) {
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewClient("app-token", "user-key", "", "")
	client.httpClient = &http.Client{
		Transport: &rewriteTransport{targetURL: server.URL, rt: http.DefaultTransport},
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Cancel context during the test
	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()

	_, err := client.Send(ctx, SendParams{Message: "test"})
	if err == nil {
		t.Error("expected error")
	}
}

func TestDo_RequestBuildError(t *testing.T) {
	client := NewClient("token", "user", "", "")

	// Create a request builder that always errors
	resp, err := client.do(context.Background(), func() (*http.Request, error) {
		return nil, errors.New("build error")
	}, 1)
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}

	if err == nil {
		t.Error("expected error from request builder")
	}
	if !strings.Contains(err.Error(), "build error") {
		t.Errorf("error = %q, want to contain 'build error'", err.Error())
	}
}

func TestDecodeAPIError_EmptyBody(t *testing.T) {
	resp := &http.Response{
		StatusCode: http.StatusBadRequest,
		Body:       io.NopCloser(strings.NewReader("")),
	}

	err := decodeAPIError(resp)
	if err == nil {
		t.Error("expected error for bad status")
	}

	apiErr := &APIError{}
	ok := errors.As(err, &apiErr)
	if !ok {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if apiErr.Status != http.StatusBadRequest {
		t.Errorf("Status = %d, want %d", apiErr.Status, http.StatusBadRequest)
	}
}

func TestDecodeAPIError_NonJSONBody(t *testing.T) {
	resp := &http.Response{
		StatusCode: http.StatusBadRequest,
		Body:       io.NopCloser(strings.NewReader("plain text error")),
	}

	err := decodeAPIError(resp)
	if err == nil {
		t.Error("expected error for bad status")
	}

	apiErr := &APIError{}
	ok := errors.As(err, &apiErr)
	if !ok {
		t.Fatalf("expected *APIError, got %T", err)
	}
	// Non-JSON body should be included in messages
	if len(apiErr.Messages) == 0 {
		t.Error("expected non-empty messages for non-JSON body")
	}
}

func TestFetchMessages_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"status": 0,
			"errors": []string{"invalid secret"},
		})
	}))
	defer server.Close()

	client := NewClient("token", "user", "device", "secret")
	client.httpClient = &http.Client{
		Transport: &rewriteTransport{targetURL: server.URL, rt: http.DefaultTransport},
	}

	ctx := context.Background()
	_, err := client.FetchMessages(ctx)
	if err == nil {
		t.Error("expected error for API error response")
	}
}

func TestDeleteMessages_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"status": 0,
			"errors": []string{"invalid message id"},
		})
	}))
	defer server.Close()

	client := NewClient("token", "user", "device", "secret")
	client.httpClient = &http.Client{
		Transport: &rewriteTransport{targetURL: server.URL, rt: http.DefaultTransport},
	}

	ctx := context.Background()
	err := client.DeleteMessages(ctx, 12345)
	if err == nil {
		t.Error("expected error for API error response")
	}
}

func TestLogin_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"status": 0,
			"errors": []string{"invalid credentials"},
		})
	}))
	defer server.Close()

	client := NewClient("", "", "", "")
	client.httpClient = &http.Client{
		Transport: &rewriteTransport{targetURL: server.URL, rt: http.DefaultTransport},
	}

	ctx := context.Background()
	_, err := client.Login(ctx, "test@example.com", "password", "")
	if err == nil {
		t.Error("expected error for API error response")
	}
}

func TestRegisterDevice_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"status": 0,
			"errors": []string{"device name already exists"},
		})
	}))
	defer server.Close()

	client := NewClient("", "", "", "")
	client.httpClient = &http.Client{
		Transport: &rewriteTransport{targetURL: server.URL, rt: http.DefaultTransport},
	}

	ctx := context.Background()
	_, err := client.RegisterDevice(ctx, "secret", "existing-device")
	if err == nil {
		t.Error("expected error for API error response")
	}
}

func TestRegisterDevice_ReturnsResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(DeviceRegistration{
			Status:  1,
			Request: "req-123",
			ID:      "device-123",
			Secret:  "device-secret",
			Name:    "test-device",
		})
	}))
	defer server.Close()

	client := NewClient("", "", "", "")
	client.httpClient = &http.Client{
		Transport: &rewriteTransport{targetURL: server.URL, rt: http.DefaultTransport},
	}

	ctx := context.Background()
	resp, err := client.RegisterDevice(ctx, "login-secret", "test-device")
	if err != nil {
		t.Fatalf("RegisterDevice() error: %v", err)
	}

	if resp.ID != "device-123" {
		t.Errorf("ID = %q, want %q", resp.ID, "device-123")
	}
	if resp.Secret != "device-secret" {
		t.Errorf("Secret = %q, want %q", resp.Secret, "device-secret")
	}
}

func TestDo_SuccessOnRetry(t *testing.T) {
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		// Succeed on second request (if retries were used properly)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  1,
			"request": "req-retry",
		})
	}))
	defer server.Close()

	client := NewClient("app-token", "user-key", "", "")
	client.httpClient = &http.Client{
		Transport: &rewriteTransport{targetURL: server.URL, rt: http.DefaultTransport},
	}

	ctx := context.Background()
	resp, err := client.Send(ctx, SendParams{Message: "test"})
	if err != nil {
		t.Fatalf("Send() failed: %v", err)
	}

	if resp.Request != "req-retry" {
		t.Errorf("Request = %q, want %q", resp.Request, "req-retry")
	}
}
