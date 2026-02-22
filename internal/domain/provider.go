package domain

// PipelineProvider is the port interface that all CI provider adapters must implement.
// The domain does not know about GitHub, GitLab, or any specific CI system.
type PipelineProvider interface {
	ListPipelines(repo Repository) ([]Pipeline, error)
	GetPipeline(repo Repository, id string) (Pipeline, error)
}
