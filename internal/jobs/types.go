// Package jobs owns the generic job lifecycle: creation, execution
// against a pluggable runner, persistence and status reporting.
//
// The queue deliberately models no domain. A job carries a title, an
// arbitrary set of parameters and an arbitrary result, and the work
// itself is delegated to a Runner, which the application registers.
//
// The queue exists to serialise work against a limited backend, to allow
// cancellation, and to give the UI a durable history.
package jobs

import (
	"context"
)

// Request is the input for submitting a job.
type Request struct {
	// Title is a short human readable label shown in the queue. It is
	// optional: the queue falls back to a generated one.
	Title string `json:"title"`
	// Prompt is the free-form instruction of the job. What it means is
	// decided by the Runner.
	Prompt string `json:"prompt"`
	// Params are the runner-specific parameters. They are stored
	// verbatim, so the application defines the shape.
	Params map[string]any `json:"params"`
	// InputArtifactIDs lists the artifact records the job consumes.
	InputArtifactIDs []string `json:"input_artifact_ids"`
}

// Job is the frontend-facing view of a job.
type Job struct {
	// ID is the PocketBase record id.
	ID string `json:"id"`
	// Title is the short label of the job.
	Title string `json:"title"`
	// Prompt is the free-form instruction of the job.
	Prompt string `json:"prompt"`
	// Params are the runner-specific parameters.
	Params map[string]any `json:"params"`
	// Result is the runner-specific result, if the job completed.
	Result map[string]any `json:"result"`
	// Status is one of pending/running/completed/failed/cancelled.
	Status string `json:"status"`
	// ErrorCode is the numeric failure code, when the runner supplies
	// one.
	ErrorCode int `json:"error_code"`
	// ErrorMessage is the human readable failure explanation.
	ErrorMessage string `json:"error_message"`
	// ArtifactIDs lists the artifact record ids the job produced.
	ArtifactIDs []string `json:"artifact_ids"`
	// InputArtifactIDs lists the artifact record ids the job consumed.
	//
	// They are tracked separately from the outputs, because a retry
	// re-uses the inputs while the outputs stay with the job that
	// produced them.
	InputArtifactIDs []string `json:"input_artifact_ids"`
	// Created and Updated are ISO-8601 timestamps.
	Created string `json:"created"`
	Updated string `json:"updated"`
}

// Result is what a Runner returns.
//
// The queue defines no fields: the application decides what a
// successful job produced. ArtifactIDs lists the files that were
// stored, and the rest of the payload is returned to the frontend
// as-is.
type Result struct {
	// ArtifactIDs lists the artifact record ids the job produced.
	ArtifactIDs []string `json:"artifact_ids"`
	// Payload is the runner-specific result, stored verbatim.
	Payload map[string]any `json:"payload"`
}

// Error is a failure reported by a Runner.
//
// It carries an optional numeric code, so the UI can branch on the
// kind of failure without parsing the message.
type Error struct {
	// Code is the numeric failure code. Zero means "no code".
	Code int `json:"code"`
	// Message is the human readable explanation.
	Message string `json:"message"`
	// Err is the underlying error, if any.
	Err error `json:"-"`
}

// Error implements the error interface.
func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.Message != "" {
		return e.Message
	}
	if e.Err != nil {
		return e.Err.Error()
	}
	return "unknown error"
}

// Unwrap exposes the underlying error to errors.Is and errors.As.
func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// Runner executes a job.
//
// This is the extension point of the queue: implement it to plug your
// own backend into the queue. The manager calls Run once per job, on a
// goroutine, with a context that is cancelled when the user cancels
// the job.
//
// A Runner must honour ctx and return promptly when it is cancelled.
// Returning an error marks the job as failed; returning a nil error
// marks it as completed.
type Runner interface {
	// Run executes the job and returns its result.
	Run(ctx context.Context, job Job) (*Result, error)
}

// RunnerFunc adapts a plain function to the Runner interface.
type RunnerFunc func(ctx context.Context, job Job) (*Result, error)

// Run calls the wrapped function.
func (f RunnerFunc) Run(ctx context.Context, job Job) (*Result, error) {
	return f(ctx, job)
}
