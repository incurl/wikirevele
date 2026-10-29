package runner

import (
	"context"
	"testing"

	"github.com/incurl/valkyrie/internal/jobs"
)

// TestExampleReturnsThePrompt verifies that the example runner echoes
// the prompt back, which is what makes the scaffold work out of the box.
func TestExampleReturnsThePrompt(t *testing.T) {
	runner := NewExample(nil, nil)

	result, err := runner.Run(context.Background(), jobs.Job{
		Prompt: "a red bicycle",
		Params: map[string]any{},
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if result.Payload["echo"] != "a red bicycle" {
		t.Errorf("payload = %v, want the prompt echoed back", result.Payload)
	}
}

// TestExampleDefaultsTheModel verifies that a missing parameter falls
// back to the default rather than to an empty string.
func TestExampleDefaultsTheModel(t *testing.T) {
	runner := NewExample(nil, nil)

	result, err := runner.Run(context.Background(), jobs.Job{Prompt: "x"})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if result.Payload["model"] != "default" {
		t.Errorf("model = %v, want %q", result.Payload["model"], "default")
	}
}

// TestExampleHonoursTheContext verifies that a cancelled job stops
// before any work is started.
//
// This is the behaviour every application runner must copy: the context
// is the only way a cancelled job can be interrupted.
func TestExampleHonoursTheContext(t *testing.T) {
	runner := NewExample(nil, nil)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := runner.Run(ctx, jobs.Job{Prompt: "x"}); err == nil {
		t.Fatal("a cancelled context should stop the runner")
	}
}
