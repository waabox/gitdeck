package tui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/waabox/gitdeck/internal/auth"
	"github.com/waabox/gitdeck/internal/domain"
	"github.com/waabox/gitdeck/internal/provider"
)

// PipelinesLoadedMsg is sent when pipelines have been fetched from the provider.
// It is exported so that tests can inject it directly into AppModel.Update.
type PipelinesLoadedMsg struct {
	Pipelines []domain.Pipeline
	Err       error
}

// PipelineDetailMsg is sent when a pipeline detail (with jobs) has been fetched.
type PipelineDetailMsg struct {
	Pipeline domain.Pipeline
	Err      error
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

// DeviceCodeMsg carries the device code response for re-authentication.
type DeviceCodeMsg struct {
	Code auth.DeviceCodeResponse
	Err  error
}

// ReAuthCompleteMsg signals that re-authentication completed.
type ReAuthCompleteMsg struct {
	Token auth.TokenResponse
	Err   error
}

// viewState indicates the current navigation level.
type viewState int

const (
	viewPipelines viewState = iota
	viewJobs
	viewSteps
	viewLogs
	viewReAuth
)

// AppModel is the root Bubbletea model for gitdeck.
type AppModel struct {
	repo     domain.Repository
	provider domain.PipelineProvider
	// Navigation
	view viewState
	// Pipeline level
	list             PipelineListModel
	selectedPipeline domain.Pipeline
	// Job level
	detail      JobDetailModel
	selectedJob domain.Job
	// Step level
	steps StepListModel
	// General state
	loading       bool
	err           error
	width         int
	height        int
	confirmAction string
	// Log viewer state
	logMode       bool
	logLoading    bool
	logContent    string
	logOffset     int
	logJobName    string
	logReturnView viewState
	// Re-auth state
	reAuthProvider string
	reAuthCode     auth.DeviceCodeResponse
	reAuthCancel   context.CancelFunc
	// Callbacks for re-auth (set by caller via exported fields)
	OnRequestCode    func(ctx context.Context, provider string) (auth.DeviceCodeResponse, error)
	OnPollToken      func(ctx context.Context, provider string, deviceCode string, interval int) (auth.TokenResponse, error)
	OnTokenRefreshed func(provider string, resp auth.TokenResponse)
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
		return PipelineDetailMsg{Pipeline: pipeline, Err: err}
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

func (m AppModel) requestDeviceCode() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		_ = cancel // cancel will be stored via DeviceCodeMsg handler
		code, err := m.OnRequestCode(ctx, m.reAuthProvider)
		if err != nil {
			cancel()
		}
		return DeviceCodeMsg{Code: code, Err: err}
	}
}

