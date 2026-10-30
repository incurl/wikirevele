package bindings

import (
	"reflect"
	"sort"
	"testing"
)

// expectedMethods is the exact set of methods the frontend may call.
//
// Every exported method on a bound service becomes a method in the
// generated TypeScript, so this list is the public API of the
// application. It is spelled out on purpose: adding a method is a
// deliberate act, and a method that returns an internal type is a
// mistake this test catches.
var expectedMethods = []string{
	"BuildEtlCommand",
	"CancelEtlJob",
	"CancelJob",
	"CancelWikidataImport",
	"ClearConsoleEntries",
	"DeleteArtifact",
	"DeleteEtlHistoryEntry",
	"DeleteJob",
	"GetSettings",
	"GetWikidataDataDir",
	"GetWikidataImportProgress",
	"GetWikidataItems",
	"ImportWikidataDirectory",
	"ImportWikidataFile",
	"ListArtifacts",
	"ListConsoleEntries",
	"ListEtlHistory",
	"ListJobs",
	"RetryJob",
	"SaveCredentials",
	"SaveUI",
	"StartEtlJob",
	"SubmitJob",
}

// TestAppServiceExposesExactlyTheIntendedMethods pins the bound surface.
//
// This is the regression test for the finding of the 2026-10-28 code
// review: JobManager, Console and Settings were bound as methods and
// returned *jobs.Manager, *console.Log and *pocketbase.SettingsStore.
// Those types hold unexported fields, so they serialise to an empty
// object, and the generated bindings offered three methods that could
// never work. Nothing called them, and nothing noticed.
func TestAppServiceExposesExactlyTheIntendedMethods(t *testing.T) {
	typ := reflect.TypeOf(&AppService{})

	var actual []string
	for i := 0; i < typ.NumMethod(); i++ {
		actual = append(actual, typ.Method(i).Name)
	}
	sort.Strings(actual)

	if len(actual) != len(expectedMethods) {
		t.Errorf("AppService has %d methods, want %d\n got: %v\nwant: %v",
			len(actual), len(expectedMethods), actual, expectedMethods)
		return
	}

	for i, name := range actual {
		if name != expectedMethods[i] {
			t.Errorf("method %d is %q, want %q\n got: %v\nwant: %v",
				i, name, expectedMethods[i], actual, expectedMethods)
			return
		}
	}
}

// TestNoBoundMethodReturnsAnInternalType verifies that no bound method
// hands back a type from an internal package.
//
// A returned value is serialised, so a type with only unexported fields
// arrives as an empty object. The frontend can do nothing with it.
func TestNoBoundMethodReturnsAnInternalType(t *testing.T) {
	typ := reflect.TypeOf(&AppService{})

	// The value types the frontend genuinely needs.
	allowed := map[string]bool{
		// A plain string survives the serialisation: it is a path
		// or a name, and the frontend can use it as it is.
		"string": true,

		"bindings.SettingsView": true,
		"jobs.Artifact":         true,
		"jobs.Job":              true,
		"jobs.Request":          true,
		"console.Entry":         true,
		"pocketbase.UI":         true,

		// The view of the imported items. The type is declared here
		// rather than in the wikidata package, because a bound method
		// must not return the type of an internal package.
		"bindings.WikidataItemsView": true,

		// The progress of an import is a struct of plain values, so it
		// survives the serialisation.
		"wikidata.ImportProgress": true,

		// The views of the ETL pipeline. They are declared in this
		// package, like the views of the items, because a bound
		// method must not return the type of an internal package.
		"bindings.EtlJobOptions":   true,
		"bindings.EtlProgress":     true,
		"bindings.EtlHistoryEntry": true,
	}

	for i := 0; i < typ.NumMethod(); i++ {
		method := typ.Method(i)
		results := method.Type.NumOut()

		// Skip the error result; check the value result.
		for r := 0; r < results; r++ {
			out := method.Type.Out(r)
			if out.Kind() == reflect.Interface {
				continue // the error interface
			}

			// A pointer or a slice of a value type is fine: the
			// frontend receives the serialised element.
			for out.Kind() == reflect.Pointer || out.Kind() == reflect.Slice {
				out = out.Elem()
			}

			name := out.String()
			if !allowed[name] {
				t.Errorf("%s returns %s, which the frontend cannot use", method.Name, name)
			}
		}
	}
}
