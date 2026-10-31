package etl

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
)

// The names of the outputs the command writes by default.
//
// They are the names the application imports, so that a run that does
// not name its outputs leaves the files where the panel looks for them.
const (
	// DefaultItemsFile is the file the items are written to.
	DefaultItemsFile = "paris-wikidata-subset.ndjson"
	// DefaultPropertiesFile is the file the properties are written to.
	DefaultPropertiesFile = "wikidata-properties.ndjson"
)

// CommandOptions are the options of the etl subcommand.
//
// The struct is exported so that the desktop application can build the
// same command line, and so that the tests can check the arguments.
type CommandOptions struct {
	// TargetQID is the entity the items are linked to.
	TargetQID string
	// RecursiveP131 tells whether the administrative hierarchy is
	// walked from the target.
	RecursiveP131 bool
	// Languages are the languages that are kept.
	Languages []string
	// OutItems is the file the items are written to.
	OutItems string
	// OutProps is the file the properties are written to.
	OutProps string
	// Overwrite tells whether an existing output file may be replaced.
	Overwrite bool
	// Events tells whether the machine readable events are written to
	// the standard output.
	//
	// A terminal run does not want them, and the desktop application
	// does. The flag decides.
	Events bool
	// Quiet tells whether the progress is reported.
	Quiet bool
}

// NewCommand creates the etl subcommand.
//
// The command reads the dump from the standard input and it is already
// decompressed: the decompressor is another process.
//
// The human readable logs go to the standard error and the machine
// readable events go to the standard output, so that the two channels
// never mix.
func NewCommand() *cobra.Command {
	opts := CommandOptions{}

	command := &cobra.Command{
		Use:   "etl",
		Short: "Filter a Wikidata dump into the two trimmed datasets",
		Long: strings.TrimSpace(`
Filter the JSON Lines of a Wikidata dump into the two datasets the
desktop application imports: the items linked to a target, and the
property dictionary.

The dump is read from the standard input, and it is already
decompressed. The decompressor is a separate process, so the Go program
never reads a compressed file:

    bzcat latest-all.json.bz2 | wikirevele etl --target-qid Q90

The command makes no network request, it opens no database, and it
writes nothing but the two output files.
`),
		Example: strings.TrimSpace(`
  # The items of Paris, in English and in French.
  bzcat latest-all.json.bz2 | wikirevele etl --target-qid Q90 --langs en,fr

  # Everything inside Shanghai, following the administrative hierarchy.
  zstdcat latest-all.json.zst | wikirevele etl --target-qid Q148 \
      --recursive-p131 --langs en,zh --out-items shanghai.ndjson \
      --out-props props.ndjson --overwrite
`),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCommand(cmd.Context(), cmd.OutOrStdout(), cmd.ErrOrStderr(), opts, cmd.InOrStdin())
		},
		// The dump comes from the standard input, so the command
		// takes no argument. An argument is a mistake, and it is
		// reported rather than ignored.
		//
		// The message says where the dump comes from, because
		// Cobra's own message ("unknown command") does not: a user
		// who typed a file name needs to know that it has to be
		// piped in.
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf(
					"the command takes no argument, the dump is read from the standard input: %q",
					args[0])
			}
			return nil
		},
	}

	flags := command.Flags()
	flags.StringVar(&opts.TargetQID, "target-qid", ParisQID,
		"the Wikidata QID the items are linked to")
	flags.BoolVar(&opts.RecursiveP131, "recursive-p131", false,
		"keep everything inside the target, following the P131 hierarchy")
	flags.StringSliceVar(&opts.Languages, "langs", nil,
		"the languages to keep, for example en,zh,fr")
	flags.StringVar(&opts.OutItems, "out-items", DefaultItemsFile,
		"the file the items are written to")
	flags.StringVar(&opts.OutProps, "out-props", DefaultPropertiesFile,
		"the file the properties are written to")
	flags.BoolVar(&opts.Overwrite, "overwrite", false,
		"allow replacing an output file that already exists")
	flags.BoolVar(&opts.Events, "events", false,
		"write the machine readable events to the standard output")
	flags.BoolVar(&opts.Quiet, "quiet", false,
		"do not report the progress")

	return command
}