func (m AppModel) pollReAuthToken() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(m.reAuthCode.ExpiresIn)*time.Second)
		defer cancel()
		token, err := m.OnPollToken(ctx, m.reAuthProvider, m.reAuthCode.DeviceCode, m.reAuthCode.Interval)
		return ReAuthCompleteMsg{Token: token, Err: err}
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
			var authErr *provider.AuthExpiredError
			if errors.As(msg.Err, &authErr) && m.OnRequestCode != nil {
				m.reAuthProvider = authErr.Provider
				m.view = viewReAuth
				m.err = nil
				return m, m.requestDeviceCode()
			}
			m.err = msg.Err
			return m, nil
		}
		if len(m.list.Pipelines()) == 0 {
			m.list = NewPipelineListModel(msg.Pipelines)
			if len(msg.Pipelines) > 0 {
				m.selectedPipeline = msg.Pipelines[0]
			}
		} else {
			m.list = m.list.UpdatePipelines(msg.Pipelines)
			for _, p := range msg.Pipelines {
				if p.ID == m.selectedPipeline.ID {
					m.selectedPipeline = p
					break
				}
			}
		}

	case PipelineDetailMsg:
		if msg.Err != nil {
			var authErr *provider.AuthExpiredError
			if errors.As(msg.Err, &authErr) && m.OnRequestCode != nil {
				m.reAuthProvider = authErr.Provider
				m.view = viewReAuth
				m.err = nil
				return m, m.requestDeviceCode()
			}
			m.err = msg.Err
			return m, nil
		}
		m.detail = NewJobDetailModel(msg.Pipeline.Jobs)

	case tickMsg:
		interval := 30 * time.Second
		if anyRunning(m.list.Pipelines()) {
			interval = 5 * time.Second
		}
		cmds := []tea.Cmd{m.loadPipelines(), tickEvery(interval)}
		if m.selectedPipeline.Status == domain.StatusRunning && m.selectedPipeline.ID != "" {
			cmds = append(cmds, m.loadPipelineDetail(m.selectedPipeline.ID))
		}
		return m, tea.Batch(cmds...)

	case actionResultMsg:
		if msg.err != nil {
			var authErr *provider.AuthExpiredError
			if errors.As(msg.err, &authErr) && m.OnRequestCode != nil {
				m.reAuthProvider = authErr.Provider
				m.view = viewReAuth
				m.err = nil
				return m, m.requestDeviceCode()
			}
			m.err = msg.err
			return m, nil
		}
		m.loading = true
		return m, m.loadPipelines()

	case LogsLoadedMsg:
		m.logLoading = false
		if msg.Err != nil {
			// Log errors are non-fatal: stay in the current view.
			return m, nil
		}
		m.logReturnView = m.view
		m.view = viewLogs
		m.logMode = true
		m.logContent = msg.Content
		m.logJobName = msg.JobName
		m.logOffset = 0
		return m, nil

	case DeviceCodeMsg:
		if msg.Err != nil {
			m.err = fmt.Errorf("re-authentication failed: %w", msg.Err)
			m.view = viewPipelines
			return m, nil
		}
		m.reAuthCode = msg.Code
		return m, m.pollReAuthToken()

	case ReAuthCompleteMsg:
		if msg.Err != nil {
			m.err = fmt.Errorf("re-authentication failed: %w", msg.Err)
			m.view = viewPipelines
			return m, nil
		}
		if m.OnTokenRefreshed != nil {
			m.OnTokenRefreshed(m.reAuthProvider, msg.Token)
		}
		m.reAuthProvider = ""
		m.reAuthCode = auth.DeviceCodeResponse{}
		m.view = viewPipelines
		m.loading = true
		m.err = nil
		return m, m.loadPipelines()

	case tea.KeyMsg:
		if m.confirmAction != "" {
			switch msg.String() {
			case "y":
				if m.selectedPipeline.ID == "" {
					m.confirmAction = ""
					return m, nil
				}
				action := m.confirmAction
				m.confirmAction = ""
				if action == "rerun" {
					return m, m.rerunPipeline(m.selectedPipeline.ID)
				}
				return m, m.cancelPipeline(m.selectedPipeline.ID)
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
		}
		switch m.view {
		case viewPipelines:
			return m.updatePipelines(msg)
		case viewJobs:
			return m.updateJobs(msg)
		case viewSteps:
			return m.updateSteps(msg)
		case viewLogs:
			return m.updateLogs(msg)
		case viewReAuth:
			if msg.String() == "esc" || msg.String() == "q" || msg.String() == "ctrl+c" {
				if msg.String() == "q" || msg.String() == "ctrl+c" {
					return m, tea.Quit
				}
				m.view = viewPipelines
				m.err = fmt.Errorf("%s session expired: press ctrl+r to retry", m.reAuthProvider)
				m.reAuthProvider = ""
				m.reAuthCode = auth.DeviceCodeResponse{}
				return m, nil
			}
		}
	}
	return m, nil
}

func (m AppModel) updatePipelines(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "down":
		m.list = m.list.MoveDown()
		m.selectedPipeline = m.list.SelectedPipeline()
	case "up":
		m.list = m.list.MoveUp()
		m.selectedPipeline = m.list.SelectedPipeline()
	case "enter":
		if len(m.list.Pipelines()) > 0 {
			m.selectedPipeline = m.list.SelectedPipeline()
			m.view = viewJobs
			return m, m.loadPipelineDetail(m.selectedPipeline.ID)
		}
	case "r":
		m.confirmAction = "rerun"
	case "x":
		m.confirmAction = "cancel"
	}
	return m, nil
}

func (m AppModel) updateJobs(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "down":
		m.detail = m.detail.MoveDown()
	case "up":
		m.detail = m.detail.MoveUp()
	case "enter":
		jobs := m.detail.Jobs()
		if len(jobs) > 0 {
			m.selectedJob = jobs[m.detail.Cursor()]
			m.steps = NewStepListModel(m.selectedJob.Steps)
			m.view = viewSteps
		}
	case "l":
		if !m.logLoading {
			jobs := m.detail.Jobs()
			if len(jobs) > 0 {
				m.logLoading = true
				return m, m.loadJobLogs(jobs[m.detail.Cursor()])
			}
		}
	case "esc":
		m.view = viewPipelines
	case "r":
		m.confirmAction = "rerun"
	case "x":
		m.confirmAction = "cancel"
	}
	return m, nil
}

func (m AppModel) updateSteps(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "down":
		m.steps = m.steps.MoveDown()
	case "up":
		m.steps = m.steps.MoveUp()
	case "l":
		if !m.logLoading {
			m.logLoading = true
			return m, m.loadJobLogs(m.selectedJob)
		}
	case "esc":
		m.view = viewJobs
	}
	return m, nil
}

