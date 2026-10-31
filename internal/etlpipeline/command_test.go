package etlpipeline

import (
	"strings"
	"testing"
)

func TestBuildCommandMakesTheWholePipeline(t *testing.T) {
	command, err := BuildCommand(JobOptions{
		DumpPath:     "/data/latest-all.json.bz2",
		TargetQIDs:   []string{"Q90"},
		Languages:    []string{"en", "fr"},
		OutItemsPath: "/out/items.ndjson",
		OutPropsPath: "/out/props.ndjson",
	}, "/apps/wikirevele")

	if err != nil {
		t.Fatalf("build: %v", err)
	}

	want := "bzcat /data/latest-all.json.bz2 | /apps/wikirevele etl " +
		"--target-qid Q90 --langs en,fr " +
		"--out-items /out/items.ndjson --out-props /out/props.ndjson --events"

	if command != want {
		t.Errorf("the command is\n  %s\nwant\n  %s", command, want)
	}
}

func TestBuildCommandAddsTheRecursiveFlag(t *testing.T) {
	command, err := BuildCommand(JobOptions{
		DumpPath:      "/data/latest-all.json.bz2",
		TargetQIDs:    []string{"Q148"},
		RecursiveP131: true,
		OutItemsPath:  "items.ndjson",
		OutPropsPath:  "props.ndjson",
	}, "./app")

	if err != nil {
		t.Fatalf("build: %v", err)
	}

	if !strings.Contains(command, "--recursive-p131") {
		t.Errorf("the command has no recursive flag: %s", command)
	}
}

func TestBuildCommandAddsTheOverwriteFlag(t *testing.T) {
	command, err := BuildCommand(JobOptions{
		DumpPath:     "dump.json.bz2",
		TargetQIDs:   []string{"Q90"},
		OutItemsPath: "items.ndjson",
		OutPropsPath: "props.ndjson",
		Overwrite:    true,
	}, "./app")

	if err != nil {
		t.Fatalf("build: %v", err)
	}

	if !strings.Contains(command, "--overwrite") {
		t.Errorf("the command has no overwrite flag: %s", command)
	}
}

func TestBuildCommandEscapesThePaths(t *testing.T) {
	command, err := BuildCommand(JobOptions{
		DumpPath:     "/data/my dumps/latest all.json.bz2",
		TargetQIDs:   []string{"Q90"},
		OutItemsPath: "/out/my items.ndjson",
		OutPropsPath: "/out/props.ndjson",
	}, "./my app")

	if err != nil {
		t.Fatalf("build: %v", err)
	}

	if !strings.Contains(command, "'/data/my dumps/latest all.json.bz2'") {
		t.Errorf("the dump path is not escaped: %s", command)
	}
	if !strings.Contains(command, "'./my app'") {
		t.Errorf("the worker path is not escaped: %s", command)
	}
	if !strings.Contains(command, "'/out/my items.ndjson'") {
		t.Errorf("the items path is not escaped: %s", command)
	}
}

func TestQuoteEscapesTheQuotes(t *testing.T) {
	cases := map[string]string{
		"/plain/path":     "/plain/path",
		"/with space":     "'/with space'",
		"/with'quote":     `'/with'\''quote'`,
		"":                "''",
		"/with$dollar":    "'/with$dollar'",
		"/with;semicolon": "'/with;semicolon'",
		"/with|pipe":      "'/with|pipe'",
	}

	for input, want := range cases {
		if got := quote(input); got != want {
			t.Errorf("quote(%q) = %s, want %s", input, got, want)
		}
	}
}

func TestDecompressorFor(t *testing.T) {
	cases := map[string]string{
		"dump.json.bz2": DecompressorBZip2,
		"dump.json.zst": DecompressorZstd,
		"dump.json.gz":  DecompressorGzip,
		"DUMP.JSON.BZ2": DecompressorBZip2,
	}

	for path, want := range cases {
		got, err := DecompressorFor(path)
		if err != nil {
			t.Errorf("DecompressorFor(%q): %v", path, err)
			continue
		}
		if got != want {
			t.Errorf("DecompressorFor(%q) = %q, want %q", path, got, want)
		}
	}
}

