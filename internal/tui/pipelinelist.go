package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/waabox/gitdeck/internal/domain"
)

// PipelineListModel is an immutable Bubbletea-compatible model for the pipeline list panel.
type PipelineListModel struct {
	pipelines []domain.Pipeline
	cursor    int
}

// NewPipelineListModel creates a pipeline list model with the given pipelines.
func NewPipelineListModel(pipelines []domain.Pipeline) PipelineListModel {
	return PipelineListModel{pipelines: pipelines, cursor: 0}
}

// MoveDown returns a new model with the cursor moved down by one.
func (m PipelineListModel) MoveDown() PipelineListModel {
	if m.cursor < len(m.pipelines)-1 {
		m.cursor++
	}
	return m
}

// MoveUp returns a new model with the cursor moved up by one.
func (m PipelineListModel) MoveUp() PipelineListModel {
	if m.cursor > 0 {
		m.cursor--
	}
	return m
}

// SelectedIndex returns the current cursor position.
func (m PipelineListModel) SelectedIndex() int {
	return m.cursor
}

// SelectedPipeline returns the currently highlighted pipeline.
// Returns zero-value Pipeline if the list is empty.
func (m PipelineListModel) SelectedPipeline() domain.Pipeline {
	if len(m.pipelines) == 0 {
		return domain.Pipeline{}
	}
	return m.pipelines[m.cursor]
}

// View renders the pipeline list as a string.
func (m PipelineListModel) View() string {
	if len(m.pipelines) == 0 {
		return "No pipelines found."
	}
	var sb strings.Builder
	for i, p := range m.pipelines {
		prefix := "  "
		if i == m.cursor {
			prefix = "> "
		}
		sb.WriteString(fmt.Sprintf("%s%s #%s %-20s %s\n",
			prefix,
			statusIcon(p.Status),
			p.ID,
			truncate(p.Branch, 20),
			formatAge(p.CreatedAt),
		))
	}
	return sb.String()
}

func statusIcon(s domain.PipelineStatus) string {
	switch s {
	case domain.StatusSuccess:
		return "✓"
	case domain.StatusFailed:
		return "✗"
	case domain.StatusRunning:
		return "●"
	case domain.StatusPending:
		return "↷"
	case domain.StatusCancelled:
		return "○"
	default:
		return "?"
	}
}

func formatAge(t time.Time) string {
	if t.IsZero() {
		return "--"
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds ago", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	default:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	}
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}
