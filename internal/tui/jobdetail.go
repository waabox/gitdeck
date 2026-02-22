package tui

import (
	"fmt"
	"strings"

	"github.com/waabox/gitdeck/internal/domain"
)

// JobDetailModel is an immutable model for the jobs panel.
type JobDetailModel struct {
	jobs     []domain.Job
	cursor   int
	expanded map[int]bool
}

// NewJobDetailModel creates a job detail model.
func NewJobDetailModel(jobs []domain.Job) JobDetailModel {
	return JobDetailModel{jobs: jobs, cursor: 0, expanded: map[int]bool{}}
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

// ToggleExpand returns a new model with the given job index expanded or collapsed.
// If the job has no steps, the toggle is a no-op.
func (m JobDetailModel) ToggleExpand(idx int) JobDetailModel {
	if idx < 0 || idx >= len(m.jobs) || len(m.jobs[idx].Steps) == 0 {
		return m
	}
	next := make(map[int]bool, len(m.expanded))
	for k, v := range m.expanded {
		next[k] = v
	}
	next[idx] = !next[idx]
	m.expanded = next
	return m
}

// Cursor returns the current cursor position.
func (m JobDetailModel) Cursor() int {
	return m.cursor
}

// Jobs returns the full job slice.
func (m JobDetailModel) Jobs() []domain.Job {
	return m.jobs
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
		if m.expanded[i] {
			for si, s := range j.Steps {
				tree := "├─"
				if si == len(j.Steps)-1 {
					tree = "└─"
				}
				stepDuration := "--"
				if s.Duration > 0 {
					stepDuration = fmt.Sprintf("%ds", int(s.Duration.Seconds()))
				}
				sb.WriteString(fmt.Sprintf("    %s %s %-21s %s\n",
					tree,
					statusIcon(s.Status),
					truncate(s.Name, 21),
					stepDuration,
				))
			}
		}
	}
	return sb.String()
}
