package etlpipeline

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/incurl/wikirevele/internal/etlhistory"
	"github.com/incurl/wikirevele/internal/wikidata"
)

// Importer imports the datasets the pipeline produced.
//
// It is the interface of the wikidata service, and the interface
// exists so that the pipeline does not have to know about the
// database.
type Importer interface {
	// ImportFile imports a dataset from a file.
	ImportFile(kind, path string) error
}

// Service runs the pipelines and records them.
//
// It is the layer between the interface and the parts: it starts the
// pipeline, records the run in the history, and imports the output
// when the run succeeds.
//
// The two halves of the job are separate on purpose. The pipeline
// writes the files, and the import reads them back into the
// database: the import can be run again, and a failed import does not
// cost the run that produced the files.
type Service struct {
	// supervisor owns the child processes.
	supervisor *Supervisor
	// history records the runs.
	history *etlhistory.Service
	// importer imports the output of a run.
	importer Importer
	// records holds the record of each run, by run identifier.
	//
	// The record is created before the run starts, and the supervisor
	// only knows the identifier of the run. The map is what links the
	// two.
	records map[string]string
	// mu guards the records.
	mu sync.Mutex
}

// NewService creates a service.
func NewService(supervisor *Supervisor, history *etlhistory.Service, importer Importer) *Service {
	service := &Service{
		supervisor: supervisor,
		history:    history,
		importer:   importer,
		records:    make(map[string]string),
	}

	// The supervisor tells the service when a run ends. The service
	// records it in the history, and imports the output when the run
	// succeeded.
	supervisor.SetOnFinished(service.finished)

	return service
}

// Supervisor returns the supervisor that owns the child processes.
//
// The application closes it when it closes, so that no pipeline is
// left running.
func (s *Service) Supervisor() *Supervisor {
	return s.supervisor
}

// Command returns the shell pipeline that runs the given job.
//
// It is the command the interface shows and the user copies. The
// command is built without running anything, so the user can see it
// before the job starts.
func (s *Service) Command(opts JobOptions) (string, error) {
	return BuildCommand(opts, WorkerCommand())
}

// Start runs a job.
//
// The run is recorded in the history before it starts, so that a run
// that crashes the application is still in the history.
func (s *Service) Start(ctx context.Context, opts JobOptions) (Progress, error) {
	if err := ValidateOptions(opts); err != nil {
		return Progress{}, err
	}

	command, err := s.Command(opts)
	if err != nil {
		return Progress{}, err
	}

	recordID, err := s.history.Start(
		command,
		opts.TargetQID,
		opts.RecursiveP131,
		strings.Join(opts.Languages, ","),
		opts.DumpPath,
		opts.OutItemsPath,
		opts.OutPropsPath,
	)
	if err != nil {
		return Progress{}, err
	}

	progress, err := s.supervisor.Start(ctx, opts)
	if err != nil {
		// The run never started: the record says so.
		_ = s.history.Finish(recordID, etlhistory.StatusFailed, err.Error())
		return Progress{}, err
	}

	// The record of the run is kept with the identifier of the run,
	// so that the history is updated when the run ends.
	s.mu.Lock()
	s.records[progress.ID] = recordID
	s.mu.Unlock()

	return progress, nil
}

// finished records how a run ended.
//
// It is called by the supervisor when the pipeline is over. The run is
// recorded in the history, and the output is imported when the run
// succeeded.
func (s *Service) finished(progress Progress) {
	s.mu.Lock()
	recordID := s.records[progress.ID]
	s.mu.Unlock()

	if recordID == "" {
		return
	}

	status := progress.Status
	if err := s.history.Finish(recordID, status, progress.Error); err != nil {
		// The history could not be updated. The run itself
		// succeeded or failed as it should, and the interface
		// shows the outcome either way.
		return
	}

	if status != StatusSucceeded {
		return
	}

	// The output of a successful run is imported, so that the panel
	// shows the new dataset without the user having to ask for it.
	_ = s.Import(progress.ItemFile, progress.PropFile)
}

// Cancel stops a running job.
//
// The whole process tree is stopped: the decompressor and the worker
// die together.
func (s *Service) Cancel(runID string) error {
	return s.supervisor.Cancel(runID)
}

// List returns the runs of the history, the most recent first.
func (s *Service) List(limit int) ([]etlhistory.Entry, error) {
	return s.history.List(limit)
}

// Delete removes a run from the history.
func (s *Service) Delete(recordID string) error {
	return s.history.Delete(recordID)
}

// Import brings the output of a run into the database.
//
// It is called when a run succeeds. The import runs in the background,
// and the progress is pushed to the interface.
func (s *Service) Import(itemsPath, propsPath string) error {
	if s.importer == nil {
		return fmt.Errorf("etlpipeline: the importer is not available")
	}

	if itemsPath != "" {
		if err := s.importer.ImportFile(wikidata.KindItems, itemsPath); err != nil {
			return err
		}
	}

	if propsPath != "" {
		if err := s.importer.ImportFile(wikidata.KindProperties, propsPath); err != nil {
			return err
		}
	}

	return nil
}
