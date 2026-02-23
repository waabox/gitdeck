# Hierarchical Navigation: Pipeline > Jobs > Steps

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace the dual-panel layout with a drill-down navigation: Pipelines list → Jobs list → Steps list, each as a full-width screen.

**Architecture:** Replace the `focusPanel` enum with a `viewState` enum (`viewPipelines`, `viewJobs`, `viewSteps`, `viewLogs`). Each state owns its own keyboard handling and full-width rendering. `Enter` navigates deeper, `Esc` navigates back. A new `StepListModel` handles the third level.

**Tech Stack:** Go, Bubble Tea, existing domain models.

---

### Task 1: Create StepListModel

**Files:**
- Create: `internal/tui/steplist.go`
- Create: `internal/tui/steplist_test.go`

**Step 1: Write the failing tests**

```go
// internal/tui/steplist_test.go
package tui_test

import (
	"strings"
	"testing"
	"time"

	"github.com/waabox/gitdeck/internal/domain"
	"github.com/waabox/gitdeck/internal/tui"
)

func TestStepListModel_RendersSteps(t *testing.T) {
	steps := []domain.Step{
		{Name: "checkout", Status: domain.StatusSuccess, Duration: 2 * time.Second},
		{Name: "run tests", Status: domain.StatusFailed, Duration: 31 * time.Second},
	}
	m := tui.NewStepListModel(steps)
	view := m.View()
	if !strings.Contains(view, "checkout") {
		t.Errorf("expected 'checkout' in view, got:\n%s", view)
	}
	if !strings.Contains(view, "run tests") {
		t.Errorf("expected 'run tests' in view, got:\n%s", view)
	}
}

func TestStepListModel_EmptyShowsMessage(t *testing.T) {
	m := tui.NewStepListModel(nil)
	view := m.View()
	if !strings.Contains(view, "No steps") {
		t.Errorf("expected empty message, got:\n%s", view)
	}
}

func TestStepListModel_NavigateDown(t *testing.T) {
	steps := []domain.Step{
		{Name: "checkout", Status: domain.StatusSuccess},
		{Name: "run tests", Status: domain.StatusFailed},
	}
	m := tui.NewStepListModel(steps)
	m = m.MoveDown()
	if m.Cursor() != 1 {
		t.Errorf("expected cursor 1, got %d", m.Cursor())
	}
}

func TestStepListModel_NavigateUp(t *testing.T) {
	steps := []domain.Step{
		{Name: "checkout", Status: domain.StatusSuccess},
		{Name: "run tests", Status: domain.StatusFailed},
	}
	m := tui.NewStepListModel(steps)
	m = m.MoveDown()
	m = m.MoveUp()
	if m.Cursor() != 0 {
		t.Errorf("expected cursor 0, got %d", m.Cursor())
	}
}

func TestStepListModel_DoesNotGoBelowMax(t *testing.T) {
	steps := []domain.Step{{Name: "only"}}
	m := tui.NewStepListModel(steps)
	m = m.MoveDown()
	if m.Cursor() != 0 {
		t.Errorf("expected cursor 0, got %d", m.Cursor())
	}
}

func TestStepListModel_DoesNotGoAboveZero(t *testing.T) {
	steps := []domain.Step{{Name: "only"}}
	m := tui.NewStepListModel(steps)
	m = m.MoveUp()
	if m.Cursor() != 0 {
		t.Errorf("expected cursor 0, got %d", m.Cursor())
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `cd /Users/waabox/code/waabox/gitdeck && go test ./internal/tui/ -run TestStepListModel -v`
Expected: compilation error — `tui.NewStepListModel` undefined.

**Step 3: Implement StepListModel**

```go
// internal/tui/steplist.go
package tui

import (
	"fmt"
	"strings"

	"github.com/waabox/gitdeck/internal/domain"
)

// StepListModel is an immutable model for the steps panel.
type StepListModel struct {
	steps  []domain.Step
	cursor int
}

// NewStepListModel creates a step list model.
func NewStepListModel(steps []domain.Step) StepListModel {
	return StepListModel{steps: steps, cursor: 0}
}

// MoveDown returns a new model with the cursor moved down by one.
func (m StepListModel) MoveDown() StepListModel {
	if m.cursor < len(m.steps)-1 {
		m.cursor++
	}
	return m
}

