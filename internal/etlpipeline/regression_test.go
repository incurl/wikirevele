package etlpipeline

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// These are the regressions of the pipe handling of the supervisor.
//
// The supervisor reads the events of the worker on a pipe. Two things
// can go wrong with that, and both of them were bugs:
//
//  1. The pipe of os/exec is closed by the wait of the process, so a
//     reader that starts after the wait loses the last events of the
//     run. The counters of a finished run were then zero.
//
//  2. A child of the worker can hold the pipe open after the worker is
//     gone. A supervisor that waits for the end of the pipe before it
//     waits for the worker never ends the run, and a cancelled run
//     never stops.
//
// The tests below pin the two behaviours.

// TestTheSupervisorReadsTheEventsOfAFinishedRun proves that the last
// events of a run are counted.
//
// The worker writes its counters and stops at once. The supervisor has
// to read them: the pipe of the worker is closed by the wait of the
// worker, and a reader that starts after it reads nothing.
func TestTheSupervisorReadsTheEventsOfAFinishedRun(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("no shell to run the test")
	}

	directory := t.TempDir()
	dump := filepath.Join(directory, "dump.json.bz2")
	if err := os.WriteFile(dump, []byte("x"), 0o600); err != nil {
		t.Fatalf("write the dump: %v", err)
	}

	launcher := &fakeLauncher{
		decompressorCommand: "echo hello",
		workerCommand: `echo '{"type":"progress","processedLines":42,"matchedItems":7,"collectedProperties":3,"speedMBs":2.5}'
echo '{"type":"done","itemFile":"items.ndjson","propFile":"props.ndjson","scanned":42,"items":7,"properties":3,"skipped":0,"malformed":0}'`,
	}

	supervisor := NewSupervisor()
	supervisor.SetLauncher(launcher)

	progress, err := supervisor.Start(context.Background(), JobOptions{
		DumpPath:     dump,
		TargetQID:    "Q90",
		OutItemsPath: filepath.Join(directory, "items.ndjson"),
		OutPropsPath: filepath.Join(directory, "props.ndjson"),
	})
	if err != nil {
		t.Fatalf("start: %v", err)
	}

	if !supervisor.Wait(progress.ID, 10*time.Second) {
		t.Fatal("the pipeline did not finish")
	}

	final, ok := supervisor.Progress(progress.ID)
	if !ok {
		t.Fatal("the run is not there any more")
	}

	if final.Status != StatusSucceeded {
		t.Errorf("the status is %q, want %q (%s)", final.Status, StatusSucceeded, final.Error)
	}
	if final.MatchedItems != 7 {
		t.Errorf("the run counted %d items, want 7", final.MatchedItems)
	}
	if final.ProcessedLines != 42 {
		t.Errorf("the run counted %d lines, want 42", final.ProcessedLines)
	}
	if final.CollectedProperties != 3 {
		t.Errorf("the run counted %d properties, want 3", final.CollectedProperties)
	}
	if final.ItemFile != "items.ndjson" {
		t.Errorf("the items file is %q, want items.ndjson", final.ItemFile)
	}
}

// TestTheSupervisorDoesNotWaitForAGrandchild proves that a run ends
// even when a child of the worker holds the pipe of the events open.
//
// The worker starts a child that outlives it. The pipe stays open, so
// a supervisor that waits for the end of the pipe before it waits for
// the worker waits for the child, and the run never ends.
func TestTheSupervisorDoesNotWaitForAGrandchild(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("no shell to run the test")
	}

	directory := t.TempDir()
	dump := filepath.Join(directory, "dump.json.bz2")
	if err := os.WriteFile(dump, []byte("x"), 0o600); err != nil {
		t.Fatalf("write the dump: %v", err)
	}

	// The child holds the standard output of the worker and sleeps for
	// a long time. The worker itself stops at once.
	launcher := &fakeLauncher{
		decompressorCommand: "echo hello",
		workerCommand:       "(sleep 120 &) ; echo done",
	}

	supervisor := NewSupervisor()
	supervisor.SetLauncher(launcher)

	progress, err := supervisor.Start(context.Background(), JobOptions{
		DumpPath:     dump,
		TargetQID:    "Q90",
		OutItemsPath: filepath.Join(directory, "items.ndjson"),
		OutPropsPath: filepath.Join(directory, "props.ndjson"),
	})
	if err != nil {
		t.Fatalf("start: %v", err)
	}

	if !supervisor.Wait(progress.ID, 15*time.Second) {
		t.Fatal("the run waited for the child of the worker")
	}

	final, _ := supervisor.Progress(progress.ID)
	if final.Status != StatusSucceeded {
		t.Errorf("the status is %q, want %q (%s)", final.Status, StatusSucceeded, final.Error)
	}
}

// TestTheSupervisorCancelsARunWithAChild proves that a cancelled run
// stops even when a child of the worker holds the pipe open.
//
// The child is the reason the run of the test above could not end. A
// cancelled run has to end the same way: the user asked for it to stop.
func TestTheSupervisorCancelsARunWithAChild(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("no shell to run the test")
	}

	directory := t.TempDir()
	dump := filepath.Join(directory, "dump.json.bz2")
	if err := os.WriteFile(dump, []byte("x"), 0o600); err != nil {
		t.Fatalf("write the dump: %v", err)
	}

	launcher := &fakeLauncher{
		decompressorCommand: "echo hello",
		workerCommand:       "(sleep 120 &) ; sleep 120",
	}

	supervisor := NewSupervisor()
	supervisor.SetLauncher(launcher)

	progress, err := supervisor.Start(context.Background(), JobOptions{
		DumpPath:     dump,
		TargetQID:    "Q90",
		OutItemsPath: filepath.Join(directory, "items.ndjson"),
		OutPropsPath: filepath.Join(directory, "props.ndjson"),
	})
	if err != nil {
		t.Fatalf("start: %v", err)
	}

	if err := supervisor.Cancel(progress.ID); err != nil {
		t.Fatalf("cancel: %v", err)
	}

	if !supervisor.Wait(progress.ID, 15*time.Second) {
		t.Fatal("the cancelled pipeline did not stop")
	}

	final, _ := supervisor.Progress(progress.ID)
	if final.Status != StatusCancelled {
		t.Errorf("the status is %q, want %q", final.Status, StatusCancelled)
	}
}
