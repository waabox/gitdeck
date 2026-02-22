package gitlab

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/waabox/gitdeck/internal/domain"
)

const defaultBaseURL = "https://gitlab.com"

// Adapter implements domain.PipelineProvider for GitLab CI.
type Adapter struct {
	token   string
	baseURL string
	client  *http.Client
}

// NewAdapter creates a GitLab CI adapter.
// baseURL can be a self-hosted GitLab instance URL; pass empty string for gitlab.com.
func NewAdapter(token string, baseURL string) *Adapter {
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	return &Adapter{
		token:   token,
		baseURL: baseURL,
		client:  &http.Client{Timeout: 15 * time.Second},
	}
}

// ListPipelines returns the most recent pipelines for the repository.
func (a *Adapter) ListPipelines(repo domain.Repository) ([]domain.Pipeline, error) {
	projectID := url.PathEscape(repo.Owner + "/" + repo.Name)
	apiURL := fmt.Sprintf("%s/api/v4/projects/%s/pipelines", a.baseURL, projectID)
	var runs []gitLabPipeline
	if err := a.get(apiURL, &runs); err != nil {
		return nil, err
	}
	pipelines := make([]domain.Pipeline, len(runs))
	for i, r := range runs {
		pipelines[i] = r.toPipeline()
	}
	return pipelines, nil
}

// GetPipeline returns a single pipeline with all its jobs.
func (a *Adapter) GetPipeline(repo domain.Repository, id domain.PipelineID) (domain.Pipeline, error) {
	projectID := url.PathEscape(repo.Owner + "/" + repo.Name)

	pipelineURL := fmt.Sprintf("%s/api/v4/projects/%s/pipelines/%s", a.baseURL, projectID, id)
	var run gitLabPipeline
	if err := a.get(pipelineURL, &run); err != nil {
		return domain.Pipeline{}, err
	}

	jobsURL := fmt.Sprintf("%s/api/v4/projects/%s/pipelines/%s/jobs", a.baseURL, projectID, id)
	var rawJobs []gitLabJob
	if err := a.get(jobsURL, &rawJobs); err != nil {
		return domain.Pipeline{}, err
	}

	pipeline := run.toPipeline()
	pipeline.Jobs = make([]domain.Job, len(rawJobs))
	for i, j := range rawJobs {
		pipeline.Jobs[i] = j.toJob()
	}
	return pipeline, nil
}

func (a *Adapter) get(apiURL string, target interface{}) error {
	req, err := http.NewRequest(http.MethodGet, apiURL, nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("PRIVATE-TOKEN", a.token)

	resp, err := a.client.Do(req)
	if err != nil {
		return fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("gitlab API error: %s", resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(target)
}

type gitLabPipeline struct {
	ID        int64  `json:"id"`
	Ref       string `json:"ref"`
	SHA       string `json:"sha"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

func (r gitLabPipeline) toPipeline() domain.Pipeline {
	created, _ := time.Parse(time.RFC3339, r.CreatedAt)
	updated, _ := time.Parse(time.RFC3339, r.UpdatedAt)
	var duration time.Duration
	if !created.IsZero() && !updated.IsZero() {
		duration = updated.Sub(created)
	}
	return domain.Pipeline{
		ID:        strconv.FormatInt(r.ID, 10),
		Branch:    r.Ref,
		CommitSHA: r.SHA,
		Status:    mapGitLabStatus(r.Status),
		CreatedAt: created,
		Duration:  duration,
	}
}

type gitLabJob struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	Stage      string `json:"stage"`
	Status     string `json:"status"`
	StartedAt  string `json:"started_at"`
	FinishedAt string `json:"finished_at"`
}

func (j gitLabJob) toJob() domain.Job {
	started, _ := time.Parse(time.RFC3339, j.StartedAt)
	finished, _ := time.Parse(time.RFC3339, j.FinishedAt)
	var duration time.Duration
	if !started.IsZero() && !finished.IsZero() {
		duration = finished.Sub(started)
	}
	return domain.Job{
		ID:        strconv.FormatInt(j.ID, 10),
		Name:      j.Name,
		Stage:     j.Stage,
		Status:    mapGitLabStatus(j.Status),
		StartedAt: started,
		Duration:  duration,
	}
}

func mapGitLabStatus(status string) domain.PipelineStatus {
	switch status {
	case "success":
		return domain.StatusSuccess
	case "failed":
		return domain.StatusFailed
	case "running":
		return domain.StatusRunning
	case "pending", "created", "waiting_for_resource", "preparing", "scheduled":
		return domain.StatusPending
	case "canceled":
		return domain.StatusCancelled
	default:
		return domain.StatusPending
	}
}
