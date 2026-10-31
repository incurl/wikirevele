package wikidata

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/pocketbase/pocketbase/core"
)

// The kinds of file the importer reads.
//
// The kind decides which collection the records are written to, and
// which key they are upserted by.
const (
	// KindItems is the file of the items linked to the target.
	KindItems = "items"
	// KindProperties is the file of the property dictionary.
	KindProperties = "properties"
)

// The names of the files the ETL worker writes.
//
// The importer looks for these names when it is given a directory, so
// that the panel and the command agree on where the datasets are.
const (
	// ItemsFileName is the file of the items.
	ItemsFileName = "paris-wikidata-subset.ndjson"
	// PropertiesFileName is the file of the property dictionary.
	PropertiesFileName = "wikidata-properties.ndjson"
)

// Service imports the datasets and reads them back.
//
// The service owns the state of the last import, so that the panel can
// show the progress of an import that is running, and the outcome of
// the one that ran before.
type Service struct {
	// app is the PocketBase application the records live in.
	app core.App
	// mu guards the progress.
	mu sync.RWMutex
	// progress is the state of the last import.
	progress ImportProgress
	// cancel stops the import that is running.
	cancel context.CancelFunc
	// running is the number of imports that are running.
	running int
	// emitter is called with the progress of the import. It may be
	// nil, in which case the progress is only remembered.
	emitter func(ImportProgress)
	// dataDir is the directory the datasets are read from.
	dataDir string
}

// NewService creates a service over the PocketBase application.
//
// The progress of the service is finished: no import has run yet, so
// there is nothing to wait for.
func NewService(app core.App) *Service {
	return &Service{
		app:      app,
		progress: ImportProgress{Done: true},
	}
}

// SetDataDir sets the directory the datasets are read from.
//
// It is the data directory of the application: the ETL worker leaves
// its output there, and the panel imports it from there.
func (s *Service) SetDataDir(dir string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dataDir = dir
}

// DataDir returns the directory the datasets are read from.
//
// It is the data directory of the application: the ETL worker is
// expected to leave its output there, and the panel imports it from
// there.
func (s *Service) DataDir() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.dataDir
}

// App returns the PocketBase application the records live in.
//
// It is the accessor the binding uses to read the records back.
func (s *Service) App() core.App {
	return s.app
}

// Progress returns the state of the last import.
func (s *Service) Progress() ImportProgress {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.progress
}

// SetProgressEmitter sets the function that is called with the
// progress of an import.
//
// The emitter is called from the goroutine of the import, so it has
// to be quick: it pushes the progress to the user interface.
func (s *Service) SetProgressEmitter(emitter func(ImportProgress)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.emitter = emitter
}

// ImportFile imports a dataset from a file.
//
// The kind decides which collection the records are written to. The
// import runs in the background, and the progress is reported through
// the emitter, so that the panel is not blocked while a large file is
// being imported.
//
// The import can be stopped with Cancel.
func (s *Service) ImportFile(kind, path string) error {
	if err := s.validateKind(kind); err != nil {
		return err
	}
	if path == "" {
		return fmt.Errorf("wikidata: the file is required")
	}

	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("wikidata: open %s: %w", path, err)
	}

	// The import runs in the background, so the file is owned by the
	// goroutine that reads it. Closing it here would close it before
	// the import has read a single line.
	if err := s.Import(kind, filepath.Base(path), file); err != nil {
		_ = file.Close()
		return err
	}

	return nil
}

