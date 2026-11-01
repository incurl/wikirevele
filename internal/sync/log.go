package sync

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// The name of the file the log of the sync is written to.
//
// The file lives next to the database of the application, and it is
// appended to: a run is a part of a history, and the history of the
// syncs is worth keeping.
const LogFileName = "sync.log"

// FileLogger writes the log of the sync to a file.
//
// The file is opened for every line and closed again. A sync of a
// million records writes a few thousand lines, and the cost of
// opening a file is nothing next to the cost of a request. The
// alternative is a file handle that is held open, and a handle that
// is held open is a handle that is lost when the application is
// killed.
type FileLogger struct {
	// path is the file the lines are written to.
	path string
	// mu guards the writes, so that two lines never mix.
	mu sync.Mutex
}

// NewFileLogger creates a logger that writes to a file in a
// directory.
func NewFileLogger(dataDir string) *FileLogger {
	return &FileLogger{path: filepath.Join(dataDir, LogFileName)}
}

// Path returns the file the log is written to.
func (l *FileLogger) Path() string { return l.path }

// LogSync records one line of the log of a sync.
//
// A line that cannot be written is not worth stopping a run for: the
// run is the work, and the log is a trace of it.
func (l *FileLogger) LogSync(message string) {
	if l == nil {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	file, err := os.OpenFile(l.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	defer file.Close()

	stamp := time.Now().Format("2006-01-02 15:04:05")
	fmt.Fprintf(file, "%s %s\n", stamp, message)
}
