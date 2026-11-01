package bindings

import (
	"time"

	"github.com/incurl/wikirevele/internal/console"
	"github.com/incurl/wikirevele/internal/etlpipeline"
	"github.com/incurl/wikirevele/internal/jobs"
	"github.com/incurl/wikirevele/internal/sync"
	"github.com/incurl/wikirevele/internal/wikidata"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// EventJobUpdated is emitted whenever a job changes state. The payload
// is a jobs.Job.
const EventJobUpdated = "job:updated"

// EventConsoleEntry is emitted whenever a debug console entry is added.
// The payload is a console.Entry.
const EventConsoleEntry = "console:entry"

// EventWikidataImportProgress is emitted whenever the progress of a
// Wikidata import changes. The payload is a wikidata.ImportProgress.
//
// The import runs in the background, so this is the only way the
// panel learns that it is moving.
const EventWikidataImportProgress = "wikidata:import:progress"

// EventEtlProgress is emitted whenever the progress of an ETL run
// changes. The payload is an etlpipeline.Progress.
//
// The pipeline runs in the background, so this is the only way the
// panel learns that it is moving.
const EventEtlProgress = "etl:progress"

// EventSyncProgress is emitted whenever the progress of a sync
// changes. The payload is a sync.Progress.
//
// The sync runs in the background, so this is the only way the panel
// learns that it is moving.
const EventSyncProgress = "sync:progress"

// EventBridge pushes job updates to the frontend as Wails events.
//
// The frontend also polls via React Query, so a missed event only delays
// a refresh rather than losing an update.
type EventBridge struct {
	app *application.App
}

// NewEventBridge creates an event bridge.
func NewEventBridge(app *application.App) *EventBridge {
	return &EventBridge{app: app}
}

// EmitJobUpdated implements jobs.EventEmitter.
func (b *EventBridge) EmitJobUpdated(job jobs.Job) {
	if b == nil || b.app == nil {
		return
	}
	b.app.Event.Emit(EventJobUpdated, job)
}

// EmitConsoleEntry implements console.Emitter.
func (b *EventBridge) EmitConsoleEntry(entry console.Entry) {
	if b == nil || b.app == nil {
		return
	}
	b.app.Event.Emit(EventConsoleEntry, entry)
}

// EmitWikidataImportProgress pushes the progress of an import.
//
// It is the emitter the wikidata service is given, so that the panel
// sees the import as it goes.
func (b *EventBridge) EmitWikidataImportProgress(progress wikidata.ImportProgress) {
	if b == nil || b.app == nil {
		return
	}
	b.app.Event.Emit(EventWikidataImportProgress, progress)
}

// EmitEtlProgress pushes the progress of an ETL run.
//
// It implements etlpipeline.Emitter, so that the panel sees the run
// as it goes.
//
// The payload is the view of the bindings and not the type of the
// pipeline package: the generator would not know the other one.
func (b *EventBridge) EmitEtlProgress(progress etlpipeline.Progress) {
	if b == nil || b.app == nil {
		return
	}
	b.app.Event.Emit(EventEtlProgress, toEtlProgress(progress))
}

// LogEtlLine records one line of the output of the worker.
//
// The worker writes its warnings to the standard error, and they are
// shown in the debug console.
//
// It implements etlpipeline.LogSink.
func (b *EventBridge) LogEtlLine(runID, line string) {
	if b == nil || b.app == nil {
		return
	}
	b.app.Event.Emit(EventConsoleEntry, console.Entry{
		Time:    time.Now().Format("15:04:05.000"),
		Level:   console.LevelWarn,
		Source:  "etl",
		Message: line,
	})
}

// EmitSyncProgress pushes the progress of a sync.
//
// It implements sync.Emitter, so that the panel sees the run as it
// goes.
func (b *EventBridge) EmitSyncProgress(progress sync.Progress) {
	if b == nil || b.app == nil {
		return
	}
	b.app.Event.Emit(EventSyncProgress, progress)
}
