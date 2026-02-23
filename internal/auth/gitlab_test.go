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
	resp, err := flow.PollToken(context.Background(), "gl_dev_abc", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.AccessToken != "glpat_real_token" {
		t.Errorf("token: want 'glpat_real_token', got '%s'", resp.AccessToken)
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
	resp, err := flow.PollToken(context.Background(), "gl_dev_abc", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.AccessToken != "glpat_after_slowdown" {
		t.Errorf("token: want 'glpat_after_slowdown', got '%s'", resp.AccessToken)
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

func TestGitLabDeviceFlow_RefreshToken_ReturnsNewTokens(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/oauth/token" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		if r.FormValue("grant_type") != "refresh_token" {
			t.Errorf("expected grant_type=refresh_token, got %s", r.FormValue("grant_type"))
		}
		if r.FormValue("refresh_token") != "old_refresh" {
			t.Errorf("expected refresh_token=old_refresh, got %s", r.FormValue("refresh_token"))
		}
		if r.FormValue("client_id") != "test_client_id" {
			t.Errorf("expected client_id=test_client_id, got %s", r.FormValue("client_id"))
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"access_token":  "new_access",
			"refresh_token": "new_refresh",
		})
	}))
	defer server.Close()

	flow := auth.NewGitLabDeviceFlow("test_client_id", server.URL)
	resp, err := flow.RefreshToken(context.Background(), "old_refresh")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.AccessToken != "new_access" {
		t.Errorf("access_token: want 'new_access', got '%s'", resp.AccessToken)
	}
	if resp.RefreshToken != "new_refresh" {
		t.Errorf("refresh_token: want 'new_refresh', got '%s'", resp.RefreshToken)
	}
}

func TestGitLabDeviceFlow_RefreshToken_ReturnsErrorOnFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"error":             "invalid_grant",
			"error_description": "refresh token revoked",
		})
	}))
	defer server.Close()

	flow := auth.NewGitLabDeviceFlow("test_client_id", server.URL)
	_, err := flow.RefreshToken(context.Background(), "revoked_refresh")
	if err == nil {
		t.Fatal("expected error for revoked refresh token, got nil")
	}
}

func TestGitLabDeviceFlow_PollToken_ReturnsRefreshToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"access_token":  "glpat_abc",
			"refresh_token": "glrt_xyz",
		})
	}))
	defer server.Close()

	flow := auth.NewGitLabDeviceFlow("test_client_id", server.URL)
	resp, err := flow.PollToken(context.Background(), "gl_dev_abc", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.RefreshToken != "glrt_xyz" {
		t.Errorf("refresh_token: want 'glrt_xyz', got '%s'", resp.RefreshToken)
	}
}
