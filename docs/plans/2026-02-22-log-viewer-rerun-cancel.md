# Log Viewer + Re-run / Cancel Pipeline — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add fullscreen job log viewing and pipeline re-run/cancel with confirmation to the gitdeck TUI.

**Architecture:** Extend `PipelineProvider` with three new methods (`GetJobLogs`, `RerunPipeline`, `CancelPipeline`), implement them in both the GitHub and GitLab adapters, then wire the new interactions into the existing Bubbletea `AppModel` state machine.

**Tech Stack:** Go 1.24, Bubbletea, net/http, httptest (tests).

---

### Task 1: Extend PipelineProvider interface

**Files:**
- Modify: `internal/domain/provider.go`

**Step 1: Write the failing test**

The interface is consumed by the adapters and faked in TUI tests. A compile error is our "failing test" here — adding the methods to the interface will break the build until both adapters implement them. Start by adding the methods.

**Step 2: Update `internal/domain/provider.go`**

Replace the existing interface body with:

```go
// PipelineProvider is the port interface that all CI provider adapters must implement.
type PipelineProvider interface {
    ListPipelines(repo Repository) ([]Pipeline, error)
    GetPipeline(repo Repository, id PipelineID) (Pipeline, error)

    // GetJobLogs returns the full raw log text for the given job ID.
    GetJobLogs(repo Repository, jobID string) (string, error)

    // RerunPipeline triggers a new run of the given pipeline.
    RerunPipeline(repo Repository, id PipelineID) error

    // CancelPipeline cancels a running pipeline.
    CancelPipeline(repo Repository, id PipelineID) error
}
```

**Step 3: Run the build to confirm it breaks**

```bash
go build ./...
```

Expected: compile errors in `internal/provider/github/adapter.go` and `internal/provider/gitlab/adapter.go` — "does not implement PipelineProvider".

**Step 4: Commit stub**

```bash
git add internal/domain/provider.go
git commit -m "feat: extend PipelineProvider with GetJobLogs, RerunPipeline, CancelPipeline"
```

---

### Task 2: GitHub adapter — GetJobLogs

**Files:**
- Modify: `internal/provider/github/adapter.go`
- Modify: `internal/provider/github/adapter_test.go`

**Step 1: Write the failing test**

Add to `adapter_test.go`:

```go
func TestGetJobLogs_ReturnsLogText(t *testing.T) {
    expectedLog := "##[group]Set up job\nRun actions/checkout@v4\n##[endgroup]\nok all tests pass"

    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if r.URL.Path == "/repos/waabox/gitdeck/actions/jobs/2001/logs" {
            w.Header().Set("Content-Type", "text/plain")
            fmt.Fprint(w, expectedLog)
            return
        }
        http.NotFound(w, r)
    }))
    defer srv.Close()

    adapter := githubprovider.NewAdapter("test-token", srv.URL, 3)
    repo := domain.Repository{Owner: "waabox", Name: "gitdeck"}

    logs, err := adapter.GetJobLogs(repo, "2001")
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if logs != expectedLog {
        t.Errorf("expected log text %q, got %q", expectedLog, logs)
    }
}
```

**Step 2: Run to confirm it fails**

```bash
go test ./internal/provider/github/... -run TestGetJobLogs -v
```

Expected: compile error — method not found.

**Step 3: Add `getText` helper and implement `GetJobLogs`**

In `internal/provider/github/adapter.go`, add `"io"` to the imports, then add after the existing `get` method:

```go
// getText fetches a URL and returns the response body as a string.
// It follows redirects (Go's default) and strips the Authorization header
// on cross-domain redirects, which is the correct behaviour for GitHub's
// log endpoint that redirects to a pre-signed S3 URL.
func (a *Adapter) getText(url string) (string, error) {
    req, err := http.NewRequest(http.MethodGet, url, nil)
    if err != nil {
        return "", fmt.Errorf("creating request: %w", err)
    }
    req.Header.Set("Authorization", "Bearer "+a.token)
    req.Header.Set("Accept", "application/vnd.github+json")

    resp, err := a.client.Do(req)
    if err != nil {
        return "", fmt.Errorf("executing request: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode >= 400 {
        return "", fmt.Errorf("github API error: %s", resp.Status)
    }
    b, err := io.ReadAll(resp.Body)
    if err != nil {
        return "", fmt.Errorf("reading log response: %w", err)
    }
    return string(b), nil
}

// GetJobLogs returns the full raw log text for the given job.
// GitHub returns a 302 redirect to a pre-signed S3 URL; the HTTP client
// follows it automatically.
func (a *Adapter) GetJobLogs(repo domain.Repository, jobID string) (string, error) {
    url := fmt.Sprintf("%s/repos/%s/%s/actions/jobs/%s/logs",
        a.baseURL, repo.Owner, repo.Name, jobID)
    return a.getText(url)
}
```

