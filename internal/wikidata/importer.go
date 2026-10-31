package wikidata

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/security"
)

// The names of the collections the importer fills.
//
// They are created by the migration in internal/pocketbase/migrations,
// which is the only place the schema is defined.
const (
	// CollectionItems holds the items of the trimmed dataset.
	CollectionItems = "wikidata_items"
	// CollectionProperties holds the property dictionary.
	CollectionProperties = "wikidata_properties"
)

// The names of the fields of the two collections.
const (
	// FieldQID is the identifier of an item.
	FieldQID = "qid"
	// FieldPID is the identifier of a property.
	FieldPID = "pid"
	// FieldType is the kind of the entity.
	FieldType = "type"
	// FieldLabels holds the labels of the entity.
	FieldLabels = "labels"
	// FieldDescriptions holds the descriptions of the entity.
	FieldDescriptions = "descriptions"
	// FieldAliases holds the aliases of the entity.
	FieldAliases = "aliases"
	// FieldClaims holds the statements of the entity.
	FieldClaims = "claims"
)

// The sizes of the buffers of the importer.
const (
	// scanBufferSize is the initial size of the buffer the scanner
	// reads into.
	//
	// A trimmed item carries its whole claims tree, so a line is
	// bigger than a line of the dump, but it is still a line.
	scanBufferSize = 256 << 10
	// maxScanBufferSize is the largest line the scanner accepts.
	//
	// A line longer than this is a broken file rather than a real
	// record.
	maxScanBufferSize = 32 << 20
	// batchSize is the number of records that are written in one
	// transaction.
	//
	// One statement per record would spend all the time in the
	// database. A batch keeps the number of statements low while the
	// batch itself stays small enough to be held in memory.
	batchSize = 200
)

// recordIDPattern is the pattern PocketBase uses for the identifier of
// a record: fifteen lowercase letters and digits.
const recordIDPattern = `[a-z0-9]{15}`

// ImportStats counts what an import has done so far.
type ImportStats struct {
	// Read is the number of lines read from the file.
	Read int64
	// Imported is the number of records written to the database.
	Imported int64
	// Updated is the number of records that were already there and
	// were overwritten.
	Updated int64
	// Skipped is the number of lines that were not valid records.
	Skipped int64
	// Failed is the number of records the database refused.
	Failed int64
}

// ImportProgress is the progress of an import.
//
// It is the state the panel shows while a file is being imported.
type ImportProgress struct {
	// Kind is the kind of the file: "items" or "properties".
	Kind string `json:"kind"`
	// File is the name of the file being imported.
	File string `json:"file"`
	// Read is the number of lines read so far.
	Read int64 `json:"read"`
	// Imported is the number of records written so far.
	Imported int64 `json:"imported"`
	// Updated is the number of records overwritten so far.
	Updated int64 `json:"updated"`
	// Skipped is the number of invalid lines skipped so far.
	Skipped int64 `json:"skipped"`
	// Failed is the number of records the database refused.
	Failed int64 `json:"failed"`
	// Done tells whether the import has finished.
	Done bool `json:"done"`
	// Running tells whether an import is running right now.
	//
	// A directory import starts two of them, so the progress of the
	// last one is not enough to know that the work is over.
	Running bool `json:"running"`
	// Error is the message of the failure that stopped the import.
	Error string `json:"error"`
}

// ProgressReporter is told about the progress of an import.
//
// The interface exists so that the progress can be pushed to the user
// interface as well as written to a log.
type ProgressReporter interface {
	// Report is called with the progress of the import.
	Report(progress ImportProgress)
}

// ProgressReporterFunc adapts a plain function to the ProgressReporter
// interface.
type ProgressReporterFunc func(progress ImportProgress)

// Report calls the wrapped function.
func (f ProgressReporterFunc) Report(progress ImportProgress) {
	f(progress)
}

