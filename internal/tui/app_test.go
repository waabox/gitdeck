package tui_test

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/waabox/gitdeck/internal/domain"
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
