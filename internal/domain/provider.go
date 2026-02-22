package domain

// PipelineID is the unique identifier for a pipeline run.
// Using a distinct type prevents confusion with other string parameters.
type PipelineID string

// PipelineProvider is the port interface that all CI provider adapters must implement.
// The domain does not know about GitHub, GitLab, or any specific CI system.
type PipelineProvider interface {
	// ListPipelines returns the most recent pipeline runs for the repository.
	// Implementations should return the last 20-30 runs as provided by the API default page size.
	ListPipelines(repo Repository) ([]Pipeline, error)

	// GetPipeline returns a single pipeline with its full job list.
	GetPipeline(repo Repository, id PipelineID) (Pipeline, error)

	// GetJobLogs returns the full raw log text for the given job ID.
	GetJobLogs(repo Repository, jobID JobID) (string, error)

	// RerunPipeline triggers a new run of the given pipeline.
	RerunPipeline(repo Repository, id PipelineID) error

	// CancelPipeline cancels a running pipeline.
	CancelPipeline(repo Repository, id PipelineID) error
}
