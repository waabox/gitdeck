package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// GitHubConfig holds authentication configuration for GitHub.
type GitHubConfig struct {
	ClientID string `toml:"client_id"`
	Token    string `toml:"token"`
}

// GitLabConfig holds authentication configuration for GitLab.
type GitLabConfig struct {
	ClientID     string `toml:"client_id"`
	Token        string `toml:"token"`
	RefreshToken string `toml:"refresh_token"`
	URL          string `toml:"url"`
}

// Config holds all gitdeck configuration.
type Config struct {
	GitHub         GitHubConfig `toml:"github"`
	GitLab         GitLabConfig `toml:"gitlab"`
	PipelineLimit  int          `toml:"pipeline_limit"`
}

const defaultPipelineLimit = 3

// PipelineLimitOrDefault returns PipelineLimit if set, otherwise defaultPipelineLimit.
func (c Config) PipelineLimitOrDefault() int {
	if c.PipelineLimit > 0 {
		return c.PipelineLimit
	}
	return defaultPipelineLimit
}

// LoadFrom reads configuration from the given TOML file path.
// If the file does not exist, it returns an empty config without error.
// Environment variables always take precedence over file values:
//   - GITHUB_TOKEN overrides github.token
//   - GITLAB_TOKEN overrides gitlab.token
//   - GITLAB_URL   overrides gitlab.url
func LoadFrom(path string) (Config, error) {
	var cfg Config
	if _, err := os.Stat(path); err == nil {
		if _, err := toml.DecodeFile(path, &cfg); err != nil {
			return Config{}, err
		}
	}
	applyEnvOverrides(&cfg)
	return cfg, nil
}

// DefaultConfigPath returns the default path for the gitdeck config file.
func DefaultConfigPath() string {
	home, _ := os.UserHomeDir()
	return home + "/.config/gitdeck/config.toml"
}

func applyEnvOverrides(cfg *Config) {
	if v := os.Getenv("GITHUB_TOKEN"); v != "" {
		cfg.GitHub.Token = v
	}
	if v := os.Getenv("GITLAB_TOKEN"); v != "" {
		cfg.GitLab.Token = v
	}
	if v := os.Getenv("GITLAB_URL"); v != "" {
		cfg.GitLab.URL = v
	}
}

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
	if encErr := toml.NewEncoder(f).Encode(cfg); encErr != nil {
		f.Close()
		return encErr
	}
	return f.Close()
}
