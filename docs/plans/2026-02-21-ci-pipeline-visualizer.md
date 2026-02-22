# CI Pipeline Visualizer Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build a lazygit-style TUI that reads the current directory's `.git` remote and visualizes CI pipeline status from GitHub Actions or GitLab CI.

**Architecture:** DDD layers with a Provider Registry pattern. The domain defines `Pipeline`, `Job`, `Repository`, and the `PipelineProvider` port interface. Adapters for GitHub and GitLab implement that interface. A registry maps remote URLs to the correct adapter at startup.

**Tech Stack:** Go 1.22+, Bubbletea (TUI), Lipgloss (styling), Bubbles (list component), BurntSushi/toml (config), standard `net/http` (API calls).

---

### Task 1: Project scaffold

**Files:**
- Create: `go.mod`
- Create: `cmd/gitdeck/main.go`
- Create: `internal/domain/pipeline.go`
- Create: `internal/domain/repository.go`
- Create: `internal/domain/provider.go`

**Step 1: Initialize Go module**

```bash
cd /Users/waabox/code/waabox/gitdeck
go mod init github.com/waabox/gitdeck
```

Expected: `go.mod` created with `module github.com/waabox/gitdeck` and `go 1.22`

**Step 2: Install dependencies**

```bash
go get github.com/charmbracelet/bubbletea@latest
go get github.com/charmbracelet/lipgloss@latest
go get github.com/charmbracelet/bubbles@latest
go get github.com/BurntSushi/toml@latest
```

**Step 3: Create domain types**

Create `internal/domain/pipeline.go`:

```go
package domain

import "time"

// PipelineStatus represents the execution state of a pipeline or job.
type PipelineStatus string

const (
	StatusPending   PipelineStatus = "pending"
	StatusRunning   PipelineStatus = "running"
	StatusSuccess   PipelineStatus = "success"
	StatusFailed    PipelineStatus = "failed"
	StatusCancelled PipelineStatus = "cancelled"
)

// Job represents a single unit of work within a pipeline.
type Job struct {
	ID        string
	Name      string
	Stage     string
	Status    PipelineStatus
	Duration  time.Duration
	StartedAt time.Time
}

// Pipeline represents a CI pipeline run.
type Pipeline struct {
	ID        string
	Branch    string
	CommitSHA string
	CommitMsg string
	Author    string
	Status    PipelineStatus
	CreatedAt time.Time
	Duration  time.Duration
	Jobs      []Job
}
```

Create `internal/domain/repository.go`:

```go
package domain

// Repository represents the git repository being observed.
type Repository struct {
	Owner     string
	Name      string
	RemoteURL string
}
```

Create `internal/domain/provider.go`:

```go
package domain

// PipelineProvider is the port interface that all CI provider adapters must implement.
// The domain does not know about GitHub, GitLab, or any specific CI system.
type PipelineProvider interface {
	ListPipelines(repo Repository) ([]Pipeline, error)
	GetPipeline(repo Repository, id string) (Pipeline, error)
}
```

**Step 4: Create minimal main.go**

Create `cmd/gitdeck/main.go`:

```go
package main

import "fmt"

func main() {
	fmt.Println("gitdeck starting...")
}
```

**Step 5: Verify it compiles**

```bash
go build ./...
```

Expected: no errors.

**Step 6: Commit**

```bash
git add .
git commit -m "feat: project scaffold with domain model"
```

---

### Task 2: Git remote detection

**Files:**
- Create: `internal/git/remote.go`
- Create: `internal/git/remote_test.go`

**Step 1: Write the failing tests**

Create `internal/git/remote_test.go`:

```go
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
	// Create a temporary fake .git directory
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
```

**Step 2: Run tests to verify they fail**

```bash
go test ./internal/git/...
```

Expected: FAIL — package does not exist yet.

**Step 3: Implement remote detection**

Create `internal/git/remote.go`:

```go
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
func ParseRemoteURL(rawURL string) (domain.Repository, error) {
	rawURL = strings.TrimSuffix(rawURL, ".git")

	// SSH format: git@github.com:owner/repo
	if strings.HasPrefix(rawURL, "git@") {
		rawURL = strings.TrimPrefix(rawURL, "git@")
		parts := strings.SplitN(rawURL, ":", 2)
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
			RemoteURL: "git@" + parts[0] + ":" + parts[1],
		}, nil
	}

	// HTTPS format: https://github.com/owner/repo
	if strings.HasPrefix(rawURL, "https://") || strings.HasPrefix(rawURL, "http://") {
		rawURL = strings.TrimPrefix(rawURL, "https://")
		rawURL = strings.TrimPrefix(rawURL, "http://")
		parts := strings.SplitN(rawURL, "/", 3)
		if len(parts) != 3 {
			return domain.Repository{}, fmt.Errorf("invalid HTTPS remote URL: %s", rawURL)
		}
		originalURL := "https://" + strings.Join(parts, "/")
		return domain.Repository{
			Owner:     parts[1],
			Name:      parts[2],
			RemoteURL: originalURL,
		}, nil
	}

	return domain.Repository{}, fmt.Errorf("unsupported remote URL format: %s", rawURL)
}
```

**Step 4: Run tests to verify they pass**

