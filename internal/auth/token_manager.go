package auth

import (
	"context"
	"fmt"
	"sync"

	"github.com/waabox/gitdeck/internal/config"
)

// defaultGitLabClientID matches the value in cmd/gitdeck/main.go.
const defaultGitLabClientID = "9df6c8abe93dc879a79ecf7681909b4a37d5c61064190a795bbf16e1ed8bffa3"

// TokenManager handles silent token refresh and config persistence.
type TokenManager struct {
	cfg        *config.Config
	configPath string
	gitlabURL  string
	mu         sync.Mutex
}

// NewTokenManager creates a TokenManager.
// gitlabURL is the base URL for GitLab OAuth endpoints (pass empty for gitlab.com default).
func NewTokenManager(cfg *config.Config, configPath string, gitlabURL string) *TokenManager {
	return &TokenManager{
		cfg:        cfg,
		configPath: configPath,
		gitlabURL:  gitlabURL,
	}
}

// RefreshGitLab attempts to refresh the GitLab access token using the stored refresh token.
// On success, it updates the config in memory and persists it to disk.
// Returns the new access token or an error.
func (tm *TokenManager) RefreshGitLab(ctx context.Context) (string, error) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if tm.cfg.GitLab.RefreshToken == "" {
		return "", fmt.Errorf("no refresh token available")
	}

	clientID := tm.cfg.GitLab.ClientID
	if clientID == "" {
		clientID = defaultGitLabClientID
	}

	flow := NewGitLabDeviceFlow(clientID, tm.gitlabURL)
	resp, err := flow.RefreshToken(ctx, tm.cfg.GitLab.RefreshToken)
	if err != nil {
		return "", fmt.Errorf("refreshing GitLab token: %w", err)
	}

	tm.cfg.GitLab.Token = resp.AccessToken
	tm.cfg.GitLab.RefreshToken = resp.RefreshToken

	if tm.configPath != "" {
		if saveErr := config.Save(tm.configPath, *tm.cfg); saveErr != nil {
			// Token refreshed in memory but save failed -- still return success
			// since the token is usable for this session
			return resp.AccessToken, fmt.Errorf("token refreshed but failed to save config: %w", saveErr)
		}
	}

	return resp.AccessToken, nil
}

// Config returns the current config pointer.
func (tm *TokenManager) Config() *config.Config {
	return tm.cfg
}

// ConfigPath returns the config file path.
func (tm *TokenManager) ConfigPath() string {
	return tm.configPath
}
