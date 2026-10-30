package jobs

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/incurl/wikirevele/internal/pocketbase"
)

// newTestManager creates a manager backed by a throwaway database.
func newTestManager(t *testing.T, opts ...ManagerOption) *Manager {
	t.Helper()

	service := pocketbase.NewService(t.TempDir())
	if err := service.Bootstrap(); err != nil {
		t.Fatalf("bootstrap pocketbase: %v", err)
	}
	t.Cleanup(func() {
		if err := service.Close(); err != nil {
			t.Errorf("close pocketbase: %v", err)
		}
	})

	return NewManager(service.App(), opts...)
}

// waitForStatus waits until the job reaches one of the wanted statuses.
func waitForStatus(t *testing.T, m *Manager, jobID string, wanted ...string) Job {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		job, err := m.GetJob(jobID)
		if err != nil {
			t.Fatalf("get job: %v", err)
		}
		for _, status := range wanted {
			if job.Status == status {
				return job
			}
		}
		time.Sleep(10 * time.Millisecond)
	}

	job, _ := m.GetJob(jobID)
	t.Fatalf("job %q stayed in status %q, wanted one of %v", jobID, job.Status, wanted)
	return Job{}
}

// TestSubmitRequiresPrompt verifies that a job without a prompt is
// rejected.
func TestSubmitRequiresPrompt(t *testing.T) {
	m := newTestManager(t)

	if _, err := m.Submit(Request{}); err == nil {
		t.Fatal("submitting a job without a prompt should fail")
	}
}

// TestJobLifecycle verifies that a job runs to completion and that its
// result is stored.
func TestJobLifecycle(t *testing.T) {
	m := newTestManager(t, WithRunner(RunnerFunc(
		func(ctx context.Context, job Job) (*Result, error) {
			return &Result{Payload: map[string]any{"echo": job.Prompt}}, nil
		},
	)))

	job, err := m.Submit(Request{Prompt: "hello there"})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if job.Status != StatusPending {
		t.Errorf("status = %q, want %q", job.Status, StatusPending)
	}

	job = waitForStatus(t, m, job.ID, StatusCompleted, StatusFailed)
	if job.ErrorMessage != "" {
		t.Fatalf("the job failed: %s", job.ErrorMessage)
	}
	if job.Result["echo"] != "hello there" {
		t.Errorf("result = %v, want the prompt echoed back", job.Result)
	}
}

// TestJobFailure verifies that a failing runner marks the job as
// failed and records the error.
func TestJobFailure(t *testing.T) {
	m := newTestManager(t, WithRunner(RunnerFunc(
		func(ctx context.Context, job Job) (*Result, error) {
			return nil, &Error{Code: 42, Message: "the remote API refused the request"}
		},
	)))

	job, err := m.Submit(Request{Prompt: "do something"})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}

	job = waitForStatus(t, m, job.ID, StatusFailed)
	if job.ErrorCode != 42 {
		t.Errorf("error code = %d, want 42", job.ErrorCode)
	}
	if job.ErrorMessage != "the remote API refused the request" {
		t.Errorf("error message = %q", job.ErrorMessage)
	}
}

// TestJobCancellation verifies that cancelling a running job stops it
// and leaves it cancelled rather than failed.
func TestJobCancellation(t *testing.T) {
	started := make(chan struct{})
	m := newTestManager(t, WithRunner(RunnerFunc(
		func(ctx context.Context, job Job) (*Result, error) {
			close(started)
			<-ctx.Done()
			return nil, ctx.Err()
		},
	)))

	job, err := m.Submit(Request{Prompt: "a long job"})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}

	<-started

	if err := m.Cancel(job.ID); err != nil {
		t.Fatalf("cancel: %v", err)
	}

	job = waitForStatus(t, m, job.ID, StatusCancelled)
	if job.Status != StatusCancelled {
		t.Errorf("status = %q, want %q", job.Status, StatusCancelled)
	}
}

// TestCancelWithoutRunnerFails verifies that a job cannot be cancelled
// once it has already failed.
func TestCancelFailedJob(t *testing.T) {
	m := newTestManager(t, WithRunner(RunnerFunc(
		func(ctx context.Context, job Job) (*Result, error) {
			return nil, errors.New("nope")
		},
	)))

	job, err := m.Submit(Request{Prompt: "fail"})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	waitForStatus(t, m, job.ID, StatusFailed)

	if err := m.Cancel(job.ID); err == nil {
		t.Fatal("cancelling a failed job should fail")
	}
}

// TestDeleteRunningJobIsRefused verifies that a running job cannot be
// deleted, so that it cannot write to the database after its removal.
func TestDeleteRunningJobIsRefused(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	m := newTestManager(t, WithRunner(RunnerFunc(
		func(ctx context.Context, job Job) (*Result, error) {
			close(started)
			<-release
			return &Result{}, nil
		},
	)))

	job, err := m.Submit(Request{Prompt: "still running"})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	<-started

	if err := m.DeleteJob(job.ID); err == nil {
		t.Fatal("deleting a running job should fail")
	}

	close(release)
	waitForStatus(t, m, job.ID, StatusCompleted)
}

// TestDeleteJobRemovesIt verifies that a finished job can be removed.
func TestDeleteJobRemovesIt(t *testing.T) {
	m := newTestManager(t, WithRunner(RunnerFunc(
		func(ctx context.Context, job Job) (*Result, error) {
			return &Result{}, nil
		},
	)))

	job, err := m.Submit(Request{Prompt: "temporary"})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	waitForStatus(t, m, job.ID, StatusCompleted)

	if err := m.DeleteJob(job.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}

	jobs, err := m.ListJobs(10)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	for _, listed := range jobs {
		if listed.ID == job.ID {
			t.Error("the deleted job is still listed")
		}
	}
}

// TestArtifactStorage verifies that a stored artifact is persisted and
// that its bytes can be read back.
func TestArtifactStorage(t *testing.T) {
	m := newTestManager(t)

	// A real 1x1 PNG.
	png := []byte{
		0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a,
		0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53,
		0xde, 0x00, 0x00, 0x00, 0x0c, 0x49, 0x44, 0x41,
		0x54, 0x08, 0xd7, 0x63, 0xf8, 0xcf, 0xc0, 0x00,
		0x00, 0x03, 0x01, 0x01, 0x00, 0x18, 0xdd, 0x8d,
		0xb0, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4e,
		0x44, 0xae, 0x42, 0x60, 0x82,
	}

	artifact, err := m.StoreArtifact("", ArtifactKindOutput, png, "sample.png", nil)
	if err != nil {
		t.Fatalf("store artifact: %v", err)
	}
	if artifact.Mime != "image/png" {
		t.Errorf("mime = %q, want image/png", artifact.Mime)
	}
	if artifact.Width != 1 || artifact.Height != 1 {
		t.Errorf("dimensions = %dx%d, want 1x1", artifact.Width, artifact.Height)
	}
	if artifact.URL == "" {
		t.Error("the artifact has no URL")
	}

	data, err := m.ArtifactBytes(artifact.ID)
	if err != nil {
		t.Fatalf("read artifact: %v", err)
	}
	if len(data) != len(png) {
		t.Errorf("read %d bytes, want %d", len(data), len(png))
	}
}
