// Package etlhistory records the runs of the extraction worker.
//
// Every run the application starts is recorded: the command that was
// run, the parameters it was run with, and what came of it.
//
// The record is what the history panel shows, and the command it holds
// is what the user copies to run the job again in a terminal.
package etlhistory

import (
	"fmt"
	"sort"
	"time"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/types"
)

// The name of the collection the runs are recorded in.
//
// It is created by the migration in internal/pocketbase/migrations,
// which is the only place the schema is defined.
const CollectionHistory = "etl_job_history"

// The names of the fields of the collection.
const (
	// FieldFullCommand is the whole shell pipeline.
	FieldFullCommand = "full_command"
	// FieldTargetQID is the entity the items were linked to.
	FieldTargetQID = "target_qid"
	// FieldRecursiveP131 says whether the hierarchy was walked.
	FieldRecursiveP131 = "recursive_p131"
	// FieldLangs are the languages that were kept.
	FieldLangs = "langs"
	// FieldDumpPath is the dump that was read.
	FieldDumpPath = "dump_file_path"
	// FieldOutItemsPath is the file the items were written to.
	FieldOutItemsPath = "out_items_path"
	// FieldOutPropsPath is the file the properties were written to.
	FieldOutPropsPath = "out_props_path"
	// FieldStartedAt is when the run began.
	FieldStartedAt = "started_at"
	// FieldFinishedAt is when the run ended.
	FieldFinishedAt = "finished_at"
	// FieldStatus is where the run is.
	FieldStatus = "status"
	// FieldErrorMessage is why the run failed.
	FieldErrorMessage = "error_message"
	// FieldNotes is the remark of the user.
	FieldNotes = "notes"
	// FieldCreated is when the record was written.
	FieldCreated = "created"
	// FieldSequence is the order of the runs.
	FieldSequence = "sequence"
)

// The statuses a run goes through.
//
// They are the statuses of the pipeline, and they are the values of
// the status field of the collection.
const (
	// StatusRunning means the run is going.
	StatusRunning = "running"
	// StatusSucceeded means the run finished and wrote its output.
	StatusSucceeded = "succeeded"
	// StatusCancelled means the user stopped it.
	StatusCancelled = "cancelled"
	// StatusFailed means the run failed.
	StatusFailed = "failed"
)

// Entry is one run of the worker, as the interface sees it.
//
// The command is the whole point of the entry: it is what the user
// copies to run the job again.
type Entry struct {
	// ID is the identifier of the record.
	ID string `json:"id"`
	// FullCommand is the shell pipeline that was run.
	FullCommand string `json:"full_command"`
	// TargetQID is the entity the items were linked to.
	TargetQID string `json:"target_qid"`
	// RecursiveP131 says whether the hierarchy was walked.
	RecursiveP131 bool `json:"recursive_p131"`
	// Langs are the languages that were kept.
	Langs string `json:"langs"`
	// DumpFilePath is the dump that was read.
	DumpFilePath string `json:"dump_file_path"`
	// OutItemsPath is the file the items were written to.
	OutItemsPath string `json:"out_items_path"`
	// OutPropsPath is the file the properties were written to.
	OutPropsPath string `json:"out_props_path"`
	// StartedAt is when the run began.
	StartedAt string `json:"started_at"`
	// FinishedAt is when the run ended. It is empty while the run is
	// going.
	FinishedAt string `json:"finished_at"`
	// Status is where the run is.
	Status string `json:"status"`
	// ErrorMessage is why the run failed. It is empty when it did not.
	ErrorMessage string `json:"error_message"`
	// Notes is the remark of the user.
	Notes string `json:"notes"`
}

// Service records the runs and reads them back.
//
// The service is the only writer of the collection: the runs are
// recorded when the pipeline starts and when it ends, and nothing else
// touches the records.
type Service struct {
	// app is the PocketBase application the records live in.
	app core.App
}

// NewService creates a service over the PocketBase application.
func NewService(app core.App) *Service {
	return &Service{app: app}
}

// Start records a run that is beginning.
//
// The record is created before the pipeline starts, so that a run that
// crashes the application is still in the history: it is marked as
// running, and it is marked as failed the next time the application
// starts.
func (s *Service) Start(command, targetQID string, recursive bool, langs, dumpPath, outItems, outProps string) (string, error) {
	collection, err := s.collection()
	if err != nil {
		return "", err
	}

	record := core.NewRecord(collection)
	record.Set(FieldFullCommand, command)
	record.Set(FieldTargetQID, targetQID)
	record.Set(FieldRecursiveP131, recursive)
	record.Set(FieldLangs, langs)
	record.Set(FieldDumpPath, dumpPath)
	record.Set(FieldOutItemsPath, outItems)
	record.Set(FieldOutPropsPath, outProps)
	record.Set(FieldStartedAt, time.Now())
	record.Set(FieldStatus, StatusRunning)
	record.Set(FieldSequence, s.nextSequence())

	if err := s.app.Save(record); err != nil {
		return "", fmt.Errorf("etlhistory: record the run: %w", err)
	}

	return record.Id, nil
}

// nextSequence returns the number of the next run.
//
// The number is the count of the runs that were recorded, plus one.
// It is what orders the history: the date fields have a resolution of
// a second, and two runs started in the same second would be in an
// arbitrary order.
func (s *Service) nextSequence() int64 {
	count, err := s.app.CountRecords(CollectionHistory)
	if err != nil {
		// The count is only for the order. A number that is a bit
		// off does not break anything: the runs are still in the
		// right order, and the next run gets the next number.
		return time.Now().UnixNano()
	}

	return int64(count) + 1
}

