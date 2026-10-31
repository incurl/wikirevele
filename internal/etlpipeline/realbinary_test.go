package etlpipeline

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// binaryLauncher starts the real application binary as the worker.
//
// It is the launcher seam pointed at the real thing: the supervisor
// is the real one, the decompressor is the real one, and the worker is
// the real binary. Only the path of the worker is given, because the
// path of the running executable is the test binary.
type binaryLauncher struct {
	// path is the binary to run as the worker.
	path string
}

// Launch starts the real decompressor and the real worker.
func (l binaryLauncher) Launch(
	ctx context.Context,
	decompressor, dump string,
	worker []string,
) (io.ReadCloser, *exec.Cmd, *exec.Cmd, error) {
	decompressorName, decompressorArgs := splitDecompressor(decompressor, dump)
	decompressorProcess := exec.CommandContext(ctx, decompressorName, decompressorArgs...)
	decompressorProcess.Stderr = os.Stderr

	pipe, err := decompressorProcess.StdoutPipe()
	if err != nil {
		return nil, nil, nil, err
	}

	workerProcess := exec.CommandContext(ctx, l.path, worker...)
	workerProcess.Stdin = pipe
	workerProcess.Stderr = os.Stderr

	// The events are read from a pipe the test owns, so that the
	// reader is not stopped by the wait of the worker.
	eventsRead, eventsWrite, err := os.Pipe()
	if err != nil {
		return nil, nil, nil, err
	}
	workerProcess.Stdout = eventsWrite

	setProcessGroup(decompressorProcess)
	setProcessGroup(workerProcess)

	if err := decompressorProcess.Start(); err != nil {
		_ = eventsRead.Close()
		_ = eventsWrite.Close()
		return nil, nil, nil, err
	}
	if err := workerProcess.Start(); err != nil {
		_ = decompressorProcess.Process.Kill()
		_ = eventsRead.Close()
		_ = eventsWrite.Close()
		return nil, nil, nil, err
	}

	_ = eventsWrite.Close()

	return eventsRead, decompressorProcess, workerProcess, nil
}

// TestTheRealBinaryRunsAsTheWorker runs the whole path the desktop
// application takes: the real decompressor, the real binary, the real
// events, and the real cancellation.
//
// It is the one test that proves the GUI can drive the worker.
func TestTheRealBinaryRunsAsTheWorker(t *testing.T) {
	if _, err := exec.LookPath("bzcat"); err != nil {
		t.Skip("no bzcat on this host")
	}
	if _, err := exec.LookPath("bzip2"); err != nil {
		t.Skip("no bzip2 on this host")
	}

	binary := filepath.Join("..", "..", "bin", "wikirevele")
	if _, err := os.Stat(binary); err != nil {
		t.Skip("the binary is not built")
	}

	directory := t.TempDir()

	// A dump with three items that point at Paris.
	source := filepath.Join(directory, "dump.json")
	content := ""
	for _, id := range []string{"Q1", "Q2", "Q3"} {
		content += `{"type":"item","id":"` + id +
			`","labels":{"en":{"language":"en","value":"` + id +
			`"}},"claims":{"P131":[{"mainsnak":{"snaktype":"value","property":"P131",` +
			`"datatype":"wikibase-item","datavalue":{"value":{"entity-type":"item",` +
			`"id":"Q90"},"type":"wikibase-entityid"}}}]}}` + "\n"
	}
	if err := os.WriteFile(source, []byte(content), 0o600); err != nil {
		t.Fatalf("write the dump: %v", err)
	}
	if out, err := exec.Command("bzip2", "-k", source).CombinedOutput(); err != nil {
		t.Fatalf("compress: %v (%s)", err, out)
	}

	dump := source + ".bz2"

	supervisor := NewSupervisor()
	supervisor.SetLauncher(binaryLauncher{path: binary})

	progress, err := supervisor.Start(context.Background(), JobOptions{
		DumpPath:     dump,
		TargetQIDs:   []string{"Q90"},
		Languages:    []string{"en"},
		OutItemsPath: filepath.Join(directory, "items.ndjson"),
		OutPropsPath: filepath.Join(directory, "props.ndjson"),
	})
	if err != nil {
		t.Fatalf("start: %v", err)
	}

	if !supervisor.Wait(progress.ID, 60*time.Second) {
		t.Fatal("the pipeline did not finish")
	}

	final, _ := supervisor.Progress(progress.ID)
	if final.Status != StatusSucceeded {
		t.Fatalf("the status is %q (%s)", final.Status, final.Error)
	}

	// The worker wrote the three items.
	raw, err := os.ReadFile(filepath.Join(directory, "items.ndjson"))
	if err != nil {
		t.Fatalf("read the items: %v", err)
	}
	lines := 0
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.TrimSpace(line) != "" {
			lines++
		}
	}
	if lines != 3 {
		t.Errorf("the items file holds %d items, want 3", lines)
	}
}
