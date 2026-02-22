package git_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/waabox/gitdeck/internal/git"
)

func TestParseRemoteURL_GitHub(t *testing.T) {
	url := "https://github.com/waabox/gitdeck.git"
	repo, err := git.ParseRemoteURL(url)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.Owner != "waabox" {
		t.Errorf("expected owner 'waabox', got '%s'", repo.Owner)
	}
	if repo.Name != "gitdeck" {
		t.Errorf("expected name 'gitdeck', got '%s'", repo.Name)
	}
	if repo.RemoteURL != url {
		t.Errorf("expected remoteURL '%s', got '%s'", url, repo.RemoteURL)
	}
}

func TestParseRemoteURL_GitHubSSH(t *testing.T) {
	url := "git@github.com:waabox/gitdeck.git"
	repo, err := git.ParseRemoteURL(url)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.Owner != "waabox" {
		t.Errorf("expected owner 'waabox', got '%s'", repo.Owner)
	}
	if repo.Name != "gitdeck" {
		t.Errorf("expected name 'gitdeck', got '%s'", repo.Name)
	}
}

func TestParseRemoteURL_GitLab(t *testing.T) {
	url := "https://gitlab.com/mygroup/myproject.git"
	repo, err := git.ParseRemoteURL(url)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.Owner != "mygroup" {
		t.Errorf("expected owner 'mygroup', got '%s'", repo.Owner)
	}
	if repo.Name != "myproject" {
		t.Errorf("expected name 'myproject', got '%s'", repo.Name)
	}
}

func TestParseRemoteURL_Invalid(t *testing.T) {
	_, err := git.ParseRemoteURL("not-a-url")
	if err == nil {
		t.Fatal("expected error for invalid URL, got nil")
	}
}

func TestDetectRepository_ReadsGitConfig(t *testing.T) {
	dir := t.TempDir()
	gitDir := filepath.Join(dir, ".git")
	if err := os.Mkdir(gitDir, 0755); err != nil {
		t.Fatal(err)
	}
	configContent := `[core]
	repositoryformatversion = 0
[remote "origin"]
	url = https://github.com/waabox/gitdeck.git
	fetch = +refs/heads/*:refs/remotes/origin/*
`
	if err := os.WriteFile(filepath.Join(gitDir, "config"), []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	repo, err := git.DetectRepository(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.Owner != "waabox" {
		t.Errorf("expected owner 'waabox', got '%s'", repo.Owner)
	}
	if repo.Name != "gitdeck" {
		t.Errorf("expected name 'gitdeck', got '%s'", repo.Name)
	}
}
