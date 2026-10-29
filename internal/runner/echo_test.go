package runner

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/incurl/valkyrie/internal/jobs"
)

// TestEchoReturnsThePrompt verifies that the shipped runner echoes the
// prompt back, which is what makes the scaffold work out of the box.
func TestEchoReturnsThePrompt(t *testing.T) {
	runner := NewEcho(nil, nil)

	result, err := runner.Run(context.Background(), jobs.Job{
		Prompt: "a red bicycle",
		Title:  "the title",
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if result.Payload["echo"] != "a red bicycle" {
		t.Errorf("echo = %v, want the prompt", result.Payload["echo"])
	}
	if result.Payload["title"] != "the title" {
		t.Errorf("title = %v", result.Payload["title"])
	}
}

// TestEchoHonoursTheContext verifies that a cancelled job stops
// promptly.
//
// This is the contract every runner must keep: the context is the only
// way a cancelled job is interrupted.
func TestEchoHonoursTheContext(t *testing.T) {
	runner := NewEcho(nil, nil)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	start := time.Now()
	_, err := runner.Run(ctx, jobs.Job{Prompt: "x"})
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("a cancelled context should stop the runner")
	}
	// The runner waits 250ms before answering; a cancelled context must
	// return without waiting for it.
	if elapsed > 200*time.Millisecond {
		t.Errorf("the runner took %v to honour the cancellation", elapsed)
	}
}

// TestEchoTrimsTheTitle verifies that the title is trimmed.
func TestEchoTrimsTheTitle(t *testing.T) {
	runner := NewEcho(nil, nil)

	result, err := runner.Run(context.Background(), jobs.Job{Prompt: "x", Title: "  padded  "})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	title, _ := result.Payload["title"].(string)
	if strings.TrimSpace(title) != title {
		t.Errorf("title = %q, want it trimmed", title)
	}
}