// Finish records how a run ended.
//
// The status is one of the four the collection holds. A run that is
// still going is not updated: a run that was never finished is
// reconciled when the application starts again.
func (s *Service) Finish(recordID, status, errorMessage string) error {
	if recordID == "" {
		return nil
	}

	record, err := s.app.FindRecordById(CollectionHistory, recordID)
	if err != nil {
		return fmt.Errorf("etlhistory: find the run %q: %w", recordID, err)
	}

	record.Set(FieldStatus, status)
	record.Set(FieldFinishedAt, time.Now())
	if errorMessage != "" {
		record.Set(FieldErrorMessage, errorMessage)
	}

	if err := s.app.Save(record); err != nil {
		return fmt.Errorf("etlhistory: record the end of the run: %w", err)
	}

	return nil
}

// List returns the runs, the most recent first.
func (s *Service) List(limit int) ([]Entry, error) {
	if limit <= 0 {
		limit = 50
	}

	records, err := s.app.FindRecordsByFilter(
		CollectionHistory,
		"",         // no filter: every run
		"-created", // the most recent first
		limit,
		0,
	)
	if err != nil {
		return nil, fmt.Errorf("etlhistory: list the runs: %w", err)
	}

	entries := make([]Entry, 0, len(records))
	sequence := make(map[string]int64, len(records))
	for _, record := range records {
		entries = append(entries, entryOf(record))
		sequence[record.Id] = int64(record.GetInt(FieldSequence))
	}

	// The most recent run is the first one. The identifier is the tie
	// breaker: two runs written in the same millisecond are rare, and
	// the order has to be the same every time the history is read.
	sort.SliceStable(entries, func(i, j int) bool {
		if sequence[entries[i].ID] != sequence[entries[j].ID] {
			return sequence[entries[i].ID] > sequence[entries[j].ID]
		}
		return entries[i].ID > entries[j].ID
	})

	if len(entries) > limit {
		entries = entries[:limit]
	}

	return entries, nil
}

// Get returns one run.
func (s *Service) Get(recordID string) (Entry, error) {
	record, err := s.app.FindRecordById(CollectionHistory, recordID)
	if err != nil {
		return Entry{}, fmt.Errorf("etlhistory: find the run %q: %w", recordID, err)
	}

	return entryOf(record), nil
}

// Delete removes a run from the history.
func (s *Service) Delete(recordID string) error {
	record, err := s.app.FindRecordById(CollectionHistory, recordID)
	if err != nil {
		return fmt.Errorf("etlhistory: find the run %q: %w", recordID, err)
	}

	if err := s.app.Delete(record); err != nil {
		return fmt.Errorf("etlhistory: delete the run: %w", err)
	}

	return nil
}

// Reconcile marks the runs that were interrupted as failed.
//
// A run that was going when the application last closed was never
// finished and never failed. Nothing would ever pick it up again, and
// the user could not tell that it did not work. The reconciliation
// marks it as failed, so that it appears in the history as what it
// was.
//
// It returns the number of runs that were marked.
func (s *Service) Reconcile() (int, error) {
	records, err := s.app.FindRecordsByFilter(
		CollectionHistory,
		FieldStatus+" = 'running'",
		"-created",
		0,
		0,
	)
	if err != nil {
		return 0, fmt.Errorf("etlhistory: find the interrupted runs: %w", err)
	}

	count := 0
	for _, record := range records {
		record.Set(FieldStatus, StatusFailed)
		record.Set(FieldErrorMessage, "the application was closed while the run was going")
		record.Set(FieldFinishedAt, time.Now())

		if err := s.app.Save(record); err != nil {
			return count, fmt.Errorf("etlhistory: mark the interrupted run: %w", err)
		}
		count++
	}

	return count, nil
}

// collection returns the collection of the runs.
func (s *Service) collection() (*core.Collection, error) {
	collection, err := s.app.FindCollectionByNameOrId(CollectionHistory)
	if err != nil {
		return nil, fmt.Errorf("etlhistory: the collection is missing: %w", err)
	}

	return collection, nil
}

// entryOf turns a record into an entry.
func entryOf(record *core.Record) Entry {
	entry := Entry{
		ID:            record.Id,
		FullCommand:   record.GetString(FieldFullCommand),
		TargetQID:     record.GetString(FieldTargetQID),
		RecursiveP131: record.GetBool(FieldRecursiveP131),
		Langs:         record.GetString(FieldLangs),
		DumpFilePath:  record.GetString(FieldDumpPath),
		OutItemsPath:  record.GetString(FieldOutItemsPath),
		OutPropsPath:  record.GetString(FieldOutPropsPath),
		Status:        record.GetString(FieldStatus),
		ErrorMessage:  record.GetString(FieldErrorMessage),
		Notes:         record.GetString(FieldNotes),
	}

	entry.StartedAt = formatTime(record.GetDateTime(FieldStartedAt))
	entry.FinishedAt = formatTime(record.GetDateTime(FieldFinishedAt))

	return entry
}

// formatTime writes a time for the interface.
//
// The time is written in the local zone and to the second: the
// history is read by a person, and the second is enough to tell two
// runs of the same day apart.
func formatTime(value types.DateTime) string {
	if value.IsZero() {
		return ""
	}

	return value.Time().Local().Format("2006-01-02 15:04:05")
}
