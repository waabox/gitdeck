package domain

// PipelineID is the unique identifier for a pipeline run.
// Using a distinct type prevents confusion with other string parameters.
type PipelineID string

// PipelineProvider is the port interface that all CI provider adapters must implement.
// The domain does not know about GitHub, GitLab, or any specific CI system.
//
// ListPipelines returns the most recent pipeline runs for the repository.
// Implementations should return the last 20-30 runs as provided by the API default page size.
//
// GetPipeline returns a single pipeline with its full job list.
//
// GetJobLogs returns the full raw log text for the given job ID.
//
// RerunPipeline triggers a new run of the given pipeline.
//
// CancelPipeline cancels a running pipeline.
type PipelineProvider interface {
	ListPipelines(repo Repository) ([]Pipeline, error)
	GetPipeline(repo Repository, id PipelineID) (Pipeline, error)
	GetJobLogs(repo Repository, jobID string) (string, error)
	RerunPipeline(repo Repository, id PipelineID) error
	CancelPipeline(repo Repository, id PipelineID) error
}
