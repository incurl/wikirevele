// Package etlpipeline builds and runs the shell pipeline that filters
// a Wikidata dump.
//
// The pipeline is two processes: a decompressor that writes the
// decompressed dump, and the etl worker of this same binary that reads
// it. The application never decompresses anything and never parses a
// dump itself: the heavy work happens in the child process.
//
// The same pipeline is what the user sees in the interface, as one
// copyable command. A user who copies it and runs it in a terminal
// gets the same result as the application does.
package etlpipeline

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/incurl/wikirevele/internal/etl"
)

// The decompressors the pipeline knows.
//
// The binary does not ship them: the host has to provide them. The
// pipeline tells the application which one it found, and the
// application tells the user what to install when there is none.
const (
	// DecompressorBZip2 decompresses a .bz2 file.
	DecompressorBZip2 = "bzcat"
	// DecompressorZstd decompresses a .zst file.
	DecompressorZstd = "zstdcat"
	// DecompressorGzip decompresses a .gz file.
	DecompressorGzip = "gzip -dc"
)

// The extensions the pipeline recognises.
const (
	// ExtensionBZip2 is the extension of a bzip2 file.
	ExtensionBZip2 = ".bz2"
	// ExtensionZstd is the extension of a zstd file.
	ExtensionZstd = ".zst"
	// ExtensionGzip is the extension of a gzip file.
	ExtensionGzip = ".gz"
	// ExtensionXZ is the extension of an xz file.
	ExtensionXZ = ".xz"
)

// JobOptions are the parameters of a pipeline run.
//
// They are what the interface collects, and they are what the command
// is built from.
type JobOptions struct {
	// DumpPath is the path of the compressed dump.
	DumpPath string `json:"dump_path"`
	// TargetQIDs are the entities the items are linked to.
	//
	// The dump is read once and every entity is checked against
	// every target, so a run over several targets costs one pass and
	// not one pass per target.
	TargetQIDs []string `json:"target_qids"`
	// TargetLabels are the names of the targets, in the same order.
	//
	// They are used in the messages and in the name of the file when
	// the file is derived from the target.
	TargetLabels []string `json:"target_labels"`
	// RecursiveP131 tells whether the administrative hierarchy is
	// walked.
	RecursiveP131 bool `json:"recursive_p131"`
	// Languages are the languages that are kept.
	Languages []string `json:"langs"`
	// OutItemsPath is the file the items are written to.
	//
	// It is the file of the first target when there is one, and the
	// files of the other targets are derived from it.
	OutItemsPath string `json:"out_items_path"`
	// OutItemsPaths holds the file of every target, in the same order
	// as the targets.
	//
	// It overrides OutItemsPath for the targets it names.
	OutItemsPaths []string `json:"out_items_paths"`
	// OutPropsPath is the file the properties are written to.
	OutPropsPath string `json:"out_props_path"`
	// ItemsOnly tells whether the run writes the items only.
	ItemsOnly bool `json:"items_only"`
	// PropertiesOnly tells whether the run writes the property
	// dictionary only.
	PropertiesOnly bool `json:"properties_only"`
	// Overwrite tells whether an existing output file may be
	// replaced.
	Overwrite bool `json:"overwrite"`
}

// BuildCommand returns the shell pipeline that runs the job.
//
// The command is the one the user sees and copies, and it is the one
// the application runs. There is only one of them, so the copy always
// does what the button did.
//
// The paths are escaped, so that a path with a space in it still
// works when the command is pasted into a terminal.
func BuildCommand(opts JobOptions, worker string) (string, error) {
	if err := ValidateOptions(opts); err != nil {
		return "", err
	}

	decompressor, err := DecompressorFor(opts.DumpPath)
	if err != nil {
		return "", err
	}

	arguments := []string{
		quote(opts.DumpPath),
		"|",
		quote(worker),
		"etl",
		"--target-qid", quote(strings.Join(opts.TargetQIDs, ",")),
	}

	if len(opts.TargetLabels) > 0 {
		arguments = append(arguments, "--target-label", quote(strings.Join(opts.TargetLabels, ",")))
	}
	if opts.RecursiveP131 {
		arguments = append(arguments, "--recursive-p131")
	}
	if len(opts.Languages) > 0 {
		arguments = append(arguments, "--langs", quote(strings.Join(opts.Languages, ",")))
	}
	arguments = append(arguments,
		"--out-items", quote(opts.OutItemsPath),
		"--out-props", quote(opts.OutPropsPath),
		"--events",
	)
	if len(opts.OutItemsPaths) > 0 {
		arguments = append(arguments,
			"--out-items-per-target", quote(strings.Join(opts.OutItemsPaths, ",")))
	}
	if opts.ItemsOnly {
		arguments = append(arguments, "--items-only")
	}
	if opts.PropertiesOnly {
		arguments = append(arguments, "--properties-only")
	}
	if opts.Overwrite {
		arguments = append(arguments, "--overwrite")
	}

	return decompressor + " " + strings.Join(arguments, " "), nil
}