**Step 4: Run to confirm it passes**

```bash
go test ./internal/provider/github/... -run TestGetJobLogs -v
```

Expected: PASS.

**Step 5: Commit**

```bash
git add internal/provider/github/adapter.go internal/provider/github/adapter_test.go
git commit -m "feat: implement GitHub GetJobLogs"
```

---

### Task 3: GitHub adapter — RerunPipeline + CancelPipeline

**Files:**
- Modify: `internal/provider/github/adapter.go`
- Modify: `internal/provider/github/adapter_test.go`

**Step 1: Write the failing tests**

Add to `adapter_test.go`:

```go
func TestRerunPipeline_PostsToCorrectEndpoint(t *testing.T) {
    rerunCalled := false

    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if r.Method == http.MethodPost && r.URL.Path == "/repos/waabox/gitdeck/actions/runs/1001/rerun" {
            rerunCalled = true
            w.WriteHeader(http.StatusNoContent)
            return
        }
        http.NotFound(w, r)
    }))
    defer srv.Close()

    adapter := githubprovider.NewAdapter("test-token", srv.URL, 3)
    repo := domain.Repository{Owner: "waabox", Name: "gitdeck"}

    err := adapter.RerunPipeline(repo, "1001")
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if !rerunCalled {
        t.Error("expected rerun endpoint to be called")
    }
}

func TestCancelPipeline_PostsToCorrectEndpoint(t *testing.T) {
    cancelCalled := false

    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if r.Method == http.MethodPost && r.URL.Path == "/repos/waabox/gitdeck/actions/runs/1001/cancel" {
            cancelCalled = true
            w.WriteHeader(http.StatusAccepted)
            return
        }
        http.NotFound(w, r)
    }))
    defer srv.Close()

    adapter := githubprovider.NewAdapter("test-token", srv.URL, 3)
    repo := domain.Repository{Owner: "waabox", Name: "gitdeck"}

    err := adapter.CancelPipeline(repo, "1001")
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if !cancelCalled {
        t.Error("expected cancel endpoint to be called")
    }
}
```

**Step 2: Run to confirm they fail**

```bash
go test ./internal/provider/github/... -run "TestRerunPipeline|TestCancelPipeline" -v
```

Expected: compile error.

**Step 3: Add `post` helper and implement both methods**

In `adapter.go`, add after `getText`:

```go
// post sends a POST request with no body and discards the response body.
// GitHub mutation endpoints (rerun, cancel) return 204 or 202 with no body.
func (a *Adapter) post(url string) error {
    req, err := http.NewRequest(http.MethodPost, url, nil)
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
    return nil
}

// RerunPipeline triggers a new run of the given workflow run.
func (a *Adapter) RerunPipeline(repo domain.Repository, id domain.PipelineID) error {
    url := fmt.Sprintf("%s/repos/%s/%s/actions/runs/%s/rerun",
        a.baseURL, repo.Owner, repo.Name, id)
    return a.post(url)
}

// CancelPipeline cancels a running workflow run.
func (a *Adapter) CancelPipeline(repo domain.Repository, id domain.PipelineID) error {
    url := fmt.Sprintf("%s/repos/%s/%s/actions/runs/%s/cancel",
        a.baseURL, repo.Owner, repo.Name, id)
    return a.post(url)
}
```

**Step 4: Run all GitHub adapter tests**

```bash
go test ./internal/provider/github/... -v
```

Expected: all PASS.

**Step 5: Commit**

