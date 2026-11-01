package sync

import (
	"fmt"

	"github.com/pocketbase/pocketbase/core"
)

// LocalSchema returns the schema of the local collections that take
// part in a sync.
//
// The shape of the answer is the one of the collections API of the
// server: the same JSON, with the same names, so that it can be
// given to the other side as it is.
//
// The identifier of a collection is the one of the local collection,
// and so is the identifier of each of its fields. That is what makes
// a relation point at the same collection on the two sides, and it is
// the reason a schema is pushed rather than created from scratch.
func (s *LocalStore) LocalSchema(names []string) ([]RemoteCollection, error) {
	out := make([]RemoteCollection, 0, len(names))

	for _, name := range names {
		collection, err := s.app.FindCollectionByNameOrId(name)
		if err != nil {
			return nil, fmt.Errorf("sync: find the collection %s: %w", name, err)
		}
		if collection.System {
			return nil, fmt.Errorf("sync: the collection %s belongs to the system and is not synced", name)
		}
		if collection.IsView() {
			return nil, fmt.Errorf("sync: the collection %s is a view and holds no records", name)
		}

		out = append(out, localCollectionSchema(collection))
	}

	return out, nil
}

// localCollectionSchema turns a collection into the JSON of the
// collections API.
//
// The whole collection is carried over, and not only the fields that
// the sync needs: a field the program does not know about still
// belongs to the user, and it has to travel.
func localCollectionSchema(collection *core.Collection) RemoteCollection {
	schema := RemoteCollection{
		"id":      collection.Id,
		"name":    collection.Name,
		"type":    collection.Type,
		"system":  false,
		"fields":  collection.Fields,
		"indexes": collection.Indexes,
	}

	// The options of the type of the collection travel with it: a
	// view carries its query in them, and an auth collection carries
	// the settings of its authentication.
	if len(collection.RawOptions) > 0 {
		schema["options"] = collection.RawOptions
	}

	// The rules travel with the schema: the remote side reads the
	// records with the same rules as the local side, and a
	// collection that is open on one side and closed on the other
	// is a difference the user would notice.
	if collection.ListRule != nil {
		schema["listRule"] = *collection.ListRule
	}
	if collection.ViewRule != nil {
		schema["viewRule"] = *collection.ViewRule
	}
	if collection.CreateRule != nil {
		schema["createRule"] = *collection.CreateRule
	}
	if collection.UpdateRule != nil {
		schema["updateRule"] = *collection.UpdateRule
	}
	if collection.DeleteRule != nil {
		schema["deleteRule"] = *collection.DeleteRule
	}

	return schema
}

// EnsureSyncFields adds the fields the sync needs to a collection.
//
// The fields are the clock of the conflict resolution, and a
// collection that does not have them cannot be synced. They are added
// when they are missing, so that a collection of an older database
// becomes syncable without the user having to write a migration.
//
// The function is idempotent: a collection that already has the
// fields is left alone.
func (s *LocalStore) EnsureSyncFields(name string) (bool, error) {
	collection, err := s.app.FindCollectionByNameOrId(name)
	if err != nil {
		return false, fmt.Errorf("sync: find the collection %s: %w", name, err)
	}
	if collection.System {
		return false, fmt.Errorf("sync: the collection %s belongs to the system and is not synced", name)
	}

	changed := false

	// The field of the time is a **text** field and not an autodate
	// one.
	//
	// The server overwrites the value of an autodate field on every
	// write, and it does it over the REST API as well: a record
	// written with an explicit time comes back with the time of the
	// write. The field of the sync has to keep the value that is
	// given to it, because it is the clock of the conflict
	// resolution, so it is stored as the text of the format of the
	// server.
	if collection.Fields.GetByName(FieldSourceUpdated) == nil {
		collection.Fields.Add(&core.TextField{
			Name: FieldSourceUpdated,
			Max:  32,
		})
		changed = true
	}

	if collection.Fields.GetByName(FieldSourceOrigin) == nil {
		collection.Fields.Add(&core.TextField{
			Name: FieldSourceOrigin,
			Max:  16,
		})
		changed = true
	}

	if !changed {
		return false, nil
	}

	if err := s.app.Save(collection); err != nil {
		return false, fmt.Errorf("sync: add the fields of the sync to %s: %w", name, err)
	}
	return true, nil
}

// EnsureSyncFieldsOf adds the fields the sync needs to every
// collection of a set.
//
// It is called before a run: a collection that is missing the fields
// gets them, and a collection that cannot be synced is reported and
// left out of the run.
func (s *LocalStore) EnsureSyncFieldsOf(names []string) ([]string, error) {
	ready := make([]string, 0, len(names))
	for _, name := range names {
		if _, err := s.EnsureSyncFields(name); err != nil {
			return ready, err
		}
		ready = append(ready, name)
	}
	return ready, nil
}
