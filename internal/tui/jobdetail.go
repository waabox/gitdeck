package tui

import (
	"fmt"
	"strings"

	"github.com/waabox/gitdeck/internal/domain"
)

// JobDetailModel is an immutable model for the jobs panel.
type JobDetailModel struct {
	jobs   []domain.Job
	cursor int
}

// NewJobDetailModel creates a job detail model.
func NewJobDetailModel(jobs []domain.Job) JobDetailModel {
	return JobDetailModel{jobs: jobs, cursor: 0}
}

// MoveDown returns a new model with the cursor moved down by one.
func (m JobDetailModel) MoveDown() JobDetailModel {
	if m.cursor < len(m.jobs)-1 {
		m.cursor++
	}
	return m
}

// MoveUp returns a new model with the cursor moved up by one.
func (m JobDetailModel) MoveUp() JobDetailModel {
	if m.cursor > 0 {
		m.cursor--
	}
	return m
}

// View renders the job list as a string.
func (m JobDetailModel) View() string {
	return m.render(false)
}

// ViewFocused renders the job list with cursor indicators.
func (m JobDetailModel) ViewFocused() string {
	return m.render(true)
}

func (m JobDetailModel) render(focused bool) string {
	if len(m.jobs) == 0 {
		return "Select a pipeline to see its jobs."
	}
	var sb strings.Builder
	for i, j := range m.jobs {
		duration := "--"
		if j.Duration > 0 {
			duration = fmt.Sprintf("%ds", int(j.Duration.Seconds()))
		}
		prefix := "  "
		if focused && i == m.cursor {
			prefix = "> "
		}
		sb.WriteString(fmt.Sprintf("%s%s %-25s %s\n",
			prefix,
			statusIcon(j.Status),
			truncate(j.Name, 25),
			duration,
		))
	}
	return sb.String()
}