```bash
git add internal/provider/github/adapter.go internal/provider/github/adapter_test.go
git commit -m "feat: implement GitHub RerunPipeline and CancelPipeline"
```

---

### Task 4: GitLab adapter — GetJobLogs

**Files:**
- Modify: `internal/provider/gitlab/adapter.go`
- Modify: `internal/provider/gitlab/adapter_test.go`

**Step 1: Write the failing test**

Add to `internal/provider/gitlab/adapter_test.go`:

```go
func TestGetJobLogs_ReturnsLogText(t *testing.T) {
    expectedLog := "Running with gitlab-runner...\nok  all tests pass"

    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if r.URL.Path == "/api/v4/projects/waabox%2Fgitdeck/jobs/3001/trace" {
            w.Header().Set("Content-Type", "text/plain")
            fmt.Fprint(w, expectedLog)
            return
        }
        http.NotFound(w, r)
    }))
    defer srv.Close()

    adapter := gitlabprovider.NewAdapter("test-token", srv.URL, 3)
    repo := domain.Repository{Owner: "waabox", Name: "gitdeck"}

    logs, err := adapter.GetJobLogs(repo, "3001")
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if logs != expectedLog {
        t.Errorf("expected log text %q, got %q", expectedLog, logs)
    }
}
```

Note: check the import alias for the GitLab package in the existing test file first.

**Step 2: Run to confirm it fails**

```bash
go test ./internal/provider/gitlab/... -run TestGetJobLogs -v
```

Expected: compile error.

**Step 3: Add `getText` helper and implement `GetJobLogs`**

In `internal/provider/gitlab/adapter.go`, add `"io"` to the imports, then add after the existing `get` method:

```go
// getText fetches a URL and returns the response body as a string.
func (a *Adapter) getText(apiURL string) (string, error) {
    req, err := http.NewRequest(http.MethodGet, apiURL, nil)
    if err != nil {
        return "", fmt.Errorf("creating request: %w", err)
    }
    req.Header.Set("PRIVATE-TOKEN", a.token)

    resp, err := a.client.Do(req)
    if err != nil {
        return "", fmt.Errorf("executing request: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode >= 400 {
        return "", fmt.Errorf("gitlab API error: %s", resp.Status)
    }
    b, err := io.ReadAll(resp.Body)
    if err != nil {
        return "", fmt.Errorf("reading log response: %w", err)
    }
    return string(b), nil
}

// GetJobLogs returns the full raw log trace for the given job.
func (a *Adapter) GetJobLogs(repo domain.Repository, jobID string) (string, error) {
    projectID := url.PathEscape(repo.Owner + "/" + repo.Name)
    apiURL := fmt.Sprintf("%s/api/v4/projects/%s/jobs/%s/trace",
        a.baseURL, projectID, jobID)
    return a.getText(apiURL)
}
```

**Step 4: Run to confirm it passes**

```bash
go test ./internal/provider/gitlab/... -run TestGetJobLogs -v
```

Expected: PASS.

**Step 5: Commit**

```bash
git add internal/provider/gitlab/adapter.go internal/provider/gitlab/adapter_test.go
git commit -m "feat: implement GitLab GetJobLogs"
```

---

### Task 5: GitLab adapter — RerunPipeline + CancelPipeline

**Files:**
- Modify: `internal/provider/gitlab/adapter.go`
- Modify: `internal/provider/gitlab/adapter_test.go`

**Step 1: Write the failing tests**

Add to `adapter_test.go`:

```go
func TestRerunPipeline_PostsToCorrectEndpoint(t *testing.T) {
    rerunCalled := false

    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if r.Method == http.MethodPost && r.URL.Path == "/api/v4/projects/waabox%2Fgitdeck/pipelines/5001/retry" {
            rerunCalled = true
            w.WriteHeader(http.StatusCreated)
            return
        }
        http.NotFound(w, r)
    }))
    defer srv.Close()

    adapter := gitlabprovider.NewAdapter("test-token", srv.URL, 3)
    repo := domain.Repository{Owner: "waabox", Name: "gitdeck"}

    err := adapter.RerunPipeline(repo, "5001")
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if !rerunCalled {
        t.Error("expected retry endpoint to be called")
    }
}

func TestCancelPipeline_PostsToCorrectEndpoint(t *testing.T) {
    cancelCalled := false

    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if r.Method == http.MethodPost && r.URL.Path == "/api/v4/projects/waabox%2Fgitdeck/pipelines/5001/cancel" {
            cancelCalled = true
            w.WriteHeader(http.StatusOK)
            return
        }
        http.NotFound(w, r)
    }))
    defer srv.Close()

    adapter := gitlabprovider.NewAdapter("test-token", srv.URL, 3)
    repo := domain.Repository{Owner: "waabox", Name: "gitdeck"}

    err := adapter.CancelPipeline(repo, "5001")
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if !cancelCalled {
        t.Error("expected cancel endpoint to be called")
    }
}
```

