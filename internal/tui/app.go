package tui

import (
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/waabox/gitdeck/internal/domain"
)

// PipelinesLoadedMsg is sent when pipelines have been fetched from the provider.
// It is exported so that tests can inject it directly into AppModel.Update.
type PipelinesLoadedMsg struct {
	Pipelines []domain.Pipeline
	Err       error
}

// pipelineDetailMsg is sent when a pipeline detail (with jobs) has been fetched.
type pipelineDetailMsg struct {
	pipeline domain.Pipeline
	err      error
}

// tickMsg is sent by the auto-refresh ticker.
type tickMsg struct{}

// actionResultMsg is sent when a pipeline action (rerun, cancel) completes.
type actionResultMsg struct {
	action string
	err    error
}

// LogsLoadedMsg is sent when job logs have been fetched from the provider.
// It is exported so that tests can inject it directly into AppModel.Update.
type LogsLoadedMsg struct {
	Content string
	JobName string
	Err     error
}

// focusPanel indicates which panel has keyboard focus.
type focusPanel int

const (
	focusList focusPanel = iota
	focusDetail
)

// AppModel is the root Bubbletea model for gitdeck.
type AppModel struct {
	repo          domain.Repository
	provider      domain.PipelineProvider
	list          PipelineListModel
	detail        JobDetailModel
	focus         focusPanel
	loading       bool
	err           error
	width         int
	height        int
	confirmAction string // "rerun" | "cancel" | ""
	// Log viewer state
	logMode    bool
	logLoading bool
	logContent string
	logOffset  int
	logJobName string
}

// NewAppModel creates the root application model.
func NewAppModel(repo domain.Repository, provider domain.PipelineProvider) AppModel {
	return AppModel{
		repo:     repo,
		provider: provider,
		list:     NewPipelineListModel(nil),
		detail:   NewJobDetailModel(nil),
		loading:  true,
	}
}

// Init triggers the initial pipeline load.
func (m AppModel) Init() tea.Cmd {
	return tea.Batch(m.loadPipelines(), tickEvery(5*time.Second))
}

func (m AppModel) loadPipelines() tea.Cmd {
	return func() tea.Msg {
		pipelines, err := m.provider.ListPipelines(m.repo)
		return PipelinesLoadedMsg{Pipelines: pipelines, Err: err}
	}
}

func (m AppModel) loadPipelineDetail(id string) tea.Cmd {
	return func() tea.Msg {
		pipeline, err := m.provider.GetPipeline(m.repo, domain.PipelineID(id))
		return pipelineDetailMsg{pipeline: pipeline, err: err}
	}
}

func (m AppModel) rerunPipeline(id string) tea.Cmd {
	return func() tea.Msg {
		err := m.provider.RerunPipeline(m.repo, domain.PipelineID(id))
		return actionResultMsg{action: "rerun", err: err}
	}
}

func (m AppModel) cancelPipeline(id string) tea.Cmd {
	return func() tea.Msg {
		err := m.provider.CancelPipeline(m.repo, domain.PipelineID(id))
		return actionResultMsg{action: "cancel", err: err}
	}
}

func (m AppModel) loadJobLogs(job domain.Job) tea.Cmd {
	return func() tea.Msg {
		content, err := m.provider.GetJobLogs(m.repo, domain.JobID(job.ID))
		return LogsLoadedMsg{Content: content, JobName: job.Name, Err: err}
	}
}

func tickEvery(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(_ time.Time) tea.Msg {
		return tickMsg{}
	})
}

// anyRunning reports whether any pipeline in the list has StatusRunning.
func anyRunning(pipelines []domain.Pipeline) bool {
	for _, p := range pipelines {
		if p.Status == domain.StatusRunning {
			return true
		}
	}
	return false
}

