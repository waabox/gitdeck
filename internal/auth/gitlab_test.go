package auth_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/waabox/gitdeck/internal/auth"
)

func TestGitLabDeviceFlow_RequestCode_ReturnsUserCode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/oauth/authorize_device" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"device_code":      "gl_dev_abc",
			"user_code":        "EFGH-5678",
			"verification_uri": "https://gitlab.com/oauth/device",
			"expires_in":       300,
			"interval":         5,
		})
	}))
	defer server.Close()

	flow := auth.NewGitLabDeviceFlow("test_client_id", server.URL)
	code, err := flow.RequestCode(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code.UserCode != "EFGH-5678" {
		t.Errorf("user code: want 'EFGH-5678', got '%s'", code.UserCode)
	}
	if code.DeviceCode != "gl_dev_abc" {
		t.Errorf("device code: want 'gl_dev_abc', got '%s'", code.DeviceCode)
	}
	if code.Interval != 5 {
		t.Errorf("interval: want 5, got %d", code.Interval)
	}
}

func TestGitLabDeviceFlow_PollToken_ReturnsTokenOnSuccess(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		if callCount < 2 {
			json.NewEncoder(w).Encode(map[string]string{"error": "authorization_pending"})
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"access_token": "glpat_real_token"})
	}))
	defer server.Close()

	flow := auth.NewGitLabDeviceFlow("test_client_id", server.URL)
	token, err := flow.PollToken(context.Background(), "gl_dev_abc", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token != "glpat_real_token" {
		t.Errorf("token: want 'glpat_real_token', got '%s'", token)
	}
}

func TestGitLabDeviceFlow_PollToken_ReturnsErrorOnAccessDenied(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"error": "access_denied"})
	}))
	defer server.Close()

	flow := auth.NewGitLabDeviceFlow("test_client_id", server.URL)
	_, err := flow.PollToken(context.Background(), "gl_dev_abc", 0)
	if err == nil {
		t.Fatal("expected error for access_denied, got nil")
	}
}

func TestGitLabDeviceFlow_PollToken_ReturnsErrorOnExpiredToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"error": "expired_token"})
	}))
	defer server.Close()

	flow := auth.NewGitLabDeviceFlow("test_client_id", server.URL)
	_, err := flow.PollToken(context.Background(), "gl_dev_abc", 0)
	if err == nil {
		t.Fatal("expected error for expired_token, got nil")
	}
}

func TestGitLabDeviceFlow_PollToken_SlowDownIncreasesInterval(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		if callCount == 1 {
			json.NewEncoder(w).Encode(map[string]string{"error": "slow_down"})
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"access_token": "glpat_after_slowdown"})
	}))
	defer server.Close()

	flow := auth.NewGitLabDeviceFlow("test_client_id", server.URL)
	token, err := flow.PollToken(context.Background(), "gl_dev_abc", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token != "glpat_after_slowdown" {
		t.Errorf("token: want 'glpat_after_slowdown', got '%s'", token)
	}
	if callCount != 2 {
		t.Errorf("expected 2 poll calls, got %d", callCount)
	}
}

func TestGitLabDeviceFlow_PollToken_ReturnsErrorOnUnknownErrorCode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"error": "some_unknown_code"})
	}))
	defer server.Close()

	flow := auth.NewGitLabDeviceFlow("test_client_id", server.URL)
	_, err := flow.PollToken(context.Background(), "gl_dev_abc", 0)
	if err == nil {
		t.Fatal("expected error for unknown error code, got nil")
	}
}

func TestGitLabDeviceFlow_PollToken_CancelledContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"error": "authorization_pending"})
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	flow := auth.NewGitLabDeviceFlow("test_client_id", server.URL)
	_, err := flow.PollToken(ctx, "gl_dev_abc", 0)
	if err == nil {
		t.Fatal("expected error for cancelled context, got nil")
	}
}
