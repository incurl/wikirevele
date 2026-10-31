package migrations_test

import (
	"testing"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/types"

	"github.com/incurl/wikirevele/internal/pocketbase"
)

// newTestService creates a service backed by a throwaway database, with
// the WikiRevele schema applied.
func newTestService(t *testing.T) *pocketbase.Service {
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

	return service
}

// tableColumns returns the names of the columns of a table.
func tableColumns(t *testing.T, app core.App, table string) []string {
	t.Helper()

	var rows []struct {
		Name string `db:"name"`
	}
	if err := app.DB().NewQuery("SELECT name FROM pragma_table_info('" + table + "')").All(&rows); err != nil {
		t.Fatalf("read the columns of %q: %v", table, err)
	}

	names := make([]string, 0, len(rows))
	for _, row := range rows {
		names = append(names, row.Name)
	}

	return names
}

// TestTheSchemaHasNoArtifactDimensions verifies that the artifacts
// collection carries no pixel dimensions.
//
// The columns held the dimensions of an image artifact, which was the
// one piece of domain knowledge inside the generic job queue. Nothing
// ever read them.
func TestTheSchemaHasNoArtifactDimensions(t *testing.T) {
	service := newTestService(t)

	collection, err := service.Collection("artifacts")
	if err != nil {
		t.Fatalf("find the artifacts collection: %v", err)
	}

	for _, name := range []string{"width", "height"} {
		if collection.Fields.GetByName(name) != nil {
			t.Errorf("the artifacts collection still has the %q field", name)
		}
	}
}

// TestTheMigrationDropsTheDimensionsOfAnExistingDatabase verifies that
// the migration removes the columns of a database that was created
// before it existed.
//
// The migration is the only thing that can remove them: an applied
// migration is never edited, so the columns of an existing installation
// are dropped by this one and not by the init migration.
func TestTheMigrationDropsTheDimensionsOfAnExistingDatabase(t *testing.T) {
	service := newTestService(t)
	app := service.App()

	// Put the columns back, the way the init migration left them.
	collection, err := app.FindCollectionByNameOrId("artifacts")
	if err != nil {
		t.Fatalf("find the artifacts collection: %v", err)
	}
	collection.Fields.Add(&core.NumberField{Name: "width"})
	collection.Fields.Add(&core.NumberField{Name: "height"})
	collection.ListRule = types.Pointer("")
	if err := app.Save(collection); err != nil {
		t.Fatalf("save the artifacts collection: %v", err)
	}

	if !contains(tableColumns(t, app, "artifacts"), "width") {
		t.Fatal("the width column was not created")
	}

	// Forget that the migration ran, so that it runs again the way it
	// would on a database that was created before it existed.
	if _, err := app.DB().Delete("_migrations", dbx.HashExp{
		"file": "1736000000_drop_artifact_dimensions.go",
	}).Execute(); err != nil {
		t.Fatalf("forget the applied migration: %v", err)
	}

	// Run the migration the way PocketBase would.
	if err := app.RunAppMigrations(); err != nil {
		t.Fatalf("run the migrations: %v", err)
	}

	collection, err = app.FindCollectionByNameOrId("artifacts")
	if err != nil {
		t.Fatalf("find the artifacts collection: %v", err)
	}
	for _, name := range []string{"width", "height"} {
		if collection.Fields.GetByName(name) != nil {
			t.Errorf("the artifacts collection still has the %q field", name)
		}
	}

	for _, name := range tableColumns(t, app, "artifacts") {
		if name == "width" || name == "height" {
			t.Errorf("the column %q is still in the table", name)
		}
	}
}

// contains reports whether a list holds a value.
func contains(values []string, value string) bool {
	for _, v := range values {
		if v == value {
			return true
		}
	}
	return false
}