```bash
go test ./internal/git/... -v
```

Expected: all tests PASS.

**Step 5: Commit**

```bash
git add internal/git/
git commit -m "feat: git remote detection with HTTPS and SSH support"
```

---

### Task 3: Config loader

**Files:**
- Create: `internal/config/config.go`
- Create: `internal/config/config_test.go`

**Step 1: Write the failing tests**

Create `internal/config/config_test.go`:

```go
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
```

**Step 2: Run tests to verify they fail**

```bash
go test ./internal/config/...
```

Expected: FAIL — package does not exist yet.

**Step 3: Implement config loader**

Create `internal/config/config.go`:

```go
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
```

**Step 4: Run tests to verify they pass**

```bash
go test ./internal/config/... -v
```

Expected: all tests PASS.

**Step 5: Commit**

```bash
git add internal/config/
git commit -m "feat: config loader with TOML file and env var support"
```

---

### Task 4: Provider Registry

**Files:**
- Create: `internal/provider/registry.go`
- Create: `internal/provider/registry_test.go`

**Step 1: Write the failing tests**

Create `internal/provider/registry_test.go`:

```go
package provider_test

import (
	"testing"

	"github.com/waabox/gitdeck/internal/domain"
	"github.com/waabox/gitdeck/internal/provider"
)

type fakeProvider struct{ name string }

func (f *fakeProvider) ListPipelines(_ domain.Repository) ([]domain.Pipeline, error) {
	return nil, nil
}
func (f *fakeProvider) GetPipeline(_ domain.Repository, _ string) (domain.Pipeline, error) {
	return domain.Pipeline{}, nil
}

func TestRegistry_DetectsGitHub(t *testing.T) {
	gh := &fakeProvider{name: "github"}
	gl := &fakeProvider{name: "gitlab"}

	reg := provider.NewRegistry()
	reg.Register("github.com", gh)
	reg.Register("gitlab.com", gl)

	p, err := reg.Detect("https://github.com/waabox/gitdeck.git")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p != gh {
		t.Error("expected github provider to be detected")
	}
}

func TestRegistry_DetectsGitLab(t *testing.T) {
	gh := &fakeProvider{name: "github"}
	gl := &fakeProvider{name: "gitlab"}

	reg := provider.NewRegistry()
	reg.Register("github.com", gh)
	reg.Register("gitlab.com", gl)

	p, err := reg.Detect("https://gitlab.com/mygroup/myproject.git")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p != gl {
		t.Error("expected gitlab provider to be detected")
	}
}

func TestRegistry_DetectsSelfHostedGitLab(t *testing.T) {
	gl := &fakeProvider{name: "gitlab-self"}

	reg := provider.NewRegistry()
	reg.Register("gitlab.mycompany.com", gl)

	p, err := reg.Detect("https://gitlab.mycompany.com/team/project.git")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p != gl {
		t.Error("expected self-hosted gitlab provider to be detected")
	}
}

func TestRegistry_ErrorOnUnknownHost(t *testing.T) {
	reg := provider.NewRegistry()

	_, err := reg.Detect("https://bitbucket.org/user/repo.git")
	if err == nil {
		t.Fatal("expected error for unknown host, got nil")
	}
}
```

**Step 2: Run tests to verify they fail**

```bash
go test ./internal/provider/...
```

Expected: FAIL — package does not exist yet.

**Step 3: Implement the registry**

Create `internal/provider/registry.go`:

```go
package provider

import (
	"fmt"
	"strings"

	"github.com/waabox/gitdeck/internal/domain"
)

// Registry maps remote URL host patterns to PipelineProvider implementations.
type Registry struct {
	entries []entry
}

type entry struct {
	host     string
	provider domain.PipelineProvider
}

// NewRegistry creates an empty provider registry.
func NewRegistry() *Registry {
	return &Registry{}
}

// Register associates a host pattern (e.g., "github.com") with a provider.
func (r *Registry) Register(host string, p domain.PipelineProvider) {
	r.entries = append(r.entries, entry{host: host, provider: p})
}

// Detect returns the provider matching the host in the given remote URL.
// Returns an error if no matching provider is registered.
func (r *Registry) Detect(remoteURL string) (domain.PipelineProvider, error) {
	for _, e := range r.entries {
		if strings.Contains(remoteURL, e.host) {
			return e.provider, nil
		}
	}
	return nil, fmt.Errorf("no provider found for remote: %s", remoteURL)
}
```

**Step 4: Run tests to verify they pass**

```bash
go test ./internal/provider/... -v
```

Expected: all tests PASS.

**Step 5: Commit**

```bash
git add internal/provider/
git commit -m "feat: provider registry with host-based detection"
```

---

### Task 5: GitHub Actions adapter

**Files:**
- Create: `internal/provider/github/adapter.go`
- Create: `internal/provider/github/adapter_test.go`

**Step 1: Write the failing tests**

Create `internal/provider/github/adapter_test.go`:

```go
package github_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/waabox/gitdeck/internal/domain"
	githubprovider "github.com/waabox/gitdeck/internal/provider/github"
)

func TestListPipelines_ReturnsWorkflowRuns(t *testing.T) {
	response := map[string]interface{}{
		"workflow_runs": []map[string]interface{}{
			{
				"id":         float64(1001),
				"head_branch": "main",
				"head_sha":   "abc1234",
				"head_commit": map[string]interface{}{
					"message": "fix: login timeout",
					"author":  map[string]interface{}{"name": "waabox"},
				},
				"status":     "completed",
				"conclusion": "success",
				"created_at": time.Now().Add(-2 * time.Minute).Format(time.RFC3339),
				"updated_at": time.Now().Format(time.RFC3339),
			},
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/repos/waabox/gitdeck/actions/runs" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	adapter := githubprovider.NewAdapter("test-token", srv.URL)
	repo := domain.Repository{Owner: "waabox", Name: "gitdeck"}

	pipelines, err := adapter.ListPipelines(repo)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pipelines) != 1 {
		t.Fatalf("expected 1 pipeline, got %d", len(pipelines))
	}
	p := pipelines[0]
	if p.ID != "1001" {
		t.Errorf("expected ID '1001', got '%s'", p.ID)
	}
	if p.Branch != "main" {
		t.Errorf("expected branch 'main', got '%s'", p.Branch)
	}
	if p.Status != domain.StatusSuccess {
		t.Errorf("expected status success, got '%s'", p.Status)
	}
}

func TestGetPipeline_ReturnsRunWithJobs(t *testing.T) {
	runResponse := map[string]interface{}{
		"id":         float64(1001),
		"head_branch": "main",
		"head_sha":   "abc1234",
		"head_commit": map[string]interface{}{
			"message": "fix: login timeout",
			"author":  map[string]interface{}{"name": "waabox"},
		},
		"status":     "completed",
		"conclusion": "failure",
		"created_at": time.Now().Add(-5 * time.Minute).Format(time.RFC3339),
		"updated_at": time.Now().Format(time.RFC3339),
	}
	jobsResponse := map[string]interface{}{
		"jobs": []map[string]interface{}{
			{
				"id":     float64(2001),
				"name":   "build",
				"status": "completed",
				"conclusion": "success",
				"started_at":   time.Now().Add(-4 * time.Minute).Format(time.RFC3339),
				"completed_at": time.Now().Add(-3 * time.Minute).Format(time.RFC3339),
			},
			{
				"id":     float64(2002),
				"name":   "test",
				"status": "completed",
				"conclusion": "failure",
				"started_at":   time.Now().Add(-3 * time.Minute).Format(time.RFC3339),
				"completed_at": time.Now().Format(time.RFC3339),
			},
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/repos/waabox/gitdeck/actions/runs/1001":
			json.NewEncoder(w).Encode(runResponse)
		case "/repos/waabox/gitdeck/actions/runs/1001/jobs":
			json.NewEncoder(w).Encode(jobsResponse)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	adapter := githubprovider.NewAdapter("test-token", srv.URL)
	repo := domain.Repository{Owner: "waabox", Name: "gitdeck"}

	pipeline, err := adapter.GetPipeline(repo, "1001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pipeline.Status != domain.StatusFailed {
		t.Errorf("expected status failed, got '%s'", pipeline.Status)
	}
	if len(pipeline.Jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(pipeline.Jobs))
	}
	if pipeline.Jobs[0].Name != "build" {
		t.Errorf("expected first job 'build', got '%s'", pipeline.Jobs[0].Name)
	}
}
```

**Step 2: Run tests to verify they fail**

```bash
go test ./internal/provider/github/...
```

Expected: FAIL — package does not exist yet.

**Step 3: Implement the GitHub adapter**

Create `internal/provider/github/adapter.go`:

```go
package github

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/waabox/gitdeck/internal/domain"
)

const defaultBaseURL = "https://api.github.com"

// Adapter implements domain.PipelineProvider for GitHub Actions.
type Adapter struct {
	token   string
	baseURL string
	client  *http.Client
}

// NewAdapter creates a GitHub Actions adapter.
// baseURL is used for testing; pass empty string to use the real GitHub API.
func NewAdapter(token string, baseURL string) *Adapter {
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	return &Adapter{
		token:   token,
		baseURL: baseURL,
		client:  &http.Client{Timeout: 15 * time.Second},
	}
}

// ListPipelines returns the most recent workflow runs for the repository.
func (a *Adapter) ListPipelines(repo domain.Repository) ([]domain.Pipeline, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/actions/runs", a.baseURL, repo.Owner, repo.Name)
	var result struct {
		WorkflowRuns []workflowRun `json:"workflow_runs"`
	}
	if err := a.get(url, &result); err != nil {
		return nil, err
	}
	pipelines := make([]domain.Pipeline, len(result.WorkflowRuns))
	for i, run := range result.WorkflowRuns {
		pipelines[i] = run.toPipeline()
	}
	return pipelines, nil
}

// GetPipeline returns a single workflow run with all its jobs.
func (a *Adapter) GetPipeline(repo domain.Repository, id string) (domain.Pipeline, error) {
	runURL := fmt.Sprintf("%s/repos/%s/%s/actions/runs/%s", a.baseURL, repo.Owner, repo.Name, id)
	var run workflowRun
	if err := a.get(runURL, &run); err != nil {
		return domain.Pipeline{}, err
	}

	jobsURL := fmt.Sprintf("%s/repos/%s/%s/actions/runs/%s/jobs", a.baseURL, repo.Owner, repo.Name, id)
	var jobsResult struct {
		Jobs []workflowJob `json:"jobs"`
	}
	if err := a.get(jobsURL, &jobsResult); err != nil {
		return domain.Pipeline{}, err
	}

	pipeline := run.toPipeline()
	pipeline.Jobs = make([]domain.Job, len(jobsResult.Jobs))
	for i, j := range jobsResult.Jobs {
		pipeline.Jobs[i] = j.toJob()
	}
	return pipeline, nil
}

func (a *Adapter) get(url string, target interface{}) error {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+a.token)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := a.client.Do(req)
	if err != nil {
		return fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("github API error: %s", resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(target)
}

// workflowRun is the raw GitHub API response shape for a workflow run.
type workflowRun struct {
	ID         int64  `json:"id"`
	HeadBranch string `json:"head_branch"`
	HeadSHA    string `json:"head_sha"`
	HeadCommit struct {
		Message string `json:"message"`
		Author  struct {
			Name string `json:"name"`
		} `json:"author"`
	} `json:"head_commit"`
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
}

func (r workflowRun) toPipeline() domain.Pipeline {
	created, _ := time.Parse(time.RFC3339, r.CreatedAt)
	updated, _ := time.Parse(time.RFC3339, r.UpdatedAt)
	var duration time.Duration
	if !created.IsZero() && !updated.IsZero() {
		duration = updated.Sub(created)
	}
	return domain.Pipeline{
		ID:        strconv.FormatInt(r.ID, 10),
		Branch:    r.HeadBranch,
		CommitSHA: r.HeadSHA,
		CommitMsg: r.HeadCommit.Message,
		Author:    r.HeadCommit.Author.Name,
		Status:    mapGitHubStatus(r.Status, r.Conclusion),
		CreatedAt: created,
		Duration:  duration,
	}
}

// workflowJob is the raw GitHub API response shape for a job.
type workflowJob struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Status      string `json:"status"`
	Conclusion  string `json:"conclusion"`
	StartedAt   string `json:"started_at"`
	CompletedAt string `json:"completed_at"`
}

func (j workflowJob) toJob() domain.Job {
	started, _ := time.Parse(time.RFC3339, j.StartedAt)
	completed, _ := time.Parse(time.RFC3339, j.CompletedAt)
	var duration time.Duration
	if !started.IsZero() && !completed.IsZero() {
		duration = completed.Sub(started)
	}
	return domain.Job{
		ID:        strconv.FormatInt(j.ID, 10),
		Name:      j.Name,
		Status:    mapGitHubStatus(j.Status, j.Conclusion),
		StartedAt: started,
		Duration:  duration,
	}
}

func mapGitHubStatus(status, conclusion string) domain.PipelineStatus {
	if status == "in_progress" || status == "queued" || status == "waiting" {
		return domain.StatusRunning
	}
	if status == "completed" {
		switch conclusion {
		case "success":
			return domain.StatusSuccess
		case "failure", "timed_out":
			return domain.StatusFailed
		case "cancelled":
			return domain.StatusCancelled
		}
	}
	return domain.StatusPending
}
```

**Step 4: Run tests to verify they pass**

```bash
go test ./internal/provider/github/... -v
```

Expected: all tests PASS.

**Step 5: Commit**

```bash
git add internal/provider/github/
git commit -m "feat: GitHub Actions adapter"
```

---

### Task 6: GitLab CI adapter

**Files:**
- Create: `internal/provider/gitlab/adapter.go`
- Create: `internal/provider/gitlab/adapter_test.go`

**Step 1: Write the failing tests**

Create `internal/provider/gitlab/adapter_test.go`:

```go
package gitlab_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/waabox/gitdeck/internal/domain"
	gitlabprovider "github.com/waabox/gitdeck/internal/provider/gitlab"
)

func TestListPipelines_ReturnsPipelines(t *testing.T) {
	response := []map[string]interface{}{
		{
			"id":         float64(201),
			"ref":        "main",
			"sha":        "def5678",
			"status":     "success",
			"created_at": time.Now().Add(-1 * time.Hour).Format(time.RFC3339),
			"updated_at": time.Now().Add(-55 * time.Minute).Format(time.RFC3339),
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v4/projects/mygroup%2Fmyproject/pipelines" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	adapter := gitlabprovider.NewAdapter("test-token", srv.URL)
	repo := domain.Repository{Owner: "mygroup", Name: "myproject"}

	pipelines, err := adapter.ListPipelines(repo)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pipelines) != 1 {
		t.Fatalf("expected 1 pipeline, got %d", len(pipelines))
	}
	if pipelines[0].ID != "201" {
		t.Errorf("expected ID '201', got '%s'", pipelines[0].ID)
	}
	if pipelines[0].Status != domain.StatusSuccess {
		t.Errorf("expected status success, got '%s'", pipelines[0].Status)
	}
}

func TestGetPipeline_ReturnsPipelineWithJobs(t *testing.T) {
	pipelineResponse := map[string]interface{}{
		"id":         float64(201),
		"ref":        "main",
		"sha":        "def5678",
		"status":     "failed",
		"created_at": time.Now().Add(-10 * time.Minute).Format(time.RFC3339),
		"updated_at": time.Now().Format(time.RFC3339),
	}
	jobsResponse := []map[string]interface{}{
		{
			"id":         float64(301),
			"name":       "build",
			"stage":      "build",
			"status":     "success",
			"started_at": time.Now().Add(-9 * time.Minute).Format(time.RFC3339),
			"finished_at": time.Now().Add(-7 * time.Minute).Format(time.RFC3339),
		},
		{
			"id":         float64(302),
			"name":       "test",
			"stage":      "test",
			"status":     "failed",
			"started_at": time.Now().Add(-7 * time.Minute).Format(time.RFC3339),
			"finished_at": time.Now().Format(time.RFC3339),
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v4/projects/mygroup%2Fmyproject/pipelines/201":
			json.NewEncoder(w).Encode(pipelineResponse)
		case "/api/v4/projects/mygroup%2Fmyproject/pipelines/201/jobs":
			json.NewEncoder(w).Encode(jobsResponse)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	adapter := gitlabprovider.NewAdapter("test-token", srv.URL)
	repo := domain.Repository{Owner: "mygroup", Name: "myproject"}

	pipeline, err := adapter.GetPipeline(repo, "201")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pipeline.Status != domain.StatusFailed {
		t.Errorf("expected status failed, got '%s'", pipeline.Status)
	}
	if len(pipeline.Jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(pipeline.Jobs))
	}
	if pipeline.Jobs[1].Stage != "test" {
		t.Errorf("expected second job stage 'test', got '%s'", pipeline.Jobs[1].Stage)
	}
}
```

**Step 2: Run tests to verify they fail**

```bash
go test ./internal/provider/gitlab/...
```

Expected: FAIL — package does not exist yet.

**Step 3: Implement the GitLab adapter**

Create `internal/provider/gitlab/adapter.go`:

```go
package gitlab

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/waabox/gitdeck/internal/domain"
)

const defaultBaseURL = "https://gitlab.com"

// Adapter implements domain.PipelineProvider for GitLab CI.
type Adapter struct {
	token   string
	baseURL string
	client  *http.Client
}

// NewAdapter creates a GitLab CI adapter.
// baseURL can be a self-hosted GitLab instance URL; pass empty string for gitlab.com.
func NewAdapter(token string, baseURL string) *Adapter {
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	return &Adapter{
		token:   token,
		baseURL: baseURL,
		client:  &http.Client{Timeout: 15 * time.Second},
	}
}

// ListPipelines returns the most recent pipelines for the repository.
func (a *Adapter) ListPipelines(repo domain.Repository) ([]domain.Pipeline, error) {
	projectID := url.PathEscape(repo.Owner + "/" + repo.Name)
	apiURL := fmt.Sprintf("%s/api/v4/projects/%s/pipelines", a.baseURL, projectID)
	var runs []gitLabPipeline
	if err := a.get(apiURL, &runs); err != nil {
		return nil, err
	}
	pipelines := make([]domain.Pipeline, len(runs))
	for i, r := range runs {
		pipelines[i] = r.toPipeline()
	}
	return pipelines, nil
}

// GetPipeline returns a single pipeline with all its jobs.
func (a *Adapter) GetPipeline(repo domain.Repository, id string) (domain.Pipeline, error) {
	projectID := url.PathEscape(repo.Owner + "/" + repo.Name)

	pipelineURL := fmt.Sprintf("%s/api/v4/projects/%s/pipelines/%s", a.baseURL, projectID, id)
	var run gitLabPipeline
	if err := a.get(pipelineURL, &run); err != nil {
		return domain.Pipeline{}, err
	}

	jobsURL := fmt.Sprintf("%s/api/v4/projects/%s/pipelines/%s/jobs", a.baseURL, projectID, id)
	var rawJobs []gitLabJob
	if err := a.get(jobsURL, &rawJobs); err != nil {
		return domain.Pipeline{}, err
	}

	pipeline := run.toPipeline()
	pipeline.Jobs = make([]domain.Job, len(rawJobs))
	for i, j := range rawJobs {
		pipeline.Jobs[i] = j.toJob()
	}
	return pipeline, nil
}

func (a *Adapter) get(apiURL string, target interface{}) error {
	req, err := http.NewRequest(http.MethodGet, apiURL, nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("PRIVATE-TOKEN", a.token)

	resp, err := a.client.Do(req)
	if err != nil {
		return fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("gitlab API error: %s", resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(target)
}

type gitLabPipeline struct {
	ID        int64  `json:"id"`
	Ref       string `json:"ref"`
	SHA       string `json:"sha"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

func (r gitLabPipeline) toPipeline() domain.Pipeline {
	created, _ := time.Parse(time.RFC3339, r.CreatedAt)
	updated, _ := time.Parse(time.RFC3339, r.UpdatedAt)
	var duration time.Duration
	if !created.IsZero() && !updated.IsZero() {
		duration = updated.Sub(created)
	}
	return domain.Pipeline{
		ID:        strconv.FormatInt(r.ID, 10),
		Branch:    r.Ref,
		CommitSHA: r.SHA,
		Status:    mapGitLabStatus(r.Status),
		CreatedAt: created,
		Duration:  duration,
	}
}