// Update handles all incoming messages and key events.
func (m AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case PipelinesLoadedMsg:
		m.loading = false
		if msg.Err != nil {
			m.err = msg.Err
			return m, nil
		}
		m.list = NewPipelineListModel(msg.Pipelines)
		// Pre-populate the detail model if the selected pipeline already carries jobs.
		if len(msg.Pipelines) > 0 && len(msg.Pipelines[0].Jobs) > 0 {
			m.detail = NewJobDetailModel(msg.Pipelines[0].Jobs)
		}

	case pipelineDetailMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.detail = NewJobDetailModel(msg.pipeline.Jobs)

	case tickMsg:
		selected := m.list.SelectedPipeline()
		interval := 30 * time.Second
		if anyRunning(m.list.Pipelines()) {
			interval = 5 * time.Second
		}
		cmds := []tea.Cmd{m.loadPipelines(), tickEvery(interval)}
		if selected.Status == domain.StatusRunning && selected.ID != "" {
			cmds = append(cmds, m.loadPipelineDetail(selected.ID))
		}
		return m, tea.Batch(cmds...)

	case actionResultMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.loading = true
		return m, m.loadPipelines()

	case LogsLoadedMsg:
		m.logLoading = false
		if msg.Err != nil {
			m.err = msg.Err
			return m, nil
		}
		m.logMode = true
		m.logContent = msg.Content
		m.logJobName = msg.JobName
		m.logOffset = 0
		return m, nil

	case tea.KeyMsg:
		// Any key dismisses the confirmation prompt (except y which confirms, and q/ctrl+c which quit).
		if m.confirmAction != "" {
			switch msg.String() {
			case "y":
				selected := m.list.SelectedPipeline()
				if selected.ID == "" {
					m.confirmAction = ""
					return m, nil
				}
				action := m.confirmAction
				m.confirmAction = ""
				if action == "rerun" {
					return m, m.rerunPipeline(selected.ID)
				}
				return m, m.cancelPipeline(selected.ID)
			case "q", "ctrl+c":
				return m, tea.Quit
			default:
				m.confirmAction = ""
				return m, nil
			}
		}
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "ctrl+r":
			m.loading = true
			return m, m.loadPipelines()
		case "r":
			if !m.logMode {
				m.confirmAction = "rerun"
			}
			return m, nil
		case "x":
			if !m.logMode {
				m.confirmAction = "cancel"
			}
			return m, nil
		case "tab":
			if m.focus == focusList {
				m.focus = focusDetail
			} else {
				m.focus = focusList
			}
		case "down":
			if m.logMode {
				maxOffset := strings.Count(m.logContent, "\n")
				if m.logOffset < maxOffset {
					m.logOffset++
				}
				return m, nil
			}
			if m.focus == focusList {
				m.list = m.list.MoveDown()
				return m, m.loadPipelineDetail(m.list.SelectedPipeline().ID)
			}
			m.detail = m.detail.MoveDown()
		case "up":
			if m.logMode {
				if m.logOffset > 0 {
					m.logOffset--
				}
				return m, nil
			}
			if m.focus == focusList {
				m.list = m.list.MoveUp()
				return m, m.loadPipelineDetail(m.list.SelectedPipeline().ID)
			}
			m.detail = m.detail.MoveUp()
		case "enter":
			if m.focus == focusList {
				m.focus = focusDetail
				return m, m.loadPipelineDetail(m.list.SelectedPipeline().ID)
			}
			m.detail = m.detail.ToggleExpand(m.detail.Cursor())
		case "l":
			if m.focus == focusDetail && !m.logMode && !m.logLoading {
				jobs := m.detail.Jobs()
				if len(jobs) > 0 {
					m.logLoading = true
					return m, m.loadJobLogs(jobs[m.detail.Cursor()])
				}
			}
		case "pgup":
			if m.logMode {
				page := m.visibleLogLines()
				if m.logOffset-page >= 0 {
					m.logOffset -= page
				} else {
					m.logOffset = 0
				}
				return m, nil
			}
		case "pgdown":
			if m.logMode {
				maxOffset := strings.Count(m.logContent, "\n")
				m.logOffset += m.visibleLogLines()
				if m.logOffset > maxOffset {
					m.logOffset = maxOffset
				}
				return m, nil
			}
		case "g":
			if m.logMode {
				m.logOffset = 0
				return m, nil
			}
		case "G":
			if m.logMode {
				lines := strings.Split(m.logContent, "\n")
				m.logOffset = len(lines) - 1
				return m, nil
			}
		case "esc":
			if m.logMode {
				m.logMode = false
				m.logContent = ""
				m.logOffset = 0
				return m, nil
			}
			m.focus = focusList
		}
	}
	return m, nil
}

