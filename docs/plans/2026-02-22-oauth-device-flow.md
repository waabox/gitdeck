# OAuth Device Authorization Flow Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Implement the OAuth 2.0 Device Authorization Flow (RFC 8628) for GitHub and GitLab so that gitdeck authenticates automatically on first run with no manual token setup.

**Architecture:** A new `internal/auth` package implements the Device Flow HTTP interactions for each provider. `config.Save` persists the obtained token. `main.go` detects the provider from the remote URL, checks for a missing token, runs the flow inline, saves the token, then launches the TUI as normal.

**Tech Stack:** Go 1.24, `net/http`, `net/http/httptest` (tests), `github.com/BurntSushi/toml` (TOML encoding)

---

## Prerequisites

Before compiling, register two OAuth Apps:

1. **GitHub** — go to `https://github.com/settings/developers` → "New OAuth App" → enable "Device Flow"
2. **GitLab** — go to `https://gitlab.com/-/user_settings/applications` → Grant type: "Device Authorization Grant" → Scope: `read_api`

Replace the placeholder constants in Tasks 2 and 3 with the real client IDs.

---

### Task 1: config.Save

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`

**Step 1: Write the failing test**

Add to `internal/config/config_test.go`:

```go
func TestSave_WritesAndReloadsCorrectly(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	cfg := config.Config{
		GitHub: config.GitHubConfig{Token: "ghp_saved"},
		GitLab: config.GitLabConfig{Token: "glpat_saved", URL: "https://gl.example.com"},
	}

	if err := config.Save(path, cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	loaded, err := config.LoadFrom(path)
	if err != nil {
		t.Fatalf("unexpected error loading: %v", err)
	}
	if loaded.GitHub.Token != "ghp_saved" {
		t.Errorf("github token: want 'ghp_saved', got '%s'", loaded.GitHub.Token)
	}
	if loaded.GitLab.Token != "glpat_saved" {
		t.Errorf("gitlab token: want 'glpat_saved', got '%s'", loaded.GitLab.Token)
	}
	if loaded.GitLab.URL != "https://gl.example.com" {
		t.Errorf("gitlab url: want 'https://gl.example.com', got '%s'", loaded.GitLab.URL)
	}
}

func TestSave_CreatesParentDirectory(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "dir", "config.toml")
	cfg := config.Config{GitHub: config.GitHubConfig{Token: "ghp_test"}}

	if err := config.Save(path, cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("file not created: %v", err)
	}
}
```

**Step 2: Run to verify the test fails**

```
go test ./internal/config/... -run TestSave -v
```

Expected: `FAIL — undefined: config.Save`

**Step 3: Implement Save in `internal/config/config.go`**

Add this import at the top: `"github.com/BurntSushi/toml"`

Add after the existing functions:

```go
// Save writes cfg to the given TOML file path, creating parent directories as needed.
// Existing file contents are overwritten. Permissions on the written file are 0600.
func Save(path string, cfg Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("opening config file: %w", err)
	}
	defer f.Close()
	return toml.NewEncoder(f).Encode(cfg)
}
```

Add missing imports to the existing import block: `"fmt"`, `"path/filepath"`.

**Step 4: Run to verify tests pass**

```
go test ./internal/config/... -v
```

Expected: all tests PASS

**Step 5: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go
git commit -m "feat: add config.Save to persist tokens to TOML file"
```

---

### Task 2: auth package — shared types + GitHub Device Flow

**Files:**
- Create: `internal/auth/device.go`
- Create: `internal/auth/github.go`
- Create: `internal/auth/github_test.go`

**Step 1: Create `internal/auth/device.go`**

```go
package auth

// DeviceCodeResponse holds the initial response from a device authorization request.
// It contains the code to show the user and the parameters needed for polling.
type DeviceCodeResponse struct {
	DeviceCode      string
	UserCode        string
	VerificationURI string
	ExpiresIn       int
	Interval        int
}
```

**Step 2: Write failing tests in `internal/auth/github_test.go`**

```go
package auth_test

import (
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
	code, err := flow.RequestCode()
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
	token, err := flow.PollToken("dev_abc", 0)
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
	_, err := flow.PollToken("dev_abc", 0)
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
	_, err := flow.PollToken("dev_abc", 0)
	if err == nil {
		t.Fatal("expected error for expired_token, got nil")
	}
}
```