// MoveUp returns a new model with the cursor moved up by one.
func (m StepListModel) MoveUp() StepListModel {
	if m.cursor > 0 {
		m.cursor--
	}
	return m
}

// Cursor returns the current cursor position.
func (m StepListModel) Cursor() int {
	return m.cursor
}

// Steps returns the full step slice.
func (m StepListModel) Steps() []domain.Step {
	return m.steps
}

// View renders the step list as a string with cursor indicators.
func (m StepListModel) View() string {
	if len(m.steps) == 0 {
		return "No steps found."
	}
	var sb strings.Builder
	for i, s := range m.steps {
		prefix := "  "
		if i == m.cursor {
			prefix = "> "
		}
		duration := "--"
		if s.Duration > 0 {
			duration = fmt.Sprintf("%ds", int(s.Duration.Seconds()))
		}
		sb.WriteString(fmt.Sprintf("%s%s %-25s %s\n",
			prefix,
			statusIcon(s.Status),
			truncate(s.Name, 25),
			duration,
		))
	}
	return sb.String()
}
```

**Step 4: Run tests to verify they pass**

Run: `cd /Users/waabox/code/waabox/gitdeck && go test ./internal/tui/ -run TestStepListModel -v`
Expected: all 6 tests PASS.

**Step 5: Commit**

```
feat: add StepListModel for step-level navigation
```

---

### Task 2: Replace focusPanel with viewState in AppModel

**Files:**
- Modify: `internal/tui/app.go`

This task changes the state model but does NOT change Update/View yet (that's next tasks). We replace `focusPanel` with `viewState`, add new fields for tracking the selected job and step list, and remove the `expanded` map from `JobDetailModel`.

**Step 1: Replace focusPanel enum and AppModel fields**

In `app.go`, replace lines 43-69:

```go
// viewState indicates the current navigation level.
type viewState int

const (
	viewPipelines viewState = iota
	viewJobs
	viewSteps
	viewLogs
)

