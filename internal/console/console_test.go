package console

import "testing"

// collector records the entries that were published.
type collector struct {
	entries []Entry
}

func (c *collector) EmitConsoleEntry(entry Entry) {
	c.entries = append(c.entries, entry)
}

// TestAppendPublishes verifies that a recorded entry is published to the
// emitter.
func TestAppendPublishes(t *testing.T) {
	sink := &collector{}
	log := New()
	log.SetEmitter(sink)

	log.Info("api", "a request was sent")

	if len(sink.entries) != 1 {
		t.Fatalf("published %d entries, want 1", len(sink.entries))
	}
	if sink.entries[0].Message != "a request was sent" {
		t.Errorf("message = %q", sink.entries[0].Message)
	}
	if sink.entries[0].Level != LevelInfo {
		t.Errorf("level = %q, want %q", sink.entries[0].Level, LevelInfo)
	}
}

// TestRawCarriesTheDetail verifies that a raw entry keeps its payload
// verbatim, which is the only level that carries one.
func TestRawCarriesTheDetail(t *testing.T) {
	log := New()

	entry := log.Raw("api", "the response arrived", `{"ok":true}`)

	if entry.Level != LevelRaw {
		t.Errorf("level = %q, want %q", entry.Level, LevelRaw)
	}
	if entry.Detail != `{"ok":true}` {
		t.Errorf("detail = %q, want the payload verbatim", entry.Detail)
	}
}

// TestEntriesAreCapped verifies that the log does not grow without
// bound.
func TestEntriesAreCapped(t *testing.T) {
	log := New()

	for i := 0; i < maxEntries+50; i++ {
		log.Info("test", "an entry")
	}

	entries := log.Entries()
	if len(entries) != maxEntries {
		t.Errorf("kept %d entries, want %d", len(entries), maxEntries)
	}
}

// TestEntriesAreOrdered verifies that the entries come back oldest
// first.
func TestEntriesAreOrdered(t *testing.T) {
	log := New()

	log.Info("first", "one")
	log.Info("second", "two")

	entries := log.Entries()
	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(entries))
	}
	if entries[0].Message != "one" || entries[1].Message != "two" {
		t.Errorf("entries are out of order: %q, %q", entries[0].Message, entries[1].Message)
	}
	if entries[0].ID >= entries[1].ID {
		t.Error("the identifiers are not increasing")
	}
}

// TestClearDiscardsTheEntries verifies that the log can be emptied.
func TestClearDiscardsTheEntries(t *testing.T) {
	log := New()
	log.Info("test", "an entry")

	log.Clear()

	if len(log.Entries()) != 0 {
		t.Error("the entries were not discarded")
	}
}

// TestFingerprintDoesNotRevealTheSecret verifies that the fingerprint
// never contains the secret itself.
func TestFingerprintDoesNotRevealTheSecret(t *testing.T) {
	const secret = "super-secret-key"

	fingerprint := Fingerprint(secret)
	if fingerprint == "" {
		t.Fatal("the fingerprint is empty")
	}
	if fingerprint == secret {
		t.Fatal("the fingerprint is the secret itself")
	}
	for _, char := range secret {
		if len(fingerprint) == 1 && fingerprint == string(char) {
			t.Fatal("the fingerprint reveals the secret")
		}
	}

	// The same secret must always give the same fingerprint, so that
	// two keys can be told apart in the log.
	if Fingerprint(secret) != fingerprint {
		t.Error("the fingerprint is not stable")
	}
	if Fingerprint("another") == fingerprint {
		t.Error("two different secrets share a fingerprint")
	}

	if Fingerprint("") != "none" {
		t.Errorf("an empty secret gave %q", Fingerprint(""))
	}
}