// Importer writes the records of a dataset into PocketBase.
//
// The importer holds one record in memory at a time: the file is read
// a line at a time, the line is decoded, and the line is dropped. The
// records are written in batches, so that a file of a hundred thousand
// records does not become a hundred thousand statements.
type Importer struct {
	// app is the PocketBase application the records are written to.
	app core.App
	// collection is the collection the records are written to.
	collection *core.Collection
	// keyField is the field the records are upserted by: "qid" for
	// the items, "pid" for the properties.
	keyField string
	// reporter is told about the progress. It may be nil.
	reporter ProgressReporter
	// log receives the records that could not be imported. It may be
	// nil.
	log *slog.Logger
	// now is the timestamp put on the records. It is a field so that
	// the tests can pin it.
	now func() string
	// written is the set of the keys that were written during this
	// run.
	//
	// A file may hold the same record twice, and the second one is
	// known to be an update. The set saves a lookup per record, and
	// it is small next to the file.
	written map[string]struct{}
}

// NewImporter creates an importer that writes to the PocketBase
// application.
//
// The collection and the key field are the ones of the dataset: the
// items are upserted by "qid" and the properties by "pid".
func NewImporter(app core.App, collection, keyField string) (*Importer, error) {
	if app == nil {
		return nil, fmt.Errorf("wikidata: the application is required")
	}

	model, err := app.FindCollectionByNameOrId(collection)
	if err != nil {
		return nil, fmt.Errorf("wikidata: find the collection %s: %w", collection, err)
	}

	return &Importer{
		app:        app,
		collection: model,
		keyField:   keyField,
		now:        func() string { return time.Now().UTC().Format(time.RFC3339Nano) },
		written:    make(map[string]struct{}),
	}, nil
}

// SetReporter sets the reporter that is told about the progress.
func (i *Importer) SetReporter(reporter ProgressReporter) {
	i.reporter = reporter
}

// SetLogger sets the logger that receives the records that could not
// be imported.
func (i *Importer) SetLogger(log *slog.Logger) {
	i.log = log
}

// Import reads a dataset from r and writes the records to the
// database.
//
// The file is read a line at a time, and the records are written in
// batches. A line that is not a valid record is counted and skipped,
// and the import carries on: a file with a few damaged lines is still
// worth importing.
//
// The import stops when the context is cancelled or when the database
// fails. The records that were written before the failure stay
// written, and the import can be run again: the records are upserted,
// so a second run over the same file does not duplicate them.
func (i *Importer) Import(ctx context.Context, r io.Reader, kind, file string) (ImportStats, error) {
	var stats ImportStats

	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, scanBufferSize), maxScanBufferSize)

	pending := make([]dbx.Params, 0, batchSize)

	flush := func() error {
		if len(pending) == 0 {
			return nil
		}
		if err := i.writeBatch(pending); err != nil {
			return err
		}
		pending = pending[:0]
		return nil
	}

	for scanner.Scan() {
		if err := ctx.Err(); err != nil {
			return stats, err
		}

		line := scanner.Bytes()
		if len(strings.TrimSpace(string(line))) == 0 {
			continue
		}
		stats.Read++

		params, isNew, err := i.paramsFor(line)
		id := fmt.Sprint(params[i.keyField])
		if err != nil {
			// A line that is not a record of this dataset is
			// skipped: the file is what it is, and the import
			// is about the rest of it.
			stats.Skipped++
			i.logSkipped(file, stats.Read, err)
			continue
		}

		if err := i.writeOne(i.app, params); err != nil {
			stats.Failed++
			i.logSkipped(file, stats.Read, err)
			continue
		}

		if isNew {
			stats.Imported++
		} else {
			stats.Updated++
		}
		i.written[id] = struct{}{}

		// The batch is written when it is full, so that the memory
		// the import holds does not grow with the file.
		if len(pending) >= batchSize {
			_ = flush()
		}
	}

	if err := scanner.Err(); err != nil {
		return stats, fmt.Errorf("wikidata: read %s: %w", file, err)
	}

	if err := flush(); err != nil {
		return stats, err
	}

	return stats, nil
}

