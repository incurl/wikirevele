package migrations_test

import (
	"testing"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

// TestTheHistoryHoldsSeveralTargets verifies that the history holds a
// run over several targets.
//
// A run may extract several entities at once, and the history records
// them: the identifiers and the output files are stored as lists, and
// the fields have to be wide enough for them.
func TestTheHistoryHoldsSeveralTargets(t *testing.T) {
	service := newTestService(t)
	app := service.App()

	collection, err := app.FindCollectionByNameOrId("etl_job_history")
	if err != nil {
		t.Fatalf("find the collection: %v", err)
	}

	// The fields must be wide enough for a list of targets.
	target := collection.Fields.GetByName("target_qid")
	if target == nil {
		t.Fatal("the target field is missing")
	}
	if text, ok := target.(*core.TextField); ok && text.Max < 100 {
		t.Errorf("the target field holds %d characters, want at least 100", text.Max)
	}

	items := collection.Fields.GetByName("out_items_path")
	if items == nil {
		t.Fatal("the items path field is missing")
	}
	if text, ok := items.(*core.TextField); ok && text.Max < 1000 {
		t.Errorf("the items path field holds %d characters, want at least 1000", text.Max)
	}

	// A run over several targets is recorded, and it reads back.
	record := core.NewRecord(collection)
	record.Set("full_command", "bzcat dump | wikirevele etl --target-qid Q90,Q148")
	record.Set("target_qid", "Q90,Q148")
	record.Set("out_items_path", "a.ndjson,b.ndjson")
	record.Set("out_props_path", "props.ndjson")
	record.Set("status", "succeeded")
	if err := app.Save(record); err != nil {
		t.Fatalf("save the record: %v", err)
	}

	var targets string
	err = app.DB().
		Select("target_qid").
		From("etl_job_history").
		Where(dbx.HashExp{"id": record.Id}).
		Row(&targets)
	if err != nil {
		t.Fatalf("read the record back: %v", err)
	}
	if targets != "Q90,Q148" {
		t.Errorf("the targets are %q, want Q90,Q148", targets)
	}
}
