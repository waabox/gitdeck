package main

import (
	"fmt"
	"os"

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

	cfg, err := config.LoadFrom(config.DefaultConfigPath())
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading config: %v\n", err)
		os.Exit(1)
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
