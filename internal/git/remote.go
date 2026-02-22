package git

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/waabox/gitdeck/internal/domain"
)

// DetectRepository reads the .git/config in the given directory and returns
// a Repository built from the origin remote URL.
func DetectRepository(dir string) (domain.Repository, error) {
	configPath := filepath.Join(dir, ".git", "config")
	f, err := os.Open(configPath)
	if err != nil {
		return domain.Repository{}, fmt.Errorf("could not open .git/config: %w", err)
	}
	defer f.Close()

	var inOrigin bool
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == `[remote "origin"]` {
			inOrigin = true
			continue
		}
		if inOrigin && strings.HasPrefix(line, "[") {
			break
		}
		if inOrigin && strings.HasPrefix(line, "url") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				return ParseRemoteURL(strings.TrimSpace(parts[1]))
			}
		}
	}
	return domain.Repository{}, errors.New("no origin remote found in .git/config")
}

// ParseRemoteURL parses a git remote URL and returns a Repository.
// Supports HTTPS (https://github.com/owner/repo.git) and SSH (git@github.com:owner/repo.git).
// The RemoteURL field in the returned Repository preserves the original input URL unchanged.
func ParseRemoteURL(rawURL string) (domain.Repository, error) {
	originalURL := rawURL
	normalized := strings.TrimSuffix(rawURL, ".git")

	// SSH format: git@github.com:owner/repo
	if strings.HasPrefix(normalized, "git@") {
		trimmed := strings.TrimPrefix(normalized, "git@")
		parts := strings.SplitN(trimmed, ":", 2)
		if len(parts) != 2 {
			return domain.Repository{}, fmt.Errorf("invalid SSH remote URL: %s", rawURL)
		}
		ownerRepo := strings.SplitN(parts[1], "/", 2)
		if len(ownerRepo) != 2 {
			return domain.Repository{}, fmt.Errorf("invalid SSH remote URL path: %s", parts[1])
		}
		return domain.Repository{
			Owner:     ownerRepo[0],
			Name:      ownerRepo[1],
			RemoteURL: originalURL,
		}, nil
	}

	// HTTPS format: https://github.com/owner/repo
	if strings.HasPrefix(normalized, "https://") || strings.HasPrefix(normalized, "http://") {
		withoutScheme := strings.TrimPrefix(normalized, "https://")
		withoutScheme = strings.TrimPrefix(withoutScheme, "http://")
		parts := strings.SplitN(withoutScheme, "/", 3)
		if len(parts) != 3 {
			return domain.Repository{}, fmt.Errorf("invalid HTTPS remote URL: %s", rawURL)
		}
		return domain.Repository{
			Owner:     parts[1],
			Name:      parts[2],
			RemoteURL: originalURL,
		}, nil
	}

	return domain.Repository{}, fmt.Errorf("unsupported remote URL format: %s", rawURL)
}
