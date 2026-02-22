package github

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/waabox/gitdeck/internal/domain"
)

const defaultBaseURL = "https://api.github.com"

// Adapter implements domain.PipelineProvider for GitHub Actions.
type Adapter struct {
	token   string
	baseURL string
	limit   int
	client  *http.Client
}

// NewAdapter creates a GitHub Actions adapter.
// baseURL is used for testing; pass empty string to use the real GitHub API.
// limit controls how many pipeline runs are fetched; must be >= 1.
func NewAdapter(token string, baseURL string, limit int) *Adapter {
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	return &Adapter{
		token:   token,
		baseURL: baseURL,
		limit:   limit,
		client:  &http.Client{Timeout: 15 * time.Second},
	}
}

// ListPipelines returns the most recent workflow runs for the repository.
func (a *Adapter) ListPipelines(repo domain.Repository) ([]domain.Pipeline, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/actions/runs?per_page=%d", a.baseURL, repo.Owner, repo.Name, a.limit)
	var result struct {
		WorkflowRuns []workflowRun `json:"workflow_runs"`
	}
	if err := a.get(url, &result); err != nil {
		return nil, err
	}
	pipelines := make([]domain.Pipeline, len(result.WorkflowRuns))
	for i, run := range result.WorkflowRuns {
		pipelines[i] = run.toPipeline()
	}
	return pipelines, nil
}

// GetPipeline returns a single workflow run with all its jobs.
func (a *Adapter) GetPipeline(repo domain.Repository, id domain.PipelineID) (domain.Pipeline, error) {
	runURL := fmt.Sprintf("%s/repos/%s/%s/actions/runs/%s", a.baseURL, repo.Owner, repo.Name, id)
	var run workflowRun
	if err := a.get(runURL, &run); err != nil {
		return domain.Pipeline{}, err
	}

	jobsURL := fmt.Sprintf("%s/repos/%s/%s/actions/runs/%s/jobs", a.baseURL, repo.Owner, repo.Name, id)
	var jobsResult struct {
		Jobs []workflowJob `json:"jobs"`
	}
	if err := a.get(jobsURL, &jobsResult); err != nil {
		return domain.Pipeline{}, err
	}

	pipeline := run.toPipeline()
	pipeline.Jobs = make([]domain.Job, len(jobsResult.Jobs))
	for i, j := range jobsResult.Jobs {
		pipeline.Jobs[i] = j.toJob()
	}
	return pipeline, nil
}

func (a *Adapter) get(url string, target interface{}) error {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+a.token)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := a.client.Do(req)
	if err != nil {
		return fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("github API error: %s", resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(target)
}

// workflowRun is the raw GitHub API response shape for a workflow run.
type workflowRun struct {
	ID         int64  `json:"id"`
	HeadBranch string `json:"head_branch"`
	HeadSHA    string `json:"head_sha"`
	HeadCommit struct {
		Message string `json:"message"`
		Author  struct {
			Name string `json:"name"`
		} `json:"author"`
	} `json:"head_commit"`
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
}

func (r workflowRun) toPipeline() domain.Pipeline {
	created, _ := time.Parse(time.RFC3339, r.CreatedAt)
	updated, _ := time.Parse(time.RFC3339, r.UpdatedAt)
	var duration time.Duration
	if !created.IsZero() && !updated.IsZero() {
		duration = updated.Sub(created)
	}
	return domain.Pipeline{
		ID:        strconv.FormatInt(r.ID, 10),
		Branch:    r.HeadBranch,
		CommitSHA: r.HeadSHA,
		CommitMsg: r.HeadCommit.Message,
		Author:    r.HeadCommit.Author.Name,
		Status:    mapGitHubStatus(r.Status, r.Conclusion),
		CreatedAt: created,
		Duration:  duration,
	}
}

// workflowJob is the raw GitHub API response shape for a job.
type workflowJob struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Status      string `json:"status"`
	Conclusion  string `json:"conclusion"`
	StartedAt   string `json:"started_at"`
	CompletedAt string `json:"completed_at"`
}

func (j workflowJob) toJob() domain.Job {
	started, _ := time.Parse(time.RFC3339, j.StartedAt)
	completed, _ := time.Parse(time.RFC3339, j.CompletedAt)
	var duration time.Duration
	if !started.IsZero() && !completed.IsZero() {
		duration = completed.Sub(started)
	}
	return domain.Job{
		ID:        strconv.FormatInt(j.ID, 10),
		Name:      j.Name,
		Status:    mapGitHubStatus(j.Status, j.Conclusion),
		StartedAt: started,
		Duration:  duration,
	}
}

func mapGitHubStatus(status, conclusion string) domain.PipelineStatus {
	if status == "in_progress" || status == "queued" || status == "waiting" {
		return domain.StatusRunning
	}
	if status == "completed" {
		switch conclusion {
		case "success":
			return domain.StatusSuccess
		case "failure", "timed_out":
			return domain.StatusFailed
		case "cancelled":
			return domain.StatusCancelled
		}
	}
	return domain.StatusPending
}
