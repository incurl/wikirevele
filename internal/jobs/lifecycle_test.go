package jobs

import (
	"context"
	"testing"
	"time"
)

// TestJobTimeoutStopsAHungRunner verifies that a runner that never
// returns is stopped and the job is marked as failed.
//
// This is the regression test for the finding of the 2026-10-28 audit:
// without a timeout, a hung runner holds its slot forever and the
// application stops making progress.
func TestJobTimeoutStopsAHungRunner(t *testing.T) {
	m := newTestManager(t, WithTimeout(200*time.Millisecond), WithRunner(RunnerFunc(
		func(ctx context.Context, job Job) (*Result, error) {
			// A runner that blocks until it is cancelled.
			<-ctx.Done()
			return nil, ctx.Err()
		},
	)))

	job, err := m.Submit(Request{Prompt: "a hung job"})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}

	finished := waitForStatus(t, m, job.ID, StatusFailed, StatusCompleted, StatusCancelled)
	if finished.Status != StatusFailed {
		t.Fatalf("status = %q, want %q", finished.Status, StatusFailed)
	}
	if finished.ErrorMessage == "" {
		t.Error("the job has no explanation")
	}
}

// TestJobTimeoutFreesTheQueue verifies that a timed-out job releases
// its slot, so that the application keeps making progress.
func TestJobTimeoutFreesTheQueue(t *testing.T) {
	m := newTestManager(t, WithTimeout(200*time.Millisecond), WithConcurrency(1), WithRunner(RunnerFunc(
		func(ctx context.Context, job Job) (*Result, error) {
			<-ctx.Done()
			return nil, ctx.Err()
		},
	)))

	first, err := m.Submit(Request{Prompt: "the first"})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	waitForStatus(t, m, first.ID, StatusFailed, StatusCompleted, StatusCancelled)

	// The queue must be free again, so the second job runs.
	second, err := m.Submit(Request{Prompt: "the second"})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	waitForStatus(t, m, second.ID, StatusFailed, StatusCompleted, StatusCancelled)
}

// TestShutdownCancelsTheRunningJobs verifies that a shutdown stops the
// jobs that are running.
//
// This is the regression test for the finding of the 2026-10-28 audit:
// without a shutdown, quitting the application killed the jobs
// mid-write.
func TestShutdownCancelsTheRunningJobs(t *testing.T) {
	started := make(chan struct{})
	stopped := make(chan struct{})

	m := newTestManager(t, WithRunner(RunnerFunc(
		func(ctx context.Context, job Job) (*Result, error) {
			close(started)
			<-ctx.Done()
			close(stopped)
			return nil, ctx.Err()
		},
	)))

	job, err := m.Submit(Request{Prompt: "a long job"})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	<-started

	m.ShutdownWithTimeout(2 * time.Second)

	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("the running job was not stopped by the shutdown")
	}

	// The job must not be left in the running state.
	finished := waitForStatus(t, m, job.ID, StatusFailed, StatusCompleted, StatusCancelled)
	if finished.Status == StatusRunning || finished.Status == StatusPending {
		t.Errorf("the job is %q, want it to be resolved", finished.Status)
	}
}

// TestShutdownIsIdempotent verifies that shutting down twice is safe.
func TestShutdownIsIdempotent(t *testing.T) {
	m := newTestManager(t)

	m.ShutdownWithTimeout(time.Second)
	m.ShutdownWithTimeout(time.Second)
}

// TestShutdownWithoutARunningJob verifies that shutting down an idle
// manager returns immediately.
func TestShutdownWithoutARunningJob(t *testing.T) {
	m := newTestManager(t)

	start := time.Now()
	m.ShutdownWithTimeout(2 * time.Second)
	elapsed := time.Since(start)

	if elapsed > time.Second {
		t.Errorf("the shutdown took %v, want it to return immediately", elapsed)
	}
}
