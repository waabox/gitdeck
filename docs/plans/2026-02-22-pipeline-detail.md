# Pipeline Detail: Auto-refresh + Steps Expansion Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** When a user selects a pipeline, its jobs auto-refresh while running, and each job can be expanded in-place to show its individual steps.

**Architecture:** Four layered changes: domain adds `Step`, GitHub adapter parses steps from the existing jobs response (no new HTTP calls), `JobDetailModel` gains immutable expansion state, and `AppModel` auto-refreshes the selected pipeline on each tick when it is running.

**Tech Stack:** Go, Bubbletea TUI framework, GitHub Actions API, GitLab CI API.

---

### Task 1: Domain — Add Step struct

**Files:**
- Modify: `internal/domain/pipeline.go`

**Step 1: Add Step to pipeline.go**

Replace the `Job` struct with:

```go
// Step represents a single step within a CI job.
type Step struct {
    Name     string
    Number   int
    Status   PipelineStatus
    Duration time.Duration
}

// Job represents a single unit of work within a pipeline.
type Job struct {
    ID        string
    Name      string
    Stage     string
    Status    PipelineStatus
    Duration  time.Duration
    StartedAt time.Time
    Steps     []Step
}
```

**Step 2: Run tests to confirm no breakage**

```
go test ./...
```

Expected: all existing tests pass (Steps field is zero-value, no compilation errors).

**Step 3: Commit**

```bash
git add internal/domain/pipeline.go
git commit -m "feat: add Step type and Steps field to Job"
```

---

### Task 2: GitHub adapter — Parse steps from jobs response

**Files:**
- Modify: `internal/provider/github/adapter.go`
- Modify: `internal/provider/github/adapter_test.go`

**Step 1: Write the failing test**

In `adapter_test.go`, add a new test after `TestGetPipeline_ReturnsRunWithJobs`:

```go
func TestGetPipeline_ParsesJobSteps(t *testing.T) {
    runResponse := map[string]interface{}{
        "id":          float64(1001),
        "head_branch": "main",
        "head_sha":    "abc1234",
        "head_commit": map[string]interface{}{
            "message": "fix: login timeout",
            "author":  map[string]interface{}{"name": "waabox"},
        },
        "status":     "in_progress",
        "conclusion": nil,
        "created_at": time.Now().Add(-2 * time.Minute).Format(time.RFC3339),
        "updated_at": time.Now().Format(time.RFC3339),
    }
    jobsResponse := map[string]interface{}{
        "jobs": []map[string]interface{}{
            {
                "id":           float64(2001),
                "name":         "test",
                "status":       "in_progress",
                "conclusion":   nil,
                "started_at":   time.Now().Add(-1 * time.Minute).Format(time.RFC3339),
                "completed_at": "",
                "steps": []map[string]interface{}{
                    {
                        "name":         "Set up job",
                        "number":       float64(1),
                        "status":       "completed",
                        "conclusion":   "success",
                        "started_at":   time.Now().Add(-60 * time.Second).Format(time.RFC3339),
                        "completed_at": time.Now().Add(-55 * time.Second).Format(time.RFC3339),
                    },
                    {
                        "name":         "Run tests",
                        "number":       float64(2),
                        "status":       "in_progress",
                        "conclusion":   nil,
                        "started_at":   time.Now().Add(-55 * time.Second).Format(time.RFC3339),
                        "completed_at": "",
                    },
                },
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

    adapter := githubprovider.NewAdapter("test-token", srv.URL, 3)
    repo := domain.Repository{Owner: "waabox", Name: "gitdeck"}

    pipeline, err := adapter.GetPipeline(repo, "1001")
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if len(pipeline.Jobs) != 1 {
        t.Fatalf("expected 1 job, got %d", len(pipeline.Jobs))
    }
    job := pipeline.Jobs[0]
    if len(job.Steps) != 2 {
        t.Fatalf("expected 2 steps, got %d", len(job.Steps))
    }
    if job.Steps[0].Name != "Set up job" {
        t.Errorf("expected first step 'Set up job', got '%s'", job.Steps[0].Name)
    }
    if job.Steps[0].Status != domain.StatusSuccess {
        t.Errorf("expected first step status success, got '%s'", job.Steps[0].Status)
    }
    if job.Steps[1].Status != domain.StatusRunning {
        t.Errorf("expected second step status running, got '%s'", job.Steps[1].Status)
    }
}
```

**Step 2: Run to verify it fails**

```
go test ./internal/provider/github/... -run TestGetPipeline_ParsesJobSteps -v
```

Expected: FAIL — steps will be empty.

**Step 3: Add step types to the adapter**

In `adapter.go`, add a `workflowStep` struct and update `workflowJob`:

