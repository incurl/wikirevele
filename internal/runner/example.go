package runner

import (
	"context"
	"fmt"

	"github.com/incurl/valkyrie/internal/console"
	"github.com/incurl/valkyrie/internal/jobs"
	"github.com/incurl/valkyrie/internal/pocketbase"
)

// Example is a runner that shows the shape an application's runner
// takes, without doing any work.
//
// It is here as a reference, and it is compiled and tested, so it cannot
// drift away from the API the way a comment can. Copy it, rename it and
// replace the body.
//
// The three things it demonstrates are the three things every runner
// must do:
//
//   - read the credentials in Go, never in the frontend;
//   - honour the context, so that a cancelled job stops;
//   - return a *jobs.Error when it wants to attach a numeric code.
type Example struct {
	// settings is the settings store. It may be nil.
	settings *pocketbase.SettingsStore
	// log records the runner traffic for the debug console. It may be
	// nil.
	log *console.Log
}

// NewExample creates an example runner.
func NewExample(settings *pocketbase.SettingsStore, log *console.Log) *Example {
	return &Example{settings: settings, log: log}
}

// Run implements jobs.Runner.
func (e *Example) Run(ctx context.Context, job jobs.Job) (*jobs.Result, error) {
	// The context is honoured first, so that a cancelled job stops
	// before any work is started.
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// The credentials are read here, in Go. The frontend never sees
	// them; it only learns whether a key is configured.
	if e.settings != nil {
		if _, found, err := e.settings.GetAPIKey(); err != nil {
			return nil, err
		} else if !found {
			return nil, fmt.Errorf("no API key is configured, add one in Settings")
		}
	}

	if e.log != nil {
		e.log.Info("runner", fmt.Sprintf("the job %q was received", job.ID))
	}

	// The parameters are whatever the frontend sent. Validate them
	// here, in the runner, where the domain rules live.
	model, _ := job.Params["model"].(string)
	if model == "" {
		model = "default"
	}

	// TODO: call the real backend with the prompt and the parameters.

	return &jobs.Result{
		Payload: map[string]any{
			"echo":  job.Prompt,
			"model": model,
		},
	}, nil
}
