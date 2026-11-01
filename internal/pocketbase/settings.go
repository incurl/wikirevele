package pocketbase

import (
	"encoding/json"
	"fmt"

	"github.com/pocketbase/pocketbase/core"
)

// Setting keys used by WikiRevele.
//
// The set is a generic one. New keys are expected to keep the
// "app.<name>" convention.
const (
	// SettingUI holds the user interface preferences.
	SettingUI = "ui"
	// SettingAPIKey holds an encrypted secret, for example the
	// credential of the remote API the application talks to.
	SettingAPIKey = "app.api_key"
	// SettingBaseURL holds the base URL of the remote API.
	SettingBaseURL = "app.base_url"
	// SettingSyncEmail holds the identity of the superuser of the
	// remote server.
	SettingSyncEmail = "app.sync_email"
	// SettingSyncPassword holds the password of the superuser of the
	// remote server.
	//
	// It is a secret and it is stored encrypted, like the API key.
	SettingSyncPassword = "app.sync_password"
)

// UI holds the user interface preferences.
//
// The zoom factor scales the whole interface, because the application
// runs in a webview whose base font size the user cannot change.
type UI struct {
	// Zoom is the interface scale, as a percentage of the base size.
	// It accepts 50 to 200.
	Zoom int `json:"zoom"`
	// Theme is the interface colour scheme: "dark" or "light".
	Theme string `json:"theme"`
}

// DefaultUI returns the factory UI preferences.
func DefaultUI() UI {
	return UI{
		Zoom:  100,
		Theme: "dark",
	}
}

// SettingsStore reads and writes the app_settings collection.
//
// Secrets are stored encrypted and are never exposed to the frontend:
// the app_settings collection has no public list or view rule, so the
// Go backend is the only reader.
type SettingsStore struct {
	app    core.App
	cipher *Cipher
}

// NewSettingsStore creates a settings store.
func NewSettingsStore(app core.App, cipher *Cipher) *SettingsStore {
	return &SettingsStore{app: app, cipher: cipher}
}

// findRecord looks up the settings record for key.
//
// It returns nil when the setting has not been written yet. A missing
// record is not an error: the caller decides whether to create it.
func (s *SettingsStore) findRecord(key string) *core.Record {
	record, err := s.app.FindFirstRecordByFilter(
		CollectionAppSettings,
		"key = {:key}",
		map[string]any{"key": key},
	)
	if err != nil {
		return nil
	}
	return record
}

// ensureRecord returns the settings record for key, creating it when it
// does not exist yet.
func (s *SettingsStore) ensureRecord(key string) (*core.Record, error) {
	record := s.findRecord(key)
	if record != nil {
		return record, nil
	}

	col, err := s.app.FindCollectionByNameOrId(CollectionAppSettings)
	if err != nil {
		return nil, fmt.Errorf("find app_settings collection: %w", err)
	}

	record = core.NewRecord(col)
	record.Set("key", key)
	record.Set("value", json.RawMessage("null"))
	record.Set("encrypted", false)
	if err := s.app.Save(record); err != nil {
		return nil, fmt.Errorf("create setting %q: %w", key, err)
	}
	return record, nil
}

// setValue writes a value for key. When encrypted is true the value is
// encrypted before it is stored.
func (s *SettingsStore) setValue(key string, value any, encrypted bool) error {
	record, err := s.ensureRecord(key)
	if err != nil {
		return err
	}

	if encrypted {
		envelope, encErr := s.cipher.EncryptJSON(value)
		if encErr != nil {
			return encErr
		}
		// The envelope is stored as a JSON string.
		record.Set("value", envelope)
	} else {
		raw, marshalErr := json.Marshal(value)
		if marshalErr != nil {
			return fmt.Errorf("marshal setting %q: %w", key, marshalErr)
		}
		record.Set("value", json.RawMessage(raw))
	}
	record.Set("encrypted", encrypted)

	if err := s.app.Save(record); err != nil {
		return fmt.Errorf("save setting %q: %w", key, err)
	}
	return nil
}

// getValue reads a value for key. When encrypted is true the stored
// envelope is decrypted.
//
// The second return value reports whether the setting was found. The
// value is read as raw JSON so that both JSON documents and JSON
// strings (the encrypted envelope) round-trip correctly.
func (s *SettingsStore) getValue(key string, encrypted bool, out any) (bool, error) {
	record := s.findRecord(key)
	if record == nil {
		return false, nil
	}

	raw, err := rawJSON(record.GetRaw("value"))
	if err != nil {
		return false, fmt.Errorf("read setting %q: %w", key, err)
	}
	if len(raw) == 0 || string(raw) == "null" {
		return false, nil
	}

	if encrypted {
		// The envelope is a JSON string: unwrap it before decrypting.
		var envelope string
		if err := json.Unmarshal(raw, &envelope); err != nil {
			return false, fmt.Errorf("decode setting %q: %w", key, err)
		}
		if err := s.cipher.DecryptJSON(envelope, out); err != nil {
			return false, fmt.Errorf("decrypt setting %q: %w", key, err)
		}
		return true, nil
	}

	if err := json.Unmarshal(raw, out); err != nil {
		return false, fmt.Errorf("unmarshal setting %q: %w", key, err)
	}
	return true, nil
}

