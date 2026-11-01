package sync

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestClientAuthenticates verifies that the client fetches a token
// and sends it with the requests.
func TestClientAuthenticates(t *testing.T) {
	var authenticated bool
	var authorized string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/auth-with-password") {
			authenticated = true
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"token":"the-token"}`))
			return
		}
		authorized = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[]}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "admin@example.com", "secret")
	if err := client.ping(context.Background()); err != nil {
		t.Fatalf("ping: %v", err)
	}

	if !authenticated {
		t.Error("the client did not authenticate")
	}
	if authorized != "the-token" {
		t.Errorf("the authorization header is %q, want the token", authorized)
	}
}

// TestClientRefusesBadCredentials verifies that a refused
// authentication is reported and not swallowed.
func TestClientRefusesBadCredentials(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"message":"Failed to authenticate."}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "admin@example.com", "wrong")
	err := client.ping(context.Background())
	if err == nil {
		t.Fatal("the bad credentials were accepted")
	}
	if !strings.Contains(err.Error(), "refused") {
		t.Errorf("the error is %q, want it to say the credentials were refused", err)
	}
}

// TestClientNeedsEverythingToRun verifies that a client without an
// address or without credentials says what is missing.
//
// The panel shows the message, so the reader knows what to fill in.
func TestClientNeedsEverythingToRun(t *testing.T) {
	cases := []struct {
		name     string
		client   *Client
		wantWord string
	}{
		{"no address", NewClient("", "admin@example.com", "secret"), "address"},
		{"no identity", NewClient("https://example.com", "", "secret"), "identity"},
		{"no password", NewClient("https://example.com", "admin@example.com", ""), "password"},
	}

	for _, test := range cases {
		err := test.client.Configured()
		if err == nil {
			t.Errorf("%s: the client was accepted", test.name)
			continue
		}
		if !strings.Contains(err.Error(), test.wantWord) {
			t.Errorf("%s: the error is %q, want it to name the %s", test.name, err, test.wantWord)
		}
	}
}

// TestUpsertRecordsUsesTheBatchAPI verifies that the records are
// written with the batch API, which is what upserts them.
func TestUpsertRecordsUsesTheBatchAPI(t *testing.T) {
	var received struct {
		Requests []struct {
			Method string         `json:"method"`
			URL    string         `json:"url"`
			Body   map[string]any `json:"body"`
		} `json:"requests"`
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/batch" {
			if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
				t.Errorf("decode the batch: %v", err)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[{"status":200,"body":{}}]`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"token":"the-token"}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "admin@example.com", "secret")
	client.SetHTTPClient(server.Client())

	records := []RemoteRecord{
		{"id": "abcdefghijklmno", "title": "one"},
		{"id": "ponmlkjihgfedcba", "title": "two"},
	}

	written, err := client.UpsertRecords(context.Background(), "notes", records)
	if err != nil {
		t.Fatalf("upsert the records: %v", err)
	}
	if written != 2 {
		t.Errorf("written %d records, want 2", written)
	}

	if len(received.Requests) != 2 {
		t.Fatalf("the batch held %d requests, want 2", len(received.Requests))
	}
	for _, request := range received.Requests {
		if request.Method != http.MethodPut {
			t.Errorf("the method is %q, want PUT", request.Method)
		}
		if request.URL != "/api/collections/notes/records" {
			t.Errorf("the url is %q", request.URL)
		}
	}
}

// TestUpsertRecordsReportsADisabledBatch verifies that the error of
// a server without the batch API says so.
//
// The batch API is disabled by default, and the error of the server
// does not say why the batch was refused.
func TestUpsertRecordsReportsADisabledBatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/batch" {
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"message":"Batch requests are not allowed."}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"token":"the-token"}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "admin@example.com", "secret")
	client.SetHTTPClient(server.Client())

	_, err := client.UpsertRecords(context.Background(), "notes",
		[]RemoteRecord{{"id": "abcdefghijklmno"}})
	if err == nil {
		t.Fatal("a refused batch was not reported")
	}
	if !strings.Contains(err.Error(), "batch API is not enabled") {
		t.Errorf("the error is %q, want it to name the batch API", err)
	}
}

// TestListRecordsFiltersOnTheSyncField verifies that the records are
// read with the filter on the field of the sync.
//
// The "updated" field of the server is overwritten on every write,
// and it cannot be the cursor of a run.
func TestListRecordsFiltersOnTheSyncField(t *testing.T) {
	var filter string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/collections/notes/records" {
			filter = r.URL.Query().Get("filter")
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"items":[]}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"token":"the-token"}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "admin@example.com", "secret")
	client.SetHTTPClient(server.Client())

	since := time.Date(2021, 6, 7, 8, 9, 10, 0, time.UTC)
	if _, err := client.ListRecords(context.Background(), "notes", since, 10); err != nil {
		t.Fatalf("list the records: %v", err)
	}

	if !strings.Contains(filter, FieldSourceUpdated) {
		t.Errorf("the filter is %q, want it on %q", filter, FieldSourceUpdated)
	}
}

// TestImportCollectionsKeepsTheOtherCollections verifies that the
// schema import does not delete the collections that are not in it.
//
// The import of the collections API deletes every collection that is
// not in the list when it is asked to, and a sync of two collections
// must not delete the third one.
func TestImportCollectionsKeepsTheOtherCollections(t *testing.T) {
	var deleteMissing bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/collections/import" {
			var payload struct {
				DeleteMissing bool `json:"deleteMissing"`
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Errorf("decode the import: %v", err)
			}
			deleteMissing = payload.DeleteMissing
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"token":"the-token"}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "admin@example.com", "secret")
	client.SetHTTPClient(server.Client())

	err := client.ImportCollections(context.Background(), []RemoteCollection{
		{"id": "abc", "name": "notes", "type": "base"},
	})
	if err != nil {
		t.Fatalf("import the collections: %v", err)
	}

	if deleteMissing {
		t.Error("the import was asked to delete the collections that are not in it")
	}
}

// TestParseDateTimeReadsTheFormatOfTheServer verifies that the dates
// of the server are read.
//
// The format is the one of the default layout of the server, and the
// comparison of the conflicts is made on it.
func TestParseDateTimeReadsTheFormatOfTheServer(t *testing.T) {
	want := time.Date(2021, 6, 7, 8, 9, 10, 0, time.UTC)

	got := parseDateTime("2021-06-07 08:09:10.000Z")
	if !got.Equal(want) {
		t.Errorf("parsed %s, want %s", got, want)
	}

	if !parseDateTime(nil).IsZero() {
		t.Error("a missing date was not the zero time")
	}
	if !parseDateTime("not a date").IsZero() {
		t.Error("a damaged date was not the zero time")
	}
}

// TestFormatDateTimeSortsLikeATime verifies that the format of the
// server sorts the same way the times sort.
//
// The filter of the server is a comparison of strings, and the
// comparison has to be the comparison of the times.
func TestFormatDateTimeSortsLikeATime(t *testing.T) {
	earlier := time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)
	later := time.Date(2021, 12, 31, 23, 59, 59, 0, time.UTC)

	if formatDateTime(earlier) >= formatDateTime(later) {
		t.Errorf("%q is not before %q", formatDateTime(earlier), formatDateTime(later))
	}
}
