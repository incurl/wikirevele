package etlpipeline

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fakeLauncher starts the processes of a test.
//
// The real worker is the application itself, and it cannot be run from
// inside a test. The launcher is the seam: the test puts its own
// processes in place of the decompressor and the worker, and the rest
// of the supervisor is the real one.
type fakeLauncher struct {
	// decompressorCommand is the command of the fake decompressor.
	decompressorCommand string
	// decompressorArgs are the arguments of the fake decompressor.
	decompressorArgs []string
	// workerCommand is the command of the fake worker.
	workerCommand string
	// workerArgs are the arguments of the fake worker.
	workerArgs []string
	// events is what the worker writes on its standard output.
	events string
	// wait is how long the worker runs before it stops.
	wait time.Duration
	// started tells whether the processes were started.
	started bool
}

// Launch starts the fake processes.
//
// The standard output of the decompressor is empty, and the events are
// what the worker writes.
func (l *fakeLauncher) Launch(
	ctx context.Context,
	decompressor, dump string,
	worker []string,
) (io.Reader, *exec.Cmd, *exec.Cmd, error) {
	l.started = true
	l.workerArgs = worker

	decompressorProcess := exec.CommandContext(ctx, "sh", "-c", l.decompressorCommand)
	workerProcess := exec.CommandContext(ctx, "sh", "-c", l.workerCommand)

	if l.wait > 0 {
		decompressorProcess = exec.CommandContext(ctx, "sh", "-c",
			fmt.Sprintf("%s; sleep %f", l.decompressorCommand, l.wait.Seconds()))
	}

	// The output is taken before the process starts.
	output, err := workerProcess.StdoutPipe()
	if err != nil {
		return nil, nil, nil, err
	}

	if err := decompressorProcess.Start(); err != nil {
		return nil, nil, nil, err
	}
	if err := workerProcess.Start(); err != nil {
		_ = decompressorProcess.Process.Kill()
		return nil, nil, nil, err
	}

	return output, decompressorProcess, workerProcess, nil
}

func TestTheSupervisorRunsThePipelineAndReadsTheEvents(t *testing.T) {
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
		workerCommand: `echo '{"type":"progress","processedLines":100,"matchedItems":5,"collectedProperties":2,"speedMBs":1.5}'
echo '{"type":"done","itemFile":"items.ndjson","propFile":"props.ndjson","scanned":100,"items":5,"properties":2,"skipped":0,"malformed":0}'`,
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

	if progress.Status != StatusRunning {
		t.Errorf("the status is %q, want %q", progress.Status, StatusRunning)
	}
	if !strings.Contains(progress.Command, "etl") {
		t.Errorf("the command is %q, want it to name the worker", progress.Command)
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
	if final.MatchedItems != 5 {
		t.Errorf("the run counted %d items, want 5", final.MatchedItems)
	}
	if final.ProcessedLines != 100 {
		t.Errorf("the run counted %d lines, want 100", final.ProcessedLines)
	}
	if final.ItemFile != "items.ndjson" {
		t.Errorf("the items file is %q, want items.ndjson", final.ItemFile)
	}
}

func TestTheSupervisorGivesTheWorkerTheOptions(t *testing.T) {
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
		workerCommand:       "exit 0",
	}

	supervisor := NewSupervisor()
	supervisor.SetLauncher(launcher)

	progress, err := supervisor.Start(context.Background(), JobOptions{
		DumpPath:      dump,
		TargetQID:     "Q148",
		RecursiveP131: true,
		Languages:     []string{"en", "zh"},
		OutItemsPath:  filepath.Join(directory, "items.ndjson"),
		OutPropsPath:  filepath.Join(directory, "props.ndjson"),
		Overwrite:     true,
	})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	supervisor.Wait(progress.ID, 10*time.Second)

	arguments := strings.Join(launcher.workerArgs, " ")
	for _, want := range []string{
		"etl",
		"--target-qid Q148",
		"--recursive-p131",
		"--langs en,zh",
		"--events",
		"--overwrite",
	} {
		if !strings.Contains(arguments, want) {
			t.Errorf("the arguments are %q, want %q", arguments, want)
		}
	}
}

func TestTheSupervisorRecordsAFailedRun(t *testing.T) {
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
		workerCommand:       "exit 3",
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
	supervisor.Wait(progress.ID, 10*time.Second)

	final, _ := supervisor.Progress(progress.ID)
	if final.Status != StatusFailed {
		t.Errorf("the status is %q, want %q", final.Status, StatusFailed)
	}
	if final.Error == "" {
		t.Error("the failure was not recorded")
	}
}

func TestTheSupervisorCancelsARun(t *testing.T) {
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
		workerCommand:       "sleep 30",
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

	if !supervisor.Wait(progress.ID, 10*time.Second) {
		t.Fatal("the cancelled pipeline did not stop")
	}

	final, _ := supervisor.Progress(progress.ID)
	if final.Status != StatusCancelled {
		t.Errorf("the status is %q, want %q", final.Status, StatusCancelled)
	}
}

func TestTheSupervisorRefusesAMissingDecompressor(t *testing.T) {
	directory := t.TempDir()
	dump := filepath.Join(directory, "dump.json.bz2")
	if err := os.WriteFile(dump, []byte("x"), 0o600); err != nil {
		t.Fatalf("write the dump: %v", err)
	}

	// The path holds nothing but the temporary directory, so the
	// decompressor is not found.
	t.Setenv("PATH", directory)

	supervisor := NewSupervisor()
	// The launcher is a guard: the run must fail before any process
	// is started. The real worker is the test binary itself, and it
	// would run the test again.
	supervisor.SetLauncher(&fakeLauncher{
		decompressorCommand: "echo hello",
		workerCommand:       "exit 0",
	})

	_, err := supervisor.Start(context.Background(), JobOptions{
		DumpPath:     dump,
		TargetQID:    "Q90",
		OutItemsPath: filepath.Join(directory, "items.ndjson"),
		OutPropsPath: filepath.Join(directory, "props.ndjson"),
	})
	if err == nil {
		t.Fatal("the pipeline started without a decompressor")
	}
	if !strings.Contains(err.Error(), "not installed") {
		t.Errorf("the error is %q, want it to say the decompressor is missing", err)
	}
}

func TestTheSupervisorRefusesAMissingDump(t *testing.T) {
	supervisor := NewSupervisor()
	// The run must fail before any process is started.
	supervisor.SetLauncher(&fakeLauncher{
		decompressorCommand: "echo hello",
		workerCommand:       "exit 0",
	})

	_, err := supervisor.Start(context.Background(), JobOptions{
		DumpPath:     filepath.Join(t.TempDir(), "there-is-no-such-dump.json.bz2"),
		TargetQID:    "Q90",
		OutItemsPath: "items.ndjson",
		OutPropsPath: "props.ndjson",
	})
	if err == nil {
		t.Fatal("the pipeline started without a dump")
	}
}

// The bytes of a pipe are read to the end, so that a test does not
// leave a process waiting.
var _ = bytes.MinRead
