package tui_test

import (
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