func (m AppModel) updateLogs(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "down":
		maxOffset := strings.Count(m.logContent, "\n")
		if m.logOffset < maxOffset {
			m.logOffset++
		}
	case "up":
		if m.logOffset > 0 {
			m.logOffset--
		}
	case "pgup":
		page := m.visibleLogLines()
		if m.logOffset-page >= 0 {
			m.logOffset -= page
		} else {
			m.logOffset = 0
		}
	case "pgdown":
		maxOffset := strings.Count(m.logContent, "\n")
		m.logOffset += m.visibleLogLines()
		if m.logOffset > maxOffset {
			m.logOffset = maxOffset
		}
	case "g":
		m.logOffset = 0
	case "G":
		lines := strings.Split(m.logContent, "\n")
		m.logOffset = len(lines) - 1
	case "esc":
		m.view = m.logReturnView
		m.logMode = false
		m.logContent = ""
		m.logOffset = 0
	}
	return m, nil
}

// View renders the full TUI.
func (m AppModel) View() string {
	if m.logLoading {
		return "Loading logs...\n"
	}
	if m.view == viewLogs {
		return m.renderLogView()
	}
	if m.view == viewReAuth {
		return m.renderReAuthView()
	}
	if m.loading && m.confirmAction == "" {
		return "Loading pipelines...\n"
	}
	if m.err != nil {
		return fmt.Sprintf("Error: %v\n\nPress 'ctrl+r' to retry or 'q' to quit.\n", m.err)
	}

	header := fmt.Sprintf(" gitdeck | %s / ⎇ %s %s / %s\n",
		m.repo.Name, m.selectedPipeline.Branch,
		shortSHA(m.selectedPipeline.CommitSHA),
		firstLine(m.selectedPipeline.CommitMsg))
	separator := "────────────────────────────────────────────────────────────\n"

	switch m.view {
	case viewPipelines:
		return m.renderPipelinesView(header, separator)
	case viewJobs:
		return m.renderJobsView(header, separator)
	case viewSteps:
		return m.renderStepsView(header, separator)
	default:
		return header
	}
}

func (m AppModel) renderPipelinesView(header, separator string) string {
	title := " Pipelines\n"
	listView := m.list.View()
	statusBar := fmt.Sprintf(" #%s by %s\n", m.selectedPipeline.ID, m.selectedPipeline.Author)
	footer := " ↑/↓: navigate   enter: open   ctrl+r: refresh   r: rerun   x: cancel   q: quit\n"
	if m.confirmAction == "rerun" {
		footer = fmt.Sprintf(" Rerun pipeline #%s on %s? [y/N] \n",
			m.selectedPipeline.ID, m.selectedPipeline.Branch)
	}
	if m.confirmAction == "cancel" {
		footer = fmt.Sprintf(" Cancel pipeline #%s on %s? [y/N] \n",
			m.selectedPipeline.ID, m.selectedPipeline.Branch)
	}
	return header + separator + title + listView + "\n" + separator + statusBar + separator + footer
}

func (m AppModel) renderJobsView(header, separator string) string {
	title := fmt.Sprintf(" Jobs for Pipeline #%s\n", m.selectedPipeline.ID)
	detailView := m.detail.ViewFocused()
	footer := " ↑/↓: navigate   enter: steps   l: logs   esc: back   r: rerun   x: cancel   q: quit\n"
	if m.confirmAction == "rerun" {
		footer = fmt.Sprintf(" Rerun pipeline #%s on %s? [y/N] \n",
			m.selectedPipeline.ID, m.selectedPipeline.Branch)
	}
	if m.confirmAction == "cancel" {
		footer = fmt.Sprintf(" Cancel pipeline #%s on %s? [y/N] \n",
			m.selectedPipeline.ID, m.selectedPipeline.Branch)
	}
	return header + separator + title + detailView + "\n" + separator + footer
}

func (m AppModel) renderStepsView(header, separator string) string {
	title := fmt.Sprintf(" Steps for Job: %s\n", m.selectedJob.Name)
	stepsView := m.steps.View()
	footer := " ↑/↓: navigate   l: logs   esc: back   q: quit\n"
	return header + separator + title + stepsView + "\n" + separator + footer
}

func (m AppModel) renderReAuthView() string {
	header := " gitdeck — Re-authentication Required\n"
	separator := "────────────────────────────────────────────────────────────\n"

	var body string
	if m.reAuthCode.UserCode == "" {
		body = fmt.Sprintf("\n Session expired for %s.\n\n Requesting authorization...\n\n", m.reAuthProvider)
	} else {
		body = fmt.Sprintf(
			"\n Session expired for %s.\n\n"+
				" Visit:  %s\n"+
				" Code:   %s\n\n"+
				" Waiting for authorization...\n\n",
			m.reAuthProvider, m.reAuthCode.VerificationURI, m.reAuthCode.UserCode)
	}

	footer := " Press ESC to cancel   q: quit\n"
	return header + separator + body + separator + footer
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
