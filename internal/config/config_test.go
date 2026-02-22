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