```go
// workflowStep is a single step within a GitHub Actions job.
type workflowStep struct {
    Name        string  `json:"name"`
    Number      float64 `json:"number"`
    Status      string  `json:"status"`
    Conclusion  string  `json:"conclusion"`
    StartedAt   string  `json:"started_at"`
    CompletedAt string  `json:"completed_at"`
}

// workflowJob is the raw GitHub API response shape for a job.
type workflowJob struct {
    ID          int64          `json:"id"`
    Name        string         `json:"name"`
    Status      string         `json:"status"`
    Conclusion  string         `json:"conclusion"`
    StartedAt   string         `json:"started_at"`
    CompletedAt string         `json:"completed_at"`
    Steps       []workflowStep `json:"steps"`
}
```

Update `workflowJob.toJob()` to map steps:

```go
func (j workflowJob) toJob() domain.Job {
    started, _ := time.Parse(time.RFC3339, j.StartedAt)
    completed, _ := time.Parse(time.RFC3339, j.CompletedAt)
    var duration time.Duration
    if !started.IsZero() && !completed.IsZero() {
        duration = completed.Sub(started)
    }
    steps := make([]domain.Step, len(j.Steps))
    for i, s := range j.Steps {
        stepStarted, _ := time.Parse(time.RFC3339, s.StartedAt)
        stepCompleted, _ := time.Parse(time.RFC3339, s.CompletedAt)
        var stepDuration time.Duration
        if !stepStarted.IsZero() && !stepCompleted.IsZero() {
            stepDuration = stepCompleted.Sub(stepStarted)
        }
        steps[i] = domain.Step{
            Name:     s.Name,
            Number:   int(s.Number),
            Status:   mapGitHubStatus(s.Status, s.Conclusion),
            Duration: stepDuration,
        }
    }
    return domain.Job{
        ID:        strconv.FormatInt(j.ID, 10),
        Name:      j.Name,
        Status:    mapGitHubStatus(j.Status, j.Conclusion),
        StartedAt: started,
        Duration:  duration,
        Steps:     steps,
    }
}
```

**Step 4: Run test to verify it passes**

```
go test ./internal/provider/github/... -v
```

Expected: all tests PASS.

**Step 5: Commit**

```bash
git add internal/provider/github/adapter.go internal/provider/github/adapter_test.go
git commit -m "feat: parse job steps from GitHub Actions API response"
```

---

### Task 3: JobDetailModel — in-place step expansion

**Files:**
- Modify: `internal/tui/jobdetail.go`
- Modify: `internal/tui/jobdetail_test.go`

**Step 1: Write failing tests**

Replace the contents of `jobdetail_test.go` with:

```go
package tui_test

import (
    "strings"
    "testing"
    "time"

    "github.com/waabox/gitdeck/internal/domain"
    "github.com/waabox/gitdeck/internal/tui"
)

func TestJobDetailModel_RendersJobs(t *testing.T) {
    jobs := []domain.Job{
        {ID: "1", Name: "build", Stage: "build", Status: domain.StatusSuccess, Duration: 45 * time.Second},
        {ID: "2", Name: "test", Stage: "test", Status: domain.StatusFailed, Duration: 72 * time.Second},
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

func TestJobDetailModel_ToggleExpand_ShowsSteps(t *testing.T) {
    jobs := []domain.Job{
        {
            ID:     "1",
            Name:   "test",
            Status: domain.StatusSuccess,
            Steps: []domain.Step{
                {Name: "checkout", Number: 1, Status: domain.StatusSuccess, Duration: 2 * time.Second},
                {Name: "run tests", Number: 2, Status: domain.StatusSuccess, Duration: 31 * time.Second},
            },
        },
    }
    m := tui.NewJobDetailModel(jobs)
    m = m.ToggleExpand(0)
    view := m.ViewFocused()
    if !strings.Contains(view, "checkout") {
        t.Errorf("expected steps to be visible after expand, got:\n%s", view)
    }
    if !strings.Contains(view, "run tests") {
        t.Errorf("expected 'run tests' step to be visible, got:\n%s", view)
    }
}

func TestJobDetailModel_ToggleExpand_HidesStepsOnSecondToggle(t *testing.T) {
    jobs := []domain.Job{
        {
            ID:     "1",
            Name:   "test",
            Status: domain.StatusSuccess,
            Steps: []domain.Step{
                {Name: "checkout", Number: 1, Status: domain.StatusSuccess, Duration: 2 * time.Second},
            },
        },
    }
    m := tui.NewJobDetailModel(jobs)
    m = m.ToggleExpand(0)
    m = m.ToggleExpand(0)
    view := m.ViewFocused()
    if strings.Contains(view, "checkout") {
        t.Errorf("expected steps to be hidden after second toggle, got:\n%s", view)
    }
}

func TestJobDetailModel_ToggleExpand_NoSteps_DoesNothing(t *testing.T) {
    jobs := []domain.Job{
        {ID: "1", Name: "build", Status: domain.StatusSuccess},
    }
    m := tui.NewJobDetailModel(jobs)
    m = m.ToggleExpand(0)
    view := m.ViewFocused()
    // Should render without error; no step rows
    if view == "" {
        t.Error("expected non-empty view even when job has no steps")
    }
}
```

