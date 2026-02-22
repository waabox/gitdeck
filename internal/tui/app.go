package tui

import (
	"fmt"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/waabox/gitdeck/internal/domain"
)

// pipelinesLoadedMsg is sent when pipelines have been fetched from the provider.
type pipelinesLoadedMsg struct {
	pipelines []domain.Pipeline
	err       error
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
		return pipelinesLoadedMsg{pipelines: pipelines, err: err}
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

	case pipelinesLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.list = NewPipelineListModel(msg.pipelines)

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

	case tea.KeyMsg:
		// Any key dismisses the confirmation prompt (except y which confirms, and q/ctrl+c which quit).
		if m.confirmAction != "" {
			switch msg.String() {
			case "y":
				action := m.confirmAction
				m.confirmAction = ""
				if action == "rerun" {
					return m, m.rerunPipeline(m.list.SelectedPipeline().ID)
				}
				return m, m.cancelPipeline(m.list.SelectedPipeline().ID)
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
			m.confirmAction = "rerun"
			return m, nil
		case "x":
			m.confirmAction = "cancel"
			return m, nil
		case "tab":
			if m.focus == focusList {
				m.focus = focusDetail
			} else {
				m.focus = focusList
			}
		case "down":
			if m.focus == focusList {
				m.list = m.list.MoveDown()
				return m, m.loadPipelineDetail(m.list.SelectedPipeline().ID)
			}
			m.detail = m.detail.MoveDown()
		case "up":
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
		case "esc":
			m.focus = focusList
		}
	}
	return m, nil
}

// View renders the full TUI.
func (m AppModel) View() string {
	if m.loading && m.confirmAction == "" {
		return "Loading pipelines...\n"
	}
	if m.err != nil {
		return fmt.Sprintf("Error: %v\n\nPress 'ctrl+r' to retry or 'q' to quit.\n", m.err)
	}

	header := fmt.Sprintf(" gitdeck  %s/%s  q:quit  ctrl+r:refresh  r:rerun  x:cancel\n",
		m.repo.Owner, m.repo.Name)
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

	selected := m.list.SelectedPipeline()
	statusBar := fmt.Sprintf(" #%s  %s  %s  \"%s\"  by %s\n",
		selected.ID, selected.Branch,
		shortSHA(selected.CommitSHA), selected.CommitMsg, selected.Author)

	footer := " ↑/↓: navigate   tab: switch panel   enter: select/expand   ctrl+r: refresh   r: rerun   x: cancel   q: quit\n"
	if m.confirmAction == "rerun" {
		footer = fmt.Sprintf(" Rerun pipeline #%s on %s? [y/N] \n", selected.ID, selected.Branch)
	}
	if m.confirmAction == "cancel" {
		footer = fmt.Sprintf(" Cancel pipeline #%s on %s? [y/N] \n", selected.ID, selected.Branch)
	}

	return header + separator +
		listHeader + listView + "\n" +
		detailHeader + detailView + "\n" +
		separator + statusBar + footer
}

// Run starts the Bubbletea program. Exits on error.
func Run(repo domain.Repository, provider domain.PipelineProvider) {
	p := tea.NewProgram(NewAppModel(repo, provider), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "gitdeck error: %v\n", err)
		os.Exit(1)
	}
}

func shortSHA(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}
