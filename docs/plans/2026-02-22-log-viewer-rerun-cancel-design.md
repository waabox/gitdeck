# Design: Log Viewer + Re-run / Cancel Pipeline

**Date**: 2026-02-22
**Status**: Approved

## Overview

Two new features for gitdeck:

1. **Log Viewer** — view the full raw log of any CI job in a fullscreen scrollable panel inside the TUI.
2. **Re-run / Cancel** — trigger a re-run or cancellation of the selected pipeline, with an inline confirmation prompt.

## Domain Layer

Extend `PipelineProvider` (`internal/domain/provider.go`) with three new methods:

```go
// GetJobLogs returns the full raw log text for a given job.
GetJobLogs(repo Repository, jobID string) (string, error)

// RerunPipeline triggers a new run of the given pipeline.
RerunPipeline(repo Repository, id PipelineID) error

// CancelPipeline cancels a running pipeline.
CancelPipeline(repo Repository, id PipelineID) error
```

Both GitHub and GitLab fully support these operations so all three methods go directly into the existing interface.

## Provider Adapters

### GitHub (`internal/provider/github/adapter.go`)

| Method | API endpoint |
|--------|-------------|
| `GetJobLogs` | `GET /repos/{owner}/{repo}/actions/jobs/{job_id}/logs` — follows redirect to raw text |
| `RerunPipeline` | `POST /repos/{owner}/{repo}/actions/runs/{run_id}/rerun` |
| `CancelPipeline` | `POST /repos/{owner}/{repo}/actions/runs/{run_id}/cancel` |

### GitLab (`internal/provider/gitlab/adapter.go`)

| Method | API endpoint |
|--------|-------------|
| `GetJobLogs` | `GET /projects/{id}/jobs/{job_id}/trace` |
| `RerunPipeline` | `POST /projects/{id}/pipelines/{pipeline_id}/retry` |
| `CancelPipeline` | `POST /projects/{id}/pipelines/{pipeline_id}/cancel` |

## TUI — Re-run / Cancel

### Keybinding changes

| Key | Action |
|-----|--------|
| `r` | Re-run selected pipeline (confirmation required) |
| `x` | Cancel selected pipeline (confirmation required) |
| `ctrl+r` | Manual refresh (replaces the old `r` binding) |

### Confirmation flow

1. User presses `r` or `x` on the selected pipeline.
2. A prompt appears at the bottom of the pipeline list panel:
   ```
   Re-run pipeline #1042 on branch main? [y/N]
   ```
3. `y` → call provider → show "Triggered." or "Cancelled." for 2 seconds → auto-refresh.
4. Any other key → cancel, return to normal state without changes.

### App state additions

```go
confirmAction  string      // "rerun" | "cancel" | ""
confirmMsg     string      // text shown in the prompt
```

## TUI — Log Viewer

### Activation

From the job detail panel, press `l` on the focused job.

### Loading state

While `GetJobLogs` is in flight, the fullscreen area shows:
```
Loading logs...
```

### Fullscreen layout

```
 gitdeck  waabox/gitdeck  [logs] build — job #42
────────────────────────────────────────────────────────────
<raw log lines, scrollable>
────────────────────────────────────────────────────────────
 ↑/↓: scroll   PgUp/PgDn: page   g/G: top/bottom   esc: back
```

### Keybindings

| Key | Action |
|-----|--------|
| `↑` / `↓` | Scroll one line |
| `PgUp` / `PgDn` | Scroll one page |
| `g` | Go to top |
| `G` | Go to bottom |
| `Esc` | Return to job detail panel |

### App state additions

```go
mode        appMode   // new value: logView
logContent  string    // raw log text
logOffset   int       // current scroll offset (line index)
logJobName  string    // displayed in header
```

No new component file — `logView` is a conditional render branch in the existing app loop, consistent with how job detail is handled today.

## Out of Scope

- Log filtering / search within the log viewer (future feature)
- Re-run of individual jobs (pipeline-level only for now)
- Streaming logs in real time (fetch-once on open)