**Step 2: Run to verify failures**

```
go test ./internal/tui/... -run TestJobDetailModel_Toggle -v
```

Expected: FAIL — `ToggleExpand` method does not exist.

**Step 3: Implement expansion in jobdetail.go**

Replace `internal/tui/jobdetail.go` with:

```go
package tui

import (
    "fmt"
    "strings"

    "github.com/waabox/gitdeck/internal/domain"
)

// JobDetailModel is an immutable model for the jobs panel.
type JobDetailModel struct {
    jobs     []domain.Job
    cursor   int
    expanded map[int]bool
}

// NewJobDetailModel creates a job detail model.
func NewJobDetailModel(jobs []domain.Job) JobDetailModel {
    return JobDetailModel{jobs: jobs, cursor: 0, expanded: map[int]bool{}}
}

// MoveDown returns a new model with the cursor moved down by one.
func (m JobDetailModel) MoveDown() JobDetailModel {
    if m.cursor < len(m.jobs)-1 {
        m.cursor++
    }
    return m
}

// MoveUp returns a new model with the cursor moved up by one.
func (m JobDetailModel) MoveUp() JobDetailModel {
    if m.cursor > 0 {
        m.cursor--
    }
    return m
}

// ToggleExpand returns a new model with the given job index expanded or collapsed.
// If the job has no steps, the toggle is a no-op.
func (m JobDetailModel) ToggleExpand(idx int) JobDetailModel {
    if idx < 0 || idx >= len(m.jobs) || len(m.jobs[idx].Steps) == 0 {
        return m
    }
    next := make(map[int]bool, len(m.expanded))
    for k, v := range m.expanded {
        next[k] = v
    }
    next[idx] = !next[idx]
    m.expanded = next
    return m
}

// Cursor returns the current cursor position.
func (m JobDetailModel) Cursor() int {
    return m.cursor
}

// View renders the job list as a string.
func (m JobDetailModel) View() string {
    return m.render(false)
}

// ViewFocused renders the job list with cursor indicators.
func (m JobDetailModel) ViewFocused() string {
    return m.render(true)
}

func (m JobDetailModel) render(focused bool) string {
    if len(m.jobs) == 0 {
        return "Select a pipeline to see its jobs."
    }
    var sb strings.Builder
    for i, j := range m.jobs {
        duration := "--"
        if j.Duration > 0 {
            duration = fmt.Sprintf("%ds", int(j.Duration.Seconds()))
        }
        prefix := "  "
        if focused && i == m.cursor {
            prefix = "> "
        }
        sb.WriteString(fmt.Sprintf("%s%s %-25s %s\n",
            prefix,
            statusIcon(j.Status),
            truncate(j.Name, 25),
            duration,
        ))
        if m.expanded[i] {
            for si, s := range j.Steps {
                tree := "├"
                if si == len(j.Steps)-1 {
                    tree = "└"
                }
                stepDuration := "--"
                if s.Duration > 0 {
                    stepDuration = fmt.Sprintf("%ds", int(s.Duration.Seconds()))
                }
                sb.WriteString(fmt.Sprintf("    %s %s %-21s %s\n",
                    tree,
                    statusIcon(s.Status),
                    truncate(s.Name, 21),
                    stepDuration,
                ))
            }
        }
    }
    return sb.String()
}
```

**Step 4: Run all tests to verify**

```
go test ./internal/tui/... -v
```

Expected: all tests PASS.

**Step 5: Commit**

```bash
git add internal/tui/jobdetail.go internal/tui/jobdetail_test.go
git commit -m "feat: add in-place step expansion to job detail panel"
```

---

### Task 4: AppModel — auto-refresh + Enter to expand

**Files:**
- Modify: `internal/tui/app.go`

**Step 1: Update AppModel**

The key changes:
1. On `tickMsg`, check if the selected pipeline is running → also reload its detail.
2. Adaptive tick interval: 5s if any pipeline is running, 30s otherwise.
3. `enter` in `focusDetail` toggles step expansion instead of being a no-op.

Replace `internal/tui/app.go` with:

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
    repo     domain.Repository
    provider domain.PipelineProvider
    list     PipelineListModel
    detail   JobDetailModel
    focus    focusPanel
    loading  bool
    err      error
    width    int
    height   int
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
    return tea.Batch(m.loadPipelines(), tickEvery(5*time.Second))
}

