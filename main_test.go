package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestCombineHandlersRoutesAPIToPocketBase verifies that /api/* requests
// are served by the PocketBase handler and everything else by the static
// frontend handler.
//
// This is the wiring that lets the stored files be served from the same
// origin as the UI without opening a second port.
func TestCombineHandlersRoutesAPIToPocketBase(t *testing.T) {
	apiCalls := 0
	staticCalls := 0

	apiHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiCalls++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	})

	staticHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		staticCalls++
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<!doctype html><html></html>"))
	})

	combined := combineHandlers(apiHandler, staticHandler)

	// An API request must reach PocketBase.
	rec := httptest.NewRecorder()
	combined.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/collections/jobs", nil))
	if apiCalls != 1 {
		t.Errorf("api handler calls = %d, want 1", apiCalls)
	}
	if staticCalls != 0 {
		t.Errorf("static handler calls = %d, want 0", staticCalls)
	}
	if !strings.Contains(rec.Body.String(), "ok") {
		t.Errorf("body = %q", rec.Body.String())
	}

	// The file route must also reach PocketBase, because that is how
	// the stored artifacts are served.
	rec = httptest.NewRecorder()
	combined.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/files/artifacts/abc/img.png", nil))
	if apiCalls != 2 {
		t.Errorf("api handler calls = %d, want 2", apiCalls)
	}

	// Everything else must reach the static frontend.
	rec = httptest.NewRecorder()
	combined.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if staticCalls != 1 {
		t.Errorf("static handler calls = %d, want 1", staticCalls)
	}
	if !strings.Contains(rec.Body.String(), "<html>") {
		t.Errorf("body = %q", rec.Body.String())
	}

	// Nested asset paths must not be mistaken for API calls.
	rec = httptest.NewRecorder()
	combined.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/assets/index.js", nil))
	if staticCalls != 2 {
		t.Errorf("static handler calls = %d, want 2", staticCalls)
	}
}

// TestSPAHandlerServesTheIndexForTheRoutes verifies that a path that
// no file matches is answered with the index, so that the routes of
// the frontend work in server mode.
//
// A deep link such as /wikidata/Q90 is a route of the frontend and not
// a file. Without the index, it is a 404, and a reload of the page
// loses it.
func TestSPAHandlerServesTheIndexForTheRoutes(t *testing.T) {
	index := []byte("<!doctype html><html><body>index</body></html>")

	staticHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write(index)
		case "/assets/app.js":
			w.Header().Set("Content-Type", "application/javascript")
			_, _ = w.Write([]byte("console.log('app')"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	handler := spaHandler(staticHandler, index)

	// A route of the frontend is answered with the index.
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/wikidata/Q90", nil))
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if !strings.Contains(rec.Body.String(), "index") {
		t.Errorf("body = %q", rec.Body.String())
	}

	// The index itself is served as it is.
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	// A file is served as it is, with its own content type.
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/assets/app.js", nil))
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/javascript" {
		t.Errorf("content type = %q, want %q", got, "application/javascript")
	}
	if !strings.Contains(rec.Body.String(), "console.log") {
		t.Errorf("body = %q", rec.Body.String())
	}

	// A missing asset stays a 404: a silent index would hide a broken
	// build and would answer a script request with HTML.
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/assets/missing.js", nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

// TestDataDirHonoursTheEnvironmentVariable verifies that the data
// directory can be relocated, so that the app can run against an
// isolated database during development and testing.
func TestDataDirHonoursTheEnvironmentVariable(t *testing.T) {
	t.Setenv("VALKYRIE_DATA_DIR", "/tmp/valkyrie-test-data")

	if got := dataDir(); got != "/tmp/valkyrie-test-data" {
		t.Errorf("data dir = %q, want the overridden one", got)
	}
}
