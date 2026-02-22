package github_test

import (
	"encoding/json"
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

	adapter := githubprovider.NewAdapter("test-token", srv.URL)
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

	adapter := githubprovider.NewAdapter("test-token", srv.URL)
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
