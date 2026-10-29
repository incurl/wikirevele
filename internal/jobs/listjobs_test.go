package jobs

import (
	"context"
	"testing"
)

// TestListJobsFetchesTheArtifactsInOneQuery verifies that the batched
// listing returns the same artifacts as the per-job one.
//
// This is the regression test for the finding of the 2026-10-28 audit:
// listing a page of N jobs used to cost N+1 queries, because every job
// collected its artifacts on its own. The frontend polls the listing
// every five seconds, so the cost grew with the history.
func TestListJobsFetchesTheArtifactsInOneQuery(t *testing.T) {
	m := newTestManager(t, WithRunner(RunnerFunc(
		func(ctx context.Context, job Job) (*Result, error) {
			return &Result{}, nil
		},
	)))

	// Three jobs, each with an output and an input.
	var jobIDs []string
	for i := 0; i < 3; i++ {
		job, err := m.Submit(Request{Prompt: "a job"})
		if err != nil {
			t.Fatalf("submit: %v", err)
		}
		waitForStatus(t, m, job.ID, StatusCompleted, StatusFailed)
		jobIDs = append(jobIDs, job.ID)

		if _, err := m.StoreArtifact(job.ID, ArtifactKindOutput, []byte("out"), "out.bin", nil); err != nil {
			t.Fatalf("store output: %v", err)
		}
		if _, err := m.StoreArtifact(job.ID, ArtifactKindInput, []byte("in"), "in.bin", nil); err != nil {
			t.Fatalf("store input: %v", err)
		}
	}

	listed, err := m.ListJobs(50)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(listed) != 3 {
		t.Fatalf("listed %d jobs, want 3", len(listed))
	}

	// Every job must carry both its output and its input.
	for _, job := range listed {
		if len(job.ArtifactIDs) != 1 {
			t.Errorf("job %q has %d outputs, want 1", job.ID, len(job.ArtifactIDs))
		}
		if len(job.InputArtifactIDs) != 1 {
			t.Errorf("job %q has %d inputs, want 1", job.ID, len(job.InputArtifactIDs))
		}
	}

	// The per-job listing must agree with the batched one.
	for _, id := range jobIDs {
		single, err := m.GetJob(id)
		if err != nil {
			t.Fatalf("get job: %v", err)
		}
		if len(single.ArtifactIDs) != 1 || len(single.InputArtifactIDs) != 1 {
			t.Errorf("job %q: outputs=%v inputs=%v", id, single.ArtifactIDs, single.InputArtifactIDs)
		}
	}
}

// TestListJobsWithNoArtifacts verifies that a page of jobs that have
// produced nothing is listed correctly.
func TestListJobsWithNoArtifacts(t *testing.T) {
	m := newTestManager(t, WithRunner(RunnerFunc(
		func(ctx context.Context, job Job) (*Result, error) {
			return &Result{}, nil
		},
	)))

	job, err := m.Submit(Request{Prompt: "a job"})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	waitForStatus(t, m, job.ID, StatusCompleted, StatusFailed)

	listed, err := m.ListJobs(50)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(listed) != 1 {
		t.Fatalf("listed %d jobs, want 1", len(listed))
	}
	if len(listed[0].ArtifactIDs) != 0 {
		t.Errorf("the job has %d artifacts, want 0", len(listed[0].ArtifactIDs))
	}
	// The slices must not be nil, or the frontend gets a null.
	if listed[0].ArtifactIDs == nil || listed[0].InputArtifactIDs == nil {
		t.Error("the artifact lists are nil, want empty slices")
	}
}

// TestListJobsOfAnEmptyTable verifies that listing nothing works.
func TestListJobsOfAnEmptyTable(t *testing.T) {
	m := newTestManager(t)

	listed, err := m.ListJobs(50)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(listed) != 0 {
		t.Errorf("listed %d jobs, want 0", len(listed))
	}
}
