package sync

import (
	"strings"
	"testing"
	"time"

	"github.com/incurl/wikirevele/internal/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

// newTestStore creates a store backed by a throwaway database.
//
// The database is the real one of the application, so that the test
// exercises the same schema and the same migrations as a run does.
func newTestStore(t *testing.T) *LocalStore {
	t.Helper()

	service := pocketbase.NewService(t.TempDir())
	if err := service.Bootstrap(); err != nil {
		t.Fatalf("bootstrap pocketbase: %v", err)
	}
	t.Cleanup(func() {
		if err := service.Close(); err != nil {
			t.Errorf("close pocketbase: %v", err)
		}
	})

	return NewLocalStore(service.App())
}

// newTestRecord creates one record of a collection.
func newTestRecord(t *testing.T, app core.App, collection *core.Collection, qid string) *core.Record {
	t.Helper()

	record := core.NewRecord(collection)
	record.Set("qid", qid)
	if err := app.Save(record); err != nil {
		t.Fatalf("create the record: %v", err)
	}
	return record
}

// TestDirectionLabelsAreStable verifies that the three directions
// have the names the interface shows.
//
// The names are part of what the frontend stores, so a rename would
// make a stored set unreadable.
func TestDirectionLabelsAreStable(t *testing.T) {
	cases := map[Direction]string{
		DirectionPush:      "Push",
		DirectionPull:      "Pull",
		DirectionReconcile: "Reconcile",
	}

	for direction, want := range cases {
		if got := direction.Label(); got != want {
			t.Errorf("direction %q label = %q, want %q", direction, got, want)
		}
	}
}

// TestSyncSetRejectsDuplicates verifies that a collection cannot be
// in the set twice.
//
// A collection twice would be synced twice, and the second run would
// find nothing to do.
func TestSyncSetRejectsDuplicates(t *testing.T) {
	set := SyncSet{Collections: []CollectionSpec{
		{Name: "notes", Direction: DirectionPush},
		{Name: "notes", Direction: DirectionPull},
	}}

	if err := set.Validate(); err == nil {
		t.Fatal("a set with a collection twice was accepted")
	}
}

// TestSyncSetRejectsUnknownDirection verifies that an unknown
// direction is refused.
func TestSyncSetRejectsUnknownDirection(t *testing.T) {
	set := SyncSet{Collections: []CollectionSpec{
		{Name: "notes", Direction: Direction("sideways")},
	}}

	if err := set.Validate(); err == nil {
		t.Fatal("a set with an unknown direction was accepted")
	}
}

// TestEnsureSyncFieldsIsIdempotent verifies that the fields of the
// sync are added once and are not added again.
//
// A second call must not fail and must not change the collection: the
// run calls it every time, and a run that fails on the second call
// would never work.
func TestEnsureSyncFieldsIsIdempotent(t *testing.T) {
	store := newTestStore(t)

	changed, err := store.EnsureSyncFields("wikidata_items")
	if err != nil {
		t.Fatalf("add the fields of the sync: %v", err)
	}
	if !changed {
		t.Fatal("the fields of the sync were not added")
	}

	changed, err = store.EnsureSyncFields("wikidata_items")
	if err != nil {
		t.Fatalf("add the fields of the sync again: %v", err)
	}
	if changed {
		t.Error("the fields of the sync were added a second time")
	}

	collection, err := store.App().FindCollectionByNameOrId("wikidata_items")
	if err != nil {
		t.Fatalf("find the collection: %v", err)
	}
	if !store.HasSyncFields(collection) {
		t.Error("the collection does not carry the fields of the sync")
	}
}

// TestEnsureSyncFieldsRefusesSystemCollections verifies that a
// collection of the system is not given the fields.
//
// The collections of the system belong to PocketBase, and adding a
// field to one of them would break the server.
func TestEnsureSyncFieldsRefusesSystemCollections(t *testing.T) {
	store := newTestStore(t)

	if _, err := store.EnsureSyncFields("_superusers"); err == nil {
		t.Fatal("a system collection was given the fields of the sync")
	}
}

// TestSourceUpdatedSurvivesALocalWrite is the test that the whole
// conflict resolution rests on.
//
// The server overwrites the "updated" field of a record on every
// write, and the field of the sync must keep the value that is given
// to it. If it did not, a pull could not tell which of the two
// records is the newer one.
func TestSourceUpdatedSurvivesALocalWrite(t *testing.T) {
	store := newTestStore(t)

	if _, err := store.EnsureSyncFields("wikidata_items"); err != nil {
		t.Fatalf("add the fields of the sync: %v", err)
	}

	collection, err := store.App().FindCollectionByNameOrId("wikidata_items")
	if err != nil {
		t.Fatalf("find the collection: %v", err)
	}

	record := newTestRecord(t, store.App(), collection, "Q1")

	stamp := time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)
	if err := store.SetSourceUpdated("wikidata_items", record, OriginLocal, stamp); err != nil {
		t.Fatalf("stamp the record: %v", err)
	}

	fresh, err := store.App().FindRecordById("wikidata_items", record.Id)
	if err != nil {
		t.Fatalf("read the record back: %v", err)
	}

	got := fresh.GetDateTime(FieldSourceUpdated).Time().UTC()
	if !got.Equal(stamp) {
		t.Errorf("source_updated = %s, want %s", got, stamp)
	}
	if origin := fresh.GetString(FieldSourceOrigin); origin != OriginLocal {
		t.Errorf("source_origin = %q, want %q", origin, OriginLocal)
	}
}