// AppModel is the root Bubbletea model for gitdeck.
type AppModel struct {
	repo     domain.Repository
	provider domain.PipelineProvider
	// Navigation
	view viewState
	// Pipeline level
	list             PipelineListModel
	selectedPipeline domain.Pipeline
	// Job level
	detail      JobDetailModel
	selectedJob domain.Job
	// Step level
	steps StepListModel
	// General state
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

Update `NewAppModel` to use `view: viewPipelines` instead of `focus`.

**Step 2: Run all existing tests**

Run: `cd /Users/waabox/code/waabox/gitdeck && go test ./internal/tui/ -v`
Expected: compilation errors in `app.go` because `Update` and `View` still reference `focusList`/`focusDetail`. That's OK — we fix those in the next tasks.

**Step 3: Do NOT commit yet** — this task is continued in Tasks 3 and 4 (they form one atomic commit together).

---

### Task 3: Rewrite AppModel.Update for hierarchical navigation

**Files:**
- Modify: `internal/tui/app.go` (Update method, lines 140-337)

Replace the entire `tea.KeyMsg` handling block. The new logic:

**viewPipelines keys:**
- `up`/`down`: navigate pipeline list (no auto-load of jobs)
- `enter`: set `selectedPipeline`, switch to `viewJobs`, fire `loadPipelineDetail`
- `r`/`x`: rerun/cancel confirmation (same as before)
- `ctrl+r`: force refresh
- `q`/`ctrl+c`: quit

**viewJobs keys:**
- `up`/`down`: navigate job list
- `enter`: set `selectedJob`, switch to `viewSteps`, populate `steps` from `selectedJob.Steps`
- `l`: load logs for selected job
- `esc`: switch back to `viewPipelines`
- `r`/`x`: rerun/cancel confirmation
- `q`/`ctrl+c`: quit

**viewSteps keys:**
- `up`/`down`: navigate step list
- `l`: load logs for `selectedJob` (same job, steps don't have separate logs)
- `esc`: switch back to `viewJobs`
- `q`/`ctrl+c`: quit

**viewLogs keys (unchanged from current logMode):**
- `up`/`down`: scroll
- `pgup`/`pgdown`: page
- `g`/`G`: top/bottom
- `esc`: switch back to previous view (viewJobs or viewSteps depending on where `l` was pressed)

Additional changes:
- `PipelinesLoadedMsg`: no longer auto-loads detail. Only updates list.
- `pipelineDetailMsg`: updates `detail` model. If already in `viewJobs`, refreshes in place.
- `tickMsg`: only refreshes pipeline list. If in `viewJobs` and selected pipeline is running, also refreshes detail.
- Remove `tab` key handling entirely.

**Step 1: Implement the rewritten Update method**

The full replacement for the `tea.KeyMsg` case (starting at line 233 in current code):

```go
case tea.KeyMsg:
    if m.confirmAction != "" {
        switch msg.String() {
        case "y":
            if m.selectedPipeline.ID == "" {
                m.confirmAction = ""
                return m, nil
            }
            action := m.confirmAction
            m.confirmAction = ""
            if action == "rerun" {
                return m, m.rerunPipeline(m.selectedPipeline.ID)
            }
            return m, m.cancelPipeline(m.selectedPipeline.ID)
        case "q", "ctrl+c":
            return m, tea.Quit
        default:
            m.confirmAction = ""
            return m, nil
        }
    }
    switch msg.String() {
    case "q", "ctrl+c":
        return m, tea.Quit
    case "ctrl+r":
        m.loading = true
        return m, m.loadPipelines()
    }
    switch m.view {
    case viewPipelines:
        return m.updatePipelines(msg)
    case viewJobs:
        return m.updateJobs(msg)
    case viewSteps:
        return m.updateSteps(msg)
    case viewLogs:
        return m.updateLogs(msg)
    }
```

Then add four private methods:

```go
func (m AppModel) updatePipelines(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
    switch msg.String() {
    case "down":
        m.list = m.list.MoveDown()
        m.selectedPipeline = m.list.SelectedPipeline()
    case "up":
        m.list = m.list.MoveUp()
        m.selectedPipeline = m.list.SelectedPipeline()
    case "enter":
        if len(m.list.Pipelines()) > 0 {
            m.selectedPipeline = m.list.SelectedPipeline()
            m.view = viewJobs
            return m, m.loadPipelineDetail(m.selectedPipeline.ID)
        }
    case "r":
        m.confirmAction = "rerun"
    case "x":
        m.confirmAction = "cancel"
    }
    return m, nil
}

func (m AppModel) updateJobs(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
    switch msg.String() {
    case "down":
        m.detail = m.detail.MoveDown()
    case "up":
        m.detail = m.detail.MoveUp()
    case "enter":
        jobs := m.detail.Jobs()
        if len(jobs) > 0 {
            m.selectedJob = jobs[m.detail.Cursor()]
            m.steps = NewStepListModel(m.selectedJob.Steps)
            m.view = viewSteps
        }
    case "l":
        if !m.logLoading {
            jobs := m.detail.Jobs()
            if len(jobs) > 0 {
                m.logLoading = true
                return m, m.loadJobLogs(jobs[m.detail.Cursor()])
            }
        }
    case "esc":
        m.view = viewPipelines
    case "r":
        m.confirmAction = "rerun"
    case "x":
        m.confirmAction = "cancel"
    }
    return m, nil
}

func (m AppModel) updateSteps(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
    switch msg.String() {
    case "down":
        m.steps = m.steps.MoveDown()
    case "up":
        m.steps = m.steps.MoveUp()
    case "l":
        if !m.logLoading {
            m.logLoading = true
            return m, m.loadJobLogs(m.selectedJob)
        }
    case "esc":
        m.view = viewJobs
    }
    return m, nil
}

func (m AppModel) updateLogs(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
    switch msg.String() {
    case "down":
        maxOffset := strings.Count(m.logContent, "\n")
        if m.logOffset < maxOffset {
            m.logOffset++
        }
    case "up":
        if m.logOffset > 0 {
            m.logOffset--
        }
    case "pgup":
        page := m.visibleLogLines()
        if m.logOffset-page >= 0 {
            m.logOffset -= page
        } else {
            m.logOffset = 0
        }
    case "pgdown":
        maxOffset := strings.Count(m.logContent, "\n")
        m.logOffset += m.visibleLogLines()
        if m.logOffset > maxOffset {
            m.logOffset = maxOffset
        }
    case "g":
        m.logOffset = 0
    case "G":
        lines := strings.Split(m.logContent, "\n")
        m.logOffset = len(lines) - 1
    case "esc":
        m.view = m.logReturnView
        m.logMode = false
        m.logContent = ""
        m.logOffset = 0
    }
    return m, nil
}
```

Note: Add `logReturnView viewState` field to `AppModel` so `Esc` from logs returns to the correct level. Set it when `l` is pressed (in `updateJobs`: `m.logReturnView = viewJobs`, in `updateSteps`: `m.logReturnView = viewSteps`).

Also update `PipelinesLoadedMsg` handler: on first load, don't auto-create detail. On subsequent loads, just update list. Remove auto-load of detail on pipeline navigation (detail only loads on `enter`).

Updated `PipelinesLoadedMsg` handler:

```go
case PipelinesLoadedMsg:
    m.loading = false
    if msg.Err != nil {
        m.err = msg.Err
        return m, nil
    }
    if len(m.list.Pipelines()) == 0 {
        m.list = NewPipelineListModel(msg.Pipelines)
        if len(msg.Pipelines) > 0 {
            m.selectedPipeline = msg.Pipelines[0]
        }
    } else {
        m.list = m.list.UpdatePipelines(msg.Pipelines)
        for _, p := range msg.Pipelines {
            if p.ID == m.selectedPipeline.ID {
                m.selectedPipeline = p
                break
            }
        }
    }
```

Updated `LogsLoadedMsg` handler — set `m.view = viewLogs` instead of `m.logMode = true`:

```go
case LogsLoadedMsg:
    m.logLoading = false
    if msg.Err != nil {
        m.err = msg.Err
        return m, nil
    }
    m.logReturnView = m.view
    m.view = viewLogs
    m.logMode = true
    m.logContent = msg.Content
    m.logJobName = msg.JobName
    m.logOffset = 0
    return m, nil
```

**Step 2: Do NOT commit yet** — continue to Task 4 (View rewrite).

---

### Task 4: Rewrite AppModel.View for hierarchical rendering

**Files:**
- Modify: `internal/tui/app.go` (View method, lines 340-390)

Replace the dual-panel `View()` with view-state-based rendering.

**Step 1: Implement the new View method**

```go
func (m AppModel) View() string {
    if m.logLoading {
        return "Loading logs...\n"
    }
    if m.view == viewLogs {
        return m.renderLogView()
    }
    if m.loading && m.confirmAction == "" {
        return "Loading pipelines...\n"
    }
    if m.err != nil {
        return fmt.Sprintf("Error: %v\n\nPress 'ctrl+r' to retry or 'q' to quit.\n", m.err)
    }

    header := fmt.Sprintf(" gitdeck | %s / ⎇ %s %s / %s\n",
        m.repo.Name, m.selectedPipeline.Branch,
        shortSHA(m.selectedPipeline.CommitSHA),
        firstLine(m.selectedPipeline.CommitMsg))
    separator := "────────────────────────────────────────────────────────────\n"

    switch m.view {
    case viewPipelines:
        return m.renderPipelinesView(header, separator)
    case viewJobs:
        return m.renderJobsView(header, separator)
    case viewSteps:
        return m.renderStepsView(header, separator)
    default:
        return header
    }
}

func (m AppModel) renderPipelinesView(header, separator string) string {
    title := " Pipelines\n"
    listView := m.list.View()
    statusBar := fmt.Sprintf(" #%s by %s\n", m.selectedPipeline.ID, m.selectedPipeline.Author)
    footer := " ↑/↓: navigate   enter: open   ctrl+r: refresh   r: rerun   x: cancel   q: quit\n"
    if m.confirmAction == "rerun" {
        footer = fmt.Sprintf(" Rerun pipeline #%s on %s? [y/N] \n",
            m.selectedPipeline.ID, m.selectedPipeline.Branch)
    }
    if m.confirmAction == "cancel" {
        footer = fmt.Sprintf(" Cancel pipeline #%s on %s? [y/N] \n",
            m.selectedPipeline.ID, m.selectedPipeline.Branch)
    }
    return header + separator + title + listView + "\n" + separator + statusBar + separator + footer
}

func (m AppModel) renderJobsView(header, separator string) string {
    title := fmt.Sprintf(" Jobs for Pipeline #%s\n", m.selectedPipeline.ID)
    detailView := m.detail.ViewFocused()
    footer := " ↑/↓: navigate   enter: steps   l: logs   esc: back   r: rerun   x: cancel   q: quit\n"
    if m.confirmAction == "rerun" {
        footer = fmt.Sprintf(" Rerun pipeline #%s on %s? [y/N] \n",
            m.selectedPipeline.ID, m.selectedPipeline.Branch)
    }
    if m.confirmAction == "cancel" {
        footer = fmt.Sprintf(" Cancel pipeline #%s on %s? [y/N] \n",
            m.selectedPipeline.ID, m.selectedPipeline.Branch)
    }
    return header + separator + title + detailView + "\n" + separator + footer
}

func (m AppModel) renderStepsView(header, separator string) string {
    title := fmt.Sprintf(" Steps for Job: %s\n", m.selectedJob.Name)
    stepsView := m.steps.View()
    footer := " ↑/↓: navigate   l: logs   esc: back   q: quit\n"
    return header + separator + title + stepsView + "\n" + separator + footer
}
```

**Step 2: Remove the `expanded` map from JobDetailModel**

In `jobdetail.go`, remove the `expanded` field and `ToggleExpand` method. Simplify the `render` method to not render steps inline. The `ViewFocused` method always shows the cursor. The `View` method (without cursor) is no longer needed but can remain for backward compatibility.

Updated `jobdetail.go` render method — remove the step expansion block (lines 93-110).

**Step 3: Run all tests**

Run: `cd /Users/waabox/code/waabox/gitdeck && go test ./internal/tui/ -v`
Expected: some existing tests will fail because they reference `tab` switching and `ToggleExpand`. Fix in Task 5.

**Step 4: Do NOT commit yet** — fix tests first in Task 5.

---

### Task 5: Update existing tests for new navigation flow

**Files:**
- Modify: `internal/tui/app_test.go`
- Modify: `internal/tui/logview_test.go`
- Modify: `internal/tui/jobdetail_test.go`

**Step 1: Update app_test.go**

Key changes:
- `TestApp_RerunKey_ShowsConfirmPrompt`: no change needed (r key works from viewPipelines)
- `TestApp_CancelKey_ShowsConfirmPrompt`: no change needed
- `TestApp_ConfirmRerun_DismissesPromptOnOtherKey`: no change needed
- `TestApp_ConfirmRerun_YKey_CallsProvider`: no change needed
- `TestApp_ConfirmCancel_YKey_CallsProvider`: no change needed
- `TestApp_RefreshPreservesSelection`: no change needed

**Step 2: Update logview_test.go**

`TestApp_LogKey_ShowsLoadingMessage`: Instead of pressing `tab` to switch to detail panel, press `enter` to drill into the pipeline (viewJobs), then press `l`.

```go
func TestApp_LogKey_ShowsLoadingMessage(t *testing.T) {
    provider := &fakeProvider{
        pipelines: []domain.Pipeline{{
            ID: "1001", Branch: "main", Status: domain.StatusFailed,
        }},
    }
    m := tui.NewAppModel(domain.Repository{Owner: "waabox", Name: "gitdeck"}, provider)

    // Seed pipelines
    m1, _ := m.Update(tui.PipelinesLoadedMsg{
        Pipelines: []domain.Pipeline{{
            ID: "1001", Branch: "main", Status: domain.StatusFailed,
            Jobs: []domain.Job{{ID: "2001", Name: "test", Status: domain.StatusFailed}},
        }},
    })
    // Press enter to drill into pipeline (switches to viewJobs)
    m2, _ := m1.(tui.AppModel).Update(tea.KeyMsg{Type: tea.KeyEnter})
    // Inject the detail response
    m3, _ := m2.(tui.AppModel).Update(tui.PipelineDetailMsg{
        Pipeline: domain.Pipeline{
            ID: "1001", Branch: "main", Status: domain.StatusFailed,
            Jobs: []domain.Job{{ID: "2001", Name: "test", Status: domain.StatusFailed}},
        },
    })
    // Press l to open logs
    m4, _ := m3.(tui.AppModel).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})

    view := m4.(tui.AppModel).View()
    if !strings.Contains(view, "Loading logs") {
        t.Errorf("expected loading message after pressing l, got:\n%s", view)
    }
}
```

Note: We need to export `pipelineDetailMsg` as `PipelineDetailMsg` so tests can inject it. Add this to `app.go`:

```go
// PipelineDetailMsg is sent when a pipeline detail (with jobs) has been fetched.
// Exported so tests can inject it.
type PipelineDetailMsg struct {
    Pipeline domain.Pipeline
    Err      error
}
```

Update internal references from `pipelineDetailMsg` to `PipelineDetailMsg` and field names from lowercase to uppercase.

`TestApp_LogView_EscReturnsToNormalView`: Should still work since LogsLoadedMsg sets viewLogs and esc returns.

`TestApp_LogView_ScrollDown_MovesOffset`: Should still work.

**Step 3: Update jobdetail_test.go**

Remove `ToggleExpand` tests (Task 2 removed ToggleExpand). Keep the basic rendering and empty-message tests.

Remove these test functions:
- `TestJobDetailModel_ToggleExpand_ShowsSteps`
- `TestJobDetailModel_ToggleExpand_HidesStepsOnSecondToggle`
- `TestJobDetailModel_ToggleExpand_NoSteps_DoesNothing`
- `TestJobDetailModel_ToggleExpand_OutOfBounds_DoesNothing`

**Step 4: Run all tests**

Run: `cd /Users/waabox/code/waabox/gitdeck && go test ./internal/tui/ -v`
Expected: all tests PASS.

**Step 5: Commit**

```
feat: replace dual-panel layout with hierarchical drill-down navigation

Pipelines, Jobs, and Steps are now separate full-width screens.
Enter drills down, Esc navigates back. Logs accessible from
both Jobs and Steps views via `l`.
```

---

### Task 6: Add new tests for hierarchical navigation

**Files:**
- Modify: `internal/tui/app_test.go`

**Step 1: Write tests for the drill-down flow**

```go
func TestApp_EnterDrillsIntoPipelineJobs(t *testing.T) {
    pipelines := []domain.Pipeline{
        {ID: "1001", Branch: "main", Status: domain.StatusSuccess},
    }
    provider := &fakeProvider{pipelines: pipelines}
    m := tui.NewAppModel(domain.Repository{Owner: "waabox", Name: "gitdeck"}, provider)

    m0, _ := m.Update(tui.PipelinesLoadedMsg{Pipelines: pipelines})
    m1, _ := m0.(tui.AppModel).Update(tea.KeyMsg{Type: tea.KeyEnter})

    // Inject detail response
    m2, _ := m1.(tui.AppModel).Update(tui.PipelineDetailMsg{
        Pipeline: domain.Pipeline{
            ID: "1001", Branch: "main",
            Jobs: []domain.Job{{ID: "j1", Name: "build", Status: domain.StatusSuccess}},
        },
    })
    view := m2.(tui.AppModel).View()
    if !strings.Contains(view, "Jobs for Pipeline #1001") {
        t.Errorf("expected jobs view header, got:\n%s", view)
    }
    if !strings.Contains(view, "build") {
        t.Errorf("expected job 'build' in view, got:\n%s", view)
    }
}

func TestApp_EscFromJobsReturnsToPipelines(t *testing.T) {
    pipelines := []domain.Pipeline{
        {ID: "1001", Branch: "main", Status: domain.StatusSuccess},
    }
    provider := &fakeProvider{pipelines: pipelines}
    m := tui.NewAppModel(domain.Repository{Owner: "waabox", Name: "gitdeck"}, provider)

    m0, _ := m.Update(tui.PipelinesLoadedMsg{Pipelines: pipelines})
    m1, _ := m0.(tui.AppModel).Update(tea.KeyMsg{Type: tea.KeyEnter})
    m2, _ := m1.(tui.AppModel).Update(tui.PipelineDetailMsg{
        Pipeline: domain.Pipeline{
            ID: "1001", Branch: "main",
            Jobs: []domain.Job{{ID: "j1", Name: "build"}},
        },
    })
    m3, _ := m2.(tui.AppModel).Update(tea.KeyMsg{Type: tea.KeyEsc})
    view := m3.(tui.AppModel).View()
    if !strings.Contains(view, "Pipelines") {
        t.Errorf("expected pipelines view after esc, got:\n%s", view)
    }
}

func TestApp_EnterFromJobsDrillsIntoSteps(t *testing.T) {
    pipelines := []domain.Pipeline{
        {ID: "1001", Branch: "main", Status: domain.StatusSuccess},
    }
    provider := &fakeProvider{pipelines: pipelines}
    m := tui.NewAppModel(domain.Repository{Owner: "waabox", Name: "gitdeck"}, provider)

    m0, _ := m.Update(tui.PipelinesLoadedMsg{Pipelines: pipelines})
    m1, _ := m0.(tui.AppModel).Update(tea.KeyMsg{Type: tea.KeyEnter})
    m2, _ := m1.(tui.AppModel).Update(tui.PipelineDetailMsg{
        Pipeline: domain.Pipeline{
            ID: "1001", Branch: "main",
            Jobs: []domain.Job{{
                ID: "j1", Name: "test",
                Steps: []domain.Step{
                    {Name: "checkout", Status: domain.StatusSuccess},
                    {Name: "run tests", Status: domain.StatusFailed},
                },
            }},
        },
    })
    // Enter on job to see steps
    m3, _ := m2.(tui.AppModel).Update(tea.KeyMsg{Type: tea.KeyEnter})
    view := m3.(tui.AppModel).View()
    if !strings.Contains(view, "Steps for Job: test") {
        t.Errorf("expected steps view header, got:\n%s", view)
    }
    if !strings.Contains(view, "checkout") {
        t.Errorf("expected step 'checkout' in view, got:\n%s", view)
    }
}

func TestApp_EscFromStepsReturnsToJobs(t *testing.T) {
    pipelines := []domain.Pipeline{
        {ID: "1001", Branch: "main", Status: domain.StatusSuccess},
    }
    provider := &fakeProvider{pipelines: pipelines}
    m := tui.NewAppModel(domain.Repository{Owner: "waabox", Name: "gitdeck"}, provider)

    m0, _ := m.Update(tui.PipelinesLoadedMsg{Pipelines: pipelines})
    m1, _ := m0.(tui.AppModel).Update(tea.KeyMsg{Type: tea.KeyEnter})
    m2, _ := m1.(tui.AppModel).Update(tui.PipelineDetailMsg{
        Pipeline: domain.Pipeline{
            ID: "1001", Branch: "main",
            Jobs: []domain.Job{{
                ID: "j1", Name: "test",
                Steps: []domain.Step{{Name: "checkout"}},
            }},
        },
    })
    m3, _ := m2.(tui.AppModel).Update(tea.KeyMsg{Type: tea.KeyEnter})
    m4, _ := m3.(tui.AppModel).Update(tea.KeyMsg{Type: tea.KeyEsc})
    view := m4.(tui.AppModel).View()
    if !strings.Contains(view, "Jobs for Pipeline") {
        t.Errorf("expected jobs view after esc from steps, got:\n%s", view)
    }
}
```

**Step 2: Run all tests**

Run: `cd /Users/waabox/code/waabox/gitdeck && go test ./internal/tui/ -v`
Expected: all tests PASS.

**Step 3: Commit**

```
test: add tests for hierarchical drill-down navigation
```

---

### Task 7: Remove dead code and cleanup

**Files:**
- Modify: `internal/tui/jobdetail.go` — remove `ToggleExpand`, `expanded` field, step rendering from `render()`, and `View()` method (only keep `ViewFocused()`). Or keep `View()` as an alias.
- Modify: `internal/tui/app.go` — remove any leftover `focusList`/`focusDetail` references.

**Step 1: Clean up jobdetail.go**

Remove `expanded` field from struct, remove from `NewJobDetailModel`, remove `ToggleExpand` method, remove step rendering from `render()`.

**Step 2: Run all tests**

Run: `cd /Users/waabox/code/waabox/gitdeck && go test ./internal/tui/ -v`
Expected: all tests PASS.

**Step 3: Commit**

```
refactor: remove dual-panel and inline-expand dead code
```

---

### Summary of navigation after implementation

| View       | Up/Down      | Enter             | Esc            | l           | r/x         |
|------------|-------------|-------------------|----------------|-------------|-------------|
| Pipelines  | Navigate    | → Jobs            | -              | -           | Rerun/Cancel|
| Jobs       | Navigate    | → Steps           | → Pipelines    | View logs   | Rerun/Cancel|
| Steps      | Navigate    | -                 | → Jobs         | View logs   | -           |
| Logs       | Scroll      | -                 | → Previous     | -           | -           |
