package etlpipeline

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTheWorkerIsNeverATestBinary(t *testing.T) {
	// The path of the running executable is the test binary when the
	// tests run. The worker must never be it: a test binary run
	// with the etl subcommand would run the tests again, and each of
	// them would spawn another one.
	path := WorkerPath()

	if !isTestBinary(path) {
		t.Skipf("the executable %q is not a test binary", path)
	}

	if got := workerPath(); got != "" {
		t.Errorf("the worker path is %q, want it to be empty for a test binary", got)
	}
}

func TestIsTestBinary(t *testing.T) {
	cases := map[string]bool{
		"/tmp/etlpipeline.test":     true,
		"/tmp/etlpipeline.test.exe": true,
		"/usr/local/bin/wikirevele": false,
		"/usr/local/bin/valkyrie":   false,
		"/tmp/notatest":             false,
	}

	for path, want := range cases {
		if got := isTestBinary(path); got != want {
			t.Errorf("isTestBinary(%q) = %v, want %v", path, got, want)
		}
	}
}

func TestTheLauncherRefusesToSpawnATestBinary(t *testing.T) {
	// The real launcher must refuse, so that a test that forgets the
	// launcher seam fails loudly instead of forking without end.
	directory := t.TempDir()
	dump := filepath.Join(directory, "dump.json.bz2")
	if err := os.WriteFile(dump, []byte("x"), 0o600); err != nil {
		t.Fatalf("write the dump: %v", err)
	}

	launcher := systemLauncher{}

	_, _, _, err := launcher.Launch(
		context.Background(),
		"cat",
		dump,
		[]string{"etl"},
	)
	if err == nil {
		t.Fatal("the launcher spawned the test binary")
	}
	if !strings.Contains(err.Error(), "cannot be spawned") {
		t.Errorf("the error is %q, want it to say the worker cannot be spawned", err)
	}
}
