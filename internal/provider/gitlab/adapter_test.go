package gitlab_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/waabox/gitdeck/internal/domain"
	gitlabprovider "github.com/waabox/gitdeck/internal/provider/gitlab"
)

func TestListPipelines_ReturnsPipelines(t *testing.T) {
	response := []map[string]interface{}{
		{
			"id":         float64(201),
			"ref":        "main",
			"sha":        "def5678",
			"status":     "success",
			"created_at": time.Now().Add(-1 * time.Hour).Format(time.RFC3339),
			"updated_at": time.Now().Add(-55 * time.Minute).Format(time.RFC3339),
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.RequestURI == "/api/v4/projects/mygroup%2Fmyproject/pipelines" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	adapter := gitlabprovider.NewAdapter("test-token", srv.URL)
	repo := domain.Repository{Owner: "mygroup", Name: "myproject"}

	pipelines, err := adapter.ListPipelines(repo)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pipelines) != 1 {
		t.Fatalf("expected 1 pipeline, got %d", len(pipelines))
	}
	if pipelines[0].ID != "201" {
		t.Errorf("expected ID '201', got '%s'", pipelines[0].ID)
	}
	if pipelines[0].Status != domain.StatusSuccess {
		t.Errorf("expected status success, got '%s'", pipelines[0].Status)
	}
}

func TestGetPipeline_ReturnsPipelineWithJobs(t *testing.T) {
	pipelineResponse := map[string]interface{}{
		"id":         float64(201),
		"ref":        "main",
		"sha":        "def5678",
		"status":     "failed",
		"created_at": time.Now().Add(-10 * time.Minute).Format(time.RFC3339),
		"updated_at": time.Now().Format(time.RFC3339),
	}
	jobsResponse := []map[string]interface{}{
		{
			"id":          float64(301),
			"name":        "build",
			"stage":       "build",
			"status":      "success",
			"started_at":  time.Now().Add(-9 * time.Minute).Format(time.RFC3339),
			"finished_at": time.Now().Add(-7 * time.Minute).Format(time.RFC3339),
		},
		{
			"id":          float64(302),
			"name":        "test",
			"stage":       "test",
			"status":      "failed",
			"started_at":  time.Now().Add(-7 * time.Minute).Format(time.RFC3339),
			"finished_at": time.Now().Format(time.RFC3339),
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.RequestURI {
		case "/api/v4/projects/mygroup%2Fmyproject/pipelines/201":
			json.NewEncoder(w).Encode(pipelineResponse)
		case "/api/v4/projects/mygroup%2Fmyproject/pipelines/201/jobs":
			json.NewEncoder(w).Encode(jobsResponse)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	adapter := gitlabprovider.NewAdapter("test-token", srv.URL)
	repo := domain.Repository{Owner: "mygroup", Name: "myproject"}

	pipeline, err := adapter.GetPipeline(repo, "201")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pipeline.Status != domain.StatusFailed {
		t.Errorf("expected status failed, got '%s'", pipeline.Status)
	}
	if len(pipeline.Jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(pipeline.Jobs))
	}
	if pipeline.Jobs[1].Stage != "test" {
		t.Errorf("expected second job stage 'test', got '%s'", pipeline.Jobs[1].Stage)
	}
}