type gitLabJob struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	Stage      string `json:"stage"`
	Status     string `json:"status"`
	StartedAt  string `json:"started_at"`
	FinishedAt string `json:"finished_at"`
}

func (j gitLabJob) toJob() domain.Job {
	started, _ := time.Parse(time.RFC3339, j.StartedAt)
	finished, _ := time.Parse(time.RFC3339, j.FinishedAt)
	var duration time.Duration
	if !started.IsZero() && !finished.IsZero() {
		duration = finished.Sub(started)
	}
	return domain.Job{
		ID:        strconv.FormatInt(j.ID, 10),
		Name:      j.Name,
		Stage:     j.Stage,
		Status:    mapGitLabStatus(j.Status),
		StartedAt: started,
		Duration:  duration,
	}
}

func mapGitLabStatus(status string) domain.PipelineStatus {
	switch status {
	case "success":
		return domain.StatusSuccess
	case "failed":
		return domain.StatusFailed
	case "running":
		return domain.StatusRunning
	case "pending", "created", "waiting_for_resource", "preparing", "scheduled":
		return domain.StatusPending
	case "canceled":
		return domain.StatusCancelled
	default:
		return domain.StatusPending
	}
}
```

**Step 4: Run tests to verify they pass**

```bash
go test ./internal/provider/gitlab/... -v
```

Expected: all tests PASS.

**Step 5: Commit**

```bash
git add internal/provider/gitlab/
git commit -m "feat: GitLab CI adapter"
```

---

### Task 7: TUI — Pipeline list model

**Files:**
- Create: `internal/tui/pipelinelist.go`
- Create: `internal/tui/pipelinelist_test.go`

**Step 1: Write the failing tests**

Create `internal/tui/pipelinelist_test.go`:

```go
package tui_test

import (
	"testing"
	"time"

	"github.com/waabox/gitdeck/internal/domain"
	"github.com/waabox/gitdeck/internal/tui"
)

func TestPipelineListModel_RendersPipelines(t *testing.T) {
	pipelines := []domain.Pipeline{
		{
			ID:        "100",
			Branch:    "main",
			CommitSHA: "abc1234",
			CommitMsg: "fix: login timeout",
			Author:    "waabox",
			Status:    domain.StatusSuccess,
			CreatedAt: time.Now().Add(-2 * time.Minute),
			Duration:  90 * time.Second,
		},
		{
			ID:     "99",
			Branch: "feat/auth",
			Status: domain.StatusFailed,
		},
	}

	m := tui.NewPipelineListModel(pipelines)
	view := m.View()

	if view == "" {
		t.Error("expected non-empty view")
	}
	if m.SelectedIndex() != 0 {
		t.Errorf("expected selected index 0, got %d", m.SelectedIndex())
	}
	if m.SelectedPipeline().ID != "100" {
		t.Errorf("expected selected pipeline ID '100', got '%s'", m.SelectedPipeline().ID)
	}
}

func TestPipelineListModel_NavigatesDown(t *testing.T) {
	pipelines := []domain.Pipeline{
		{ID: "1", Status: domain.StatusSuccess},
		{ID: "2", Status: domain.StatusFailed},
	}
	m := tui.NewPipelineListModel(pipelines)
	m = m.MoveDown()
	if m.SelectedIndex() != 1 {
		t.Errorf("expected selected index 1 after moving down, got %d", m.SelectedIndex())
	}
}

func TestPipelineListModel_DoesNotGoAboveZero(t *testing.T) {
	pipelines := []domain.Pipeline{{ID: "1"}}
	m := tui.NewPipelineListModel(pipelines)
	m = m.MoveUp()
	if m.SelectedIndex() != 0 {
		t.Errorf("expected selected index 0, got %d", m.SelectedIndex())
	}
}
```

**Step 2: Run tests to verify they fail**

```bash
go test ./internal/tui/...
```

Expected: FAIL — package does not exist.

**Step 3: Implement the pipeline list model**

Create `internal/tui/pipelinelist.go`:

```go
package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/waabox/gitdeck/internal/domain"
)

// PipelineListModel is an immutable Bubbletea-compatible model for the pipeline list panel.
type PipelineListModel struct {
	pipelines []domain.Pipeline
	cursor    int
}

// NewPipelineListModel creates a pipeline list model with the given pipelines.
func NewPipelineListModel(pipelines []domain.Pipeline) PipelineListModel {
	return PipelineListModel{pipelines: pipelines, cursor: 0}
}

// MoveDown returns a new model with the cursor moved down by one.
func (m PipelineListModel) MoveDown() PipelineListModel {
	if m.cursor < len(m.pipelines)-1 {
		m.cursor++
	}
	return m
}

// MoveUp returns a new model with the cursor moved up by one.
func (m PipelineListModel) MoveUp() PipelineListModel {
	if m.cursor > 0 {
		m.cursor--
	}
	return m
}

// SelectedIndex returns the current cursor position.
func (m PipelineListModel) SelectedIndex() int {
	return m.cursor
}

