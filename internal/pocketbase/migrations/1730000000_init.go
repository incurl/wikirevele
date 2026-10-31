// Package migrations contains the Go migrations that create and evolve
// the WikiRevele PocketBase schema.
//
// Migrations are registered with PocketBase's own migration list and are
// applied by Service.Bootstrap, so PocketBase's own migration tooling
// and bookkeeping are used rather than a custom mechanism.
//
// The file name is the migration's identity and its ordering key: the
// list is sorted by name, so a new migration must be named with a
// timestamp that sorts after the ones already applied.
package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/types"
)

// registerMigration is a small alias so the migration bodies stay
// readable.
func registerMigration(file string, up func(core.App) error) {
	core.AppMigrations.Register(up, func(core.App) error { return nil }, file)
}

// init registers the WikiRevele schema migration.
func init() {
	registerMigration("1730000000_init.go", func(app core.App) error {
		return createWikiReveleSchema(app)
	})
}

// createWikiReveleSchema creates the WikiRevele collections.
//
// The collections are created in dependency order so that relation
// fields can reference an existing collection id.
func createWikiReveleSchema(app core.App) error {
	// --- jobs -------------------------------------------------------
	// The generic job table. The application decides what a job means;
	// the scaffold only owns the lifecycle, the status and the
	// parameters.
	jobs := core.NewBaseCollection("jobs")
	jobs.Fields.Add(
		&core.TextField{Name: "title", Max: 200},
		&core.TextField{Name: "prompt"},
		&core.SelectField{
			Name:      "status",
			Values:    []string{"pending", "running", "completed", "failed", "cancelled"},
			MaxSelect: 1,
		},
		&core.JSONField{Name: "params"},
		&core.JSONField{Name: "result"},
		&core.TextField{Name: "error_message"},
		&core.NumberField{Name: "error_code"},
		&core.AutodateField{Name: "created", OnCreate: true},
		&core.AutodateField{Name: "updated", OnUpdate: true},
	)
	// The frontend may read the jobs, but every write goes through the
	// Go services.
	jobs.ListRule = types.Pointer("")
	jobs.ViewRule = types.Pointer("")
	if err := app.Save(jobs); err != nil {
		return err
	}

	// --- artifacts --------------------------------------------------
	// The files a job produced or consumed.
	artifacts := core.NewBaseCollection("artifacts")
	artifacts.Fields.Add(
		&core.RelationField{
			Name:         "job",
			CollectionId: jobs.Id,
			MaxSelect:    1,
		},
		&core.FileField{Name: "file", MaxSelect: 1, MaxSize: 25 << 20},
		&core.SelectField{
			Name:      "kind",
			Values:    []string{"input", "output"},
			MaxSelect: 1,
		},
		&core.TextField{Name: "name", Max: 200},
		&core.TextField{Name: "mime", Max: 100},
		&core.NumberField{Name: "bytes"},
		&core.NumberField{Name: "width"},
		&core.NumberField{Name: "height"},
		&core.JSONField{Name: "meta"},
		&core.AutodateField{Name: "created", OnCreate: true},
	)
	artifacts.ListRule = types.Pointer("")
	artifacts.ViewRule = types.Pointer("")
	if err := app.Save(artifacts); err != nil {
		return err
	}

	// --- app_settings -----------------------------------------------
	// Never exposed to the frontend: the API key is only ever read by
	// the Go backend.
	appSettings := core.NewBaseCollection("app_settings")
	appSettings.Fields.Add(
		&core.TextField{Name: "key", Required: true, Max: 100},
		&core.JSONField{Name: "value"},
		&core.BoolField{Name: "encrypted"},
		&core.AutodateField{Name: "updated", OnUpdate: true},
	)
	if err := app.Save(appSettings); err != nil {
		return err
	}

	return nil
}
