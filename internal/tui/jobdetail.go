package tui

import (
	"fmt"
	"strings"

	"github.com/waabox/gitdeck/internal/domain"
)

// JobDetailModel is an immutable model for the jobs panel.
type JobDetailModel struct {
	jobs []domain.Job
}

// NewJobDetailModel creates a job detail model.
func NewJobDetailModel(jobs []domain.Job) JobDetailModel {
	return JobDetailModel{jobs: jobs}
}

// View renders the job list as a string.
func (m JobDetailModel) View() string {
	if len(m.jobs) == 0 {
		return "Select a pipeline to see its jobs."
	}
	var sb strings.Builder
	for _, j := range m.jobs {
		duration := "--"
		if j.Duration > 0 {
			duration = fmt.Sprintf("%ds", int(j.Duration.Seconds()))
		}
		sb.WriteString(fmt.Sprintf("  %s %-25s %s\n",
			statusIcon(j.Status),
			truncate(j.Name, 25),
			duration,
		))
	}
	return sb.String()
}
