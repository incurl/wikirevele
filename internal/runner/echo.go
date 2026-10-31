// Package runner holds the example job runner of the scaffold.
//
// It is the extension point of Wikirevele: replace it with a runner that
// talks to your own backend. The queue, the persistence, the
// cancellation and the events all work the same way, whatever the
// runner does.
//
// The echo runner returns the prompt it was given, so that the whole
// pipeline can be exercised without any external service.
package runner

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/incurl/wikirevele/internal/console"
	"github.com/incurl/wikirevele/internal/jobs"
	"github.com/incurl/wikirevele/internal/pocketbase"
)

// Echo is a runner that echoes the prompt back.
//
// It honours the context, so the cancellation path can be tested
// without any external service.
type Echo struct {
	// settings is the settings store. It may be nil.
	settings *pocketbase.SettingsStore
	// log records the runner traffic for the debug console. It may be
	// nil.
	log *console.Log
	// delay is how long the runner waits before answering. It makes
	// the running state visible in the UI.
	delay time.Duration
}

// NewEcho creates an echo runner.
func NewEcho(settings *pocketbase.SettingsStore, log *console.Log) *Echo {
	return &Echo{settings: settings, log: log, delay: 250 * time.Millisecond}
}

// Run implements jobs.Runner.
func (e *Echo) Run(ctx context.Context, job jobs.Job) (*jobs.Result, error) {
	if e.log != nil {
		e.log.Info("runner", fmt.Sprintf("the job %q was received", job.ID))
	}

	// The context is honoured, so a cancelled job stops promptly.
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(e.delay):
	}

	// The stored credentials are read here, never in the frontend. The
	// key itself is never logged, only its fingerprint.
	if e.settings != nil {
		if apiKey, found, err := e.settings.GetAPIKey(); err == nil && found {
			if e.log != nil {
				e.log.Info("runner", fmt.Sprintf(
					"using the credential %s", console.Fingerprint(apiKey)))
			}
		}
	}

	result := &jobs.Result{
		Payload: map[string]any{
			"echo":  job.Prompt,
			"title": strings.TrimSpace(job.Title),
		},
	}

	if e.log != nil {
		e.log.Info("runner", "the job was answered")
	}

	return result, nil
}
