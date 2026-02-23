package tui_test

import (
	"context"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/waabox/gitdeck/internal/auth"
	"github.com/waabox/gitdeck/internal/domain"
	"github.com/waabox/gitdeck/internal/provider"
	"github.com/waabox/gitdeck/internal/tui"
)

// fakeProvider satisfies domain.PipelineProvider for TUI tests.
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
func (f *fakeProvider) GetJobLogs(_ domain.Repository, _ domain.JobID) (string, error) {
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

	if !strings.Contains(view, "Rerun pipeline") {
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

	if !strings.Contains(view, "Cancel pipeline") {
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

	if strings.Contains(view, "Rerun pipeline") {
		t.Errorf("expected confirm prompt to be dismissed after 'n', got:\n%s", view)
	}
}

func TestApp_ConfirmRerun_YKey_CallsProvider(t *testing.T) {
	pipelines := []domain.Pipeline{{ID: "1001", Branch: "main", Status: domain.StatusFailed}}
	provider := &fakeProvider{pipelines: pipelines}
	m := tui.NewAppModel(domain.Repository{Owner: "waabox", Name: "gitdeck"}, provider)

	// Seed the list by delivering a pipelinesLoadedMsg before any key press.
	m0, _ := m.Update(tui.PipelinesLoadedMsg{Pipelines: pipelines})
	// Press r to show confirm prompt, then y to confirm.
	m1, _ := m0.(tui.AppModel).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
	_, cmd := m1.(tui.AppModel).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	if cmd != nil {
		cmd() // executes the rerunPipeline command, which calls provider.RerunPipeline
	}

	if !provider.rerunCalled {
		t.Error("expected RerunPipeline to be called after confirming with y")
	}
}

func TestApp_ConfirmCancel_YKey_CallsProvider(t *testing.T) {
	pipelines := []domain.Pipeline{{ID: "1001", Branch: "main", Status: domain.StatusRunning}}
	provider := &fakeProvider{pipelines: pipelines}
	m := tui.NewAppModel(domain.Repository{Owner: "waabox", Name: "gitdeck"}, provider)

	// Seed the list by delivering a pipelinesLoadedMsg before any key press.
	m0, _ := m.Update(tui.PipelinesLoadedMsg{Pipelines: pipelines})
	// Press x to show confirm prompt, then y to confirm.
	m1, _ := m0.(tui.AppModel).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	_, cmd := m1.(tui.AppModel).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	if cmd != nil {
		cmd() // executes the cancelPipeline command, which calls provider.CancelPipeline
	}

	if !provider.cancelCalled {
		t.Error("expected CancelPipeline to be called after confirming with y")
	}
}

func TestApp_RefreshPreservesSelection(t *testing.T) {
	initialPipelines := []domain.Pipeline{
		{ID: "1", Branch: "main", Status: domain.StatusSuccess},
		{ID: "2", Branch: "feat/auth", Status: domain.StatusRunning},
		{ID: "3", Branch: "fix/bug", Status: domain.StatusFailed},
	}
	provider := &fakeProvider{pipelines: initialPipelines}
	m := tui.NewAppModel(domain.Repository{Owner: "waabox", Name: "gitdeck"}, provider)

	// First load populates the list and selects pipeline[0].
	m0, _ := m.Update(tui.PipelinesLoadedMsg{Pipelines: initialPipelines})
	app := m0.(tui.AppModel)

	// Navigate down to pipeline "2" (index 1).
	m1, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{0}, Alt: false})
	// Use the down key properly
	m1, _ = app.Update(tea.KeyMsg{Type: tea.KeyDown})
	app = m1.(tui.AppModel)

	// Simulate an auto-refresh with updated data (pipeline "2" now succeeded).
	refreshedPipelines := []domain.Pipeline{
		{ID: "1", Branch: "main", Status: domain.StatusSuccess},
		{ID: "2", Branch: "feat/auth", Status: domain.StatusSuccess},
		{ID: "3", Branch: "fix/bug", Status: domain.StatusFailed},
	}
	m2, _ := app.Update(tui.PipelinesLoadedMsg{Pipelines: refreshedPipelines})
	app = m2.(tui.AppModel)

	// The view should still reference pipeline "2", not pipeline "1".
	view := app.View()
	if !strings.Contains(view, "#2") {
		t.Errorf("expected view to show pipeline #2 after refresh, got:\n%s", view)
	}
	if strings.Contains(view, "#1 by") {
		t.Errorf("expected status bar NOT to show pipeline #1, got:\n%s", view)
	}
}

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

