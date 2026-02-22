package tui_test

import (
	"testing"
	"time"

	"github.com/waabox/gitdeck/internal/domain"
	"github.com/waabox/gitdeck/internal/tui"
)

func TestPipelineListModel_RendersPipelines(t *testing.T) {
	pipelines := []domain.Pipeline{
		{
			ID:        "100",
			Branch:    "main",
			CommitSHA: "abc1234",
			CommitMsg: "fix: login timeout",
			Author:    "waabox",
			Status:    domain.StatusSuccess,
			CreatedAt: time.Now().Add(-2 * time.Minute),
			Duration:  90 * time.Second,
		},
		{
			ID:     "99",
			Branch: "feat/auth",
			Status: domain.StatusFailed,
		},
	}

	m := tui.NewPipelineListModel(pipelines)
	view := m.View()

	if view == "" {
		t.Error("expected non-empty view")
	}
	if m.SelectedIndex() != 0 {
		t.Errorf("expected selected index 0, got %d", m.SelectedIndex())
	}
	if m.SelectedPipeline().ID != "100" {
		t.Errorf("expected selected pipeline ID '100', got '%s'", m.SelectedPipeline().ID)
	}
}

func TestPipelineListModel_NavigatesDown(t *testing.T) {
	pipelines := []domain.Pipeline{
		{ID: "1", Status: domain.StatusSuccess},
		{ID: "2", Status: domain.StatusFailed},
	}
	m := tui.NewPipelineListModel(pipelines)
	m = m.MoveDown()
	if m.SelectedIndex() != 1 {
		t.Errorf("expected selected index 1 after moving down, got %d", m.SelectedIndex())
	}
}

func TestPipelineListModel_DoesNotGoAboveZero(t *testing.T) {
	pipelines := []domain.Pipeline{{ID: "1"}}
	m := tui.NewPipelineListModel(pipelines)
	m = m.MoveUp()
	if m.SelectedIndex() != 0 {
		t.Errorf("expected selected index 0, got %d", m.SelectedIndex())
	}
}