// SelectedPipeline returns the currently highlighted pipeline.
// Returns zero-value Pipeline if the list is empty.
func (m PipelineListModel) SelectedPipeline() domain.Pipeline {
	if len(m.pipelines) == 0 {
		return domain.Pipeline{}
	}
	return m.pipelines[m.cursor]
}

// View renders the pipeline list as a string.
func (m PipelineListModel) View() string {
	if len(m.pipelines) == 0 {
		return "No pipelines found."
	}
	var sb strings.Builder
	for i, p := range m.pipelines {
		prefix := "  "
		if i == m.cursor {
			prefix = "> "
		}
		sb.WriteString(fmt.Sprintf("%s%s #%s %-20s %s  %s\n",
			prefix,
			statusIcon(p.Status),
			p.ID,
			truncate(p.Branch, 20),
			statusIcon(p.Status),
			formatAge(p.CreatedAt),
		))
	}
	return sb.String()
}

func statusIcon(s domain.PipelineStatus) string {
	switch s {
	case domain.StatusSuccess:
		return "✓"
	case domain.StatusFailed:
		return "✗"
	case domain.StatusRunning:
		return "●"
	case domain.StatusPending:
		return "↷"
	case domain.StatusCancelled:
		return "○"
	default:
		return "?"
	}
}

func formatAge(t time.Time) string {
	if t.IsZero() {
		return "--"
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds ago", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	default:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	}
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}
```

**Step 4: Run tests to verify they pass**

```bash
go test ./internal/tui/... -v
```

Expected: all tests PASS.

**Step 5: Commit**

```bash
git add internal/tui/pipelinelist.go internal/tui/pipelinelist_test.go
git commit -m "feat: TUI pipeline list model"
```

---

### Task 8: TUI — Job detail model

**Files:**
- Create: `internal/tui/jobdetail.go`
- Create: `internal/tui/jobdetail_test.go`

**Step 1: Write the failing tests**

Create `internal/tui/jobdetail_test.go`:

```go
package tui_test

import (
	"testing"
	"time"

	"github.com/waabox/gitdeck/internal/domain"
	"github.com/waabox/gitdeck/internal/tui"
)

func TestJobDetailModel_RendersJobs(t *testing.T) {
	jobs := []domain.Job{
		{ID: "1", Name: "build", Stage: "build", Status: domain.StatusSuccess, Duration: 45 * time.Second},
		{ID: "2", Name: "test",  Stage: "test",  Status: domain.StatusFailed,  Duration: 72 * time.Second},
	}
	m := tui.NewJobDetailModel(jobs)
	view := m.View()
	if view == "" {
		t.Error("expected non-empty view")
	}
}

func TestJobDetailModel_EmptyShowsMessage(t *testing.T) {
	m := tui.NewJobDetailModel(nil)
	view := m.View()
	if view == "" {
		t.Error("expected non-empty view for empty jobs")
	}
}
```

**Step 2: Run tests to verify they fail**

```bash
go test ./internal/tui/...
```

Expected: FAIL — file does not exist.

**Step 3: Implement the job detail model**

Create `internal/tui/jobdetail.go`:

```go
package tui

import (
	"fmt"
	"strings"

	"github.com/waabox/gitdeck/internal/domain"
)

// JobDetailModel is an immutable model for the jobs panel.
type JobDetailModel struct {
	jobs []domain.Job
}

// NewJobDetailModel creates a job detail model.
func NewJobDetailModel(jobs []domain.Job) JobDetailModel {
	return JobDetailModel{jobs: jobs}
}

// View renders the job list as a string.
func (m JobDetailModel) View() string {
	if len(m.jobs) == 0 {
		return "Select a pipeline to see its jobs."
	}
	var sb strings.Builder
	for _, j := range m.jobs {
		duration := "--"
		if j.Duration > 0 {
			duration = fmt.Sprintf("%ds", int(j.Duration.Seconds()))
		}
		sb.WriteString(fmt.Sprintf("  %s %-25s %s\n",
			statusIcon(j.Status),
			truncate(j.Name, 25),
			duration,
		))
	}
	return sb.String()
}
```

**Step 4: Run tests to verify they pass**

```bash
go test ./internal/tui/... -v
```

Expected: all tests PASS.

**Step 5: Commit**

```bash
git add internal/tui/jobdetail.go internal/tui/jobdetail_test.go
git commit -m "feat: TUI job detail model"
```

---

### Task 9: TUI — Bubbletea root app and wiring

**Files:**
- Create: `internal/tui/app.go`
- Modify: `cmd/gitdeck/main.go`

**Step 1: Create the Bubbletea root model**

Create `internal/tui/app.go`:

```go
package tui

import (
	"fmt"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/waabox/gitdeck/internal/domain"
)

// pipelinesLoadedMsg is sent when pipelines have been fetched from the provider.
type pipelinesLoadedMsg struct {
	pipelines []domain.Pipeline
	err       error
}

// pipelineDetailMsg is sent when a pipeline detail (with jobs) has been fetched.
type pipelineDetailMsg struct {
	pipeline domain.Pipeline
	err      error
}

// tickMsg is sent by the auto-refresh ticker.
type tickMsg struct{}

// focusPanel indicates which panel has keyboard focus.
type focusPanel int

const (
	focusList focusPanel = iota
	focusDetail
)

// AppModel is the root Bubbletea model for gitdeck.
type AppModel struct {
	repo         domain.Repository
	provider     domain.PipelineProvider
	list         PipelineListModel
	detail       JobDetailModel
	focus        focusPanel
	loading      bool
	err          error
	width        int
	height       int
}

// NewAppModel creates the root application model.
func NewAppModel(repo domain.Repository, provider domain.PipelineProvider) AppModel {
	return AppModel{
		repo:     repo,
		provider: provider,
		list:     NewPipelineListModel(nil),
		detail:   NewJobDetailModel(nil),
		loading:  true,
	}
}

// Init triggers the initial pipeline load.
func (m AppModel) Init() tea.Cmd {
	return tea.Batch(m.loadPipelines(), tickEvery(30*time.Second))
}

func (m AppModel) loadPipelines() tea.Cmd {
	return func() tea.Msg {
		pipelines, err := m.provider.ListPipelines(m.repo)
		return pipelinesLoadedMsg{pipelines: pipelines, err: err}
	}
}

func (m AppModel) loadPipelineDetail(id string) tea.Cmd {
	return func() tea.Msg {
		pipeline, err := m.provider.GetPipeline(m.repo, id)
		return pipelineDetailMsg{pipeline: pipeline, err: err}
	}
}

func tickEvery(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(_ time.Time) tea.Msg {
		return tickMsg{}
	})
}

