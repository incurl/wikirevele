package pocketbase

import (
	"testing"
)

// newTestStore creates a settings store backed by a throwaway database.
func newTestStore(t *testing.T) *SettingsStore {
	t.Helper()

	service := NewService(t.TempDir())
	if err := service.Bootstrap(); err != nil {
		t.Fatalf("bootstrap pocketbase: %v", err)
	}
	t.Cleanup(func() {
		if err := service.Close(); err != nil {
			t.Errorf("close pocketbase: %v", err)
		}
	})

	cipher, err := NewCipher(service.DataDir())
	if err != nil {
		t.Fatalf("create cipher: %v", err)
	}

	return NewSettingsStore(service.App(), cipher)
}

// TestSchemaIsCreated verifies that the migrations create the expected
// collections.
func TestSchemaIsCreated(t *testing.T) {
	service := NewService(t.TempDir())
	if err := service.Bootstrap(); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	t.Cleanup(func() {
		_ = service.Close()
	})

	// The names are spelled out here on purpose: this test asserts what
	// the migration created, so it must not follow a constant that could
	// be renamed on the other side. The jobs package names the same
	// collections for the code that uses them.
	for _, name := range []string{"jobs", "artifacts", CollectionAppSettings} {
		if _, err := service.Collection(name); err != nil {
			t.Errorf("collection %q: %v", name, err)
		}
	}
}

// TestUIPreferencesRoundTrip verifies that the UI preferences survive a
// write and a read.
func TestUIPreferencesRoundTrip(t *testing.T) {
	store := newTestStore(t)

	// The default is returned before anything is written.
	ui, err := store.GetUI()
	if err != nil {
		t.Fatalf("get ui: %v", err)
	}
	if ui.Zoom != 100 || ui.Theme != "dark" {
		t.Errorf("default ui = %+v", ui)
	}

	if err := store.SetUI(UI{Zoom: 150, Theme: "light"}); err != nil {
		t.Fatalf("set ui: %v", err)
	}

	ui, err = store.GetUI()
	if err != nil {
		t.Fatalf("get ui: %v", err)
	}
	if ui.Zoom != 150 || ui.Theme != "light" {
		t.Errorf("ui = %+v, want zoom 150 and the light theme", ui)
	}
}

// TestAPIKeyIsEncrypted verifies that the API key is stored encrypted
// and read back correctly.
func TestAPIKeyIsEncrypted(t *testing.T) {
	service := NewService(t.TempDir())
	if err := service.Bootstrap(); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	t.Cleanup(func() {
		_ = service.Close()
	})

	cipher, err := NewCipher(service.DataDir())
	if err != nil {
		t.Fatalf("create cipher: %v", err)
	}
	store := NewSettingsStore(service.App(), cipher)

	if err := store.SetAPIKey("super-secret"); err != nil {
		t.Fatalf("set api key: %v", err)
	}

	// The stored record must not contain the plain key.
	record := store.findRecord(SettingAPIKey)
	if record == nil {
		t.Fatal("the setting was not stored")
	}
	if !record.GetBool("encrypted") {
		t.Error("the setting is not marked as encrypted")
	}
	if got := record.GetString("value"); got == "super-secret" {
		t.Error("the API key is stored in plain text")
	}

	key, found, err := store.GetAPIKey()
	if err != nil {
		t.Fatalf("get api key: %v", err)
	}
	if !found {
		t.Fatal("the API key was not found")
	}
	if key != "super-secret" {
		t.Errorf("api key = %q, want it decrypted back", key)
	}
}

// TestBaseURLFallsBackToTheDefault verifies the fallback of an unset
// base URL.
func TestBaseURLFallsBackToTheDefault(t *testing.T) {
	store := newTestStore(t)

	baseURL, err := store.GetBaseURL()
	if err != nil {
		t.Fatalf("get base url: %v", err)
	}
	if baseURL != DefaultBaseURL {
		t.Errorf("base url = %q, want %q", baseURL, DefaultBaseURL)
	}

	if err := store.SetBaseURL("https://example.test"); err != nil {
		t.Fatalf("set base url: %v", err)
	}

	baseURL, err = store.GetBaseURL()
	if err != nil {
		t.Fatalf("get base url: %v", err)
	}
	if baseURL != "https://example.test" {
		t.Errorf("base url = %q", baseURL)
	}
}
