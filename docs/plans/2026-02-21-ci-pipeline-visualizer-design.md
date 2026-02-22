# CI Pipeline Visualizer — Design

**Date:** 2026-02-21
**Status:** Approved

## Overview

`gitdeck` is a TUI application (lazygit-style) built with Go + Bubbletea.
It runs from any directory containing a `.git` folder, reads the `origin` remote,
detects the CI provider, and visualizes pipeline status using the GitHub or GitLab API.

## Architecture

Clean DDD layers with a Provider Registry pattern.

```
gitdeck/
├── cmd/gitdeck/main.go
├── internal/
│   ├── domain/
│   │   ├── pipeline.go       # Pipeline, Job, Stage, PipelineStatus
│   │   ├── repository.go     # Repository (owner, name, remote URL)
│   │   └── provider.go       # Port interface: PipelineProvider
│   ├── git/
│   │   └── remote.go         # Reads .git from CWD, extracts owner/repo/provider
│   ├── config/
│   │   └── config.go         # Reads ~/.config/gitdeck/config.toml + env vars
│   ├── provider/
│   │   ├── registry.go       # ProviderRegistry: detects by URL → adapter
│   │   ├── github/
│   │   │   └── adapter.go    # GitHub Actions adapter
│   │   └── gitlab/
│   │       └── adapter.go    # GitLab CI adapter
│   └── tui/
│       ├── app.go             # Bubbletea root model
│       ├── pipeline_list.go   # Screen: list of recent pipelines
│       └── pipeline_detail.go # Screen: detail with jobs/stages
└── go.mod
```

## Domain Model

```go
type PipelineStatus string

const (
    StatusPending   PipelineStatus = "pending"
    StatusRunning   PipelineStatus = "running"
    StatusSuccess   PipelineStatus = "success"
    StatusFailed    PipelineStatus = "failed"
    StatusCancelled PipelineStatus = "cancelled"
)

type Job struct {
    ID        string
    Name      string
    Stage     string
    Status    PipelineStatus
    Duration  time.Duration
    StartedAt time.Time
}

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

type Repository struct {
    Owner     string
    Name      string
    RemoteURL string
}

// Port interface — the only contact point between domain and infrastructure
type PipelineProvider interface {
    ListPipelines(repo Repository) ([]Pipeline, error)
    GetPipeline(repo Repository, id string) (Pipeline, error)
}
```

## TUI Layout

```
┌─────────────────────────────────────────────────────────────┐
│ gitdeck  owner/repo  [github]                    q:quit r:refresh │
├──────────────────────────┬──────────────────────────────────┤
│ PIPELINES                │ JOBS                             │
│                          │                                  │
│ ● #124 main   ✓ 2m ago  │ ● build          ✓  45s         │
│ ● #123 feat/x ✗ 1h ago  │ ● test-unit      ✓  1m12s       │
│ ● #122 main   ✓ 3h ago  │ ● test-e2e       ✗  2m03s       │
│ ● #121 main   ✓ 5h ago  │ ● deploy-staging ↷  --          │
│                          │                                  │
├──────────────────────────┴──────────────────────────────────┤
│ #124  main  abc1234  "fix: login timeout"  by waabox        │
└─────────────────────────────────────────────────────────────┘
  j/k: navegar   enter: seleccionar   esc: volver   r: refresh
```

**Status icons:** `✓` success · `✗` failed · `●` running · `↷` pending · `○` cancelled

**Navigation:**
- Left panel active by default — `j/k` move pipeline list
- `enter` loads jobs for selected pipeline into right panel
- `tab` switches focus between panels
- `r` manual refresh; auto-refresh every 30s when a pipeline is `running`

## Configuration

File: `~/.config/gitdeck/config.toml`

```toml
[github]
token = "ghp_..."

[gitlab]
token = "glpat_..."
url = "https://gitlab.mycompany.com"  # optional, for self-hosted instances
```

Env vars take precedence: `GITHUB_TOKEN`, `GITLAB_TOKEN`, `GITLAB_URL`.

## Provider Registry

Detects the provider from the remote URL:

| Remote URL pattern | Provider         |
|--------------------|------------------|
| `github.com`       | GitHub Actions   |
| `gitlab.com`       | GitLab CI        |
| custom (`GITLAB_URL`) | GitLab CI self-hosted |

If no provider matches → error: `"no provider found for remote: <url>"`

## Startup Flow

1. `main.go` detects CWD → finds `.git` → parses `origin` remote
2. `ProviderRegistry.Detect(remoteURL)` → selects correct adapter
3. TUI starts → calls `provider.ListPipelines(repo)` → renders list
4. User selects pipeline → calls `provider.GetPipeline(repo, id)` → renders detail

## Implementation Phases

1. **Core domain + project scaffold** — go.mod, domain model, port interface
2. **Git remote detection** — parse CWD `.git/config`, extract owner/repo/provider
3. **Config loader** — TOML + env var fallback
4. **Provider Registry** — registry + detection logic
5. **GitHub Actions adapter** — implement PipelineProvider for GitHub
6. **GitLab CI adapter** — implement PipelineProvider for GitLab
7. **TUI: pipeline list** — Bubbletea model, render pipelines
8. **TUI: pipeline detail** — jobs panel, keyboard navigation
9. **TUI: polish** — status bar, auto-refresh, error states