**Step 3: Run to verify tests fail**

```
go test ./internal/auth/... -run TestGitHub -v
```

Expected: `FAIL — undefined: auth.NewGitHubDeviceFlow`

**Step 4: Implement `internal/auth/github.go`**

```go
package auth

import (
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
func (f *GitHubDeviceFlow) RequestCode() (DeviceCodeResponse, error) {
	data := url.Values{}
	data.Set("client_id", f.clientID)
	data.Set("scope", "repo,workflow")

	req, err := http.NewRequest(http.MethodPost, f.baseURL+"/login/device/code", strings.NewReader(data.Encode()))
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
// Handles authorization_pending, slow_down, expired_token, and access_denied error codes.
func (f *GitHubDeviceFlow) PollToken(deviceCode string, interval int) (string, error) {
	if interval < 0 {
		interval = 5
	}
	for {
		if interval > 0 {
			time.Sleep(time.Duration(interval) * time.Second)
		}

		data := url.Values{}
		data.Set("client_id", f.clientID)
		data.Set("device_code", deviceCode)
		data.Set("grant_type", "urn:ietf:params:oauth:grant-type:device_code")

		req, err := http.NewRequest(http.MethodPost, f.baseURL+"/login/oauth/access_token", strings.NewReader(data.Encode()))
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
		case "authorization_pending":
			// keep polling
		case "slow_down":
			interval += 5
		case "expired_token":
			return "", fmt.Errorf("device code expired — run gitdeck again to restart authentication")
		case "access_denied":
			return "", fmt.Errorf("access denied by user")
		default:
			return "", fmt.Errorf("unexpected error from GitHub: %s", raw.Error)
		}
	}
}
```

**Step 5: Run to verify tests pass**

```
go test ./internal/auth/... -run TestGitHub -v
```

Expected: all TestGitHub tests PASS

**Step 6: Commit**

```bash
git add internal/auth/device.go internal/auth/github.go internal/auth/github_test.go
git commit -m "feat: implement GitHub OAuth device authorization flow"
```

---

### Task 3: GitLab Device Flow

**Files:**
- Create: `internal/auth/gitlab.go`
- Create: `internal/auth/gitlab_test.go`

**Step 1: Write failing tests in `internal/auth/gitlab_test.go`**

```go
package auth_test

import (
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
	code, err := flow.RequestCode()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code.UserCode != "EFGH-5678" {
		t.Errorf("user code: want 'EFGH-5678', got '%s'", code.UserCode)
	}
	if code.DeviceCode != "gl_dev_abc" {
		t.Errorf("device code: want 'gl_dev_abc', got '%s'", code.DeviceCode)
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
	token, err := flow.PollToken("gl_dev_abc", 0)
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
	_, err := flow.PollToken("gl_dev_abc", 0)
	if err == nil {
		t.Fatal("expected error for access_denied, got nil")
	}
}
```

**Step 2: Run to verify tests fail**

```
go test ./internal/auth/... -run TestGitLab -v
```

Expected: `FAIL — undefined: auth.NewGitLabDeviceFlow`

**Step 3: Implement `internal/auth/gitlab.go`**

