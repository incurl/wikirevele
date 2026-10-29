package bindings

import (
	"context"
	"testing"
	"time"

	"github.com/incurl/valkyrie/internal/console"
	"github.com/incurl/valkyrie/internal/jobs"
	"github.com/incurl/valkyrie/internal/pocketbase"
)

// newTestService creates an application service backed by a throwaway
// database.
func newTestService(t *testing.T, runner jobs.Runner) (*AppService, *jobs.Manager) {
	t.Helper()

	pb := pocketbase.NewService(t.TempDir())
	if err := pb.Bootstrap(); err != nil {
		t.Fatalf("bootstrap pocketbase: %v", err)
	}
	t.Cleanup(func() {
		_ = pb.Close()
	})

	cipher, err := pocketbase.NewCipher(pb.DataDir())
	if err != nil {
		t.Fatalf("create cipher: %v", err)
	}
	settings := pocketbase.NewSettingsStore(pb.App(), cipher)

	log := console.New()
	manager := jobs.NewManager(pb.App(), jobs.WithRunner(runner), jobs.WithLog(log))

	return NewAppService(pb, settings, manager, log), manager
}

// okRunner is a runner that always succeeds.
func okRunner() jobs.Runner {
	return jobs.RunnerFunc(func(ctx context.Context, job jobs.Job) (*jobs.Result, error) {
		return &jobs.Result{}, nil
	})
}

// waitForStatus waits until the job reaches one of the wanted statuses.
func waitForStatus(t *testing.T, m *jobs.Manager, jobID string, wanted ...string) {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		job, err := m.GetJob(jobID)
		if err != nil {
			t.Fatalf("get job: %v", err)
		}
		for _, status := range wanted {
			if job.Status == status {
				return
			}
		}
		time.Sleep(10 * time.Millisecond)
	}

	job, _ := m.GetJob(jobID)
	t.Fatalf("job %q stayed in status %q, wanted one of %v", jobID, job.Status, wanted)
}

// TestRetryJobKeepsTheOriginalOutputs is the regression test for the
// data-loss bug found in the 2026-10-28 audit.
//
// A retry used to re-link the original job's *outputs* to the new job.
// The original then lost its artifacts, and deleting the retry deleted
// the original's files.
func TestRetryJobKeepsTheOriginalOutputs(t *testing.T) {
	service, manager := newTestService(t, okRunner())

	// The original job, which produced an artifact.
	original, err := service.SubmitJob(jobs.Request{Prompt: "a red bicycle"})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	waitForStatus(t, manager, original.ID, jobs.StatusCompleted, jobs.StatusFailed)

	artifact, err := manager.StoreArtifact(original.ID, jobs.ArtifactKindOutput, []byte("image bytes"), "result.png", nil)
	if err != nil {
		t.Fatalf("store artifact: %v", err)
	}

	before, err := manager.GetJob(original.ID)
	if err != nil {
		t.Fatalf("get job: %v", err)
	}
	if len(before.ArtifactIDs) != 1 {
		t.Fatalf("the original has %d artifacts, want 1", len(before.ArtifactIDs))
	}

	// The retry.
	retry, err := service.RetryJob(original.ID)
	if err != nil {
		t.Fatalf("retry: %v", err)
	}
	waitForStatus(t, manager, retry.ID, jobs.StatusCompleted, jobs.StatusFailed)

	// The original must still own its artifact.
	after, err := manager.GetJob(original.ID)
	if err != nil {
		t.Fatalf("get job: %v", err)
	}
	if len(after.ArtifactIDs) != 1 {
		t.Errorf("the original now has %d artifacts, want 1: the retry stole them",
			len(after.ArtifactIDs))
	}
	if after.ArtifactIDs[0] != artifact.ID {
		t.Errorf("the original now points at %v, want %q", after.ArtifactIDs, artifact.ID)
	}

	// Deleting the retry must not delete the original's files.
	if err := service.DeleteJob(retry.ID); err != nil {
		t.Fatalf("delete retry: %v", err)
	}
	if _, err := manager.GetArtifact(artifact.ID); err != nil {
		t.Errorf("deleting the retry deleted the original's artifact: %v", err)
	}
}

