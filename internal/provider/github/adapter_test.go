package github_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/waabox/gitdeck/internal/domain"
	githubprovider "github.com/waabox/gitdeck/internal/provider/github"
)

func TestListPipelines_ReturnsWorkflowRuns(t *testing.T) {
	response := map[string]interface{}{
		"workflow_runs": []map[string]interface{}{
			{
				"id":          float64(1001),
				"head_branch": "main",
				"head_sha":    "abc1234",
				"head_commit": map[string]interface{}{
					"message": "fix: login timeout",
					"author":  map[string]interface{}{"name": "waabox"},
				},
				"status":     "completed",
				"conclusion": "success",
				"created_at": time.Now().Add(-2 * time.Minute).Format(time.RFC3339),
				"updated_at": time.Now().Format(time.RFC3339),
			},
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/repos/waabox/gitdeck/actions/runs" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	adapter := githubprovider.NewAdapter("test-token", srv.URL, 3)
	repo := domain.Repository{Owner: "waabox", Name: "gitdeck"}

	pipelines, err := adapter.ListPipelines(repo)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pipelines) != 1 {
		t.Fatalf("expected 1 pipeline, got %d", len(pipelines))
	}
	p := pipelines[0]
	if p.ID != "1001" {
		t.Errorf("expected ID '1001', got '%s'", p.ID)
	}
	if p.Branch != "main" {
		t.Errorf("expected branch 'main', got '%s'", p.Branch)
	}
	if p.Status != domain.StatusSuccess {
		t.Errorf("expected status success, got '%s'", p.Status)
	}
}

func TestGetPipeline_ReturnsRunWithJobs(t *testing.T) {
	runResponse := map[string]interface{}{
		"id":          float64(1001),
		"head_branch": "main",
		"head_sha":    "abc1234",
		"head_commit": map[string]interface{}{
			"message": "fix: login timeout",
			"author":  map[string]interface{}{"name": "waabox"},
		},
		"status":     "completed",
		"conclusion": "failure",
		"created_at": time.Now().Add(-5 * time.Minute).Format(time.RFC3339),
		"updated_at": time.Now().Format(time.RFC3339),
	}
	jobsResponse := map[string]interface{}{
		"jobs": []map[string]interface{}{
			{
				"id":           float64(2001),
				"name":         "build",
				"status":       "completed",
				"conclusion":   "success",
				"started_at":   time.Now().Add(-4 * time.Minute).Format(time.RFC3339),
				"completed_at": time.Now().Add(-3 * time.Minute).Format(time.RFC3339),
			},
			{
				"id":           float64(2002),
				"name":         "test",
				"status":       "completed",
				"conclusion":   "failure",
				"started_at":   time.Now().Add(-3 * time.Minute).Format(time.RFC3339),
				"completed_at": time.Now().Format(time.RFC3339),
			},
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/repos/waabox/gitdeck/actions/runs/1001":
			json.NewEncoder(w).Encode(runResponse)
		case "/repos/waabox/gitdeck/actions/runs/1001/jobs":
			json.NewEncoder(w).Encode(jobsResponse)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	adapter := githubprovider.NewAdapter("test-token", srv.URL, 3)
	repo := domain.Repository{Owner: "waabox", Name: "gitdeck"}

	pipeline, err := adapter.GetPipeline(repo, "1001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pipeline.Status != domain.StatusFailed {
		t.Errorf("expected status failed, got '%s'", pipeline.Status)
	}
	if len(pipeline.Jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(pipeline.Jobs))
	}
	if pipeline.Jobs[0].Name != "build" {
		t.Errorf("expected first job 'build', got '%s'", pipeline.Jobs[0].Name)
	}
}

func TestGetPipeline_ParsesJobSteps(t *testing.T) {
	runResponse := map[string]interface{}{
		"id":          float64(1001),
		"head_branch": "main",
		"head_sha":    "abc1234",
		"head_commit": map[string]interface{}{
			"message": "fix: login timeout",
			"author":  map[string]interface{}{"name": "waabox"},
		},
		"status":     "in_progress",
		"conclusion": nil,
		"created_at": time.Now().Add(-2 * time.Minute).Format(time.RFC3339),
		"updated_at": time.Now().Format(time.RFC3339),
	}
	jobsResponse := map[string]interface{}{
		"jobs": []map[string]interface{}{
			{
				"id":           float64(2001),
				"name":         "test",
				"status":       "in_progress",
				"conclusion":   nil,
				"started_at":   time.Now().Add(-1 * time.Minute).Format(time.RFC3339),
				"completed_at": "",
				"steps": []map[string]interface{}{
					{
						"name":         "Set up job",
						"status":       "completed",
						"conclusion":   "success",
						"started_at":   time.Now().Add(-60 * time.Second).Format(time.RFC3339),
						"completed_at": time.Now().Add(-55 * time.Second).Format(time.RFC3339),
					},
					{
						"name":         "Run tests",
						"status":       "in_progress",
						"conclusion":   nil,
						"started_at":   time.Now().Add(-55 * time.Second).Format(time.RFC3339),
						"completed_at": "",
					},
				},
			},
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/repos/waabox/gitdeck/actions/runs/1001":
			json.NewEncoder(w).Encode(runResponse)
		case "/repos/waabox/gitdeck/actions/runs/1001/jobs":
			json.NewEncoder(w).Encode(jobsResponse)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	adapter := githubprovider.NewAdapter("test-token", srv.URL, 3)
	repo := domain.Repository{Owner: "waabox", Name: "gitdeck"}

	pipeline, err := adapter.GetPipeline(repo, "1001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pipeline.Jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(pipeline.Jobs))
	}
	job := pipeline.Jobs[0]
	if len(job.Steps) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(job.Steps))
	}
	if job.Steps[0].Name != "Set up job" {
		t.Errorf("expected first step 'Set up job', got '%s'", job.Steps[0].Name)
	}
	if job.Steps[0].Status != domain.StatusSuccess {
		t.Errorf("expected first step status success, got '%s'", job.Steps[0].Status)
	}
	if job.Steps[1].Status != domain.StatusRunning {
		t.Errorf("expected second step status running, got '%s'", job.Steps[1].Status)
	}
}

func TestListPipelines_Returns_ErrUnauthorized_On401(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	adapter := githubprovider.NewAdapter("expired-token", srv.URL, 3)
	repo := domain.Repository{Owner: "owner", Name: "repo"}

	_, err := adapter.ListPipelines(repo)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, domain.ErrUnauthorized) {
		t.Errorf("expected ErrUnauthorized, got: %v", err)
	}
}

func TestGetJobLogs_Returns_ErrUnauthorized_On401(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	adapter := githubprovider.NewAdapter("expired-token", srv.URL, 3)
	repo := domain.Repository{Owner: "owner", Name: "repo"}

	_, err := adapter.GetJobLogs(repo, domain.JobID("123"))
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, domain.ErrUnauthorized) {
		t.Errorf("expected ErrUnauthorized, got: %v", err)
	}
}