```go
package auth

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// gitlabClientID is the OAuth Application client ID registered at
// https://gitlab.com/-/user_settings/applications (or your self-hosted instance).
// Replace this constant with your real client ID before building.
const gitlabClientID = "REPLACE_WITH_YOUR_GITLAB_OAUTH_APP_CLIENT_ID"

const gitlabDefaultBaseURL = "https://gitlab.com"

// GitLabDeviceFlow implements the OAuth 2.0 Device Authorization Flow for GitLab.
// Works with gitlab.com and self-hosted GitLab instances.
// See https://docs.gitlab.com/ee/api/oauth2.html#device-authorization-grant-flow
type GitLabDeviceFlow struct {
	clientID string
	baseURL  string
	client   *http.Client
}

// NewGitLabDeviceFlow creates a GitLabDeviceFlow.
// Pass an empty baseURL to use gitlab.com. Pass a custom URL for self-hosted instances or tests.
func NewGitLabDeviceFlow(clientID string, baseURL string) *GitLabDeviceFlow {
	if baseURL == "" {
		baseURL = gitlabDefaultBaseURL
	}
	return &GitLabDeviceFlow{
		clientID: clientID,
		baseURL:  baseURL,
		client:   &http.Client{Timeout: 15 * time.Second},
	}
}

// NewDefaultGitLabDeviceFlow creates a GitLabDeviceFlow for gitlab.com using the embedded client ID.
func NewDefaultGitLabDeviceFlow(baseURL string) *GitLabDeviceFlow {
	return NewGitLabDeviceFlow(gitlabClientID, baseURL)
}

// RequestCode requests a device code and user code from GitLab.
func (f *GitLabDeviceFlow) RequestCode() (DeviceCodeResponse, error) {
	data := url.Values{}
	data.Set("client_id", f.clientID)
	data.Set("scope", "read_api")

	req, err := http.NewRequest(http.MethodPost, f.baseURL+"/oauth/authorize_device", strings.NewReader(data.Encode()))
	if err != nil {
		return DeviceCodeResponse{}, fmt.Errorf("creating request: %w", err)
	}
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

// PollToken polls the GitLab token endpoint until an access token is granted or an error occurs.
// interval is the polling interval in seconds; pass 0 to skip the sleep delay (useful in tests).
func (f *GitLabDeviceFlow) PollToken(deviceCode string, interval int) (string, error) {
	if interval < 0 {
		interval = 5
	}
	for {
		if interval > 0 {
			time.Sleep(time.Duration(interval) * time.Second)
		}

		data := url.Values{}
		data.Set("client_id", f.clientID)
		data.Set("device_code", deviceCode)
		data.Set("grant_type", "urn:ietf:params:oauth:grant-type:device_code")

		req, err := http.NewRequest(http.MethodPost, f.baseURL+"/oauth/token", strings.NewReader(data.Encode()))
		if err != nil {
			return "", fmt.Errorf("creating request: %w", err)
		}
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
		case "authorization_pending":
			// keep polling
		case "slow_down":
			interval += 5
		case "expired_token":
			return "", fmt.Errorf("device code expired — run gitdeck again to restart authentication")
		case "access_denied":
			return "", fmt.Errorf("access denied by user")
		default:
			return "", fmt.Errorf("unexpected error from GitLab: %s", raw.Error)
		}
	}
}
```

**Step 4: Run to verify all auth tests pass**

```
go test ./internal/auth/... -v
```

Expected: all tests PASS

**Step 5: Commit**

```bash
git add internal/auth/gitlab.go internal/auth/gitlab_test.go
git commit -m "feat: implement GitLab OAuth device authorization flow"
```

---

### Task 4: Wire OAuth into main.go

**Files:**
- Modify: `cmd/gitdeck/main.go`

**Step 1: Read the current main.go and understand the structure**

Current flow: detect repo → load config → build registry → detect provider → run TUI.
New flow inserts an auth check between "load config" and "build registry".

**Step 2: Update `cmd/gitdeck/main.go`**

Replace the entire file content with:

```go
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/waabox/gitdeck/internal/auth"
	"github.com/waabox/gitdeck/internal/config"
	"github.com/waabox/gitdeck/internal/git"
	"github.com/waabox/gitdeck/internal/provider"
	githubprovider "github.com/waabox/gitdeck/internal/provider/github"
	gitlabprovider "github.com/waabox/gitdeck/internal/provider/gitlab"
	"github.com/waabox/gitdeck/internal/tui"
)

func main() {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error getting current directory: %v\n", err)
		os.Exit(1)
	}

	repo, err := git.DetectRepository(cwd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error detecting git remote: %v\n", err)
		os.Exit(1)
	}

	configPath := config.DefaultConfigPath()
	cfg, err := config.LoadFrom(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading config: %v\n", err)
		os.Exit(1)
	}

	if strings.Contains(repo.RemoteURL, "github.com") && cfg.GitHub.Token == "" {
		token, authErr := runGitHubAuth()
		if authErr != nil {
			fmt.Fprintf(os.Stderr, "GitHub authentication failed: %v\n", authErr)
			os.Exit(1)
		}
		cfg.GitHub.Token = token
		if saveErr := config.Save(configPath, cfg); saveErr != nil {
			fmt.Fprintf(os.Stderr, "warning: could not save token to config: %v\n", saveErr)
		}
	} else if isGitLabRemote(repo.RemoteURL, cfg.GitLab.URL) && cfg.GitLab.Token == "" {
		token, authErr := runGitLabAuth(cfg.GitLab.URL)
		if authErr != nil {
			fmt.Fprintf(os.Stderr, "GitLab authentication failed: %v\n", authErr)
			os.Exit(1)
		}
		cfg.GitLab.Token = token
		if saveErr := config.Save(configPath, cfg); saveErr != nil {
			fmt.Fprintf(os.Stderr, "warning: could not save token to config: %v\n", saveErr)
		}
	}

	registry := provider.NewRegistry()
	registry.Register("github.com", githubprovider.NewAdapter(cfg.GitHub.Token, ""))

	gitLabURL := cfg.GitLab.URL
	registry.Register("gitlab.com", gitlabprovider.NewAdapter(cfg.GitLab.Token, gitLabURL))
	if gitLabURL != "" {
		registry.Register(gitLabURL, gitlabprovider.NewAdapter(cfg.GitLab.Token, gitLabURL))
	}

	ciProvider, err := registry.Detect(repo.RemoteURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error detecting CI provider: %v\n", err)
		os.Exit(1)
	}

	tui.Run(repo, ciProvider)
}

// isGitLabRemote returns true if the remote URL points to gitlab.com or the configured self-hosted URL.
func isGitLabRemote(remoteURL string, configuredURL string) bool {
	if strings.Contains(remoteURL, "gitlab.com") {
		return true
	}
	return configuredURL != "" && strings.Contains(remoteURL, configuredURL)
}

// runGitHubAuth runs the GitHub Device Authorization Flow interactively.
// It prints instructions to stdout and blocks until the user completes authorization.
func runGitHubAuth() (string, error) {
	flow := auth.NewDefaultGitHubDeviceFlow()
	code, err := flow.RequestCode()
	if err != nil {
		return "", fmt.Errorf("requesting device code: %w", err)
	}
	fmt.Printf("No GitHub token found. Starting OAuth authentication...\n")
	fmt.Printf("Visit:      %s\n", code.VerificationURI)
	fmt.Printf("Enter code: %s\n", code.UserCode)
	fmt.Printf("Waiting for authorization...\n")
	token, err := flow.PollToken(code.DeviceCode, code.Interval)
	if err != nil {
		return "", err
	}
	fmt.Printf("Authenticated ✓  Token saved to ~/.config/gitdeck/config.toml\n")
	return token, nil
}

// runGitLabAuth runs the GitLab Device Authorization Flow interactively.
// baseURL is the GitLab instance URL; pass empty string for gitlab.com.
func runGitLabAuth(baseURL string) (string, error) {
	flow := auth.NewDefaultGitLabDeviceFlow(baseURL)
	code, err := flow.RequestCode()
	if err != nil {
		return "", fmt.Errorf("requesting device code: %w", err)
	}
	fmt.Printf("No GitLab token found. Starting OAuth authentication...\n")
	fmt.Printf("Visit:      %s\n", code.VerificationURI)
	fmt.Printf("Enter code: %s\n", code.UserCode)
	fmt.Printf("Waiting for authorization...\n")
	token, err := flow.PollToken(code.DeviceCode, code.Interval)
	if err != nil {
		return "", err
	}
	fmt.Printf("Authenticated ✓  Token saved to ~/.config/gitdeck/config.toml\n")
	return token, nil
}
```

**Step 3: Verify it compiles**

```
go build ./...
```

Expected: no errors

**Step 4: Run all tests to make sure nothing regressed**

```
go test ./...
```

Expected: all tests PASS

**Step 5: Commit**

```bash
git add cmd/gitdeck/main.go
git commit -m "feat: auto-trigger OAuth device flow when no token is configured"
```

---

## Done

After completing all 4 tasks:

- `config.Save` persists tokens back to `~/.config/gitdeck/config.toml`
- `internal/auth` has fully tested GitHub and GitLab Device Flow implementations
- `main.go` auto-triggers the flow on first run when no token is present
- Existing manual token configuration (TOML + env vars) continues to work unchanged

**Final verification:**

```bash
go build ./...
go test ./...
```

Both should succeed with zero errors.