func TestDecompressorForRefusesAnUnknownFile(t *testing.T) {
	if _, err := DecompressorFor("dump.json"); err == nil {
		t.Fatal("the pipeline accepted a file that is not compressed")
	}
}

func TestValidateRefusesTheSameFileTwice(t *testing.T) {
	err := Validate(JobOptions{
		DumpPath:     "/tmp/dump.json.bz2",
		TargetQIDs:   []string{"Q90"},
		OutItemsPath: "same.ndjson",
		OutPropsPath: "same.ndjson",
	})

	if err == nil {
		t.Fatal("the pipeline accepted one file for both outputs")
	}
}

func TestValidateRefusesAMissingDump(t *testing.T) {
	err := Validate(JobOptions{
		DumpPath:     "/tmp/there-is-no-such-dump.json.bz2",
		TargetQIDs:   []string{"Q90"},
		OutItemsPath: "items.ndjson",
		OutPropsPath: "props.ndjson",
	})

	if err == nil {
		t.Fatal("the pipeline accepted a dump that is not there")
	}
}

func TestBuildCommandCarriesEveryTarget(t *testing.T) {
	// The command of a run over several targets names all of them.
	command, err := BuildCommand(JobOptions{
		DumpPath:     "/data/latest-all.json.bz2",
		TargetQIDs:   []string{"Q90", "Q148"},
		TargetLabels: []string{"paris", "shanghai"},
		OutItemsPath: "/out/paris.ndjson",
		OutPropsPath: "/out/props.ndjson",
	}, "/apps/wikirevele")

	if err != nil {
		t.Fatalf("build: %v", err)
	}

	if !strings.Contains(command, "--target-qid Q90,Q148") {
		t.Errorf("the command does not name both targets: %s", command)
	}
	if !strings.Contains(command, "--target-label paris,shanghai") {
		t.Errorf("the command does not name both labels: %s", command)
	}
}

func TestBuildCommandCarriesTheGranularFlags(t *testing.T) {
	// The items only and the properties only are in the command.
	items, err := BuildCommand(JobOptions{
		DumpPath:     "dump.json.bz2",
		TargetQIDs:   []string{"Q90"},
		OutItemsPath: "items.ndjson",
		OutPropsPath: "props.ndjson",
		ItemsOnly:    true,
	}, "./app")
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if !strings.Contains(items, "--items-only") {
		t.Errorf("the command has no items only flag: %s", items)
	}

	properties, err := BuildCommand(JobOptions{
		DumpPath:       "dump.json.bz2",
		TargetQIDs:     []string{"Q90"},
		OutItemsPath:   "items.ndjson",
		OutPropsPath:   "props.ndjson",
		PropertiesOnly: true,
	}, "./app")
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if !strings.Contains(properties, "--properties-only") {
		t.Errorf("the command has no properties only flag: %s", properties)
	}
}

func TestValidateRefusesBothOutputsAtOnce(t *testing.T) {
	err := ValidateOptions(JobOptions{
		DumpPath:       "/tmp/dump.json.bz2",
		TargetQIDs:     []string{"Q90"},
		OutItemsPath:   "items.ndjson",
		OutPropsPath:   "props.ndjson",
		ItemsOnly:      true,
		PropertiesOnly: true,
	})

	if err == nil {
		t.Fatal("the pipeline accepted a run that writes nothing")
	}
}

func TestValidateRefusesMoreNamesThanTargets(t *testing.T) {
	err := ValidateOptions(JobOptions{
		DumpPath:     "/tmp/dump.json.bz2",
		TargetQIDs:   []string{"Q90"},
		TargetLabels: []string{"paris", "shanghai"},
		OutItemsPath: "items.ndjson",
		OutPropsPath: "props.ndjson",
	})

	if err == nil {
		t.Fatal("the pipeline accepted more names than targets")
	}
}