// ImportDirectory imports both datasets of a directory.
//
// The files are the ones the ETL worker writes. A file that is not
// there is not an error: the panel shows what it has.
func (s *Service) ImportDirectory(dir string) error {
	if dir == "" {
		return fmt.Errorf("wikidata: the directory is required")
	}

	items := filepath.Join(dir, ItemsFileName)
	properties := filepath.Join(dir, PropertiesFileName)

	found := false
	for _, path := range []string{items, properties} {
		if _, err := os.Stat(path); err == nil {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("wikidata: no dataset in %s", dir)
	}

	if _, err := os.Stat(items); err == nil {
		if err := s.ImportFile(KindItems, items); err != nil {
			return err
		}
	}
	if _, err := os.Stat(properties); err == nil {
		if err := s.ImportFile(KindProperties, properties); err != nil {
			return err
		}
	}

	return nil
}

// Import imports a dataset from a reader.
//
// The import runs in the background. The progress is reported through
// the emitter as the import goes.
func (s *Service) Import(kind, name string, r io.Reader) error {
	if err := s.validateKind(kind); err != nil {
		return err
	}

	collection, keyField, err := s.targetOf(kind)
	if err != nil {
		return err
	}

	importer, err := NewImporter(s.app, collection, keyField)
	if err != nil {
		return err
	}

	importer.SetLogger(slog.Default())
	importer.SetReporter(ProgressReporterFunc(func(progress ImportProgress) {
		s.setProgress(progress)
	}))

	ctx, cancel := context.WithCancel(context.Background())

	s.mu.Lock()
	s.cancel = cancel
	s.running++
	s.progress = ImportProgress{Kind: kind, File: name}
	s.mu.Unlock()

	go func() {
		defer cancel()

		// The import is over, whatever the outcome: the count goes
		// back, so that a caller can wait for the last one.
		defer func() {
			s.mu.Lock()
			s.running--
			s.mu.Unlock()
		}()

		// The reader is closed when the import is over: the file was
		// opened for this import and nobody else holds it.
		if closer, ok := r.(io.Closer); ok {
			defer closer.Close()
		}

		stats, err := importer.Import(ctx, r, kind, name)

		progress := ImportProgress{
			Kind:     kind,
			File:     name,
			Read:     stats.Read,
			Imported: stats.Imported,
			Updated:  stats.Updated,
			Skipped:  stats.Skipped,
			Failed:   stats.Failed,
			Done:     true,
		}
		if err != nil {
			progress.Error = err.Error()
		}

		// The count of the running imports goes down before the
		// progress is set, so that the progress says whether another
		// import is still running.
		s.mu.Lock()
		s.running--
		stillRunning := s.running > 0
		s.mu.Unlock()

		progress.Running = stillRunning
		if stillRunning {
			progress.Done = false
		}

		s.setProgress(progress)
	}()

	return nil
}

// Running tells whether an import is running.
func (s *Service) Running() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running > 0
}

// Cancel stops the import that is running.
//
// The records that were written before the cancel stay written, and the
// import can be run again: the records are upserted.
func (s *Service) Cancel() {
	s.mu.RLock()
	cancel := s.cancel
	s.mu.RUnlock()

	if cancel != nil {
		cancel()
	}
}

// Close stops the imports that are running.
//
// The application calls it before the database is closed, so that an
// import is never writing to a database that is going away. The import
// that is stopped leaves the records it wrote written.
func (s *Service) Close() {
	s.Cancel()
}

// setProgress remembers the progress and tells the emitter.
func (s *Service) setProgress(progress ImportProgress) {
	s.mu.Lock()
	s.progress = progress
	emitter := s.emitter
	s.mu.Unlock()

	if emitter != nil {
		emitter(progress)
	}
}

// targetOf returns the collection and the key field of a kind.
func (s *Service) targetOf(kind string) (string, string, error) {
	switch kind {
	case KindItems:
		return CollectionItems, FieldQID, nil
	case KindProperties:
		return CollectionProperties, FieldPID, nil
	default:
		return "", "", fmt.Errorf("wikidata: unknown kind %q", kind)
	}
}

// validateKind tells whether a kind is one the importer knows.
func (s *Service) validateKind(kind string) error {
	switch kind {
	case KindItems, KindProperties:
		return nil
	default:
		return fmt.Errorf("wikidata: unknown kind %q", kind)
	}
}

// CountItems returns the number of imported items.
func (s *Service) CountItems() (int64, error) {
	return NewReader(s.app).CountItems()
}

// CountProperties returns the number of imported properties.
func (s *Service) CountProperties() (int64, error) {
	return NewReader(s.app).CountProperties()
}

// IsDatasetFile tells whether a path names one of the datasets.
//
// The panel uses it to accept a drag and a drop of the right file.
func IsDatasetFile(path string) bool {
	name := strings.ToLower(filepath.Base(path))
	return name == ItemsFileName || name == PropertiesFileName
}