// DecompressorFor returns the decompressor that reads the given dump.
//
// The extension of the file decides. A file with no known extension is
// refused: the pipeline cannot guess, and a wrong guess would feed
// compressed bytes to the worker.
func DecompressorFor(path string) (string, error) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ExtensionBZip2:
		return DecompressorBZip2, nil
	case ExtensionZstd:
		return DecompressorZstd, nil
	case ExtensionGzip:
		return DecompressorGzip, nil
	case ExtensionXZ:
		return "xz -dc", nil
	default:
		return "", fmt.Errorf(
			"etlpipeline: %s is not a compressed dump: it must end in %s, %s, %s or %s",
			path, ExtensionBZip2, ExtensionZstd, ExtensionGzip, ExtensionXZ)
	}
}

// ValidateOptions checks the options of a job.
//
// The checks are the ones that save the user from a run that produces
// nothing: a target that is not an identifier, a language that is not a
// code, and an output that is not named.
//
// The dump is not checked here: the command is built before the file
// is picked, and a path that is not there yet is not a mistake in the
// command.
func ValidateOptions(opts JobOptions) error {
	if opts.DumpPath == "" {
		return fmt.Errorf("etlpipeline: the dump is required")
	}
	if len(opts.TargetQIDs) == 0 {
		return fmt.Errorf("etlpipeline: the target is required")
	}
	for _, qid := range opts.TargetQIDs {
		if !etl.IsQID(qid) {
			return fmt.Errorf(
				"etlpipeline: the target %q is not a Wikidata identifier, such as Q90",
				qid)
		}
	}
	if len(opts.TargetLabels) > len(opts.TargetQIDs) {
		return fmt.Errorf(
			"etlpipeline: %d names were given for %d targets",
			len(opts.TargetLabels), len(opts.TargetQIDs))
	}
	if opts.ItemsOnly && opts.PropertiesOnly {
		return fmt.Errorf(
			"etlpipeline: the items only and the properties only cannot be asked " +
				"for at the same time: the run would write nothing")
	}
	if opts.WritesItems() && opts.OutItemsPath == "" && len(opts.OutItemsPaths) == 0 {
		return fmt.Errorf("etlpipeline: the items output is required")
	}
	if opts.WritesProperties() && opts.OutPropsPath == "" {
		return fmt.Errorf("etlpipeline: the properties output is required")
	}
	if opts.OutItemsPath != "" && opts.OutItemsPath == opts.OutPropsPath {
		return fmt.Errorf(
			"etlpipeline: the items and the properties cannot be written to the same file")
	}

	return nil
}

// WritesItems tells whether the run writes the items.
//
// The run writes the items unless it was asked for the property
// dictionary only.
func (o JobOptions) WritesItems() bool {
	return !o.PropertiesOnly
}

// WritesProperties tells whether the run writes the property
// dictionary.
//
// The run writes the dictionary unless it was asked for the items
// only.
func (o JobOptions) WritesProperties() bool {
	return !o.ItemsOnly
}

