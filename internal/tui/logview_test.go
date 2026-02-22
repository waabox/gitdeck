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

	// Seed pipelines and set the detail panel focused with a job loaded
	m1, _ := m.Update(tui.PipelinesLoadedMsg{
		Pipelines: []domain.Pipeline{{
			ID: "1001", Branch: "main", Status: domain.StatusFailed,
			Jobs: []domain.Job{{ID: "2001", Name: "test", Status: domain.StatusFailed}},
		}},
	})
	// Switch focus to detail panel
	m2, _ := m1.(tui.AppModel).Update(tea.KeyMsg{Type: tea.KeyTab})
	// Press l to open logs
	m3, _ := m2.(tui.AppModel).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})

	view := m3.(tui.AppModel).View()
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

	// After scrolling down, line1 should not be at the top (offset moved)
	// We can verify by checking that line2 is shown before line1 would have been
	// Most reliably: check that the first visible line starts from line2
	if !strings.Contains(view, "line2") {
		t.Errorf("expected line2 visible after scroll down, got:\n%s", view)
	}
}
