package sync

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
)

// newTestServer starts a PocketBase server with a superuser and
// returns its address.
//
// The server is a real one: the sync talks to it over HTTP, exactly
// as it talks to the remote server of the user.
func newTestServer(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()

	app := pocketbase.NewWithConfig(pocketbase.Config{DefaultDataDir: dir})
	if err := app.Bootstrap(); err != nil {
		t.Fatalf("bootstrap pocketbase: %v", err)
	}
	t.Cleanup(func() {
		_ = app.ResetBootstrapState()
	})

	// The superuser the sync authenticates as.
	superusers, err := app.FindCollectionByNameOrId(core.CollectionNameSuperusers)
	if err != nil {
		t.Fatalf("find the superusers: %v", err)
	}
	admin := core.NewRecord(superusers)
	admin.SetEmail("admin@example.com")
	admin.SetPassword("secret1234")
	if err := app.Save(admin); err != nil {
		t.Fatalf("create the superuser: %v", err)
	}

	// The batch API is off by default, and the sync needs it.
	app.Settings().Batch.Enabled = true
	if err := app.Save(app.Settings()); err != nil {
		t.Fatalf("enable the batch api: %v", err)
	}

	router, err := apis.NewRouter(app)
	if err != nil {
		t.Fatalf("create the router: %v", err)
	}
	mux, err := router.BuildMux()
	if err != nil {
		t.Fatalf("build the mux: %v", err)
	}

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	return server.URL
}