// Validate checks the options of a job and the files they name.
//
// It is the check that is done before a run starts: the dump has to be
// there, and the outputs have to be writable.
func Validate(opts JobOptions) error {
	if err := ValidateOptions(opts); err != nil {
		return err
	}

	if _, err := os.Stat(opts.DumpPath); err != nil {
		return fmt.Errorf("etlpipeline: the dump %s is not readable: %w", opts.DumpPath, err)
	}

	paths := []string{}
	if opts.WritesItems() {
		paths = append(paths, opts.OutItemsPath)
		paths = append(paths, opts.OutItemsPaths...)
	}
	if opts.WritesProperties() {
		paths = append(paths, opts.OutPropsPath)
	}

	for _, path := range paths {
		if path == "" {
			continue
		}
		if err := checkWritable(path); err != nil {
			return err
		}
	}

	return nil
}

// checkWritable tells whether a file can be written.
//
// The check is on the directory, because a file that does not exist
// yet cannot be opened: the directory is what has to be writable.
func checkWritable(path string) error {
	directory := filepath.Dir(path)
	if directory == "" {
		directory = "."
	}

	info, err := os.Stat(directory)
	if err != nil {
		return fmt.Errorf("etlpipeline: the directory %s is not readable: %w", directory, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("etlpipeline: %s is not a directory", directory)
	}

	// The file itself is checked when it exists: an existing file that
	// cannot be written is a mistake the user can see.
	if file, err := os.Stat(path); err == nil {
		if file.Mode().Perm()&0o200 == 0 {
			return fmt.Errorf("etlpipeline: %s cannot be written", path)
		}
	}

	return nil
}

// quote escapes a path for the shell.
//
// A path that holds a space, a quote or any other character the shell
// would read is put between single quotes, and the single quotes it
// holds are closed, escaped and opened again.
//
// The result is safe to paste into a terminal on any platform the
// application runs on.
func quote(value string) string {
	if value == "" {
		return "''"
	}

	// A path that holds nothing the shell would read is left alone,
	// so that the command stays readable.
	if isPlain(value) {
		return value
	}

	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}

// isPlain tells whether a value can be written without quotes.
//
// The safe characters are the letters, the digits and the marks that
// the shell does not read: the path of a file in a plain directory
// needs no quotes, and a command full of quotes is hard to read.
func isPlain(value string) bool {
	for i := 0; i < len(value); i++ {
		c := value[i]

		switch {
		case c >= 'a' && c <= 'z':
		case c >= 'A' && c <= 'Z':
		case c >= '0' && c <= '9':
		case c == '-' || c == '_' || c == '.' || c == '/' || c == ':' || c == '=' || c == ',':
		default:
			return false
		}
	}

	return true
}

// WorkerPath returns the path of the binary that runs the worker.
//
// The application spawns itself, with the etl subcommand, so that the
// worker is always the same build as the application.
//
// The path of the running executable is used. It is the only path that
// is certainly right: the binary may have been moved, and the working
// directory may have changed.
func WorkerPath() string {
	if path, err := os.Executable(); err == nil {
		return path
	}

	// The executable path is not always available: on some systems it
	// is read from the process information. The command of the
	// process is the fallback.
	if len(os.Args) > 0 {
		return os.Args[0]
	}

	return "wikirevele"
}

// workerPath returns the path of the binary to spawn as the worker.
//
// It refuses to spawn a **test binary**. A test binary run with the
// etl subcommand would run the tests again, and each of them would
// spawn another one: the tests would fork without end.
//
// The guard is on the name of the executable, because that is what
// tells a test binary from the application.
func workerPath() string {
	path := WorkerPath()

	if isTestBinary(path) {
		// The caller is a test. The worker cannot be the test
		// binary, and the tests that need a worker use the
		// launcher seam instead.
		return ""
	}

	return path
}

// isTestBinary tells whether a path names a test binary.
//
// The name of a Go test binary ends with ".test", and the name of the
// test of this package is the one that matters.
func isTestBinary(path string) bool {
	name := filepath.Base(path)

	return strings.HasSuffix(name, ".test") || strings.HasSuffix(name, ".test.exe")
}

// WorkerCommand returns the command that runs the worker.
//
// It is the path of the binary and the subcommand, quoted.
func WorkerCommand() string {
	path := WorkerPath()

	if runtime.GOOS == "windows" {
		return quote(path)
	}

	return quote(path)
}
