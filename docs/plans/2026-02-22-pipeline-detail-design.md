# Pipeline Detail: Auto-refresh + Steps Expansion

**Date:** 2026-02-22

## Problem

When a user selects a pipeline in gitdeck, the jobs panel loads once and stays stale. If a pipeline is actively running, job statuses do not update. Additionally, there is no way to inspect the individual steps within a job.

## Goals

1. Auto-refresh job statuses for the selected pipeline while it is running.
2. Allow users to expand a job in-place to see its steps.

## Non-goals

- Three-panel layout (rejected in favour of simpler in-place expansion).
- GitLab step detail (GitLab jobs map directly to steps; no nested step API).

---

## Design

### Auto-refresh

`AppModel` tracks `selectedPipelineID string`. On every tick:

- Always reload the pipeline list.
- If `selectedPipelineID` is non-empty AND the selected pipeline has `StatusRunning`, also dispatch `loadPipelineDetail`.

Tick interval:

- **5 seconds** while any pipeline in the list is running.
- **30 seconds** when all pipelines are complete (reduces API pressure).

### Domain changes

Add `Step` to `internal/domain/pipeline.go`:

```go
type Step struct {
    Name     string
    Number   int
    Status   PipelineStatus
    Duration time.Duration
}
```

Add `Steps []Step` to `Job`. No other domain changes needed.

### Adapter changes (GitHub)

The GitHub `/actions/runs/{id}/jobs` response already includes a `steps` array per job. Parse `steps` into `[]domain.Step` inside `workflowJob.toJob()`. No additional HTTP requests are needed.

GitLab: `Steps` remains empty. Expand shows nothing (graceful no-op).

### TUI changes

**`JobDetailModel`** gains:

- `expanded map[int]bool` — tracks which job indices are expanded.
- `ToggleExpand(idx int) JobDetailModel` — returns new model with the index toggled.

**Key bindings in the Jobs panel (focused):**

- `Enter` — toggle expansion of the job under the cursor.
- `↑` / `↓` — navigate jobs (existing behaviour).
- `Esc` — collapse all, return focus to pipeline list (existing behaviour).

**Rendering expanded jobs:**

```
  ✓ build   12s
> ✓ test    38s
    ├ ✓ checkout    2s
    ├ ✓ setup-go    5s
    └ ✓ run tests  31s
  ● lint      3s
```

Tree characters `├` and `└` are used for visual clarity. Steps with zero duration show `--`.

---

## Open questions

None. Design approved by user.
