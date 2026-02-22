package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/waabox/gitdeck/internal/config"
)

func TestLoad_FromFile(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")
	content := `
[github]
token = "ghp_testtoken"

[gitlab]
token = "glpat_testtoken"
url = "https://gitlab.example.com"
`
	if err := os.WriteFile(configPath, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.LoadFrom(configPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.GitHub.Token != "ghp_testtoken" {
		t.Errorf("expected GitHub token 'ghp_testtoken', got '%s'", cfg.GitHub.Token)
	}
	if cfg.GitLab.Token != "glpat_testtoken" {
		t.Errorf("expected GitLab token 'glpat_testtoken', got '%s'", cfg.GitLab.Token)
	}
	if cfg.GitLab.URL != "https://gitlab.example.com" {
		t.Errorf("expected GitLab URL 'https://gitlab.example.com', got '%s'", cfg.GitLab.URL)
	}
}

func TestLoad_EnvVarsTakePrecedence(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")
	content := `
[github]
token = "ghp_fromfile"
`
	if err := os.WriteFile(configPath, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	t.Setenv("GITHUB_TOKEN", "ghp_fromenv")
	t.Setenv("GITLAB_TOKEN", "glpat_fromenv")
	t.Setenv("GITLAB_URL", "https://gitlab.myco.com")

	cfg, err := config.LoadFrom(configPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.GitHub.Token != "ghp_fromenv" {
		t.Errorf("expected env token 'ghp_fromenv', got '%s'", cfg.GitHub.Token)
	}
	if cfg.GitLab.Token != "glpat_fromenv" {
		t.Errorf("expected env token 'glpat_fromenv', got '%s'", cfg.GitLab.Token)
	}
	if cfg.GitLab.URL != "https://gitlab.myco.com" {
		t.Errorf("expected env URL 'https://gitlab.myco.com', got '%s'", cfg.GitLab.URL)
	}
}

func TestLoad_MissingFileIsNotError(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "ghp_onlyenv")
	cfg, err := config.LoadFrom("/nonexistent/path/config.toml")
	if err != nil {
		t.Fatalf("missing file should not be an error, got: %v", err)
	}
	if cfg.GitHub.Token != "ghp_onlyenv" {
		t.Errorf("expected token from env, got '%s'", cfg.GitHub.Token)
	}
}
