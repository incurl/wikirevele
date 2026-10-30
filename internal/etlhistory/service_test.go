package etlhistory

import (
	"testing"

	wikirevelepb "github.com/incurl/wikirevele/internal/pocketbase"
)

// newTestService bootstraps a PocketBase and a history service.
func newTestService(t *testing.T) *Service {
	t.Helper()

	service := wikirevelepb.NewService(t.TempDir())
	if err := service.Bootstrap(); err != nil {
		t.Fatalf("bootstrap pocketbase: %v", err)
	}
	t.Cleanup(func() {
		_ = service.Close()
	})

	return NewService(service.App())
}

func TestTheRunIsRecorded(t *testing.T) {
	service := newTestService(t)

	command := "bzcat dump.json.bz2 | wikirevele etl --target-qid Q90"

	id, err := service.Start(command, "Q90", true, "en,fr", "dump.json.bz2", "items.ndjson", "props.ndjson")
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	if id == "" {
		t.Fatal("the run has no identifier")
	}

	entries, err := service.List(10)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("the history holds %d runs, want 1", len(entries))
	}

	entry := entries[0]
	if entry.FullCommand != command {
		t.Errorf("the command is %q, want %q", entry.FullCommand, command)
	}
	if entry.TargetQID != "Q90" {
		t.Errorf("the target is %q, want Q90", entry.TargetQID)
	}
	if !entry.RecursiveP131 {
		t.Error("the recursive flag was not recorded")
	}
	if entry.Langs != "en,fr" {
		t.Errorf("the languages are %q, want en,fr", entry.Langs)
	}
	if entry.Status != StatusRunning {
		t.Errorf("the status is %q, want %q", entry.Status, StatusRunning)
	}
	if entry.FinishedAt != "" {
		t.Errorf("the run finished at %q, want nothing while it is going", entry.FinishedAt)
	}
}

func TestTheEndOfTheRunIsRecorded(t *testing.T) {
	service := newTestService(t)

	id, err := service.Start("bzcat dump | wikirevele etl", "Q90", false, "en", "dump", "items", "props")
	if err != nil {
		t.Fatalf("start: %v", err)
	}

	if err := service.Finish(id, StatusSucceeded, ""); err != nil {
		t.Fatalf("finish: %v", err)
	}

	entry, err := service.Get(id)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if entry.Status != StatusSucceeded {
		t.Errorf("the status is %q, want %q", entry.Status, StatusSucceeded)
	}
	if entry.FinishedAt == "" {
		t.Error("the end of the run was not recorded")
	}
	if entry.ErrorMessage != "" {
		t.Errorf("the error is %q, want nothing", entry.ErrorMessage)
	}
}

func TestTheFailureIsRecorded(t *testing.T) {
	service := newTestService(t)

	id, err := service.Start("bzcat dump | wikirevele etl", "Q90", false, "en", "dump", "items", "props")
	if err != nil {
		t.Fatalf("start: %v", err)
	}

	if err := service.Finish(id, StatusFailed, "the decompressor is missing"); err != nil {
		t.Fatalf("finish: %v", err)
	}

	entry, err := service.Get(id)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if entry.Status != StatusFailed {
		t.Errorf("the status is %q, want %q", entry.Status, StatusFailed)
	}
	if entry.ErrorMessage != "the decompressor is missing" {
		t.Errorf("the error is %q, want the reason", entry.ErrorMessage)
	}
}

func TestTheInterruptedRunsAreMarkedAsFailed(t *testing.T) {
	service := newTestService(t)

	// A run that was going when the application closed.
	if _, err := service.Start("bzcat dump | wikirevele etl", "Q90", false, "en", "dump", "items", "props"); err != nil {
		t.Fatalf("start: %v", err)
	}

	count, err := service.Reconcile()
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if count != 1 {
		t.Errorf("%d runs were marked, want 1", count)
	}

	entries, err := service.List(10)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if entries[0].Status != StatusFailed {
		t.Errorf("the status is %q, want %q", entries[0].Status, StatusFailed)
	}

	// The reconciliation is not done twice: a run that is already
	// marked is left alone.
	count, err = service.Reconcile()
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if count != 0 {
		t.Errorf("%d runs were marked, want 0", count)
	}
}

func TestTheRunsAreListedTheMostRecentFirst(t *testing.T) {
	service := newTestService(t)

	for _, target := range []string{"Q90", "Q148", "Q662548"} {
		if _, err := service.Start("command "+target, target, false, "en", "dump", "items", "props"); err != nil {
			t.Fatalf("start: %v", err)
		}
	}

	entries, err := service.List(10)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("the history holds %d runs, want 3", len(entries))
	}

	if entries[0].TargetQID != "Q662548" {
		t.Errorf("the first run is %q, want the most recent one", entries[0].TargetQID)
	}
}

func TestARunIsRemoved(t *testing.T) {
	service := newTestService(t)

	id, err := service.Start("command", "Q90", false, "en", "dump", "items", "props")
	if err != nil {
		t.Fatalf("start: %v", err)
	}

	if err := service.Delete(id); err != nil {
		t.Fatalf("delete: %v", err)
	}

	entries, err := service.List(10)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("the history holds %d runs, want 0", len(entries))
	}
}
