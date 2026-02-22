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
				{Name: "checkout", Status: domain.StatusSuccess, Duration: 2 * time.Second},
				{Name: "run tests", Status: domain.StatusSuccess, Duration: 31 * time.Second},
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
				{Name: "checkout", Status: domain.StatusSuccess, Duration: 2 * time.Second},
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
	if view == "" {
		t.Error("expected non-empty view even when job has no steps")
	}
}

func TestJobDetailModel_ToggleExpand_OutOfBounds_DoesNothing(t *testing.T) {
	jobs := []domain.Job{
		{ID: "1", Name: "build", Status: domain.StatusSuccess},
	}
	m := tui.NewJobDetailModel(jobs)
	original := m.View()
	m = m.ToggleExpand(-1)
	m = m.ToggleExpand(99)
	if m.View() != original {
		t.Error("expected view to be unchanged after out-of-bounds toggle")
	}
}
