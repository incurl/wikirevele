package etl

import (
	"encoding/json"
	"fmt"
	"io"
)

// The kinds of event the worker emits.
//
// The events are the machine readable channel between the worker and
// whoever drives it. The desktop application reads them to show the
// progress of a run; a terminal user ignores them.
const (
	// EventProgress reports that the run is moving.
	EventProgress = "progress"
	// EventDone reports that the run finished and what it produced.
	EventDone = "done"
	// EventError reports that the run failed.
	EventError = "error"
)

// ProgressEvent is the progress of a run.
//
// It is emitted periodically while the worker reads the dump, so that
// the driver can show a progress bar and a speed.
type ProgressEvent struct {
	// Type is always "progress".
	Type string `json:"type"`
	// ProcessedLines is the number of lines read from the input.
	ProcessedLines int64 `json:"processedLines"`
	// MatchedItems is the number of items that were kept.
	MatchedItems int64 `json:"matchedItems"`
	// CollectedProperties is the number of properties that were written.
	CollectedProperties int64 `json:"collectedProperties"`
	// SpeedMBs is the rate of the run, in megabytes per second.
	SpeedMBs float64 `json:"speedMBs"`
}

// DoneEvent reports that the run finished.
//
// It carries the paths of the files that were written, so that the
// driver knows what to import without having to guess.
type DoneEvent struct {
	// Type is always "done".
	Type string `json:"type"`
	// ItemFiles are the files the items were written to, one per
	// target.
	//
	// It is empty when the run wrote the property dictionary only.
	ItemFiles []string `json:"itemFiles"`
	// PropFile is the file the properties were written to.
	//
	// It is empty when the run wrote the items only.
	PropFile string `json:"propFile"`
	// Scanned is the number of lines read from the input.
	Scanned int64 `json:"scanned"`
	// Items is the number of items that were kept.
	Items int64 `json:"items"`
	// Properties is the number of properties that were written.
	Properties int64 `json:"properties"`
	// Skipped is the number of lines that were not kept.
	Skipped int64 `json:"skipped"`
	// Malformed is the number of lines that could not be parsed.
	Malformed int64 `json:"malformed"`
}

// ErrorEvent reports that the run failed.
//
// The message is the human readable reason, and it is also what the
// history of the job records.
type ErrorEvent struct {
	// Type is always "error".
	Type string `json:"type"`
	// Message is what went wrong.
	Message string `json:"message"`
}

// Event is any of the events the worker writes.
//
// The worker writes one JSON object per line, and the kind of event is
// told by the type field. A reader decodes the object into this
// structure and looks at the type: the fields that belong to another
// kind of event are then empty.
//
// The fields are in one structure so that a reader needs one type and
// not three.
type Event struct {
	// Type is the kind of event: "progress", "done" or "error".
	Type string `json:"type"`

	// ProcessedLines is the number of lines read from the input.
	// It belongs to a progress event.
	ProcessedLines int64 `json:"processedLines,omitempty"`
	// MatchedItems is the number of items that were kept.
	// It belongs to a progress event.
	MatchedItems int64 `json:"matchedItems,omitempty"`
	// CollectedProperties is the number of properties that were
	// written. It belongs to a progress event.
	CollectedProperties int64 `json:"collectedProperties,omitempty"`
	// SpeedMBs is the rate of the run, in megabytes per second.
	// It belongs to a progress event.
	SpeedMBs float64 `json:"speedMBs,omitempty"`

	// ItemFile is the file the items were written to.
	// It belongs to a done event.
	ItemFile string `json:"itemFile,omitempty"`
	// ItemFiles are the files the items were written to, one per
	// target. It belongs to a done event.
	ItemFiles []string `json:"itemFiles,omitempty"`
	// PropFile is the file the properties were written to.
	// It belongs to a done event.
	PropFile string `json:"propFile,omitempty"`
	// Scanned is the number of lines read from the input.
	// It belongs to a done event.
	Scanned int64 `json:"scanned,omitempty"`
	// Items is the number of items that were kept.
	// It belongs to a done event.
	Items int64 `json:"items,omitempty"`
	// Properties is the number of properties that were written.
	// It belongs to a done event.
	Properties int64 `json:"properties,omitempty"`
	// Skipped is the number of lines that were not kept.
	// It belongs to a done event.
	Skipped int64 `json:"skipped,omitempty"`
	// Malformed is the number of lines that could not be parsed.
	// It belongs to a done event.
	Malformed int64 `json:"malformed,omitempty"`

	// Message is what went wrong. It belongs to an error event.
	Message string `json:"message,omitempty"`
}

// EventWriter writes the events of a run.
//
// The events go to the standard output, and the human readable logs go
// to the standard error: the two channels never mix, so a driver can
// read the events without having to filter the logs out.
type EventWriter struct {
	// out is where the events go.
	out io.Writer
	// enabled tells whether the events are written at all.
	//
	// A terminal run does not want them: they would fill the
	// terminal with JSON. The desktop application turns them on.
	enabled bool
}

// NewEventWriter creates an event writer.
//
// The events are only written when enabled is true. A writer that is
// disabled writes nothing, and the run behaves as if the channel did
// not exist.
func NewEventWriter(out io.Writer, enabled bool) *EventWriter {
	return &EventWriter{out: out, enabled: enabled}
}

// Enabled tells whether the events are written.
func (w *EventWriter) Enabled() bool {
	return w != nil && w.enabled
}

// Progress writes a progress event.
func (w *EventWriter) Progress(event ProgressEvent) error {
	if !w.Enabled() {
		return nil
	}
	return w.write(Event{
		Type:                EventProgress,
		ProcessedLines:      event.ProcessedLines,
		MatchedItems:        event.MatchedItems,
		CollectedProperties: event.CollectedProperties,
		SpeedMBs:            event.SpeedMBs,
	})
}

// Done writes the event of a finished run.
func (w *EventWriter) Done(event DoneEvent) error {
	if !w.Enabled() {
		return nil
	}

	// The first file is also carried on its own, so that a reader that
	// knows about one target does not have to know about the list.
	first := ""
	if len(event.ItemFiles) > 0 {
		first = event.ItemFiles[0]
	}

	return w.write(Event{
		Type:       EventDone,
		ItemFile:   first,
		ItemFiles:  event.ItemFiles,
		PropFile:   event.PropFile,
		Scanned:    event.Scanned,
		Items:      event.Items,
		Properties: event.Properties,
		Skipped:    event.Skipped,
		Malformed:  event.Malformed,
	})
}

// Error writes the event of a failed run.
func (w *EventWriter) Error(message string) error {
	if !w.Enabled() {
		return nil
	}
	return w.write(Event{Type: EventError, Message: message})
}

// write writes one event as a line of JSON.
//
// The encoder adds the newline, so that the events can be read one at
// a time by a driver that reads the stream line by line.
func (w *EventWriter) write(event any) error {
	encoder := json.NewEncoder(w.out)
	if err := encoder.Encode(event); err != nil {
		return fmt.Errorf("etl: write the event: %w", err)
	}
	return nil
}
