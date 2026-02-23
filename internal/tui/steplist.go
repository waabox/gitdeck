package tui

import (
	"fmt"
	"strings"

	"github.com/waabox/gitdeck/internal/domain"
)

// StepListModel is an immutable model for the steps panel.
type StepListModel struct {
	steps  []domain.Step
	cursor int
}

// NewStepListModel creates a step list model.
func NewStepListModel(steps []domain.Step) StepListModel {
	return StepListModel{steps: steps, cursor: 0}
}

// MoveDown returns a new model with the cursor moved down by one.
func (m StepListModel) MoveDown() StepListModel {
	if m.cursor < len(m.steps)-1 {
		m.cursor++
	}
	return m
}

// MoveUp returns a new model with the cursor moved up by one.
func (m StepListModel) MoveUp() StepListModel {
	if m.cursor > 0 {
		m.cursor--
	}
	return m
}

// Cursor returns the current cursor position.
func (m StepListModel) Cursor() int {
	return m.cursor
}

// Steps returns the full step slice.
func (m StepListModel) Steps() []domain.Step {
	return m.steps
}

// View renders the step list as a string with cursor indicators.
func (m StepListModel) View() string {
	if len(m.steps) == 0 {
		return "No steps found."
	}
	var sb strings.Builder
	for i, s := range m.steps {
		prefix := "  "
		if i == m.cursor {
			prefix = "> "
		}
		duration := "--"
		if s.Duration > 0 {
			duration = fmt.Sprintf("%ds", int(s.Duration.Seconds()))
		}
		sb.WriteString(fmt.Sprintf("%s%s %-25s %s\n",
			prefix,
			statusIcon(s.Status),
			truncate(s.Name, 25),
			duration,
		))
	}
	return sb.String()
}
