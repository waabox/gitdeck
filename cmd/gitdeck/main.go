package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/waabox/gitdeck/internal/auth"
	"github.com/waabox/gitdeck/internal/config"
	"github.com/waabox/gitdeck/internal/git"
	"github.com/waabox/gitdeck/internal/provider"
	githubprovider "github.com/waabox/gitdeck/internal/provider/github"
	gitlabprovider "github.com/waabox/gitdeck/internal/provider/gitlab"
	"github.com/waabox/gitdeck/internal/tui"
)

// version is set at build time via -ldflags "-X main.version=x.y.z".
var version = "dev"

// defaultGitHubClientID is the Client ID of the gitdeck OAuth app registered on github.com.
// It is non-confidential (no secret required) so it is safe to distribute with the binary.
// Users can override it by setting github.client_id in ~/.config/gitdeck/config.toml.
const defaultGitHubClientID = "Ov23liw1KWtnqgtO7qvT"

// defaultGitLabClientID is the Application ID of the gitdeck OAuth app registered on gitlab.com.
// It is non-confidential (no secret required) so it is safe to distribute with the binary.
// Users can override it by setting gitlab.client_id in ~/.config/gitdeck/config.toml.
const defaultGitLabClientID = "9df6c8abe93dc879a79ecf7681909b4a37d5c61064190a795bbf16e1ed8bffa3"

func main() {
	versionFlag := flag.Bool("version", false, "print version and exit")
	flag.Parse()
	if *versionFlag {
		fmt.Println("gitdeck", version)
		os.Exit(0)
	}

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

	ctx := context.Background()

	if strings.Contains(repo.RemoteURL, "github.com") && cfg.GitHub.Token == "" {
		resp, authErr := runGitHubAuth(ctx, cfg.GitHub.ClientID)
		if authErr != nil {
			fmt.Fprintf(os.Stderr, "GitHub authentication failed: %v\n", authErr)
			os.Exit(1)
		}
		cfg.GitHub.Token = resp.AccessToken
		if saveErr := config.Save(configPath, cfg); saveErr != nil {
			fmt.Fprintf(os.Stderr, "warning: could not save token to config: %v (you will need to re-authenticate next run)\n", saveErr)
		} else {
			fmt.Fprintf(os.Stderr, "Authenticated. Token saved to %s\n", configPath)
		}
	} else if isGitLabRemote(repo.RemoteURL, cfg.GitLab.URL) && cfg.GitLab.Token == "" {
		resp, authErr := runGitLabAuth(ctx, cfg.GitLab.ClientID, cfg.GitLab.URL)
		if authErr != nil {
			fmt.Fprintf(os.Stderr, "GitLab authentication failed: %v\n", authErr)
			os.Exit(1)
		}
		cfg.GitLab.Token = resp.AccessToken
		cfg.GitLab.RefreshToken = resp.RefreshToken
		if saveErr := config.Save(configPath, cfg); saveErr != nil {
			fmt.Fprintf(os.Stderr, "warning: could not save token to config: %v (you will need to re-authenticate next run)\n", saveErr)
		} else {
			fmt.Fprintf(os.Stderr, "Authenticated. Token saved to %s\n", configPath)
		}
	}

	limit := cfg.PipelineLimitOrDefault()
	gitLabURL := cfg.GitLab.URL

	// Create adapters
	githubAdapter := githubprovider.NewAdapter(cfg.GitHub.Token, "", limit)
	gitlabAdapter := gitlabprovider.NewAdapter(cfg.GitLab.Token, gitLabURL, limit)

	// Create token manager for silent refresh
	tokenManager := auth.NewTokenManager(&cfg, configPath, gitLabURL)

	// Wrap with refreshing logic
	githubProvider := provider.NewRefreshingProvider(
		githubAdapter, "github",
		func() (string, error) { return "", fmt.Errorf("GitHub OAuth tokens cannot be refreshed") },
		func(token string) { githubAdapter.SetToken(token) },
	)
	gitlabProvider := provider.NewRefreshingProvider(
		gitlabAdapter, "gitlab",
		func() (string, error) { return tokenManager.RefreshGitLab(context.Background()) },
		func(token string) { gitlabAdapter.SetToken(token) },
	)

	registry := provider.NewRegistry()
	registry.Register("github.com", githubProvider)
	registry.Register("gitlab.com", gitlabProvider)
	if gitLabURL != "" {
		registry.Register(gitLabURL, gitlabProvider)
	}

	ciProvider, err := registry.Detect(repo.RemoteURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error detecting CI provider: %v\n", err)
		os.Exit(1)
	}

	app := tui.NewAppModel(repo, ciProvider)
	app.OnRequestCode = func(ctx context.Context, providerName string) (auth.DeviceCodeResponse, error) {
		var clientID string
		var baseURL string
		switch providerName {
		case "gitlab":
			clientID = cfg.GitLab.ClientID
			if clientID == "" {
				clientID = defaultGitLabClientID
			}
			baseURL = cfg.GitLab.URL
			flow := auth.NewGitLabDeviceFlow(clientID, baseURL)
			return flow.RequestCode(ctx)
		case "github":
			clientID = cfg.GitHub.ClientID
			if clientID == "" {
				clientID = defaultGitHubClientID
			}
			flow := auth.NewGitHubDeviceFlow(clientID, "")
			return flow.RequestCode(ctx)
		}
		return auth.DeviceCodeResponse{}, fmt.Errorf("unknown provider: %s", providerName)
	}
	app.OnPollToken = func(ctx context.Context, providerName string, deviceCode string, interval int) (auth.TokenResponse, error) {
		var clientID string
		switch providerName {
		case "gitlab":
			clientID = cfg.GitLab.ClientID
			if clientID == "" {
				clientID = defaultGitLabClientID
			}
			flow := auth.NewGitLabDeviceFlow(clientID, cfg.GitLab.URL)
			return flow.PollToken(ctx, deviceCode, interval)
		case "github":
			clientID = cfg.GitHub.ClientID
			if clientID == "" {
				clientID = defaultGitHubClientID
			}
			flow := auth.NewGitHubDeviceFlow(clientID, "")
			return flow.PollToken(ctx, deviceCode, interval)
		}
		return auth.TokenResponse{}, fmt.Errorf("unknown provider: %s", providerName)
	}
	app.OnTokenRefreshed = func(providerName string, resp auth.TokenResponse) {
		switch providerName {
		case "gitlab":
			cfg.GitLab.Token = resp.AccessToken
			cfg.GitLab.RefreshToken = resp.RefreshToken
			gitlabAdapter.SetToken(resp.AccessToken)
		case "github":
			cfg.GitHub.Token = resp.AccessToken
			githubAdapter.SetToken(resp.AccessToken)
		}
		config.Save(configPath, cfg)
	}

	p := tea.NewProgram(app, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "gitdeck error: %v\n", err)
		os.Exit(1)
	}
}