**Step 2: Run to confirm they fail**

```bash
go test ./internal/provider/gitlab/... -run "TestRerunPipeline|TestCancelPipeline" -v
```

Expected: compile error.

**Step 3: Add `post` helper and implement both methods**

In `adapter.go`, add after `getText`:

```go
// post sends a POST request with no body and discards the response body.
func (a *Adapter) post(apiURL string) error {
    req, err := http.NewRequest(http.MethodPost, apiURL, nil)
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
    return nil
}

// RerunPipeline retries a failed or cancelled pipeline.
func (a *Adapter) RerunPipeline(repo domain.Repository, id domain.PipelineID) error {
    projectID := url.PathEscape(repo.Owner + "/" + repo.Name)
    apiURL := fmt.Sprintf("%s/api/v4/projects/%s/pipelines/%s/retry",
        a.baseURL, projectID, id)
    return a.post(apiURL)
}

// CancelPipeline cancels a running pipeline.
func (a *Adapter) CancelPipeline(repo domain.Repository, id domain.PipelineID) error {
    projectID := url.PathEscape(repo.Owner + "/" + repo.Name)
    apiURL := fmt.Sprintf("%s/api/v4/projects/%s/pipelines/%s/cancel",
        a.baseURL, projectID, id)
    return a.post(apiURL)
}
```

**Step 4: Run all GitLab adapter tests**

```bash
go test ./internal/provider/gitlab/... -v
```

Expected: all PASS.

**Step 5: Run full test suite to confirm nothing is broken**

```bash
go test ./...
```

Expected: all PASS. The build is now green — all three interface methods are implemented.

**Step 6: Commit**

```bash
git add internal/provider/gitlab/adapter.go internal/provider/gitlab/adapter_test.go
git commit -m "feat: implement GitLab RerunPipeline and CancelPipeline"
```

---

### Task 6: TUI — Re-run / Cancel with confirmation

**Files:**
- Modify: `internal/tui/app.go`
- Create: `internal/tui/app_test.go`

**Context:** `app.go` currently binds `r` to manual refresh. We move that to `ctrl+r` and use `r` for re-run. The confirmation is an inline prompt rendered at the bottom of the pipeline list area.

**Step 1: Write failing tests**

Create `internal/tui/app_test.go`:

```go
package tui_test

import (
    "testing"

    tea "github.com/charmbracelet/bubbletea"
    "github.com/waabox/gitdeck/internal/domain"
    "github.com/waabox/gitdeck/internal/tui"
)

// fakeProvider is a minimal domain.PipelineProvider for TUI tests.
type fakeProvider struct {
    pipelines    []domain.Pipeline
    rerunCalled  bool
    cancelCalled bool
}

func (f *fakeProvider) ListPipelines(_ domain.Repository) ([]domain.Pipeline, error) {
    return f.pipelines, nil
}
func (f *fakeProvider) GetPipeline(_ domain.Repository, _ domain.PipelineID) (domain.Pipeline, error) {
    return domain.Pipeline{}, nil
}
func (f *fakeProvider) GetJobLogs(_ domain.Repository, _ string) (string, error) {
    return "log output", nil
}
func (f *fakeProvider) RerunPipeline(_ domain.Repository, _ domain.PipelineID) error {
    f.rerunCalled = true
    return nil
}
func (f *fakeProvider) CancelPipeline(_ domain.Repository, _ domain.PipelineID) error {
    f.cancelCalled = true
    return nil
}

func TestApp_RerunKey_ShowsConfirmPrompt(t *testing.T) {
    provider := &fakeProvider{
        pipelines: []domain.Pipeline{{ID: "1001", Branch: "main", Status: domain.StatusFailed}},
    }
    m := tui.NewAppModel(domain.Repository{Owner: "waabox", Name: "gitdeck"}, provider)

    updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
    view := updated.(tui.AppModel).View()

    if !containsString(view, "Rerun pipeline") {
        t.Errorf("expected confirm prompt in view, got:\n%s", view)
    }
}

func TestApp_CancelKey_ShowsConfirmPrompt(t *testing.T) {
    provider := &fakeProvider{
        pipelines: []domain.Pipeline{{ID: "1001", Branch: "main", Status: domain.StatusRunning}},
    }
    m := tui.NewAppModel(domain.Repository{Owner: "waabox", Name: "gitdeck"}, provider)

    updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
    view := updated.(tui.AppModel).View()

    if !containsString(view, "Cancel pipeline") {
        t.Errorf("expected confirm prompt in view, got:\n%s", view)
    }
}

func TestApp_ConfirmRerun_DismissesPromptOnOtherKey(t *testing.T) {
    provider := &fakeProvider{
        pipelines: []domain.Pipeline{{ID: "1001", Branch: "main", Status: domain.StatusFailed}},
    }
    m := tui.NewAppModel(domain.Repository{Owner: "waabox", Name: "gitdeck"}, provider)

    m1, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
    m2, _ := m1.(tui.AppModel).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
    view := m2.(tui.AppModel).View()

    if containsString(view, "Rerun pipeline") {
        t.Errorf("expected confirm prompt to be dismissed, got:\n%s", view)
    }
}

func containsString(s, substr string) bool {
    return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStringHelper(s, substr))
}

func containsStringHelper(s, substr string) bool {
    for i := 0; i <= len(s)-len(substr); i++ {
        if s[i:i+len(substr)] == substr {
            return true
        }
    }
    return false
}
```

Note: `containsString` is a simple helper since we avoid importing `strings` just for Contains in tests — actually, `strings.Contains` is fine here. Replace with:

```go
import "strings"
// ...
if !strings.Contains(view, "Rerun pipeline") {
```

Remove the `containsString` helper and use `strings.Contains` directly.

**Step 2: Run to confirm they fail**

```bash
go test ./internal/tui/... -run "TestApp_Rerun|TestApp_Cancel|TestApp_Confirm" -v
```

