package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

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
		token, authErr := runGitHubAuth(ctx, cfg.GitHub.ClientID)
		if authErr != nil {
			fmt.Fprintf(os.Stderr, "GitHub authentication failed: %v\n", authErr)
			os.Exit(1)
		}
		cfg.GitHub.Token = token
		if saveErr := config.Save(configPath, cfg); saveErr != nil {
			fmt.Fprintf(os.Stderr, "warning: could not save token to config: %v (you will need to re-authenticate next run)\n", saveErr)
		} else {
			fmt.Fprintf(os.Stderr, "Authenticated. Token saved to %s\n", configPath)
		}
	} else if isGitLabRemote(repo.RemoteURL, cfg.GitLab.URL) && cfg.GitLab.Token == "" {
		token, authErr := runGitLabAuth(ctx, cfg.GitLab.ClientID, cfg.GitLab.URL)
		if authErr != nil {
			fmt.Fprintf(os.Stderr, "GitLab authentication failed: %v\n", authErr)
			os.Exit(1)
		}
		cfg.GitLab.Token = token
		if saveErr := config.Save(configPath, cfg); saveErr != nil {
			fmt.Fprintf(os.Stderr, "warning: could not save token to config: %v (you will need to re-authenticate next run)\n", saveErr)
		} else {
			fmt.Fprintf(os.Stderr, "Authenticated. Token saved to %s\n", configPath)
		}
	}

	limit := cfg.PipelineLimitOrDefault()

	registry := provider.NewRegistry()
	registry.Register("github.com", githubprovider.NewAdapter(cfg.GitHub.Token, "", limit))

	gitLabURL := cfg.GitLab.URL
	registry.Register("gitlab.com", gitlabprovider.NewAdapter(cfg.GitLab.Token, gitLabURL, limit))
	if gitLabURL != "" {
		registry.Register(gitLabURL, gitlabprovider.NewAdapter(cfg.GitLab.Token, gitLabURL, limit))
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
// All prompts are written to stderr so stdout remains clean for piping.
// It blocks until the user completes authorization or an error occurs.
func runGitHubAuth(ctx context.Context, clientID string) (string, error) {
	if clientID == "" {
		return "", fmt.Errorf("github.client_id is not set in config — add it to ~/.config/gitdeck/config.toml")
	}
	flow := auth.NewGitHubDeviceFlow(clientID, "")
	code, err := flow.RequestCode(ctx)
	if err != nil {
		return "", fmt.Errorf("requesting device code: %w", err)
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
func runGitLabAuth(ctx context.Context, clientID string, baseURL string) (string, error) {
	if clientID == "" {
		clientID = defaultGitLabClientID
	}
	flow := auth.NewGitLabDeviceFlow(clientID, baseURL)
	code, err := flow.RequestCode(ctx)
	if err != nil {
		return "", fmt.Errorf("requesting device code: %w", err)
	}
	fmt.Fprintf(os.Stderr, "No GitLab token found. Starting OAuth authentication...\n")
	fmt.Fprintf(os.Stderr, "Visit:      %s\n", code.VerificationURI)
	fmt.Fprintf(os.Stderr, "Enter code: %s\n", code.UserCode)
	fmt.Fprintf(os.Stderr, "Waiting for authorization...\n")
	codeCtx, cancel := context.WithTimeout(ctx, time.Duration(code.ExpiresIn)*time.Second)
	defer cancel()
	return flow.PollToken(codeCtx, code.DeviceCode, code.Interval)
}
