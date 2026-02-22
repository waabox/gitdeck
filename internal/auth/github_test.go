package auth_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/waabox/gitdeck/internal/auth"
)

func TestGitHubDeviceFlow_RequestCode_ReturnsUserCode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/login/device/code" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"device_code":      "dev_abc",
			"user_code":        "ABCD-1234",
			"verification_uri": "https://github.com/login/device",
			"expires_in":       900,
			"interval":         5,
		})
	}))
	defer server.Close()

	flow := auth.NewGitHubDeviceFlow("test_client_id", server.URL)
	code, err := flow.RequestCode(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code.UserCode != "ABCD-1234" {
		t.Errorf("user code: want 'ABCD-1234', got '%s'", code.UserCode)
	}
	if code.DeviceCode != "dev_abc" {
		t.Errorf("device code: want 'dev_abc', got '%s'", code.DeviceCode)
	}
	if code.Interval != 5 {
		t.Errorf("interval: want 5, got %d", code.Interval)
	}
}

func TestGitHubDeviceFlow_PollToken_ReturnsTokenOnSuccess(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		if callCount < 2 {
			json.NewEncoder(w).Encode(map[string]string{"error": "authorization_pending"})
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"access_token": "gho_real_token"})
	}))
	defer server.Close()

	flow := auth.NewGitHubDeviceFlow("test_client_id", server.URL)
	// interval=0 disables the sleep delay in tests
	token, err := flow.PollToken(context.Background(), "dev_abc", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token != "gho_real_token" {
		t.Errorf("token: want 'gho_real_token', got '%s'", token)
	}
}

func TestGitHubDeviceFlow_PollToken_ReturnsErrorOnAccessDenied(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"error": "access_denied"})
	}))
	defer server.Close()

	flow := auth.NewGitHubDeviceFlow("test_client_id", server.URL)
	_, err := flow.PollToken(context.Background(), "dev_abc", 0)
	if err == nil {
		t.Fatal("expected error for access_denied, got nil")
	}
}

func TestGitHubDeviceFlow_PollToken_ReturnsErrorOnExpiredToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"error": "expired_token"})
	}))
	defer server.Close()

	flow := auth.NewGitHubDeviceFlow("test_client_id", server.URL)
	_, err := flow.PollToken(context.Background(), "dev_abc", 0)
	if err == nil {
		t.Fatal("expected error for expired_token, got nil")
	}
}

func TestGitHubDeviceFlow_PollToken_SlowDownIncreasesInterval(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		if callCount == 1 {
			json.NewEncoder(w).Encode(map[string]string{"error": "slow_down"})
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"access_token": "gho_after_slowdown"})
	}))
	defer server.Close()

	flow := auth.NewGitHubDeviceFlow("test_client_id", server.URL)
	token, err := flow.PollToken(context.Background(), "dev_abc", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token != "gho_after_slowdown" {
		t.Errorf("token: want 'gho_after_slowdown', got '%s'", token)
	}
	if callCount != 2 {
		t.Errorf("expected 2 poll calls, got %d", callCount)
	}
}

func TestGitHubDeviceFlow_PollToken_ReturnsErrorOnUnknownErrorCode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"error": "some_unknown_code"})
	}))
	defer server.Close()

	flow := auth.NewGitHubDeviceFlow("test_client_id", server.URL)
	_, err := flow.PollToken(context.Background(), "dev_abc", 0)
	if err == nil {
		t.Fatal("expected error for unknown error code, got nil")
	}
}

func TestGitHubDeviceFlow_PollToken_CancelledContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"error": "authorization_pending"})
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	flow := auth.NewGitHubDeviceFlow("test_client_id", server.URL)
	_, err := flow.PollToken(ctx, "dev_abc", 0)
	if err == nil {
		t.Fatal("expected error for cancelled context, got nil")
	}
}
