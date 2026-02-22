package domain

import "time"

// PipelineStatus represents the execution state of a pipeline or job.
type PipelineStatus string

const (
	StatusPending   PipelineStatus = "pending"
	StatusRunning   PipelineStatus = "running"
	StatusSuccess   PipelineStatus = "success"
	StatusFailed    PipelineStatus = "failed"
	StatusCancelled PipelineStatus = "cancelled"
)

// Job represents a single unit of work within a pipeline.
type Job struct {
	ID        string
	Name      string
	Stage     string
	Status    PipelineStatus
	Duration  time.Duration
	StartedAt time.Time
}

// Pipeline represents a CI pipeline run.
type Pipeline struct {
	ID        string
	Branch    string
	CommitSHA string
	CommitMsg string
	Author    string
	Status    PipelineStatus
	CreatedAt time.Time
	Duration  time.Duration
	Jobs      []Job
}
