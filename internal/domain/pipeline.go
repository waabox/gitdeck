package domain

import "time"

// PipelineStatus represents the execution state of a pipeline, job, or step.
type PipelineStatus string

const (
	StatusPending   PipelineStatus = "pending"
	StatusRunning   PipelineStatus = "running"
	StatusSuccess   PipelineStatus = "success"
	StatusFailed    PipelineStatus = "failed"
	StatusCancelled PipelineStatus = "cancelled"
)

// Step represents a single step within a CI job.
type Step struct {
	Name     string
	Status   PipelineStatus
	Duration time.Duration
}

// Job represents a single unit of work within a pipeline.
type Job struct {
	ID        string
	Name      string
	Stage     string
	Status    PipelineStatus
	Duration  time.Duration
	StartedAt time.Time
	Steps     []Step
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
