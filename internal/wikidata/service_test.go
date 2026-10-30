package wikidata

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	wikirevelepb "github.com/incurl/wikirevele/internal/pocketbase"
)

// newTestService bootstraps a PocketBase and a Wikidata service.
func newTestService(t *testing.T) *Service {
	t.Helper()

	service := wikirevelepb.NewService(t.TempDir())
	if err := service.Bootstrap(); err != nil {
		t.Fatalf("bootstrap pocketbase: %v", err)
	}
	t.Cleanup(func() {
		_ = service.Close()
	})

	return NewService(service.App())
}

// writeDataset writes a dataset of items to a file.
func writeDataset(t *testing.T, dir, name string, count int) string {
	t.Helper()

	path := filepath.Join(dir, name)
	content := ""
	for i := range count {
		content += `{"id":"Q` + itoa(i) + `","type":"item","labels":{"en":{"language":"en","value":"Item ` + itoa(i) + `"}},"claims":{}}` + "\n"
	}

	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write the dataset: %v", err)
	}

	return path
}

func TestServiceImportsAFile(t *testing.T) {
	service := newTestService(t)
	dir := t.TempDir()
	path := writeDataset(t, dir, ItemsFileName, 5)

	if err := service.ImportFile(KindItems, path); err != nil {
		t.Fatalf("import: %v", err)
	}

	// The import runs in the background, so the test waits for it.
	waitForDone(t, service)

	progress := service.Progress()
	if !progress.Done {
		t.Errorf("the import is not finished: %+v", progress)
	}
	if progress.Error != "" {
		t.Errorf("the import failed: %s", progress.Error)
	}
	if progress.Imported != 5 {
		t.Errorf("imported = %d, want 5", progress.Imported)
	}

	count, err := service.CountItems()
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 5 {
		t.Errorf("count = %d, want 5", count)
	}
}

func TestServiceImportsADirectory(t *testing.T) {
	service := newTestService(t)
	dir := t.TempDir()
	writeDataset(t, dir, ItemsFileName, 3)
	writeDataset(t, dir, PropertiesFileName, 2)

	if err := service.ImportDirectory(dir); err != nil {
		t.Fatalf("import: %v", err)
	}

	waitForDone(t, service)

	items, err := service.CountItems()
	if err != nil {
		t.Fatalf("count the items: %v", err)
	}
	if items != 3 {
		t.Errorf("items = %d, want 3", items)
	}

	properties, err := service.CountProperties()
	if err != nil {
		t.Fatalf("count the properties: %v", err)
	}
	if properties != 2 {
		t.Errorf("properties = %d, want 2", properties)
	}
}

func TestServiceRejectsAnUnknownKind(t *testing.T) {
	service := newTestService(t)
	dir := t.TempDir()
	path := writeDataset(t, dir, ItemsFileName, 1)

	if err := service.ImportFile("something", path); err == nil {
		t.Error("the import should have been refused")
	}
}

func TestServiceReportsAMissingFile(t *testing.T) {
	service := newTestService(t)

	if err := service.ImportFile(KindItems, filepath.Join(t.TempDir(), "nothing.ndjson")); err == nil {
		t.Error("the import should have failed")
	}
}

func TestServiceStartsFinished(t *testing.T) {
	service := newTestService(t)

	// No import has run, so there is nothing to wait for.
	if !service.Progress().Done {
		t.Error("the progress of a service that never imported should be finished")
	}
}

// waitForDone waits for the imports that are running to finish.
//
// A directory import starts two of them, so the wait is for the count
// of the running imports and not for the progress of one.
func waitForDone(t *testing.T, service *Service) {
	t.Helper()

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		progress := service.Progress()
		if !progress.Running && progress.Done {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf("the import did not finish: %+v", service.Progress())
}
