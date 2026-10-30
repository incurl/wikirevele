package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/types"
)

// init registers the migration that creates the collection the ETL
// runs are recorded in.
//
// The file name is the identity and the ordering key of the migration,
// so it sorts after the two that are already applied.
func init() {
	registerMigration("1734000000_add_etl_job_history.go", func(app core.App) error {
		return createEtlJobHistorySchema(app)
	})
}

// createEtlJobHistorySchema creates the collection of the ETL history.
//
// The collection holds one record per run of the extraction worker.
// The record is the command that was run, the parameters it was run
// with, and what came of it.
//
// The command is the important part: it is the whole pipeline, and a
// user can copy it out of the history and run it again in a terminal.
// The history is therefore a list of commands that worked, and not
// only a log.
func createEtlJobHistorySchema(app core.App) error {
	// The collection is already there when the migration ran before.
	if _, err := app.FindCollectionByNameOrId("etl_job_history"); err == nil {
		return nil
	}

	history := core.NewBaseCollection("etl_job_history")
	history.Fields.Add(
		// The whole shell pipeline, as it is shown to the user and
		// as it is run. It is required: a record without it cannot
		// be copied and run again.
		&core.TextField{Name: "full_command", Required: true, Max: 4000},
		// The entity the items were linked to.
		&core.TextField{Name: "target_qid", Max: 32},
		// Whether the administrative hierarchy was walked.
		&core.BoolField{Name: "recursive_p131"},
		// The languages that were kept.
		&core.TextField{Name: "langs", Max: 200},
		// The files of the run.
		&core.TextField{Name: "dump_file_path", Max: 1000},
		&core.TextField{Name: "out_items_path", Max: 1000},
		&core.TextField{Name: "out_props_path", Max: 1000},
		// When the run started and when it ended.
		&core.DateField{Name: "started_at"},
		&core.DateField{Name: "finished_at"},
		// Where the run is.
		&core.SelectField{
			Name:      "status",
			Values:    []string{"running", "succeeded", "cancelled", "failed"},
			MaxSelect: 1,
		},
		// Why the run failed, when it did.
		&core.TextField{Name: "error_message", Max: 2000},
		// A remark of the user.
		&core.TextField{Name: "notes", Max: 2000},
		// The order of the runs.
		//
		// The date fields have a resolution of a second, and two
		// runs started in the same second would be in an
		// arbitrary order. The sequence is a number that grows with
		// every run, and it is what the history is sorted on.
		&core.NumberField{Name: "sequence"},
		&core.AutodateField{Name: "created", OnCreate: true},
		&core.AutodateField{Name: "updated", OnUpdate: true},
	)

	// The runs are read by the frontend, and every write goes through
	// the Go services: the history is the record of what the
	// application did, and nothing else may change it.
	history.ListRule = types.Pointer("")
	history.ViewRule = types.Pointer("")

	return app.Save(history)
}
