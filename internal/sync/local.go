package sync

import (
	"fmt"
	"time"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

// LocalStore reads and writes the records of the local database.
//
// The local side is a PocketBase like the remote one, and it is
// reached through its Go API and not through its REST API: the
// application embeds it, and the Go API is the one that honours an
// explicit value of the "source_updated" field. The REST API
// overwrites it with the time of the write, and the whole conflict
// resolution rests on that field keeping its value.
type LocalStore struct {
	// app is the PocketBase application the records live in.
	app core.App
}

// NewLocalStore creates a store over the local application.
func NewLocalStore(app core.App) *LocalStore {
	return &LocalStore{app: app}
}

// App returns the PocketBase application the records are read from.
func (s *LocalStore) App() core.App { return s.app }

// HasSyncFields tells whether a collection carries the fields the
// sync needs.
//
// A collection without them cannot be synced: the field of the time
// is what the comparison of the two sides is made on, and without it
// the sync would overwrite the newer record with the older one
// without knowing that it did.
func (s *LocalStore) HasSyncFields(collection *core.Collection) bool {
	if collection == nil {
		return false
	}
	return collection.Fields.GetByName(FieldSourceUpdated) != nil &&
		collection.Fields.GetByName(FieldSourceOrigin) != nil
}

// LocalRecord is one record of the local database, as the sync sees
// it.
type LocalRecord struct {
	// ID is the identifier of the record.
	ID string `json:"id"`
	// Collection is the name of the collection it belongs to.
	Collection string `json:"collection"`
	// Data holds the fields of the record, and not the two the sync
	// owns: those are read apart.
	Data map[string]any `json:"data"`
	// SourceUpdated is the time the record was last changed on the
	// side it came from.
	SourceUpdated time.Time `json:"source_updated"`
	// SourceOrigin is the side the record was last written on.
	SourceOrigin string `json:"source_origin"`
}

// ToRemote turns a local record into the shape the remote server
// takes.
//
// The two fields the sync owns are set to the local side: a record
// that is pushed was changed here.
func (r LocalRecord) ToRemote(origin string) RemoteRecord {
	remote := RemoteRecord{}
	for key, value := range r.Data {
		remote[key] = value
	}
	remote["id"] = r.ID
	remote[FieldSourceUpdated] = formatDateTime(r.SourceUpdated)
	remote[FieldSourceOrigin] = origin
	return remote
}

// ListRecords returns the local records of a collection that changed
// since a time.
//
// The filter is on the field of the sync, and the records are read in
// pages so that a collection of a million records does not have to be
// held in memory.
func (s *LocalStore) ListRecords(collection string, since time.Time, limit int) ([]LocalRecord, error) {
	if limit <= 0 {
		limit = PageSize
	}

	out := make([]LocalRecord, 0, limit)
	offset := 0

	for len(out) < limit {
		page := limit - len(out)
		if page > PageSize {
			page = PageSize
		}

		filter := ""
		params := dbx.Params{}
		if !since.IsZero() {
			filter = fmt.Sprintf("%s > {:since}", FieldSourceUpdated)
			// The time is given to the query as the text of the
			// format of the server, so that the comparison of the
			// database is the comparison of the strings and not a
			// comparison of two formats.
			params["since"] = formatDateTime(since)
		}

		records, err := s.app.FindRecordsByFilter(collection, filter, FieldSourceUpdated, page, offset, params)
		if err != nil {
			return nil, fmt.Errorf("sync: list the records of %s: %w", collection, err)
		}

		for _, record := range records {
			out = append(out, s.toLocal(record))
		}

		if len(records) < page {
			break
		}
		offset += len(records)
	}

	return out, nil
}

// toLocal turns a record of the local database into the shape the
// sync works with.
func (s *LocalStore) toLocal(record *core.Record) LocalRecord {
	data := map[string]any{}
	for key, value := range record.FieldsData() {
		if key == FieldSourceUpdated || key == FieldSourceOrigin {
			continue
		}
		data[key] = value
	}

	origin := record.GetString(FieldSourceOrigin)

	return LocalRecord{
		ID:            record.Id,
		Collection:    record.Collection().Name,
		Data:          data,
		SourceUpdated: parseDateTime(record.GetString(FieldSourceUpdated)),
		SourceOrigin:  origin,
	}
}

// SaveRecords writes records into the local database.
//
// The records are written one by one and in a transaction, so that a
// failure does not leave half of a batch behind.
//
// A record that is already there is updated and one that is not is
// created: the identifier of the record is the key, and it is the
// same on the two sides.
func (s *LocalStore) SaveRecords(collection string, records []RemoteRecord, origin string) (int, error) {
	if len(records) == 0 {
		return 0, nil
	}

	written := 0
	err := s.app.RunInTransaction(func(txApp core.App) error {
		for _, remote := range records {
			if err := s.saveOne(txApp, collection, remote, origin); err != nil {
				return err
			}
			written++
		}
		return nil
	})
	if err != nil {
		return written, err
	}

	return written, nil
}

// saveOne writes one record into the local database.
//
// The record is looked up by its identifier, and it is created when
// there is none. The value of the "source_updated" field is set with
// SetRaw and not with Set: the field is an autodate one, and the Go
// API keeps the value that is given to it, while the REST API would
// replace it with the time of the write.
func (s *LocalStore) saveOne(app core.App, collection string, remote RemoteRecord, origin string) error {
	target, err := app.FindCollectionByNameOrId(collection)
	if err != nil {
		return fmt.Errorf("sync: find the collection %s: %w", collection, err)
	}

	record, err := app.FindRecordById(target, remote.ID())
	if err != nil {
		// The record is not there: it is created.
		record = core.NewRecord(target)

		// The identifier of the record is the one of the other
		// side. Without it the record would be written under a
		// new identifier, and the two sides would hold two
		// records of the same thing.
		record.SetRaw(core.FieldNameId, remote.ID())
	}

	for key, value := range remote {
		if key == "id" || key == FieldSourceUpdated || key == FieldSourceOrigin {
			continue
		}
		record.Set(key, value)
	}

	record.Set(FieldSourceOrigin, origin)

	// The time is written as it arrived: the field is a text one,
	// and the text of the format of the server sorts the same way
	// the times sort.
	if value, ok := remote[FieldSourceUpdated].(string); ok && value != "" {
		record.Set(FieldSourceUpdated, value)
	}

	if err := app.Save(record); err != nil {
		return fmt.Errorf("sync: save the record %s of %s: %w", remote.ID(), collection, err)
	}
	return nil
}

// SetSourceUpdated stamps a local record with the time it was changed
// on the side it came from.
//
// It is called when a record is written by the user, so that the
// record carries the time of the change and not the time of the last
// sync.
func (s *LocalStore) SetSourceUpdated(collection string, record *core.Record, origin string, when time.Time) error {
	record.Set(FieldSourceOrigin, origin)
	record.Set(FieldSourceUpdated, formatDateTime(when))

	if err := s.app.Save(record); err != nil {
		return fmt.Errorf("sync: stamp the record %s of %s: %w", record.Id, collection, err)
	}
	return nil
}