// waitForSync waits until the sync that is running is over.
//
// The run happens in the background, and the test has to wait for it
// before it looks at the result.
func waitForSync(t *testing.T, service *Service) {
	t.Helper()

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if !service.Running() {
			progress := service.Progress()
			if progress.Error != "" {
				t.Fatalf("the sync failed: %s", progress.Error)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("the sync did not finish in time")
}

// TestPushMovesTheRecordsToTheServer is the end to end test of a
// push.
//
// A record is written locally, the sync runs, and the record is on
// the server: that is the whole promise of a push.
func TestPushMovesTheRecordsToTheServer(t *testing.T) {
	address := newTestServer(t)
	store := newTestStore(t)

	if _, err := store.EnsureSyncFields("wikidata_items"); err != nil {
		t.Fatalf("add the fields of the sync: %v", err)
	}

	collection, err := store.App().FindCollectionByNameOrId("wikidata_items")
	if err != nil {
		t.Fatalf("find the collection: %v", err)
	}

	local := newTestRecord(t, store.App(), collection, "Q90")
	if err := store.SetSourceUpdated("wikidata_items", local, OriginLocal,
		time.Date(2021, 5, 6, 7, 8, 9, 0, time.UTC)); err != nil {
		t.Fatalf("stamp the record: %v", err)
	}

	service := NewService(store, func() (*Client, error) {
		return NewClient(address, "admin@example.com", "secret1234"), nil
	})
	service.SetSyncSet(SyncSet{Collections: []CollectionSpec{
		{Name: "wikidata_items", Direction: DirectionPush, IncludeSchema: true},
	}})

	if err := service.Run(context.Background()); err != nil {
		t.Fatalf("run the sync: %v", err)
	}
	waitForSync(t, service)

	// The record has to be on the server.
	client := NewClient(address, "admin@example.com", "secret1234")
	remote, err := client.ListRecords(context.Background(), "wikidata_items", time.Time{}, 10)
	if err != nil {
		t.Fatalf("read the remote records: %v", err)
	}

	if len(remote) != 1 {
		t.Fatalf("the server holds %d records, want 1", len(remote))
	}
	if remote[0].ID() != local.Id {
		t.Errorf("the remote record is %q, want %q", remote[0].ID(), local.Id)
	}
	if qid, _ := remote[0]["qid"].(string); qid != "Q90" {
		t.Errorf("the remote qid is %q, want Q90", qid)
	}
}

// TestPullBringsTheRecordsBack verifies that a pull writes the
// records of the server into the local database.
//
// The record is written on the server with the fields of the sync,
// and it has to be here afterwards.
func TestPullBringsTheRecordsBack(t *testing.T) {
	address := newTestServer(t)
	store := newTestStore(t)

	if _, err := store.EnsureSyncFields("wikidata_items"); err != nil {
		t.Fatalf("add the fields of the sync: %v", err)
	}

	// The schema and the record are put on the server by hand.
	client := NewClient(address, "admin@example.com", "secret1234")

	schema, err := store.LocalSchema([]string{"wikidata_items"})
	if err != nil {
		t.Fatalf("read the schema: %v", err)
	}
	if err := client.ImportCollections(context.Background(), schema); err != nil {
		t.Fatalf("push the schema: %v", err)
	}

	stamp := formatDateTime(time.Date(2022, 3, 4, 5, 6, 7, 0, time.UTC))
	remote := []RemoteRecord{{
		"id":               "abcdefghijklmno",
		"qid":              "Q1",
		"type":             "item",
		FieldSourceUpdated: stamp,
		FieldSourceOrigin:  OriginRemote,
	}}
	if _, err := client.UpsertRecords(context.Background(), "wikidata_items", remote); err != nil {
		t.Fatalf("push the record: %v", err)
	}

	service := NewService(store, func() (*Client, error) {
		return NewClient(address, "admin@example.com", "secret1234"), nil
	})
	service.SetSyncSet(SyncSet{Collections: []CollectionSpec{
		{Name: "wikidata_items", Direction: DirectionPull},
	}})

	if err := service.Run(context.Background()); err != nil {
		t.Fatalf("run the sync: %v", err)
	}
	waitForSync(t, service)

	// The record has to be here.
	local, err := store.App().FindRecordById("wikidata_items", "abcdefghijklmno")
	if err != nil {
		t.Fatalf("the record was not pulled: %v", err)
	}
	if qid := local.GetString("qid"); qid != "Q1" {
		t.Errorf("the local qid is %q, want Q1", qid)
	}

	// And the time of the change has to have travelled with it: it
	// is what the conflict resolution is made on.
	got := local.GetDateTime(FieldSourceUpdated).Time().UTC()
	want := time.Date(2022, 3, 4, 5, 6, 7, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("source_updated = %s, want %s", got, want)
	}
}

// TestRunIsIdempotent verifies that a second run of a sync does not
// duplicate anything.
//
// The records are upserted by their identifier, and the cursor moves
// forward, so a second run has nothing to do.
func TestRunIsIdempotent(t *testing.T) {
	address := newTestServer(t)
	store := newTestStore(t)

	if _, err := store.EnsureSyncFields("wikidata_items"); err != nil {
		t.Fatalf("add the fields of the sync: %v", err)
	}

	collection, err := store.App().FindCollectionByNameOrId("wikidata_items")
	if err != nil {
		t.Fatalf("find the collection: %v", err)
	}
	local := newTestRecord(t, store.App(), collection, "Q90")
	if err := store.SetSourceUpdated("wikidata_items", local, OriginLocal, time.Now()); err != nil {
		t.Fatalf("stamp the record: %v", err)
	}

	service := NewService(store, func() (*Client, error) {
		return NewClient(address, "admin@example.com", "secret1234"), nil
	})
	service.SetSyncSet(SyncSet{Collections: []CollectionSpec{
		{Name: "wikidata_items", Direction: DirectionPush, IncludeSchema: true},
	}})

	for range 2 {
		if err := service.Run(context.Background()); err != nil {
			t.Fatalf("run the sync: %v", err)
		}
		waitForSync(t, service)
	}

	client := NewClient(address, "admin@example.com", "secret1234")
	remote, err := client.ListRecords(context.Background(), "wikidata_items", time.Time{}, 10)
	if err != nil {
		t.Fatalf("read the remote records: %v", err)
	}
	if len(remote) != 1 {
		t.Errorf("the server holds %d records, want 1", len(remote))
	}
}

// TestReconcileKeepsTheNewerRecord verifies the reconcile.
//
// The record is newer on one side, and the newer one has to win.
func TestReconcileKeepsTheNewerRecord(t *testing.T) {
	address := newTestServer(t)
	store := newTestStore(t)

	if _, err := store.EnsureSyncFields("wikidata_items"); err != nil {
		t.Fatalf("add the fields of the sync: %v", err)
	}

	collection, err := store.App().FindCollectionByNameOrId("wikidata_items")
	if err != nil {
		t.Fatalf("find the collection: %v", err)
	}
	local := newTestRecord(t, store.App(), collection, "Q90")
	local.Set("type", "the local type")
	if err := store.SetSourceUpdated("wikidata_items", local, OriginLocal,
		time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("stamp the record: %v", err)
	}

	// The same record is newer on the server.
	client := NewClient(address, "admin@example.com", "secret1234")
	schema, err := store.LocalSchema([]string{"wikidata_items"})
	if err != nil {
		t.Fatalf("read the schema: %v", err)
	}
	if err := client.ImportCollections(context.Background(), schema); err != nil {
		t.Fatalf("push the schema: %v", err)
	}
	remote := []RemoteRecord{{
		"id":               local.Id,
		"qid":              "Q90",
		"type":             "the remote type",
		FieldSourceUpdated: formatDateTime(time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)),
		FieldSourceOrigin:  OriginRemote,
	}}
	if _, err := client.UpsertRecords(context.Background(), "wikidata_items", remote); err != nil {
		t.Fatalf("push the record: %v", err)
	}

	service := NewService(store, func() (*Client, error) {
		return NewClient(address, "admin@example.com", "secret1234"), nil
	})
	service.SetSyncSet(SyncSet{Collections: []CollectionSpec{
		{Name: "wikidata_items", Direction: DirectionReconcile},
	}})

	if err := service.Run(context.Background()); err != nil {
		t.Fatalf("run the sync: %v", err)
	}
	waitForSync(t, service)

	// The remote record was newer, so it is the one that is here.
	fresh, err := store.App().FindRecordById("wikidata_items", local.Id)
	if err != nil {
		t.Fatalf("read the record: %v", err)
	}
	if kind := fresh.GetString("type"); kind != "the remote type" {
		t.Errorf("the local type is %q, want the remote one", kind)
	}
}

// TestImportCollectionsDoesNotDeleteTheOtherCollections verifies that
// the schema of a sync leaves the other collections alone.
//
// The import of the collections API deletes every collection that is
// not in the list when it is asked to.
func TestImportCollectionsDoesNotDeleteTheOtherCollections(t *testing.T) {
	address := newTestServer(t)
	store := newTestStore(t)

	client := NewClient(address, "admin@example.com", "secret1234")

	schema, err := store.LocalSchema([]string{"wikidata_items", "wikidata_properties"})
	if err != nil {
		t.Fatalf("read the schema: %v", err)
	}
	if err := client.ImportCollections(context.Background(), schema); err != nil {
		t.Fatalf("push the schema: %v", err)
	}

	// A second import of one collection must not remove the other.
	one, err := store.LocalSchema([]string{"wikidata_items"})
	if err != nil {
		t.Fatalf("read the schema: %v", err)
	}
	if err := client.ImportCollections(context.Background(), one); err != nil {
		t.Fatalf("push the schema: %v", err)
	}

	collections, err := client.Collections(context.Background())
	if err != nil {
		t.Fatalf("list the remote collections: %v", err)
	}

	names := map[string]bool{}
	for _, collection := range collections {
		names[collection.Name()] = true
	}
	if !names["wikidata_properties"] {
		t.Error("the collection of the properties was deleted by the import")
	}
}

// TestTheBatchOfARunIsSmallEnough verifies that a batch fits in the
// limit of the server.
//
// The server refuses a batch that is too long, and the limit is
// configurable: a smaller batch works against a server that was
// never configured.
func TestTheBatchOfARunIsSmallEnough(t *testing.T) {
	if BatchSize > 50 {
		t.Errorf("the batch holds %d requests, want at most the fifty of the server", BatchSize)
	}
}

// TestTheClientTalksToTheRootOfTheServer verifies that the client
// builds the address of the API from the root of the server.
//
// The server may be behind a path, and the API is always at /api.
func TestTheClientTalksToTheRootOfTheServer(t *testing.T) {
	client := NewClient("https://ruis.example.com/", "admin@example.com", "secret")

	if got := client.BaseURL(); got != "https://ruis.example.com" {
		t.Errorf("the base url is %q", got)
	}
	if got := client.apiURL("/collections", nil); got != "https://ruis.example.com/api/collections" {
		t.Errorf("the api url is %q", got)
	}
}
