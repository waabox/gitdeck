package auth_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/waabox/gitdeck/internal/auth"
	"github.com/waabox/gitdeck/internal/config"
)

func TestTokenManager_RefreshGitLab_UpdatesTokensAndSaves(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"access_token":  "new_access",
			"refresh_token": "new_refresh",
		})
	}))
	defer server.Close()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")
	cfg := &config.Config{}
	cfg.GitLab.Token = "old_access"
	cfg.GitLab.RefreshToken = "old_refresh"
	cfg.GitLab.ClientID = "test_client"

	tm := auth.NewTokenManager(cfg, cfgPath, server.URL)

	newToken, err := tm.RefreshGitLab(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if newToken != "new_access" {
		t.Errorf("expected new_access, got %s", newToken)
	}
	if cfg.GitLab.Token != "new_access" {
		t.Errorf("expected cfg token updated, got %s", cfg.GitLab.Token)
	}
	if cfg.GitLab.RefreshToken != "new_refresh" {
		t.Errorf("expected cfg refresh_token updated, got %s", cfg.GitLab.RefreshToken)
	}

	// Verify config was persisted to disk
	loaded, err := config.LoadFrom(cfgPath)
	if err != nil {
		t.Fatalf("loading saved config: %v", err)
	}
	if loaded.GitLab.Token != "new_access" {
		t.Errorf("expected persisted token 'new_access', got '%s'", loaded.GitLab.Token)
	}
	if loaded.GitLab.RefreshToken != "new_refresh" {
		t.Errorf("expected persisted refresh_token 'new_refresh', got '%s'", loaded.GitLab.RefreshToken)
	}
}

func TestTokenManager_RefreshGitLab_ReturnsErrorWhenNoRefreshToken(t *testing.T) {
	cfg := &config.Config{}
	cfg.GitLab.Token = "old_access"
	// No refresh token set

	tm := auth.NewTokenManager(cfg, "", "")

	_, err := tm.RefreshGitLab(context.Background())
	if err == nil {
		t.Fatal("expected error when no refresh token, got nil")
	}
}

func TestTokenManager_RefreshGitLab_ReturnsErrorOnHTTPFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid_grant"})
	}))
	defer server.Close()

	cfg := &config.Config{}
	cfg.GitLab.Token = "old_access"
	cfg.GitLab.RefreshToken = "revoked_refresh"
	cfg.GitLab.ClientID = "test_client"

	tm := auth.NewTokenManager(cfg, "", server.URL)

	_, err := tm.RefreshGitLab(context.Background())
	if err == nil {
		t.Fatal("expected error for failed refresh, got nil")
	}
}
