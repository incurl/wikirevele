package etl

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIsQID(t *testing.T) {
	cases := map[string]bool{
		"Q90":     true,
		"Q1":      true,
		"Q148":    true,
		"Q662548": true,
		"P131":    false,
		"Q":       false,
		"Q90A":    false,
		"q90":     false,
		"":        false,
		"90":      false,
	}

	for value, want := range cases {
		if got := IsQID(value); got != want {
			t.Errorf("IsQID(%q) = %v, want %v", value, got, want)
		}
	}
}

func TestIsPID(t *testing.T) {
	cases := map[string]bool{
		"P131":  true,
		"P1":    true,
		"Q90":   false,
		"P":     false,
		"P131A": false,
		"":      false,
	}

	for value, want := range cases {
		if got := IsPID(value); got != want {
			t.Errorf("IsPID(%q) = %v, want %v", value, got, want)
		}
	}
}

func TestIsLanguageCode(t *testing.T) {
	cases := map[string]bool{
		"en":         true,
		"fr":         true,
		"zh":         true,
		"zh-hans":    true,
		"zh-min-nan": true,
		"EN":         false,
		"e":          true,
		"":           false,
		"en-":        false,
		"-en":        false,
		"en--fr":     false,
		"english":    true,
		"e n":        false,
	}

	for value, want := range cases {
		if got := IsLanguageCode(value); got != want {
			t.Errorf("IsLanguageCode(%q) = %v, want %v", value, got, want)
		}
	}
}

func TestTrimOptionsForCleansTheLanguages(t *testing.T) {
	opts := TrimOptionsFor([]string{" en ", "fr", "en", ""})

	if len(opts.LabelLanguages) != 2 {
		t.Fatalf("the languages are %v, want two", opts.LabelLanguages)
	}
	if opts.LabelLanguages[0] != "en" || opts.LabelLanguages[1] != "fr" {
		t.Errorf("the languages are %v, want en and fr", opts.LabelLanguages)
	}
}

func TestTrimOptionsForFallsBackOnTheDefaults(t *testing.T) {
	opts := TrimOptionsFor(nil)

	if len(opts.LabelLanguages) != 2 {
		t.Errorf("the languages are %v, want the defaults", opts.LabelLanguages)
	}
}

func TestTheCommandRefusesABadTarget(t *testing.T) {
	err := validateOptions(CommandOptions{
		TargetQID: "Paris",
		OutItems:  "items.ndjson",
		OutProps:  "props.ndjson",
	})

	if err == nil {
		t.Fatal("the command accepted a target that is not an identifier")
	}
}

func TestTheCommandRefusesABadLanguage(t *testing.T) {
	err := validateOptions(CommandOptions{
		TargetQID: "Q90",
		Languages: []string{"en", "NOT A LANGUAGE"},
		OutItems:  "items.ndjson",
		OutProps:  "props.ndjson",
	})

	if err == nil {
		t.Fatal("the command accepted a language that is not a code")
	}
}

func TestTheCommandRefusesTheSameFileTwice(t *testing.T) {
	err := validateOptions(CommandOptions{
		TargetQID: "Q90",
		OutItems:  "same.ndjson",
		OutProps:  "same.ndjson",
	})

	if err == nil {
		t.Fatal("the command accepted one file for both outputs")
	}
}

func TestTheCommandRefusesAnExistingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "items.ndjson")
	if err := os.WriteFile(path, []byte("old"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	if _, err := openOutput(path, false); err == nil {
		t.Fatal("the command replaced a file that was not asked for")
	}

	file, err := openOutput(path, true)
	if err != nil {
		t.Fatalf("the command refused to replace a file with --overwrite: %v", err)
	}
	_ = file.Close()
}

func TestTheCommandReadsTheDumpFromTheStandardInput(t *testing.T) {
	// The dump comes from the standard input, so the command takes no
	// argument. An argument is a mistake and it is reported.
	command := NewCommand()
	command.SetArgs([]string{"some-file.ndjson"})
	command.SetOut(&bytes.Buffer{})
	command.SetErr(&bytes.Buffer{})

	err := command.Execute()
	if err == nil {
		t.Fatal("the command accepted an argument")
	}
}

func TestTheCommandWritesTheEventsWhenAsked(t *testing.T) {
	dir := t.TempDir()
	items := filepath.Join(dir, "items.ndjson")
	props := filepath.Join(dir, "props.ndjson")

	dump := itemWith(t, "Q75", "Q90")

	command := NewCommand()
	command.SetArgs([]string{
		"--target-qid", "Q90",
		"--langs", "en",
		"--out-items", items,
		"--out-props", props,
		"--events",
		"--quiet",
	})

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	command.SetOut(stdout)
	command.SetErr(stderr)
	command.SetIn(strings.NewReader(dump))

	if err := command.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	// The events are on the standard output, one JSON object per line.
	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	if len(lines) == 0 || lines[0] == "" {
		t.Fatalf("no event was written: %q", stdout.String())
	}

	var done DoneEvent
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &done); err != nil {
		t.Fatalf("the last event is not JSON: %v", err)
	}
	if done.Type != EventDone {
		t.Errorf("the last event is %q, want %q", done.Type, EventDone)
	}
	if done.ItemFile != items {
		t.Errorf("the event names %q, want %q", done.ItemFile, items)
	}
	if done.Items != 1 {
		t.Errorf("the event counts %d items, want 1", done.Items)
	}
}

func TestTheCommandIsQuietOnTheStandardOutputByDefault(t *testing.T) {
	dir := t.TempDir()

	command := NewCommand()
	command.SetArgs([]string{
		"--target-qid", "Q90",
		"--out-items", filepath.Join(dir, "items.ndjson"),
		"--out-props", filepath.Join(dir, "props.ndjson"),
		"--quiet",
	})

	stdout := &bytes.Buffer{}
	command.SetOut(stdout)
	command.SetErr(&bytes.Buffer{})
	command.SetIn(strings.NewReader(itemWith(t, "Q75", "Q90")))

	if err := command.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	// Without --events, the standard output stays empty: a terminal
	// user should not see JSON.
	if stdout.Len() != 0 {
		t.Errorf("the standard output is %q, want it empty", stdout.String())
	}
}