// View renders the full TUI.
func (m AppModel) View() string {
	if m.logLoading {
		return "Loading logs...\n"
	}
	if m.logMode {
		return m.renderLogView()
	}
	if m.loading && m.confirmAction == "" {
		return "Loading pipelines...\n"
	}
	if m.err != nil {
		return fmt.Sprintf("Error: %v\n\nPress 'ctrl+r' to retry or 'q' to quit.\n", m.err)
	}

	selected := m.list.SelectedPipeline()
	header := fmt.Sprintf(" gitdeck | %s / ⎇ %s %s / %s\n",
		m.repo.Name, selected.Branch, shortSHA(selected.CommitSHA), firstLine(selected.CommitMsg))
	separator := "────────────────────────────────────────────────────────────\n"

	listHeader := " PIPELINES\n"
	detailHeader := " JOBS\n"
	if m.focus == focusList {
		listHeader = " PIPELINES [active]\n"
	} else {
		detailHeader = " JOBS [active]\n"
	}

	listView := m.list.View()
	detailView := m.detail.View()
	if m.focus == focusDetail {
		detailView = m.detail.ViewFocused()
	}
	statusBar := fmt.Sprintf("#%s by %s\n", selected.ID, selected.Author)

	footer := " ↑/↓: navigate   tab: switch panel   enter: select/expand   ctrl+r: refresh   r: rerun   x: cancel   q: quit\n"
	if m.focus == focusDetail {
		footer = " ↑/↓: navigate   tab: switch panel   enter: expand   l: logs   r: rerun   x: cancel   q: quit\n"
	}
	if m.confirmAction == "rerun" {
		footer = fmt.Sprintf(" Rerun pipeline #%s on %s? [y/N] \n", selected.ID, selected.Branch)
	}
	if m.confirmAction == "cancel" {
		footer = fmt.Sprintf(" Cancel pipeline #%s on %s? [y/N] \n", selected.ID, selected.Branch)
	}

	return header + separator +
		listHeader + listView + "\n" +
		detailHeader + detailView + "\n" +
		separator + statusBar + separator + footer
}

// Run starts the Bubbletea program. Exits on error.
func Run(repo domain.Repository, provider domain.PipelineProvider) {
	p := tea.NewProgram(NewAppModel(repo, provider), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "gitdeck error: %v\n", err)
		os.Exit(1)
	}
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

func shortSHA(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}

// visibleLogLines returns the number of log lines visible in the current terminal height.
func (m AppModel) visibleLogLines() int {
	lines := m.height - 4 // account for header, separator, and footer
	if lines < 10 {
		return 10
	}
	return lines
}

// renderLogView renders the fullscreen log viewer.
func (m AppModel) renderLogView() string {
	header := fmt.Sprintf(" gitdeck  %s/%s  [logs] %s\n",
		m.repo.Owner, m.repo.Name, m.logJobName)
	separator := "────────────────────────────────────────────────────────────\n"
	footer := " ↑/↓: scroll   PgUp/PgDn: page   g/G: top/bottom   esc: back\n"

	lines := strings.Split(m.logContent, "\n")
	visibleCount := m.visibleLogLines()

	start := m.logOffset
	if start < 0 {
		start = 0
	}
	if start >= len(lines) {
		start = len(lines) - 1
	}
	end := start + visibleCount
	if end > len(lines) {
		end = len(lines)
	}

	body := strings.Join(lines[start:end], "\n")
	return header + separator + body + "\n" + separator + footer
}
