package sync

import (
	"os"
	"strings"
	"testing"
)

// TestFileLoggerWritesALine verifies that the log of the sync is
// written to the file.
//
// The log is the trace of a run, and a run that says nothing is a run
// the reader cannot learn from.
func TestFileLoggerWritesALine(t *testing.T) {
	dir := t.TempDir()
	logger := NewFileLogger(dir)

	logger.LogSync("the first line")
	logger.LogSync("the second line")

	content, err := os.ReadFile(logger.Path())
	if err != nil {
		t.Fatalf("read the log: %v", err)
	}

	text := string(content)
	if !strings.Contains(text, "the first line") {
		t.Errorf("the log does not hold the first line: %q", text)
	}
	if !strings.Contains(text, "the second line") {
		t.Errorf("the log does not hold the second line: %q", text)
	}
}

// TestFileLoggerAppends verifies that the log of a second run is
// written after the first one.
//
// The log is the history of the syncs, and a run that erased the
// previous one would lose it.
func TestFileLoggerAppends(t *testing.T) {
	dir := t.TempDir()
	logger := NewFileLogger(dir)

	logger.LogSync("the first run")
	logger.LogSync("the second run")

	content, err := os.ReadFile(logger.Path())
	if err != nil {
		t.Fatalf("read the log: %v", err)
	}

	text := string(content)
	first := strings.Index(text, "the first run")
	second := strings.Index(text, "the second run")
	if first < 0 || second < 0 {
		t.Fatalf("the log holds %q", text)
	}
	if first > second {
		t.Error("the second run was written before the first one")
	}
}
