package jobs

import (
	"testing"

	"github.com/pocketbase/pocketbase/core"
)

// TestReconcileMarksInterruptedJobsAsFailed verifies that a job left
// behind by a crash is resolved instead of staying pending or running
// forever.
//
// This is the regression test for the finding of the 2026-10-28 audit:
// an interrupted job was never resumed and could not even be deleted.
func TestReconcileMarksInterruptedJobsAsFailed(t *testing.T) {
	m := newTestManager(t)

	// Two jobs that a crash left behind.
	interrupted := []string{}
	for _, status := range []string{StatusPending, StatusRunning} {
		record := newJobRecord(t, m, status)
		interrupted = append(interrupted, record)
	}

	// A job that finished normally, which must not be touched.
	finished := newJobRecord(t, m, StatusCompleted)

	reconciled, err := m.Reconcile()
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if reconciled != 2 {
		t.Errorf("reconciled %d jobs, want 2", reconciled)
	}

	for _, id := range interrupted {
		job, err := m.GetJob(id)
		if err != nil {
			t.Fatalf("get job: %v", err)
		}
		if job.Status != StatusFailed {
			t.Errorf("job %q is %q, want %q", id, job.Status, StatusFailed)
		}
		if job.ErrorMessage == "" {
			t.Errorf("job %q has no explanation", id)
		}
	}

	// The finished job is untouched.
	job, err := m.GetJob(finished)
	if err != nil {
		t.Fatalf("get job: %v", err)
	}
	if job.Status != StatusCompleted {
		t.Errorf("the finished job is %q, want %q", job.Status, StatusCompleted)
	}
}

// TestReconcileIsIdempotent verifies that reconciling twice is safe.
func TestReconcileIsIdempotent(t *testing.T) {
	m := newTestManager(t)
	newJobRecord(t, m, StatusRunning)

	first, err := m.Reconcile()
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	second, err := m.Reconcile()
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	if first != 1 {
		t.Errorf("the first reconcile touched %d jobs, want 1", first)
	}
	if second != 0 {
		t.Errorf("the second reconcile touched %d jobs, want 0", second)
	}
}

// TestReconciledJobCanBeDeleted verifies that the user can clear an
// interrupted job, which was impossible before: DeleteJob refuses to
// remove a running job.
func TestReconciledJobCanBeDeleted(t *testing.T) {
	m := newTestManager(t)
	id := newJobRecord(t, m, StatusRunning)

	if err := m.DeleteJob(id); err == nil {
		t.Fatal("a running job should not be deletable before the reconciliation")
	}

	if _, err := m.Reconcile(); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	if err := m.DeleteJob(id); err != nil {
		t.Errorf("the reconciled job is still not deletable: %v", err)
	}
}

// newJobRecord creates a job record in the given status and returns
// its id.
func newJobRecord(t *testing.T, m *Manager, status string) string {
	t.Helper()

	col, err := m.app.FindCollectionByNameOrId(CollectionJobs)
	if err != nil {
		t.Fatalf("find collection: %v", err)
	}

	record := core.NewRecord(col)
	record.Set("title", "an interrupted job")
	record.Set("prompt", "a prompt")
	record.Set("status", status)
	record.Set("params", map[string]any{})

	if err := m.app.Save(record); err != nil {
		t.Fatalf("save record: %v", err)
	}
	return record.Id
}
