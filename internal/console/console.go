// Package console keeps the application debug log.
//
// The log records what the application and the remote APIs did: the
// requests that were sent, the responses that came back and the errors
// that were raised. The frontend shows it in a panel at the bottom of
// the window.
//
// The log is held in memory only. It is a debugging aid, not an audit
// trail, so it is not persisted and it is capped to keep the memory
// usage bounded.
package console

import (
	"fmt"
	"sync"
	"time"
)

// Level classifies a log entry.
type Level string

const (
	// LevelInfo is a normal event.
	LevelInfo Level = "info"
	// LevelWarn is a recoverable problem.
	LevelWarn Level = "warn"
	// LevelError is a failure.
	LevelError Level = "error"
	// LevelRaw is a verbatim API response or request body.
	LevelRaw Level = "raw"
)

// Entry is a single log record.
type Entry struct {
	// ID is a monotonically increasing identifier, used as the React key
	// and to order the entries.
	ID int64 `json:"id"`
	// Time is when the entry was recorded.
	Time string `json:"time"`
	// Level is the severity of the entry.
	Level Level `json:"level"`
	// Source names the component that produced the entry, for example
	// "api" or "job".
	Source string `json:"source"`
	// Message is the human readable text.
	Message string `json:"message"`
	// Detail holds the verbatim payload, such as the raw API response
	// body. It may be empty.
	Detail string `json:"detail"`
}

// Emitter is notified whenever an entry is added.
//
// The Wails layer implements it to push entries to the frontend.
type Emitter interface {
	// EmitConsoleEntry publishes a log entry.
	EmitConsoleEntry(entry Entry)
}

// maxEntries bounds how many entries are kept.
const maxEntries = 500

// Log collects entries in memory.
type Log struct {
	mu      sync.Mutex
	entries []Entry
	nextID  int64

	emitter Emitter
}

// New creates an empty log.
func New() *Log {
	return &Log{nextID: 1}
}

// SetEmitter attaches the emitter once it becomes available.
func (l *Log) SetEmitter(e Emitter) {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.emitter = e
}

// Append records an entry and publishes it.
//
// The level and the source are given explicitly; the timestamp is added
// here.
func (l *Log) Append(level Level, source, message, detail string) Entry {
	if l == nil {
		return Entry{}
	}

	l.mu.Lock()
	entry := Entry{
		ID:      l.nextID,
		Time:    time.Now().Format("15:04:05.000"),
		Level:   level,
		Source:  source,
		Message: message,
		Detail:  detail,
	}
	l.nextID++
	l.entries = append(l.entries, entry)
	if len(l.entries) > maxEntries {
		l.entries = l.entries[len(l.entries)-maxEntries:]
	}
	emitter := l.emitter
	l.mu.Unlock()

	if emitter != nil {
		emitter.EmitConsoleEntry(entry)
	}
	return entry
}

// Info records a normal event.
func (l *Log) Info(source, message string) Entry {
	return l.Append(LevelInfo, source, message, "")
}

// Warn records a recoverable problem.
func (l *Log) Warn(source, message string) Entry {
	return l.Append(LevelWarn, source, message, "")
}

// Error records a failure.
func (l *Log) Error(source, message string) Entry {
	return l.Append(LevelError, source, message, "")
}

// Raw records a verbatim payload, such as an API response body.
//
// It is the only level that carries a detail, and the only one the
// frontend shows in its detail view: a runner that wants the reader to
// see the exact bytes the API returned logs them here.
func (l *Log) Raw(source, message, detail string) Entry {
	return l.Append(LevelRaw, source, message, detail)
}

// Entries returns the recorded entries, oldest first.
func (l *Log) Entries() []Entry {
	if l == nil {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	out := make([]Entry, len(l.entries))
	copy(out, l.entries)
	return out
}

// Clear discards the recorded entries.
func (l *Log) Clear() {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries = nil
}

// Fingerprint returns a short, non-reversible fingerprint of a secret.
//
// It lets the debug console say which key was used without ever
// revealing the key itself.
func Fingerprint(secret string) string {
	if secret == "" {
		return "none"
	}
	// A simple, stable checksum is enough: the value only has to tell
	// two keys apart in a log line.
	var sum uint32 = 2166136261
	for i := 0; i < len(secret); i++ {
		sum ^= uint32(secret[i])
		sum *= 16777619
	}
	return fmt.Sprintf("…%04x", sum&0xFFFF)
}
