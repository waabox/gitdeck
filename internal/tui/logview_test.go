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

func TestApp_LogsLoaded_RendersLogContent(t *testing.T) {
	provider := &fakeProvider{
		pipelines: []domain.Pipeline{{ID: "1001", Branch: "main", Status: domain.StatusFailed}},
	}
	m := tui.NewAppModel(domain.Repository{Owner: "waabox", Name: "gitdeck"}, provider)

	// Inject logs directly
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

func TestApp_LogView_ScrollDown_MovesOffset(t *testing.T) {
	provider := &fakeProvider{}
	m := tui.NewAppModel(domain.Repository{Owner: "waabox", Name: "gitdeck"}, provider)

	// Load 10 lines of logs
	logContent := "line1\nline2\nline3\nline4\nline5\nline6\nline7\nline8\nline9\nline10"
	m1, _ := m.Update(tui.LogsLoadedMsg{Content: logContent, JobName: "test", Err: nil})
	// Scroll down once
	m2, _ := m1.(tui.AppModel).Update(tea.KeyMsg{Type: tea.KeyDown})
	view := m2.(tui.AppModel).View()

	if !strings.Contains(view, "line2") {
		t.Errorf("expected line2 visible after scroll down, got:\n%s", view)
	}
}