func (m AppModel) loadPipelines() tea.Cmd {
    return func() tea.Msg {
        pipelines, err := m.provider.ListPipelines(m.repo)
        return pipelinesLoadedMsg{pipelines: pipelines, err: err}
    }
}

func (m AppModel) loadPipelineDetail(id string) tea.Cmd {
    return func() tea.Msg {
        pipeline, err := m.provider.GetPipeline(m.repo, domain.PipelineID(id))
        return pipelineDetailMsg{pipeline: pipeline, err: err}
    }
}

func tickEvery(d time.Duration) tea.Cmd {
    return tea.Tick(d, func(_ time.Time) tea.Msg {
        return tickMsg{}
    })
}

// anyRunning reports whether any pipeline in the list has StatusRunning.
func anyRunning(pipelines []domain.Pipeline) bool {
    for _, p := range pipelines {
        if p.Status == domain.StatusRunning {
            return true
        }
    }
    return false
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
        selected := m.list.SelectedPipeline()
        interval := 30 * time.Second
        if anyRunning(m.list.Pipelines()) {
            interval = 5 * time.Second
        }
        cmds := []tea.Cmd{m.loadPipelines(), tickEvery(interval)}
        if selected.Status == domain.StatusRunning && selected.ID != "" {
            cmds = append(cmds, m.loadPipelineDetail(selected.ID))
        }
        return m, tea.Batch(cmds...)

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
        case "down":
            if m.focus == focusList {
                m.list = m.list.MoveDown()
                return m, m.loadPipelineDetail(m.list.SelectedPipeline().ID)
            }
            m.detail = m.detail.MoveDown()
        case "up":
            if m.focus == focusList {
                m.list = m.list.MoveUp()
                return m, m.loadPipelineDetail(m.list.SelectedPipeline().ID)
            }
            m.detail = m.detail.MoveUp()
        case "enter":
            if m.focus == focusList {
                m.focus = focusDetail
                return m, m.loadPipelineDetail(m.list.SelectedPipeline().ID)
            }
            m.detail = m.detail.ToggleExpand(m.detail.Cursor())
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
    if m.focus == focusList {
        listHeader = " PIPELINES [active]\n"
    } else {
        detailHeader = " JOBS [active]\n"
    }

    listView := m.list.View()
    detailView := m.detail.View()
    if m.focus == focusDetail {
        detailView = m.detail.ViewFocused()
    }

    selected := m.list.SelectedPipeline()
    statusBar := fmt.Sprintf(" #%s  %s  %s  \"%s\"  by %s\n",
        selected.ID, selected.Branch,
        shortSHA(selected.CommitSHA), selected.CommitMsg, selected.Author)

    footer := " ↑/↓: navigate   tab: switch panel   enter: select/expand   r: refresh   q: quit\n"

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

Note: `tickMsg` now calls `m.list.Pipelines()` — you must add a `Pipelines()` accessor to `PipelineListModel`.

**Step 2: Add Pipelines() accessor to pipelinelist.go**

In `internal/tui/pipelinelist.go`, add after `SelectedPipeline()`:

```go
// Pipelines returns the full pipeline slice.
func (m PipelineListModel) Pipelines() []domain.Pipeline {
    return m.pipelines
}
```

**Step 3: Build to confirm it compiles**

```
go build ./...
```

Expected: no errors.

**Step 4: Run all tests**

```
go test ./...
```

Expected: all tests PASS.

**Step 5: Commit**

```bash
git add internal/tui/app.go internal/tui/pipelinelist.go
git commit -m "feat: auto-refresh selected pipeline detail and expand steps with enter"
```

---

### Task 5: Update README keyboard shortcuts

**Files:**
- Modify: `README.md`

**Step 1: Update the keyboard shortcuts table**

Find the footer line in the README's ASCII screenshot and the keyboard shortcuts section. Update `enter: select` to `enter: select/expand`:

In the ASCII art block, change:
```
 ↑/↓: navigate   tab: switch panel   enter: select   r: refresh   q: quit
```
to:
```
 ↑/↓: navigate   tab: switch panel   enter: select/expand   r: refresh   q: quit
```

Also add a note about steps:

In the Features section, add after "Job detail panel with per-job navigation":
```
- Per-job step detail: press `enter` on a job to expand its individual steps inline
```

**Step 2: Commit**

```bash
git add README.md
git commit -m "docs: update shortcuts and feature list for step expansion"
```

---

### Task 6: Final verification

**Step 1: Full test suite**

```
go test ./... -v
```

Expected: all tests PASS, no compilation errors.

**Step 2: Build the binary**

```
go build -o /tmp/gitdeck ./cmd/gitdeck
```

Expected: binary produced without errors.
