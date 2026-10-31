package migrations

import (
	"github.com/pocketbase/pocketbase/core"
)

// init registers the migration that widens the fields of the history
// that hold the targets of a run.
//
// The file name is the identity and the ordering key of the migration,
// so it sorts after the ones that are already applied.
func init() {
	registerMigration("1738000000_widen_etl_history_targets.go", func(app core.App) error {
		return widenEtlHistoryTargets(app)
	})
}

// widenEtlHistoryTargets lets the history hold the targets of a run
// over several targets.
//
// A run may extract several entities at once, and the history records
// them. The identifiers and the output files are stored as
// comma-separated lists, the way the command line carries them, so
// that the record of a run is the run itself.
//
// The fields were too narrow for the lists: "target_qid" held one
// identifier, and it now holds as many as a run has targets.
func widenEtlHistoryTargets(app core.App) error {
	collection, err := app.FindCollectionByNameOrId("etl_job_history")
	if err != nil {
		return err
	}

	changed := false

	if field := collection.Fields.GetByName("target_qid"); field != nil {
		if number, ok := field.(*core.TextField); ok && number.Max < MaxTargetQIDLength {
			number.Max = MaxTargetQIDLength
			changed = true
		}
	}

	if field := collection.Fields.GetByName("out_items_path"); field != nil {
		if text, ok := field.(*core.TextField); ok && text.Max < MaxItemsPathLength {
			text.Max = MaxItemsPathLength
			changed = true
		}
	}

	if !changed {
		return nil
	}

	return app.Save(collection)
}

// The sizes the widened fields take.
//
// They are generous: a run over many targets holds many identifiers,
// and the paths of the files are long.
const (
	// MaxTargetQIDLength is the length of the list of the targets of
	// a run.
	MaxTargetQIDLength = 500
	// MaxItemsPathLength is the length of the list of the item files
	// of a run.
	MaxItemsPathLength = 4000
)
