package bindings

import (
	"github.com/incurl/valkyrie/internal/console"
	"github.com/incurl/valkyrie/internal/jobs"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// EventJobUpdated is emitted whenever a job changes state. The payload
// is a jobs.Job.
const EventJobUpdated = "job:updated"

// EventConsoleEntry is emitted whenever a debug console entry is added.
// The payload is a console.Entry.
const EventConsoleEntry = "console:entry"

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