// runCommand runs the worker.
//
// It is the whole of the etl subcommand: the flags are read, the
// output files are opened, the dump is read and the result is
// written.
func runCommand(ctx context.Context, stdout, stderr io.Writer, opts CommandOptions, input io.Reader) error {
	if err := validateOptions(opts); err != nil {
		return err
	}

	// The dump comes from the standard input unless the caller gave
	// another reader.
	if input == nil {
		input = os.Stdin
	}

	// The context is cancelled on Ctrl-C, so that the run stops and
	// the output that was already written is flushed.
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	itemsFile, err := openOutput(opts.OutItems, opts.Overwrite)
	if err != nil {
		return err
	}
	defer itemsFile.Close()

	propertiesFile, err := openOutput(opts.OutProps, opts.Overwrite)
	if err != nil {
		return err
	}
	defer propertiesFile.Close()

	// The files are removed when the run fails before it produced
	// anything: an empty file is worse than no file, because the
	// application would import it as an empty dataset and the panel
	// would show nothing.
	//
	// A run that was interrupted keeps its output: what was written
	// is usable, and the import upserts, so a re-run is safe.
	produced := false
	defer func() {
		if produced {
			return
		}
		_ = itemsFile.Close()
		_ = propertiesFile.Close()
		_ = os.Remove(opts.OutItems)
		_ = os.Remove(opts.OutProps)
	}()

	// The progress and the warnings go to the standard error, so that
	// the standard output stays free for the events.
	logger := slog.New(slog.NewTextHandler(stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))

	var reporter Reporter
	if !opts.Quiet {
		reporter = NewTextReporter(stderr)
	}

	events := NewEventWriter(stdout, opts.Events)

	pipeline := NewPipeline(Options{
		Items:      itemsFile,
		Properties: propertiesFile,
		Target:     opts.TargetQID,
		Languages:  opts.Languages,
		Recursive:  opts.RecursiveP131,
		Reporter:   reporter,
		Logger:     logger,
		Events:     events,
	})

	// The dump is read from the standard input. It is already
	// decompressed: the decompressor is another process.
	stats, runErr := pipeline.Run(ctx, input)
	if runErr != nil && stats.Scanned == 0 {
		// The run failed before it read anything: the files are
		// removed, so that no empty dataset is left behind.
		_ = events.Error(runErr.Error())
		return runErr
	}

	// The administrative hierarchy is walked when the run asked for it,
	// and the items that are not inside the target are dropped.
	if err := pipeline.Finish(opts.OutItems); err != nil {
		_ = events.Error(err.Error())
		return err
	}

	// The output is flushed even when the run was stopped, so that the
	// work that was done is not lost.
	if err := pipeline.Flush(); err != nil {
		_ = events.Error(err.Error())
		return err
	}

	produced = true

	// The final report says what the run produced, whether it finished
	// or was interrupted. A quiet run says nothing at all.
	if !opts.Quiet {
		fmt.Fprintf(stderr,
			"done scanned=%d items=%d properties=%d skipped=%d malformed=%d\n",
			stats.Scanned, stats.Items, stats.Properties, stats.Skipped, stats.Malformed)
	}

	if runErr != nil {
		_ = events.Error(runErr.Error())
		return runErr
	}

	return events.Done(DoneEvent{
		ItemFile:   opts.OutItems,
		PropFile:   opts.OutProps,
		Scanned:    stats.Scanned,
		Items:      stats.Items,
		Properties: stats.Properties,
		Skipped:    stats.Skipped,
		Malformed:  stats.Malformed,
	})
}

// validateOptions checks the options of the command.
//
// The checks are the ones that save the user from a run that produces
// nothing: a target that is not an identifier, a language list that is
// empty, and an output file that already exists.
func validateOptions(opts CommandOptions) error {
	if !IsQID(opts.TargetQID) {
		return fmt.Errorf(
			"etl: the target %q is not a Wikidata identifier, such as Q90",
			opts.TargetQID)
	}

	for _, language := range opts.Languages {
		if !IsLanguageCode(language) {
			return fmt.Errorf(
				"etl: the language %q is not a language code, such as en or zh",
				language)
		}
	}

	if opts.OutItems == "" {
		return fmt.Errorf("etl: the items output is required")
	}
	if opts.OutProps == "" {
		return fmt.Errorf("etl: the properties output is required")
	}
	if opts.OutItems == opts.OutProps {
		return fmt.Errorf(
			"etl: the items and the properties cannot be written to the same file")
	}

	return nil
}

// openOutput opens an output file for writing.
//
// The file is created when it does not exist, and it is refused when
// it exists and the overwrite was not asked for: a run that silently
// replaced a dataset would be a surprise.
func openOutput(path string, overwrite bool) (*os.File, error) {
	if !overwrite {
		if _, err := os.Stat(path); err == nil {
			return nil, fmt.Errorf(
				"etl: %s already exists, use --overwrite to replace it", path)
		}
	}

	file, err := os.Create(path)
	if err != nil {
		return nil, fmt.Errorf("etl: create %s: %w", path, err)
	}

	return file, nil
}