// TestSelectWinnersKeepsTheNewerRecord verifies the conflict
// resolution.
//
// The remote record wins when it was changed later, and the local one
// is left alone when it is the newer of the two.
func TestSelectWinnersKeepsTheNewerRecord(t *testing.T) {
	store := newTestStore(t)

	if _, err := store.EnsureSyncFields("wikidata_items"); err != nil {
		t.Fatalf("add the fields of the sync: %v", err)
	}

	collection, err := store.App().FindCollectionByNameOrId("wikidata_items")
	if err != nil {
		t.Fatalf("find the collection: %v", err)
	}

	service := &Service{store: store}

	// A record that is not here yet is always taken.
	unknown := RemoteRecord{
		"id":               "abcdefghijklmno",
		FieldSourceUpdated: formatDateTime(time.Now()),
	}
	accepted, skipped := service.selectWinners("wikidata_items", []RemoteRecord{unknown})
	if len(accepted) != 1 || skipped != 0 {
		t.Fatalf("a record that was not here: accepted %d, skipped %d", len(accepted), skipped)
	}

	// A record that is here is taken when the remote one is newer.
	local := newTestRecord(t, store.App(), collection, "Q2")
	if err := store.SetSourceUpdated("wikidata_items", local, OriginLocal,
		time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)); err != nil {
		t.Fatalf("stamp the record: %v", err)
	}

	newer := RemoteRecord{
		"id":               local.Id,
		FieldSourceUpdated: formatDateTime(time.Date(2021, 1, 2, 3, 4, 5, 0, time.UTC)),
	}
	accepted, skipped = service.selectWinners("wikidata_items", []RemoteRecord{newer})
	if len(accepted) != 1 || skipped != 0 {
		t.Fatalf("a newer remote record: accepted %d, skipped %d", len(accepted), skipped)
	}

	// And it is left alone when the local one is the newer.
	older := RemoteRecord{
		"id":               local.Id,
		FieldSourceUpdated: formatDateTime(time.Date(2019, 1, 2, 3, 4, 5, 0, time.UTC)),
	}
	accepted, skipped = service.selectWinners("wikidata_items", []RemoteRecord{older})
	if len(accepted) != 0 || skipped != 1 {
		t.Fatalf("an older remote record: accepted %d, skipped %d", len(accepted), skipped)
	}
}

// TestOrderPlacesDependenciesFirst verifies that a collection is
// synced before the one that points at it.
//
// A relation has to point at something that is already there, or the
// record that holds it cannot be written.
func TestOrderPlacesDependenciesFirst(t *testing.T) {
	store := newTestStore(t)

	items, err := store.App().FindCollectionByNameOrId("wikidata_items")
	if err != nil {
		t.Fatalf("find the items: %v", err)
	}
	properties, err := store.App().FindCollectionByNameOrId("wikidata_properties")
	if err != nil {
		t.Fatalf("find the properties: %v", err)
	}

	// A relation from the items to the properties.
	items.Fields.Add(&core.RelationField{
		Name:         "linked_property",
		CollectionId: properties.Id,
	})
	if err := store.App().Save(items); err != nil {
		t.Fatalf("add the relation: %v", err)
	}

	service := &Service{store: store}

	set := SyncSet{Collections: []CollectionSpec{
		{Name: "wikidata_items", Direction: DirectionPush},
		{Name: "wikidata_properties", Direction: DirectionPush},
	}}

	ordered, err := service.order(set)
	if err != nil {
		t.Fatalf("order the collections: %v", err)
	}

	if len(ordered) != 2 {
		t.Fatalf("ordered %d collections, want 2", len(ordered))
	}
	if ordered[0].Name != "wikidata_properties" {
		t.Errorf("the first collection is %q, want the properties first", ordered[0].Name)
	}
}

// TestCollectionsLeavesTheSystemOut verifies that the collections of
// the system are not offered.
//
// They belong to PocketBase and not to the data of the user.
func TestCollectionsLeavesTheSystemOut(t *testing.T) {
	store := newTestStore(t)
	service := NewService(store, func() (*Client, error) { return nil, nil })

	collections, err := service.Collections()
	if err != nil {
		t.Fatalf("list the collections: %v", err)
	}

	for _, collection := range collections {
		if strings.HasPrefix(collection.Name, "_") {
			t.Errorf("the system collection %q was offered", collection.Name)
		}
	}

	found := false
	for _, collection := range collections {
		if collection.Name == "wikidata_items" {
			found = true
		}
	}
	if !found {
		t.Error("the collection of the items was not offered")
	}
}