Expected: compile errors — `tui.AppModel` is not exported as a type (it is — it's already public), `NewAppModel` is public. The tests should compile but the confirm prompt won't be in the view yet.

**Step 3: Add confirm state to `AppModel`**

In `internal/tui/app.go`, update the `AppModel` struct:

```go
type AppModel struct {
    repo          domain.Repository
    provider      domain.PipelineProvider
    list          PipelineListModel
    detail        JobDetailModel
    focus         focusPanel
    loading       bool
    err           error
    width         int
    height        int
    confirmAction string // "rerun" | "cancel" | ""
}
```

**Step 4: Add new message type**

Below the existing message type declarations in `app.go`:

```go
// actionResultMsg is sent when a pipeline action (rerun, cancel) completes.
type actionResultMsg struct {
    action string
    err    error
}
```

**Step 5: Add provider command helpers**

Below `loadPipelineDetail` in `app.go`:

```go
func (m AppModel) rerunPipeline(id string) tea.Cmd {
    return func() tea.Msg {
        err := m.provider.RerunPipeline(m.repo, domain.PipelineID(id))
        return actionResultMsg{action: "rerun", err: err}
    }
}

func (m AppModel) cancelPipeline(id string) tea.Cmd {
    return func() tea.Msg {
        err := m.provider.CancelPipeline(m.repo, domain.PipelineID(id))
        return actionResultMsg{action: "cancel", err: err}
    }
}
```

**Step 6: Update the key handler in `Update`**

In the `tea.KeyMsg` switch, make the following changes:

- Change `case "r":` (refresh) to `case "ctrl+r":`
- Add new cases:

```go
case "r":
    if m.confirmAction == "" {
        m.confirmAction = "rerun"
        return m, nil
    }
case "x":
    if m.confirmAction == "" {
        m.confirmAction = "cancel"
        return m, nil
    }
case "y":
    if m.confirmAction == "rerun" {
        m.confirmAction = ""
        return m, m.rerunPipeline(m.list.SelectedPipeline().ID)
    }
    if m.confirmAction == "cancel" {
        m.confirmAction = ""
        return m, m.cancelPipeline(m.list.SelectedPipeline().ID)
    }
```

And add a catch-all to dismiss the confirmation on any other key, at the top of the `tea.KeyMsg` case:

```go
case tea.KeyMsg:
    // Pressing any key while a confirm prompt is shown (other than y/q) dismisses it.
    if m.confirmAction != "" {
        key := msg.String()
        if key != "y" && key != "q" && key != "ctrl+c" {
            m.confirmAction = ""
            return m, nil
        }
    }
    switch msg.String() {
    // ... existing cases ...
```

Also handle `actionResultMsg` in the message switch (add above `tea.KeyMsg`):

```go
case actionResultMsg:
    if msg.err != nil {
        m.err = msg.err
        return m, nil
    }
    return m, m.loadPipelines()
```

**Step 7: Render the confirm prompt in `View`**

In the `View()` method, update the `footer` line to show the prompt when active:

```go
footer := " ↑/↓: navigate   tab: switch panel   enter: select/expand   ctrl+r: refresh   r: rerun   x: cancel   q: quit\n"
if m.confirmAction == "rerun" {
    selected := m.list.SelectedPipeline()
    footer = fmt.Sprintf(" Rerun pipeline #%s on %s? [y/N] ", selected.ID, selected.Branch)
}
if m.confirmAction == "cancel" {
    selected := m.list.SelectedPipeline()
    footer = fmt.Sprintf(" Cancel pipeline #%s on %s? [y/N] ", selected.ID, selected.Branch)
}
```

Also update the header from `r:refresh` to `ctrl+r:refresh  r:rerun  x:cancel`.

**Step 8: Run the tests**

```bash
go test ./internal/tui/... -v
```

Expected: all PASS.

**Step 9: Run full suite**

```bash
go test ./...
```

Expected: all PASS.

**Step 10: Commit**

```bash
git add internal/tui/app.go internal/tui/app_test.go
git commit -m "feat: add pipeline rerun and cancel with confirmation prompt"
```

---

### Task 7: TUI — Log viewer (fullscreen mode)

**Files:**
- Modify: `internal/tui/app.go`
- Create: `internal/tui/logview_test.go`

**Context:** Pressing `l` while the job detail panel is focused fetches the logs for the selected job and switches the app into a fullscreen `logView` mode. `Esc` returns to the normal two-panel view.

**Step 1: Write failing tests**

Create `internal/tui/logview_test.go`:

```go
package tui_test

import (
    "strings"
    "testing"

    tea "github.com/charmbracelet/bubbletea"
    "github.com/waabox/gitdeck/internal/domain"
    "github.com/waabox/gitdeck/internal/tui"
)

func TestApp_LogKey_ShowsLoadingMessage(t *testing.T) {
    provider := &fakeProvider{
        pipelines: []domain.Pipeline{{
            ID: "1001", Branch: "main", Status: domain.StatusFailed,
            Jobs: []domain.Job{{ID: "2001", Name: "test", Status: domain.StatusFailed}},
        }},
    }
    m := tui.NewAppModel(domain.Repository{Owner: "waabox", Name: "gitdeck"}, provider)

    // Focus the detail panel first, then press l.
    m1, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("\t")})
    m2, _ := m1.(tui.AppModel).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})

    view := m2.(tui.AppModel).View()
    if !strings.Contains(view, "Loading logs") {
        t.Errorf("expected loading message, got:\n%s", view)
    }
}

func TestApp_LogsLoaded_RendersLogContent(t *testing.T) {
    provider := &fakeProvider{
        pipelines: []domain.Pipeline{{
            ID: "1001", Branch: "main", Status: domain.StatusFailed,
            Jobs: []domain.Job{{ID: "2001", Name: "test", Status: domain.StatusFailed}},
        }},
    }
    m := tui.NewAppModel(domain.Repository{Owner: "waabox", Name: "gitdeck"}, provider)

    // Simulate logs already loaded by sending the message directly.
    m1, _ := m.Update(tui.LogsLoadedMsg{Content: "line1\nline2\nline3", JobName: "test", Err: nil})
    view := m1.(tui.AppModel).View()

    if !strings.Contains(view, "line1") {
        t.Errorf("expected log content in view, got:\n%s", view)
    }
    if !strings.Contains(view, "[logs]") {
        t.Errorf("expected [logs] header in view, got:\n%s", view)
    }
}

func TestApp_LogView_EscReturnsToNormalView(t *testing.T) {
    provider := &fakeProvider{
        pipelines: []domain.Pipeline{{ID: "1001", Branch: "main", Status: domain.StatusFailed}},
    }
    m := tui.NewAppModel(domain.Repository{Owner: "waabox", Name: "gitdeck"}, provider)

    m1, _ := m.Update(tui.LogsLoadedMsg{Content: "line1\nline2", JobName: "test", Err: nil})
    m2, _ := m1.(tui.AppModel).Update(tea.KeyMsg{Type: tea.KeyEsc})
    view := m2.(tui.AppModel).View()

    if strings.Contains(view, "[logs]") {
        t.Errorf("expected to exit log view on esc, got:\n%s", view)
    }
}
```

Note: `tui.LogsLoadedMsg` must be exported so the test can inject it directly. Name it `LogsLoadedMsg` (capital L) in the implementation.

**Step 2: Run to confirm they fail**

```bash
go test ./internal/tui/... -run "TestApp_Log" -v
```

Expected: compile errors — `tui.LogsLoadedMsg` not found yet.

**Step 3: Add log view state and message types to `app.go`**

Update `AppModel` struct:

```go
type AppModel struct {
    repo          domain.Repository
    provider      domain.PipelineProvider
    list          PipelineListModel
    detail        JobDetailModel
    focus         focusPanel
    loading       bool
    err           error
    width         int
    height        int
    confirmAction string
    // Log viewer state
    logMode    bool
    logLoading bool
    logContent string
    logOffset  int
    logJobName string
}
```

Add the exported message type (exported so tests can inject it):

```go
// LogsLoadedMsg is sent when job logs have been fetched from the provider.
// Exported so tests can inject it directly.
type LogsLoadedMsg struct {
    Content string
    JobName string
    Err     error
}
```

**Step 4: Add the log fetch command**

```go
func (m AppModel) loadJobLogs(job domain.Job) tea.Cmd {
    return func() tea.Msg {
        content, err := m.provider.GetJobLogs(m.repo, job.ID)
        return LogsLoadedMsg{Content: content, JobName: job.Name, Err: err}
    }
}
```

**Step 5: Handle `LogsLoadedMsg` in `Update`**

Add before the `tea.KeyMsg` case:

```go
case LogsLoadedMsg:
    if msg.Err != nil {
        m.logLoading = false
        m.err = msg.Err
        return m, nil
    }
    m.logLoading = false
    m.logMode = true
    m.logContent = msg.Content
    m.logJobName = msg.JobName
    m.logOffset = 0
    return m, nil
```

**Step 6: Handle `l` key and log scroll keys in `Update`**

Inside the `tea.KeyMsg` switch, add:

```go
case "l":
    if m.focus == focusDetail && !m.logMode {
        jobs := m.detail.Jobs()
        if len(jobs) > 0 {
            m.logLoading = true
            return m, m.loadJobLogs(jobs[m.detail.Cursor()])
        }
    }
```

For log scroll, add these cases (they should only act when `m.logMode` is true):

```go
case "pgup":
    if m.logMode {
        page := m.visibleLogLines()
        if m.logOffset-page >= 0 {
            m.logOffset -= page
        } else {
            m.logOffset = 0
        }
    }
case "pgdown":
    if m.logMode {
        m.logOffset += m.visibleLogLines()
    }
case "g":
    if m.logMode {
        m.logOffset = 0
    }
case "G":
    if m.logMode {
        lines := strings.Count(m.logContent, "\n")
        m.logOffset = lines
    }
```

The existing `up`/`down` cases need to also handle log scrolling when in log mode:

```go
case "down":
    if m.logMode {
        m.logOffset++
        return m, nil
    }
    // ... existing pipeline/job navigation ...
case "up":
    if m.logMode {
        if m.logOffset > 0 {
            m.logOffset--
        }
        return m, nil
    }
    // ... existing pipeline/job navigation ...
```

The `esc` case needs to exit log mode first:

```go
case "esc":
    if m.logMode {
        m.logMode = false
        m.logContent = ""
        m.logOffset = 0
        return m, nil
    }
    m.focus = focusList
```

**Step 7: Add `visibleLogLines` helper**

```go
func (m AppModel) visibleLogLines() int {
    lines := m.height - 4 // header + separator + footer
    if lines < 1 {
        return 10
    }
    return lines
}
```

**Step 8: Add `Jobs()` accessor to `JobDetailModel`**

The log fetch needs the selected job. Add to `internal/tui/jobdetail.go`:

```go
// Jobs returns the full job slice.
func (m JobDetailModel) Jobs() []domain.Job {
    return m.jobs
}
```

**Step 9: Render the log view in `View()`**

At the top of the `View()` method, before the loading/error checks, add:

```go
if m.logLoading {
    return "Loading logs...\n"
}
if m.logMode {
    return m.renderLogView()
}
```

Add `renderLogView` as a method on `AppModel` (can be in `app.go` or a new file `internal/tui/logview.go` — either works, but keeping it in `app.go` is simpler):

```go
func (m AppModel) renderLogView() string {
    header := fmt.Sprintf(" gitdeck  %s/%s  [logs] %s\n",
        m.repo.Owner, m.repo.Name, m.logJobName)
    separator := "────────────────────────────────────────────────────────────\n"
    footer := " ↑/↓: scroll   PgUp/PgDn: page   g/G: top/bottom   esc: back\n"

    lines := strings.Split(m.logContent, "\n")
    visibleCount := m.visibleLogLines()

    start := m.logOffset
    if start >= len(lines) {
        start = max(0, len(lines)-1)
    }
    end := start + visibleCount
    if end > len(lines) {
        end = len(lines)
    }

    body := strings.Join(lines[start:end], "\n")
    return header + separator + body + "\n" + separator + footer
}

func max(a, b int) int {
    if a > b {
        return a
    }
    return b
}
```

Note: Go 1.21+ has a built-in `max`. If using Go 1.24, you can remove the `max` helper and use the built-in directly.

Also add `"strings"` to the imports in `app.go` since it now uses `strings.Count` and `strings.Split`.

**Step 10: Run all tests**

```bash
go test ./internal/tui/... -v
```

Expected: all PASS.

**Step 11: Run full suite**

```bash
go test ./...
```

Expected: all PASS.

**Step 12: Commit**

```bash
git add internal/tui/app.go internal/tui/jobdetail.go internal/tui/logview_test.go
git commit -m "feat: add fullscreen job log viewer with scroll"
```

---

### Task 8: Update README keyboard shortcuts

**Files:**
- Modify: `README.md`

**Step 1: Update the keyboard shortcuts table**

Replace the existing shortcuts table with:

```markdown
| Key              | Action                                       |
|------------------|----------------------------------------------|
| `↑` / `↓`        | Navigate pipelines or jobs / scroll logs     |
| `Enter`          | Select pipeline, focus job detail panel      |
| `Tab`            | Switch focus between panels                  |
| `l`              | View full logs for selected job (fullscreen) |
| `r`              | Re-run selected pipeline (asks confirmation) |
| `x`              | Cancel selected pipeline (asks confirmation) |
| `PgUp` / `PgDn`  | Scroll logs by page (in log viewer)          |
| `g` / `G`        | Jump to top / bottom of log                  |
| `Esc`            | Exit log viewer / return focus to pipelines  |
| `ctrl+r`         | Refresh pipelines now                        |
| `q` / `Ctrl+C`   | Quit                                         |
```

Also update the Features section to mention the new capabilities.

**Step 2: Commit**

```bash
git add README.md
git commit -m "docs: update keyboard shortcuts for log viewer and rerun/cancel"
```

---

### Final verification

```bash
go test ./...
go build ./...
```

Both must exit 0 with no errors before declaring the work complete.