func TestApp_AuthExpiredError_ShowsReAuthView(t *testing.T) {
	pipelines := []domain.Pipeline{{ID: "1", Branch: "main"}}
	prov := &fakeProvider{pipelines: pipelines}
	m := tui.NewAppModel(domain.Repository{Owner: "o", Name: "n"}, prov)

	// Set up the callback so the re-auth triggers
	m.OnRequestCode = func(ctx context.Context, providerName string) (auth.DeviceCodeResponse, error) {
		return auth.DeviceCodeResponse{
			UserCode:        "ABCD-1234",
			VerificationURI: "https://gitlab.com/oauth/device",
			DeviceCode:      "dev123",
			ExpiresIn:       300,
			Interval:        5,
		}, nil
	}

	// Seed with initial data first
	m0, _ := m.Update(tui.PipelinesLoadedMsg{Pipelines: pipelines})
	app := m0.(tui.AppModel)

	// Simulate AuthExpiredError
	authErr := &provider.AuthExpiredError{Provider: "gitlab"}
	m1, cmd := app.Update(tui.PipelinesLoadedMsg{Err: authErr})

	// The cmd should be the requestDeviceCode command
	if cmd == nil {
		t.Fatal("expected a command to be returned for device code request")
	}

	// Execute the command to get DeviceCodeMsg
	cmdResult := cmd()
	m2, _ := m1.(tui.AppModel).Update(cmdResult)
	view := m2.(tui.AppModel).View()

	if !strings.Contains(view, "Session expired") {
		t.Errorf("expected 'Session expired' in view, got:\n%s", view)
	}
	if !strings.Contains(view, "ABCD-1234") {
		t.Errorf("expected user code in view, got:\n%s", view)
	}
}

func TestApp_ReAuthComplete_ReloadsPipelines(t *testing.T) {
	prov := &fakeProvider{
		pipelines: []domain.Pipeline{{ID: "1", Branch: "main"}},
	}
	m := tui.NewAppModel(domain.Repository{Owner: "o", Name: "n"}, prov)

	tokenRefreshed := false
	m.OnTokenRefreshed = func(providerName string, resp auth.TokenResponse) {
		tokenRefreshed = true
	}

	// Simulate successful re-auth
	m1, cmd := m.Update(tui.ReAuthCompleteMsg{
		Token: auth.TokenResponse{AccessToken: "new-token", RefreshToken: "new-refresh"},
	})
	app := m1.(tui.AppModel)

	if !tokenRefreshed {
		t.Error("expected OnTokenRefreshed to be called")
	}
	if cmd == nil {
		t.Fatal("expected loadPipelines command after re-auth")
	}

	view := app.View()
	if !strings.Contains(view, "Loading pipelines") {
		t.Errorf("expected loading state after re-auth, got:\n%s", view)
	}
}

func TestApp_EscFromReAuth_ReturnsToErrorView(t *testing.T) {
	prov := &fakeProvider{
		pipelines: []domain.Pipeline{{ID: "1", Branch: "main"}},
	}
	m := tui.NewAppModel(domain.Repository{Owner: "o", Name: "n"}, prov)

	// Manually set re-auth state
	m.OnRequestCode = func(ctx context.Context, providerName string) (auth.DeviceCodeResponse, error) {
		return auth.DeviceCodeResponse{UserCode: "CODE"}, nil
	}

	// Trigger re-auth
	authErr := &provider.AuthExpiredError{Provider: "gitlab"}
	m1, cmd := m.Update(tui.PipelinesLoadedMsg{Err: authErr})
	if cmd != nil {
		result := cmd()
		m1, _ = m1.(tui.AppModel).Update(result)
	}

	// Press ESC
	m2, _ := m1.(tui.AppModel).Update(tea.KeyMsg{Type: tea.KeyEsc})
	view := m2.(tui.AppModel).View()

	if !strings.Contains(view, "session expired") {
		t.Errorf("expected session expired error message, got:\n%s", view)
	}
	if !strings.Contains(view, "ctrl+r") {
		t.Errorf("expected retry hint, got:\n%s", view)
	}
}