// TestRetryJobReusesTheInputs verifies that a retry still re-uses the
// inputs of the job it repeats, which is the behaviour that was wanted
// in the first place.
func TestRetryJobReusesTheInputs(t *testing.T) {
	service, manager := newTestService(t, okRunner())

	// An input artifact that belongs to no job yet.
	input, err := manager.StoreArtifact("", jobs.ArtifactKindInput, []byte("input bytes"), "reference.png", nil)
	if err != nil {
		t.Fatalf("store artifact: %v", err)
	}

	original, err := service.SubmitJob(jobs.Request{
		Prompt:           "a red bicycle",
		InputArtifactIDs: []string{input.ID},
	})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	waitForStatus(t, manager, original.ID, jobs.StatusCompleted, jobs.StatusFailed)

	before, err := manager.GetJob(original.ID)
	if err != nil {
		t.Fatalf("get job: %v", err)
	}
	if len(before.InputArtifactIDs) != 1 {
		t.Fatalf("the original has %d inputs, want 1", len(before.InputArtifactIDs))
	}

	retry, err := service.RetryJob(original.ID)
	if err != nil {
		t.Fatalf("retry: %v", err)
	}
	waitForStatus(t, manager, retry.ID, jobs.StatusCompleted, jobs.StatusFailed)

	// The retry must have an input of its own.
	repeated, err := manager.GetJob(retry.ID)
	if err != nil {
		t.Fatalf("get job: %v", err)
	}
	if len(repeated.InputArtifactIDs) != 1 {
		t.Errorf("the retry has %d inputs, want 1", len(repeated.InputArtifactIDs))
	}

	// And the original must still have its own input.
	after, err := manager.GetJob(original.ID)
	if err != nil {
		t.Fatalf("get job: %v", err)
	}
	if len(after.InputArtifactIDs) != 1 {
		t.Errorf("the original now has %d inputs, want 1", len(after.InputArtifactIDs))
	}
}

// TestSubmitJobKeepsTheInputs verifies that a job owns the inputs it
// was given.
func TestSubmitJobKeepsTheInputs(t *testing.T) {
	service, manager := newTestService(t, okRunner())

	input, err := manager.StoreArtifact("", jobs.ArtifactKindInput, []byte("input bytes"), "reference.png", nil)
	if err != nil {
		t.Fatalf("store artifact: %v", err)
	}

	job, err := service.SubmitJob(jobs.Request{
		Prompt:           "a red bicycle",
		InputArtifactIDs: []string{input.ID},
	})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	waitForStatus(t, manager, job.ID, jobs.StatusCompleted, jobs.StatusFailed)

	stored, err := manager.GetJob(job.ID)
	if err != nil {
		t.Fatalf("get job: %v", err)
	}
	if len(stored.InputArtifactIDs) != 1 || stored.InputArtifactIDs[0] != input.ID {
		t.Errorf("inputs = %v, want [%q]", stored.InputArtifactIDs, input.ID)
	}
}

// TestSettingsNeverExposeTheSecret verifies that the API key is not
// returned to the frontend.
func TestSettingsNeverExposeTheSecret(t *testing.T) {
	service, _ := newTestService(t, okRunner())

	if err := service.SaveCredentials("sk-live-SECRET", "https://api.test"); err != nil {
		t.Fatalf("save credentials: %v", err)
	}

	settings, err := service.GetSettings()
	if err != nil {
		t.Fatalf("get settings: %v", err)
	}
	if !settings.HasAPIKey {
		t.Error("the settings do not report that a key is stored")
	}
	if settings.BaseURL != "https://api.test" {
		t.Errorf("base url = %q", settings.BaseURL)
	}
}