// Update handles all incoming messages and key events.
func (m AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case pipelinesLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.list = NewPipelineListModel(msg.pipelines)

	case pipelineDetailMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.detail = NewJobDetailModel(msg.pipeline.Jobs)

	case tickMsg:
		return m, tea.Batch(m.loadPipelines(), tickEvery(30*time.Second))

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "r":
			m.loading = true
			return m, m.loadPipelines()
		case "tab":
			if m.focus == focusList {
				m.focus = focusDetail
			} else {
				m.focus = focusList
			}
		case "j", "down":
			if m.focus == focusList {
				m.list = m.list.MoveDown()
				return m, m.loadPipelineDetail(m.list.SelectedPipeline().ID)
			}
		case "k", "up":
			if m.focus == focusList {
				m.list = m.list.MoveUp()
				return m, m.loadPipelineDetail(m.list.SelectedPipeline().ID)
			}
		case "enter":
			if m.focus == focusList {
				m.focus = focusDetail
				return m, m.loadPipelineDetail(m.list.SelectedPipeline().ID)
			}
		case "esc":
			m.focus = focusList
		}
	}
	return m, nil
}

// View renders the full TUI.
func (m AppModel) View() string {
	if m.loading {
		return "Loading pipelines...\n"
	}
	if m.err != nil {
		return fmt.Sprintf("Error: %v\n\nPress 'r' to retry or 'q' to quit.\n", m.err)
	}

	header := fmt.Sprintf(" gitdeck  %s/%s  q:quit  r:refresh\n",
		m.repo.Owner, m.repo.Name)
	separator := "────────────────────────────────────────────────────────────\n"

	listHeader := " PIPELINES\n"
	detailHeader := " JOBS\n"

	listView := m.list.View()
	detailView := m.detail.View()

	selected := m.list.SelectedPipeline()
	statusBar := fmt.Sprintf(" #%s  %s  %s  \"%s\"  by %s\n",
		selected.ID, selected.Branch,
		shortSHA(selected.CommitSHA), selected.CommitMsg, selected.Author)

	footer := " j/k: navigate   tab: switch panel   enter: select   r: refresh   q: quit\n"

	return header + separator +
		listHeader + listView + "\n" +
		detailHeader + detailView + "\n" +
		separator + statusBar + footer
}

// Run starts the Bubbletea program. Exits on error.
func Run(repo domain.Repository, provider domain.PipelineProvider) {
	p := tea.NewProgram(NewAppModel(repo, provider), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "gitdeck error: %v\n", err)
		os.Exit(1)
	}
}

func shortSHA(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}
```

**Step 2: Wire everything together in main.go**

Replace `cmd/gitdeck/main.go`:

```go
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
```

**Step 3: Verify it compiles**

```bash
go build ./...
```

Expected: no errors.

**Step 4: Run all tests**

```bash
go test ./... -v
```

Expected: all tests PASS.

**Step 5: Commit**

```bash
git add internal/tui/app.go cmd/gitdeck/main.go
git commit -m "feat: wire Bubbletea app with full pipeline + job navigation"
```

---

### Task 10: Final verification

**Step 1: Run the full test suite**

```bash
go test ./... -v -count=1
```

Expected: all tests PASS with zero failures.

**Step 2: Build the binary**

```bash
go build -o gitdeck ./cmd/gitdeck/
```

Expected: `gitdeck` binary created.

**Step 3: Verify binary works in this repo**

```bash
GITHUB_TOKEN=your_token_here ./gitdeck
```

Expected: TUI launches, shows pipelines for this repository.

**Step 4: Commit final state**

```bash
git add go.sum
git commit -m "feat: complete CI pipeline visualizer v1"
```
