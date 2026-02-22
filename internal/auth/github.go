package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// githubClientID is the OAuth App client ID registered at https://github.com/settings/developers.
// Replace this constant with your real client ID before building.
const githubClientID = "REPLACE_WITH_YOUR_GITHUB_OAUTH_APP_CLIENT_ID"

const githubDefaultBaseURL = "https://github.com"

// GitHubDeviceFlow implements the OAuth 2.0 Device Authorization Flow for GitHub.
// See https://docs.github.com/en/apps/oauth-apps/building-oauth-apps/authorizing-oauth-apps#device-flow
type GitHubDeviceFlow struct {
	clientID string
	baseURL  string
	client   *http.Client
}

// NewGitHubDeviceFlow creates a GitHubDeviceFlow.
// Pass an empty baseURL to use the real GitHub API. Pass a test server URL in tests.
func NewGitHubDeviceFlow(clientID string, baseURL string) *GitHubDeviceFlow {
	if baseURL == "" {
		baseURL = githubDefaultBaseURL
	}
	return &GitHubDeviceFlow{
		clientID: clientID,
		baseURL:  baseURL,
		client:   &http.Client{Timeout: 15 * time.Second},
	}
}

// NewDefaultGitHubDeviceFlow creates a GitHubDeviceFlow using the embedded client ID.
func NewDefaultGitHubDeviceFlow() *GitHubDeviceFlow {
	return NewGitHubDeviceFlow(githubClientID, "")
}

// RequestCode requests a device code and user code from GitHub.
// The returned DeviceCodeResponse.UserCode must be shown to the user along with VerificationURI.
// ctx is used to cancel the request (e.g. when the user quits the TUI).
func (f *GitHubDeviceFlow) RequestCode(ctx context.Context) (DeviceCodeResponse, error) {
	data := url.Values{}
	data.Set("client_id", f.clientID)
	data.Set("scope", "repo,workflow")

	endpoint, err := url.JoinPath(f.baseURL, "/login/device/code")
	if err != nil {
		return DeviceCodeResponse{}, fmt.Errorf("building URL: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return DeviceCodeResponse{}, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := f.client.Do(req)
	if err != nil {
		return DeviceCodeResponse{}, fmt.Errorf("requesting device code: %w", err)
	}
	defer resp.Body.Close()

	var raw struct {
		DeviceCode      string `json:"device_code"`
		UserCode        string `json:"user_code"`
		VerificationURI string `json:"verification_uri"`
		ExpiresIn       int    `json:"expires_in"`
		Interval        int    `json:"interval"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return DeviceCodeResponse{}, fmt.Errorf("decoding device code response: %w", err)
	}
	return DeviceCodeResponse{
		DeviceCode:      raw.DeviceCode,
		UserCode:        raw.UserCode,
		VerificationURI: raw.VerificationURI,
		ExpiresIn:       raw.ExpiresIn,
		Interval:        raw.Interval,
	}, nil
}

// PollToken polls the GitHub token endpoint until an access token is granted or an error occurs.
// interval is the polling interval in seconds; pass 0 to skip the sleep delay (useful in tests).
// ctx is used to cancel the polling loop (e.g. when the user quits the TUI).
// Handles authorization_pending, slow_down, expired_token, and access_denied error codes.
func (f *GitHubDeviceFlow) PollToken(ctx context.Context, deviceCode string, interval int) (string, error) {
	if interval <= 0 {
		// interval=0 means no sleep (test mode); negative is treated as no-sleep too
		interval = 0
	}

	tokenEndpoint, err := url.JoinPath(f.baseURL, "/login/oauth/access_token")
	if err != nil {
		return "", fmt.Errorf("building URL: %w", err)
	}

	for {
		if interval > 0 {
			select {
			case <-time.After(time.Duration(interval) * time.Second):
			case <-ctx.Done():
				return "", ctx.Err()
			}
		} else {
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			default:
			}
		}

		data := url.Values{}
		data.Set("client_id", f.clientID)
		data.Set("device_code", deviceCode)
		data.Set("grant_type", "urn:ietf:params:oauth:grant-type:device_code")

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenEndpoint, strings.NewReader(data.Encode()))
		if err != nil {
			return "", fmt.Errorf("creating request: %w", err)
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		resp, err := f.client.Do(req)
		if err != nil {
			return "", fmt.Errorf("polling token: %w", err)
		}

		var raw struct {
			AccessToken string `json:"access_token"`
			Error       string `json:"error"`
		}
		decodeErr := json.NewDecoder(resp.Body).Decode(&raw)
		resp.Body.Close()
		if decodeErr != nil {
			return "", fmt.Errorf("decoding token response: %w", decodeErr)
		}

		switch raw.Error {
		case "":
			if raw.AccessToken != "" {
				return raw.AccessToken, nil
			}
			// server returned neither token nor error — check context and retry
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			default:
			}
		case "authorization_pending":
			// keep polling
		case "slow_down":
			interval += 5
		case "expired_token":
			return "", fmt.Errorf("device code expired — run gitdeck again to restart authentication")
		case "access_denied":
			return "", fmt.Errorf("access denied by user")
		default:
			errMsg := raw.Error
			if len(errMsg) > 100 {
				errMsg = errMsg[:100]
			}
			return "", fmt.Errorf("unexpected error from GitHub: %s", errMsg)
		}
	}
}