// isGitLabRemote returns true if the remote URL points to gitlab.com or the configured self-hosted URL.
func isGitLabRemote(remoteURL string, configuredURL string) bool {
	if strings.Contains(remoteURL, "gitlab.com") {
		return true
	}
	return configuredURL != "" && strings.Contains(remoteURL, configuredURL)
}

// runGitHubAuth runs the GitHub Device Authorization Flow interactively.
// All prompts are written to stderr so stdout remains clean for piping.
// It blocks until the user completes authorization or an error occurs.
func runGitHubAuth(ctx context.Context, clientID string) (auth.TokenResponse, error) {
	if clientID == "" {
		clientID = defaultGitHubClientID
	}
	flow := auth.NewGitHubDeviceFlow(clientID, "")
	code, err := flow.RequestCode(ctx)
	if err != nil {
		return auth.TokenResponse{}, fmt.Errorf("requesting device code: %w", err)
	}
	fmt.Fprintf(os.Stderr, "No GitHub token found. Starting OAuth authentication...\n")
	fmt.Fprintf(os.Stderr, "Visit:      %s\n", code.VerificationURI)
	fmt.Fprintf(os.Stderr, "Enter code: %s\n", code.UserCode)
	fmt.Fprintf(os.Stderr, "Waiting for authorization...\n")
	codeCtx, cancel := context.WithTimeout(ctx, time.Duration(code.ExpiresIn)*time.Second)
	defer cancel()
	return flow.PollToken(codeCtx, code.DeviceCode, code.Interval)
}

// runGitLabAuth runs the GitLab Device Authorization Flow interactively.
// All prompts are written to stderr so stdout remains clean for piping.
// baseURL is the GitLab instance base URL; pass empty string for gitlab.com.
func runGitLabAuth(ctx context.Context, clientID string, baseURL string) (auth.TokenResponse, error) {
	if clientID == "" {
		clientID = defaultGitLabClientID
	}
	flow := auth.NewGitLabDeviceFlow(clientID, baseURL)
	code, err := flow.RequestCode(ctx)
	if err != nil {
		return auth.TokenResponse{}, fmt.Errorf("requesting device code: %w", err)
	}
	fmt.Fprintf(os.Stderr, "No GitLab token found. Starting OAuth authentication...\n")
	fmt.Fprintf(os.Stderr, "Visit:      %s\n", code.VerificationURI)
	fmt.Fprintf(os.Stderr, "Enter code: %s\n", code.UserCode)
	fmt.Fprintf(os.Stderr, "Waiting for authorization...\n")
	codeCtx, cancel := context.WithTimeout(ctx, time.Duration(code.ExpiresIn)*time.Second)
	defer cancel()
	return flow.PollToken(codeCtx, code.DeviceCode, code.Interval)
}
