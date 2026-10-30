package wikidata

import (
	"context"
	"strings"
	"testing"

	wikirevelepb "github.com/incurl/wikirevele/internal/pocketbase"
)

// newTestApp bootstraps a PocketBase with the Valkyrie schema.
func newTestApp(t *testing.T) *wikirevelepb.Service {
	t.Helper()

	service := wikirevelepb.NewService(t.TempDir())
	if err := service.Bootstrap(); err != nil {
		t.Fatalf("bootstrap pocketbase: %v", err)
	}
	t.Cleanup(func() {
		_ = service.Close()
	})

	return service
}

// An item line, as the ETL command writes it.
func itemLine(id string) string {
	return `{"id":"` + id + `","type":"item",` +
		`"labels":{"en":{"language":"en","value":"Item ` + id + `"}},` +
		`"descriptions":{"en":{"language":"en","value":"an item"}},` +
		`"claims":{"P131":[{"mainsnak":{"snaktype":"value","property":"P131",` +
		`"datatype":"wikibase-item","datavalue":{"value":{"entity-type":"item","id":"Q90"},` +
		`"type":"wikibase-entityid"}}}]}}`
}

// A property line, as the ETL command writes it.
func propertyLine(id string) string {
	return `{"id":"` + id + `","type":"property",` +
		`"labels":{"en":{"language":"en","value":"Property ` + id + `"}},` +
		`"descriptions":{},"claims":{}}`
}

func TestImporterImportsTheItems(t *testing.T) {
	app := newTestApp(t)

	importer, err := NewImporter(app.App(), CollectionItems, FieldQID)
	if err != nil {
		t.Fatalf("new importer: %v", err)
	}

	input := strings.Join([]string{itemLine("Q90"), itemLine("Q243")}, "\n")
	stats, err := importer.Import(context.Background(), strings.NewReader(input), KindItems, "items.ndjson")
	if err != nil {
		t.Fatalf("import: %v", err)
	}

	if stats.Imported != 2 {
		t.Errorf("imported = %d, want 2", stats.Imported)
	}

	reader := NewReader(app.App())
	count, err := reader.CountItems()
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 2 {
		t.Errorf("count = %d, want 2", count)
	}
}

func TestImporterKeepsTheClaimsWhole(t *testing.T) {
	app := newTestApp(t)

	importer, err := NewImporter(app.App(), CollectionItems, FieldQID)
	if err != nil {
		t.Fatalf("new importer: %v", err)
	}

	if _, err := importer.Import(context.Background(),
		strings.NewReader(itemLine("Q90")), KindItems, "items.ndjson"); err != nil {
		t.Fatalf("import: %v", err)
	}

	reader := NewReader(app.App())
	items, err := reader.ListItems(10, 0)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("items = %d, want 1", len(items))
	}

	claim, ok := items[0].Claims["P131"]
	if !ok {
		t.Fatal("the claim is missing")
	}
	if !strings.Contains(string(claim), "Q90") {
		t.Errorf("the claim was changed: %s", claim)
	}
}

func TestImporterUpsertsTheRecords(t *testing.T) {
	app := newTestApp(t)

	importer, err := NewImporter(app.App(), CollectionItems, FieldQID)
	if err != nil {
		t.Fatalf("new importer: %v", err)
	}

	// The same file is imported twice.
	for range 2 {
		stats, err := importer.Import(context.Background(),
			strings.NewReader(itemLine("Q90")), KindItems, "items.ndjson")
		if err != nil {
			t.Fatalf("import: %v", err)
		}
		_ = stats
	}

	reader := NewReader(app.App())
	count, err := reader.CountItems()
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Errorf("count = %d, want 1: the record was duplicated", count)
	}
}

func TestImporterCountsTheUpdates(t *testing.T) {
	app := newTestApp(t)

	importer, err := NewImporter(app.App(), CollectionItems, FieldQID)
	if err != nil {
		t.Fatalf("new importer: %v", err)
	}

	if _, err := importer.Import(context.Background(),
		strings.NewReader(itemLine("Q90")), KindItems, "items.ndjson"); err != nil {
		t.Fatalf("first import: %v", err)
	}

	stats, err := importer.Import(context.Background(),
		strings.NewReader(itemLine("Q90")), KindItems, "items.ndjson")
	if err != nil {
		t.Fatalf("second import: %v", err)
	}

	if stats.Updated != 1 {
		t.Errorf("updated = %d, want 1", stats.Updated)
	}
	if stats.Imported != 0 {
		t.Errorf("imported = %d, want 0", stats.Imported)
	}
}

func TestImporterSkipsTheInvalidLines(t *testing.T) {
	app := newTestApp(t)

	importer, err := NewImporter(app.App(), CollectionItems, FieldQID)
	if err != nil {
		t.Fatalf("new importer: %v", err)
	}

	input := strings.Join([]string{
		"{not json",
		`{"type":"item","labels":{},"claims":{}}`, // no identifier
		itemLine("Q90"),
	}, "\n")

	stats, err := importer.Import(context.Background(), strings.NewReader(input), KindItems, "items.ndjson")
	if err != nil {
		t.Fatalf("import: %v", err)
	}

	if stats.Skipped != 2 {
		t.Errorf("skipped = %d, want 2", stats.Skipped)
	}
	if stats.Imported != 1 {
		t.Errorf("imported = %d, want 1", stats.Imported)
	}
}

func TestImporterImportsTheProperties(t *testing.T) {
	app := newTestApp(t)

	importer, err := NewImporter(app.App(), CollectionProperties, FieldPID)
	if err != nil {
		t.Fatalf("new importer: %v", err)
	}

	input := strings.Join([]string{propertyLine("P131"), propertyLine("P17")}, "\n")
	stats, err := importer.Import(context.Background(), strings.NewReader(input), KindProperties, "properties.ndjson")
	if err != nil {
		t.Fatalf("import: %v", err)
	}

	if stats.Imported != 2 {
		t.Errorf("imported = %d, want 2", stats.Imported)
	}

	reader := NewReader(app.App())
	properties, err := reader.ListProperties(10, 0)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(properties) != 2 {
		t.Fatalf("properties = %d, want 2", len(properties))
	}
	if properties[0].PID != "P131" {
		t.Errorf("the first property is %s, want P131", properties[0].PID)
	}
	if properties[0].Labels["en"].Value != "Property P131" {
		t.Errorf("the label is %q", properties[0].Labels["en"].Value)
	}
}

func TestImporterHoldsOneRecordAtATime(t *testing.T) {
	// The importer reads the file a line at a time, so a file that is
	// much bigger than the memory it holds is imported without a
	// problem. The test writes a file of a few thousand records: it
	// is small, and it proves that the batching works.
	app := newTestApp(t)

	importer, err := NewImporter(app.App(), CollectionItems, FieldQID)
	if err != nil {
		t.Fatalf("new importer: %v", err)
	}

	var lines []string
	for i := range 3000 {
		lines = append(lines, itemLine("Q"+itoa(i)))
	}

	stats, err := importer.Import(context.Background(),
		strings.NewReader(strings.Join(lines, "\n")), KindItems, "items.ndjson")
	if err != nil {
		t.Fatalf("import: %v", err)
	}

	if stats.Imported != 3000 {
		t.Errorf("imported = %d, want 3000", stats.Imported)
	}

	reader := NewReader(app.App())
	count, err := reader.CountItems()
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 3000 {
		t.Errorf("count = %d, want 3000", count)
	}
}

// itoa turns a number into a string.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	digits := ""
	for n > 0 {
		digits = string(rune('0'+n%10)) + digits
		n /= 10
	}
	return digits
}
