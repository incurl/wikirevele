// Package sync moves collections between WikiRevele and a remote
// PocketBase server, which the user calls Ruis.
//
// The two sides are both PocketBase, and PocketBase already has the
// two operations a sync needs: a schema can be exported and imported
// as JSON, and a record can be upserted by its identifier. The work
// here is the part neither of them does: choosing the collections,
// ordering them so that a relation always points at something that
// already exists, reading only what changed since the last run, and
// deciding what happens when both sides changed the same record.
//
// The sync is always started from WikiRevele. Ruis never calls back.
package sync

import (
	"fmt"
	"strings"
	"time"
)

// Direction is the way the records of a collection travel.
type Direction string

const (
	// DirectionPush sends the local records to Ruis.
	DirectionPush Direction = "push"
	// DirectionPull takes the remote records into the local database.
	DirectionPull Direction = "pull"
	// DirectionReconcile sends the local records to Ruis and takes the
	// remote ones back, and lets the newer record win.
	DirectionReconcile Direction = "reconcile"
)

// Directions returns the directions a collection can be synced in.
//
// The order is the order of the interface: the reader sees the three
// choices in the same order every time.
func Directions() []Direction {
	return []Direction{DirectionPush, DirectionPull, DirectionReconcile}
}

// IsValid tells whether a direction is one the sync knows.
func (d Direction) IsValid() bool {
	switch d {
	case DirectionPush, DirectionPull, DirectionReconcile:
		return true
	default:
		return false
	}
}

// Label returns the name of the direction, as the interface shows it.
func (d Direction) Label() string {
	switch d {
	case DirectionPush:
		return "Push"
	case DirectionPull:
		return "Pull"
	case DirectionReconcile:
		return "Reconcile"
	default:
		return string(d)
	}
}

// The fields the sync owns in every collection it syncs.
//
// The server overwrites the "updated" field of a record on every
// write, so it cannot be the clock of a conflict: whichever side
// writes last stamps its own clock and wins. These two fields are
// written by the sync alone, and they are what the comparison is
// made on.
const (
	// FieldSourceUpdated holds the time the record was last changed on
	// the side it came from.
	//
	// It is an autodate field, so it is a date, and the sync sets it
	// explicitly with SetRaw: the Go API honours an explicit value,
	// while the REST API always overwrites it with the time of the
	// write.
	FieldSourceUpdated = "source_updated"
	// FieldSourceOrigin tells which side the record was last written
	// on. It is "local" or "remote".
	FieldSourceOrigin = "source_origin"
)

// The values of FieldSourceOrigin.
const (
	// OriginLocal means the record was last written in WikiRevele.
	OriginLocal = "local"
	// OriginRemote means the record was last written in Ruis.
	OriginRemote = "remote"
)

// The names of the settings the sync keeps.
//
// The set of the collections and the times of the last runs are kept
// in the app_settings collection, next to the credentials, because
// they are configuration of the application and not data of the user.
const (
	// SettingSyncSet holds the collections that take part in a sync.
	SettingSyncSet = "app.sync_set"
	// SettingSyncCursor holds the time of the last run, per
	// collection and per direction.
	SettingSyncCursor = "app.sync_cursor"
)

// CollectionSpec is one collection that takes part in a sync.
//
// It is what the user chooses in the interface: the collection, the
// direction, and whether its schema travels as well as its records.
type CollectionSpec struct {
	// Name is the name of the collection.
	Name string `json:"name"`
	// Direction is the way its records travel.
	Direction Direction `json:"direction"`
	// IncludeSchema tells whether the schema of the collection is
	// pushed to Ruis as well.
	//
	// A pull never needs it: the schema of a collection that is
	// pulled is the schema Ruis already has.
	IncludeSchema bool `json:"include_schema"`
}

// Validate tells whether a spec is one the sync can run.
func (s CollectionSpec) Validate() error {
	if strings.TrimSpace(s.Name) == "" {
		return fmt.Errorf("sync: a collection of the sync set has no name")
	}
	if !s.Direction.IsValid() {
		return fmt.Errorf("sync: collection %q has an unknown direction %q", s.Name, s.Direction)
	}
	return nil
}

// SyncSet is the whole set of the collections that take part in a
// sync.
type SyncSet struct {
	// Collections are the collections, in the order the sync runs
	// them in.
	Collections []CollectionSpec `json:"collections"`
}

// Validate tells whether the set is one the sync can run.
func (s SyncSet) Validate() error {
	seen := map[string]bool{}
	for _, spec := range s.Collections {
		if err := spec.Validate(); err != nil {
			return err
		}
		if seen[spec.Name] {
			return fmt.Errorf("sync: collection %q is in the sync set twice", spec.Name)
		}
		seen[spec.Name] = true
	}
	return nil
}

// Names returns the names of the collections of the set.
func (s SyncSet) Names() []string {
	names := make([]string, 0, len(s.Collections))
	for _, spec := range s.Collections {
		names = append(names, spec.Name)
	}
	return names
}

// Spec returns the spec of a collection of the set.
func (s SyncSet) Spec(name string) (CollectionSpec, bool) {
	for _, spec := range s.Collections {
		if spec.Name == name {
			return spec, true
		}
	}
	return CollectionSpec{}, false
}

// Cursor is the time of the last successful run, per collection and
// per direction.
//
// It is what makes the sync incremental: the next run only moves the
// records that changed since the time recorded here.
type Cursor struct {
	// Push is the time of the last push of a collection.
	Push time.Time `json:"push"`
	// Pull is the time of the last pull of a collection.
	Pull time.Time `json:"pull"`
}

// CursorSet holds the cursors of every collection.
//
// The key of the map is the name of the collection.
type CursorSet struct {
	// Cursors are the cursors, by collection name.
	Cursors map[string]Cursor `json:"cursors"`
}

// NewCursorSet returns an empty cursor set.
func NewCursorSet() *CursorSet {
	return &CursorSet{Cursors: map[string]Cursor{}}
}

// Get returns the cursor of a collection, and whether there was one.
func (c *CursorSet) Get(name string) (Cursor, bool) {
	if c == nil || c.Cursors == nil {
		return Cursor{}, false
	}
	cursor, ok := c.Cursors[name]
	return cursor, ok
}

// Set records the cursor of a collection.
func (c *CursorSet) Set(name string, cursor Cursor) {
	if c.Cursors == nil {
		c.Cursors = map[string]Cursor{}
	}
	c.Cursors[name] = cursor
}
