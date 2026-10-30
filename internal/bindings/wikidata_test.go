package bindings

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The line of an item, as the ETL command writes it.
const testItemLine = `{"id":"Q90","type":"item",` +
	`"labels":{"en":{"language":"en","value":"Paris"}},` +
	`"descriptions":{"en":{"language":"en","value":"The capital of France"}},` +
	`"claims":{"P131":[{"mainsnak":{"snaktype":"value","property":"P131",` +
	`"datatype":"wikibase-item","datavalue":{"value":{"entity-type":"item","id":"Q90"},` +
	`"type":"wikibase-entityid"}}}]}}`

// The line of a property, as the ETL command writes it.
const testPropertyLine = `{"id":"P131","type":"property",` +
	`"labels":{"en":{"language":"en","value":"located in the administrative territorial entity"}},` +
	`"descriptions":{},"claims":{}}`

// writeDataset writes a dataset of the given lines.
func writeDataset(t *testing.T, dir, name string, lines ...string) string {
	t.Helper()

	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
		t.Fatalf("write the dataset: %v", err)
	}

	return path
}

func TestImportWikidataFileFillsTheDatabase(t *testing.T) {
	service, _ := newTestService(t, okRunner())

	dir := t.TempDir()
	path := writeDataset(t, dir, "paris-wikidata-subset.ndjson", testItemLine)

	if err := service.ImportWikidataFile("items", path); err != nil {
		t.Fatalf("import: %v", err)
	}

	// The import runs in the background, so the test waits for it.
	waitForImport(t, service)

	progress := service.GetWikidataImportProgress()
	if !progress.Done {
		t.Fatalf("the import is not finished: %+v", progress)
	}
	if progress.Error != "" {
		t.Fatalf("the import failed: %s", progress.Error)
	}
	if progress.Imported != 1 {
		t.Errorf("imported = %d, want 1", progress.Imported)
	}
}

func TestGetWikidataItemsReadsTheImportedRecords(t *testing.T) {
	service, _ := newTestService(t, okRunner())

	dir := t.TempDir()
	writeDataset(t, dir, "paris-wikidata-subset.ndjson", testItemLine)
	writeDataset(t, dir, "wikidata-properties.ndjson", testPropertyLine)

	if err := service.ImportWikidataDirectory(dir); err != nil {
		t.Fatalf("import: %v", err)
	}
	waitForImport(t, service)

	view, err := service.GetWikidataItems(10, 0)
	if err != nil {
		t.Fatalf("get the items: %v", err)
	}

	if view.Total != 1 {
		t.Errorf("total = %d, want 1", view.Total)
	}
	if len(view.Items) != 1 {
		t.Fatalf("items = %d, want 1", len(view.Items))
	}

	item := view.Items[0]
	if item.QID != "Q90" {
		t.Errorf("qid = %q, want Q90", item.QID)
	}
	if item.Labels["en"] != "Paris" {
		t.Errorf("label = %q, want Paris", item.Labels["en"])
	}
	if item.Descriptions["en"] != "The capital of France" {
		t.Errorf("description = %q", item.Descriptions["en"])
	}

	// The claims are the statements of the dataset.
	claims, ok := item.Claims["P131"].([]any)
	if !ok {
		t.Fatalf("the claim is not a list: %T", item.Claims["P131"])
	}
	if len(claims) != 1 {
		t.Errorf("claims = %d, want 1", len(claims))
	}

	// The property dictionary gives the name of the property.
	if view.Properties["P131"] != "located in the administrative territorial entity" {
		t.Errorf("the property name is %q", view.Properties["P131"])
	}
}

func TestGetWikidataItemsIsEmptyWithoutADataset(t *testing.T) {
	service, _ := newTestService(t, okRunner())

	view, err := service.GetWikidataItems(10, 0)
	if err != nil {
		t.Fatalf("get the items: %v", err)
	}

	if view.Total != 0 {
		t.Errorf("total = %d, want 0", view.Total)
	}
	if len(view.Items) != 0 {
		t.Errorf("items = %d, want 0", len(view.Items))
	}
}

func TestImportWikidataFileRejectsAnUnknownKind(t *testing.T) {
	service, _ := newTestService(t, okRunner())

	dir := t.TempDir()
	path := writeDataset(t, dir, "paris-wikidata-subset.ndjson", testItemLine)

	if err := service.ImportWikidataFile("something", path); err == nil {
		t.Error("the import should have been refused")
	}
}

// waitForImport waits for the import that is running to finish.
func waitForImport(t *testing.T, service *AppService) {
	t.Helper()

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		progress := service.GetWikidataImportProgress()
		if !progress.Running && progress.Done {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf("the import did not finish: %+v", service.GetWikidataImportProgress())
}
