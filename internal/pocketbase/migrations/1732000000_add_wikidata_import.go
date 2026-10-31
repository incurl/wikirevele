package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/types"
)

// init registers the migration that creates the collections the
// Wikidata importer fills.
//
// The file name is the identity and the ordering key of the migration,
// so it sorts after 1730000000_init.go and the collections are created
// once the base schema exists.
func init() {
	registerMigration("1732000000_add_wikidata_import.go", func(app core.App) error {
		return createWikidataImportSchema(app)
	})
}

// createWikidataImportSchema creates the collections of the imported
// Wikidata datasets.
//
// The two collections hold what the ETL worker produced: the items
// that are linked to Paris, and the property dictionary. They are
// written by the importer only, so they are not exposed to the
// frontend: the list rule and the view rule are left empty, which
// means that only the Go backend can read them.
func createWikidataImportSchema(app core.App) error {
	// --- wikidata_items ---------------------------------------------
	// One record per item of the trimmed dataset.
	//
	// The claims are stored as JSON, whole: the panel reads the
	// qualifiers and the references of a statement, and a schema that
	// flattened them would lose that.
	items := core.NewBaseCollection("wikidata_items")
	items.Fields.Add(
		&core.TextField{Name: "qid", Required: true, Max: 32},
		&core.TextField{Name: "type", Max: 32},
		&core.JSONField{Name: "labels"},
		&core.JSONField{Name: "descriptions"},
		&core.JSONField{Name: "aliases"},
		&core.JSONField{Name: "claims"},
		&core.AutodateField{Name: "imported_at", OnCreate: true},
	)
	// The identifier is the key the records are upserted by, so it has
	// to be unique.
	items.AddIndex("idx_wikidata_items_qid", true, "qid", "")
	items.ListRule = types.Pointer("")
	items.ViewRule = types.Pointer("")
	if err := app.Save(items); err != nil {
		return err
	}

	// --- wikidata_properties ---------------------------------------
	// One record per property of the dictionary.
	properties := core.NewBaseCollection("wikidata_properties")
	properties.Fields.Add(
		&core.TextField{Name: "pid", Required: true, Max: 32},
		&core.TextField{Name: "type", Max: 32},
		&core.JSONField{Name: "labels"},
		&core.JSONField{Name: "descriptions"},
		&core.JSONField{Name: "aliases"},
		&core.JSONField{Name: "claims"},
		&core.AutodateField{Name: "imported_at", OnCreate: true},
	)
	properties.AddIndex("idx_wikidata_properties_pid", true, "pid", "")
	properties.ListRule = types.Pointer("")
	properties.ViewRule = types.Pointer("")
	if err := app.Save(properties); err != nil {
		return err
	}

	return nil
}