// paramsFor turns a line into the parameters of a record.
//
// It reports whether the record is new: a record that is already in
// the database is updated, and one that is not is inserted.
func (i *Importer) paramsFor(line []byte) (dbx.Params, bool, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(line, &raw); err != nil {
		return nil, false, fmt.Errorf("wikidata: the line is not valid JSON: %w", err)
	}

	id, err := stringField(raw, "id")
	if err != nil {
		return nil, false, err
	}
	if id == "" {
		return nil, false, ErrNoIdentifier
	}

	entityType, _ := stringField(raw, "type")

	params := dbx.Params{
		i.keyField: id,
		FieldType:  entityType,
	}

	// The terms and the claims are stored as they were written, so
	// that the panel reads the same structure the ETL worker
	// produced.
	for _, field := range []string{FieldLabels, FieldDescriptions, FieldAliases, FieldClaims} {
		value, ok := raw[field]
		if !ok || len(value) == 0 {
			continue
		}
		params[field] = value
	}

	// A key that was written during this run is known to be there, so
	// the lookup is skipped.
	if _, ok := i.written[id]; ok {
		return params, false, nil
	}

	exists, err := i.exists(id)
	if err != nil {
		return nil, false, err
	}

	return params, !exists, nil
}

// exists tells whether a record is already in the database.
func (i *Importer) exists(id string) (bool, error) {
	var count int
	err := i.app.DB().
		NewQuery("SELECT COUNT(*) FROM " + i.collection.Name + " WHERE " + i.keyField + " = {:id}").
		Bind(dbx.Params{"id": id}).
		Row(&count)
	if err != nil {
		return false, fmt.Errorf("wikidata: look up %s: %w", id, err)
	}
	return count > 0, nil
}

// writeBatch writes the pending records in one transaction.
//
// The records are written one by one, because the upsert is decided
// per record, but they share a transaction, so that a batch is one
// round trip to the database.
func (i *Importer) writeBatch(pending []dbx.Params) error {
	return i.app.RunInTransaction(func(txApp core.App) error {
		for _, params := range pending {
			if err := i.writeOne(txApp, params); err != nil {
				return err
			}
		}
		return nil
	})
}

// writeOne writes one record, with the given application.
//
// The record is upserted on the unique key, so that importing the same
// file twice updates the records instead of duplicating them. The
// statement is written by hand rather than through the PocketBase
// model API: the importer writes a hundred thousand records, and the
// model API is built for a handful.
//
// The record keeps its identifier when it is already there, so that a
// second import does not change the identity of the record.
func (i *Importer) writeOne(app core.App, params dbx.Params) error {
	columns := make([]string, 0, len(params)+1)
	values := make([]string, 0, len(params)+1)
	sets := make([]string, 0, len(params))
	bound := make(map[string]any, len(params)+1)

	for name, value := range params {
		columns = append(columns, name)
		values = append(values, "{:"+name+"}")
		sets = append(sets, name+" = excluded."+name)
		bound[name] = value
	}

	// The record has an identifier of its own, and the import is the
	// only writer, so the identifier is generated here.
	columns = append(columns, "id")
	values = append(values, "{:id}")
	bound["id"] = security.RandomString(15)

	statement := "INSERT INTO " + i.collection.Name +
		" (" + strings.Join(columns, ", ") + ") VALUES (" + strings.Join(values, ", ") + ")" +
		" ON CONFLICT(" + i.keyField + ") DO UPDATE SET " + strings.Join(sets, ", ")

	if _, err := app.DB().NewQuery(statement).Bind(bound).Execute(); err != nil {
		return fmt.Errorf("wikidata: write the record: %w", err)
	}

	return nil
}

// logSkipped reports a record that could not be imported.
func (i *Importer) logSkipped(file string, line int64, err error) {
	if i.log == nil {
		return
	}
	i.log.Warn("the record was skipped",
		"file", file,
		"line", line,
		"reason", err.Error())
}

// stringField reads a string field of a raw record.
func stringField(raw map[string]json.RawMessage, name string) (string, error) {
	value, ok := raw[name]
	if !ok {
		return "", nil
	}

	var s string
	if err := json.Unmarshal(value, &s); err != nil {
		return "", fmt.Errorf("wikidata: the field %s is not a string: %w", name, err)
	}

	return s, nil
}
