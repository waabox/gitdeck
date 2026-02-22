package config

import (
	"os"

	"github.com/BurntSushi/toml"
)

// GitHubConfig holds authentication configuration for GitHub.
type GitHubConfig struct {
	Token string `toml:"token"`
}

// GitLabConfig holds authentication configuration for GitLab.
type GitLabConfig struct {
	Token string `toml:"token"`
	URL   string `toml:"url"`
}

// Config holds all gitdeck configuration.
type Config struct {
	GitHub GitHubConfig `toml:"github"`
	GitLab GitLabConfig `toml:"gitlab"`
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