// rawJSON normalises a PocketBase JSON field value into raw JSON bytes.
// Depending on how the value was written it may already be a []byte, a
// string or a decoded Go value.
func rawJSON(value any) ([]byte, error) {
	switch typed := value.(type) {
	case nil:
		return nil, nil
	case []byte:
		return typed, nil
	case string:
		return []byte(typed), nil
	case json.RawMessage:
		return typed, nil
	default:
		return json.Marshal(value)
	}
}

// SetUI stores the interface preferences.
func (s *SettingsStore) SetUI(ui UI) error {
	return s.setValue(SettingUI, ui, false)
}

// GetUI returns the interface preferences, falling back to the factory
// defaults.
func (s *SettingsStore) GetUI() (UI, error) {
	ui := DefaultUI()
	found, err := s.getValue(SettingUI, false, &ui)
	if err != nil {
		return DefaultUI(), err
	}
	if !found {
		return DefaultUI(), nil
	}
	return ui, nil
}

// SetAPIKey stores the API key encrypted.
func (s *SettingsStore) SetAPIKey(apiKey string) error {
	return s.setValue(SettingAPIKey, apiKey, true)
}

// GetAPIKey returns the decrypted API key. The second return value
// reports whether a key was configured.
//
// The key is read by the Go backend only. It is never returned to the
// frontend, which only ever learns whether one is present.
func (s *SettingsStore) GetAPIKey() (string, bool, error) {
	var key string
	found, err := s.getValue(SettingAPIKey, true, &key)
	if err != nil {
		return "", false, err
	}
	return key, found, nil
}

// SetBaseURL stores the API base URL.
func (s *SettingsStore) SetBaseURL(baseURL string) error {
	return s.setValue(SettingBaseURL, baseURL, false)
}

// GetBaseURL returns the configured base URL, falling back to the
// factory default.
func (s *SettingsStore) GetBaseURL() (string, error) {
	baseURL := DefaultBaseURL
	found, err := s.getValue(SettingBaseURL, false, &baseURL)
	if err != nil {
		return DefaultBaseURL, err
	}
	if !found || baseURL == "" {
		return DefaultBaseURL, nil
	}
	return baseURL, nil
}

// DefaultBaseURL is the default remote API host.
//
// It points at a placeholder host on purpose: an application that talks
// to a real API replaces it with its own default.
const DefaultBaseURL = "https://api.example.com"

// SetSyncEmail stores the identity of the superuser of the remote
// server.
//
// The identity is not a secret, and it is stored as it is: the reader
// of the settings shows it, so that the user knows which account the
// sync uses.
func (s *SettingsStore) SetSyncEmail(email string) error {
	return s.setValue(SettingSyncEmail, email, false)
}

// GetSyncEmail returns the identity of the superuser of the remote
// server.
func (s *SettingsStore) GetSyncEmail() (string, error) {
	var email string
	found, err := s.getValue(SettingSyncEmail, false, &email)
	if err != nil {
		return "", err
	}
	if !found {
		return "", nil
	}
	return email, nil
}

// SetSyncPassword stores the password of the superuser of the remote
// server.
//
// The password is a secret: it is encrypted before it is written, and
// it is never returned to the frontend.
func (s *SettingsStore) SetSyncPassword(password string) error {
	return s.setValue(SettingSyncPassword, password, true)
}

// GetSyncPassword returns the decrypted password of the superuser of
// the remote server.
//
// The second return value reports whether a password is configured.
//
// The password is read by the Go backend only. It is never returned
// to the frontend, which only ever learns whether one is present.
func (s *SettingsStore) GetSyncPassword() (string, bool, error) {
	var password string
	found, err := s.getValue(SettingSyncPassword, true, &password)
	if err != nil {
		return "", false, err
	}
	return password, found, nil
}

// GetRaw reads a setting as raw JSON.
//
// The sync keeps its configuration in the settings of the application,
// and the values are held as JSON documents. The store of the package
// reads them through this method, so that the shape of the settings
// stays in one place.
func (s *SettingsStore) GetRaw(key string, out any) (bool, error) {
	return s.getValue(key, false, out)
}

// SetRaw writes a setting from a value.
//
// The value is stored as it is, and it is not encrypted: the
// configuration of the sync is not a secret.
func (s *SettingsStore) SetRaw(key string, value any) error {
	return s.setValue(key, value, false)
}