func TestRerunPipeline_Returns_ErrUnauthorized_On401(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	adapter := githubprovider.NewAdapter("expired-token", srv.URL, 3)
	repo := domain.Repository{Owner: "owner", Name: "repo"}

	err := adapter.RerunPipeline(repo, domain.PipelineID("123"))
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, domain.ErrUnauthorized) {
		t.Errorf("expected ErrUnauthorized, got: %v", err)
	}
}

func TestRerunPipeline_PostsToCorrectEndpoint(t *testing.T) {
	rerunCalled := false

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/repos/waabox/gitdeck/actions/runs/1001/rerun" {
			rerunCalled = true
			w.WriteHeader(http.StatusNoContent)
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	adapter := githubprovider.NewAdapter("test-token", srv.URL, 3)
	repo := domain.Repository{Owner: "waabox", Name: "gitdeck"}

	err := adapter.RerunPipeline(repo, domain.PipelineID("1001"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !rerunCalled {
		t.Error("expected rerun endpoint to be called")
	}
}

func TestCancelPipeline_PostsToCorrectEndpoint(t *testing.T) {
	cancelCalled := false

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/repos/waabox/gitdeck/actions/runs/1001/cancel" {
			cancelCalled = true
			w.WriteHeader(http.StatusAccepted)
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	adapter := githubprovider.NewAdapter("test-token", srv.URL, 3)
	repo := domain.Repository{Owner: "waabox", Name: "gitdeck"}

	err := adapter.CancelPipeline(repo, domain.PipelineID("1001"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cancelCalled {
		t.Error("expected cancel endpoint to be called")
	}
}

func TestGetJobLogs_ReturnsLogText(t *testing.T) {
	expectedLog := "##[group]Set up job\nRun actions/checkout@v4\n##[endgroup]\nok all tests pass"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/repos/waabox/gitdeck/actions/jobs/2001/logs" {
			w.Header().Set("Content-Type", "text/plain")
			fmt.Fprint(w, expectedLog)
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	adapter := githubprovider.NewAdapter("test-token", srv.URL, 3)
	repo := domain.Repository{Owner: "waabox", Name: "gitdeck"}

	logs, err := adapter.GetJobLogs(repo, domain.JobID("2001"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if logs != expectedLog {
		t.Errorf("expected log text %q, got %q", expectedLog, logs)
	}
}
