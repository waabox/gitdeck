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
