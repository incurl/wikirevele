package sync

import (
	"encoding/json"
	"fmt"
)

// Settings holds the configuration of the sync.
//
// It is read from the app_settings collection of the application, next
// to the credentials, because it is configuration of the application
// and not data of the user.
type Settings struct {
	// Set is the set of the collections that take part in a sync.
	Set SyncSet `json:"set"`
	// Cursors are the times of the last runs, by collection.
	Cursors *CursorSet `json:"cursors"`
}

// Store reads and writes the configuration of the sync.
//
// The store is a thin adapter over the settings of the application:
// the values are held as JSON in the app_settings collection, and
// the collection is not exposed to the frontend, so the credentials
// and the sync set stay on the Go side.
type Store struct {
	// reader reads a setting.
	reader func(key string, out any) (bool, error)
	// writer writes a setting.
	writer func(key string, value any) error
}

// NewStore creates a store over the settings of the application.
func NewStore(reader func(key string, out any) (bool, error), writer func(key string, value any) error) *Store {
	return &Store{reader: reader, writer: writer}
}

// Load reads the configuration of the sync.
//
// A configuration that was never written is not an error: the sync
// starts empty, and the reader chooses what to sync.
func (s *Store) Load() (Settings, error) {
	settings := Settings{
		Set:     SyncSet{},
		Cursors: NewCursorSet(),
	}

	found, err := s.reader(SettingSyncSet, &settings.Set)
	if err != nil {
		return settings, fmt.Errorf("sync: read the set of the collections: %w", err)
	}
	if !found {
		settings.Set = SyncSet{}
	}

	found, err = s.reader(SettingSyncCursor, &settings.Cursors)
	if err != nil {
		return settings, fmt.Errorf("sync: read the times of the last runs: %w", err)
	}
	if !found || settings.Cursors == nil {
		settings.Cursors = NewCursorSet()
	}

	return settings, nil
}

// Save writes the configuration of the sync.
//
// The set of the collections and the times of the last runs are
// written together: they describe the same state, and one of them
// without the other is not a state a run can start from.
func (s *Store) Save(settings Settings) error {
	if err := s.writer(SettingSyncSet, settings.Set); err != nil {
		return fmt.Errorf("sync: write the set of the collections: %w", err)
	}

	cursors := settings.Cursors
	if cursors == nil {
		cursors = NewCursorSet()
	}
	if err := s.writer(SettingSyncCursor, cursors); err != nil {
		return fmt.Errorf("sync: write the times of the last runs: %w", err)
	}

	return nil
}

// MarshalSet turns the set of the collections into the JSON that is
// written to the settings.
//
// It is here so that the shape of the setting is written in one
// place, and not in the caller and in the reader.
func MarshalSet(set SyncSet) ([]byte, error) {
	raw, err := json.Marshal(set)
	if err != nil {
		return nil, fmt.Errorf("sync: marshal the set of the collections: %w", err)
	}
	return raw, nil
}
