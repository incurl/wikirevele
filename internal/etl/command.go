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
	// TargetQIDs are the entities the items are linked to.
	//
	// The dump is read once and every entity is checked against
	// every target, so a run over several targets costs one pass and
	// not one pass per target.
	TargetQIDs []string
	// TargetLabels are the names of the targets, in the same order.
	//
	// They are used in the messages and in the name of the file when
	// the file is derived from the target.
	TargetLabels []string
	// RecursiveP131 tells whether the administrative hierarchy is
	// walked from the targets.
	RecursiveP131 bool
	// Languages are the languages that are kept.
	Languages []string
	// OutItems is the file the items are written to.
	//
	// It is the file of the first target when there is one, and the
	// files of the other targets are derived from it.
	OutItems string
	// OutItemsPerTarget holds the file of every target, in the same
	// order as the targets.
	//
	// It overrides OutItems for the targets it names.
	OutItemsPerTarget []string
	// OutProps is the file the properties are written to.
	OutProps string
	// ItemsOnly tells whether the run writes the items only.
	ItemsOnly bool
	// PropertiesOnly tells whether the run writes the property
	// dictionary only.
	PropertiesOnly bool
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
	flags.StringSliceVar(&opts.TargetQIDs, "target-qid", []string{ParisQID},
		"the Wikidata QIDs the items are linked to, for example Q90,Q148")
	flags.StringSliceVar(&opts.TargetLabels, "target-label", nil,
		"the names of the targets, in the same order as the QIDs")
	flags.BoolVar(&opts.RecursiveP131, "recursive-p131", false,
		"keep everything inside the targets, following the P131 hierarchy")
	flags.StringSliceVar(&opts.Languages, "langs", nil,
		"the languages to keep, for example en,zh,fr")
	flags.StringVar(&opts.OutItems, "out-items", DefaultItemsFile,
		"the file the items are written to")
	flags.StringSliceVar(&opts.OutItemsPerTarget, "out-items-per-target", nil,
		"the file of every target, in the same order as the targets")
	flags.StringVar(&opts.OutProps, "out-props", DefaultPropertiesFile,
		"the file the properties are written to")
	flags.BoolVar(&opts.ItemsOnly, "items-only", false,
		"write the items only, and not the property dictionary")
	flags.BoolVar(&opts.PropertiesOnly, "properties-only", false,
		"write the property dictionary only, and not the items")
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

	// The targets are resolved before anything is opened: the files
	// of the targets depend on them.
	specs, err := ResolveTargets(opts)
	if err != nil {
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

	// The items of every target have their own file.
	var itemsFiles []*os.File
	var targets []*Target

	for _, spec := range specs {
		// A run that writes the property dictionary only has no
		// file of items: there is nothing to write them to.
		if spec.ItemsPath == "" {
			continue
		}

		itemsFile, err := openOutput(spec.ItemsPath, opts.Overwrite)
		if err != nil {
			closeAll(itemsFiles)
			return err
		}
		itemsFiles = append(itemsFiles, itemsFile)

		targets = append(targets, NewTarget(
			spec.QID,
			spec.Label,
			spec.ItemsPath,
			NewEntityWriter(itemsFile),
			opts.RecursiveP131,
		))
	}

	// The property dictionary has one file: it is the same for every
	// target, so it is written once and not once per target.
	var propertiesFile *os.File
	if opts.WritesProperties() {
		propertiesFile, err = openOutput(opts.OutProps, opts.Overwrite)
		if err != nil {
			closeAll(itemsFiles)
			return err
		}
	}

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
		closeAll(itemsFiles)
		if propertiesFile != nil {
			_ = propertiesFile.Close()
			_ = os.Remove(opts.OutProps)
		}
		for _, spec := range specs {
			_ = os.Remove(spec.ItemsPath)
		}
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
		Targets:         targets,
		Properties:      propertiesFile,
		WriteProperties: opts.WritesProperties(),
		Languages:       opts.Languages,
		Recursive:       opts.RecursiveP131,
		Reporter:        reporter,
		Logger:          logger,
		Events:          events,
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
	// and the items that are not inside a target are dropped.
	if err := pipeline.Finish(); err != nil {
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

		// Every target is reported on its own, so that a run over
		// several targets says what each of them produced.
		for _, target := range targets {
			logTarget(logger, target)
		}
	}

	if runErr != nil {
		_ = events.Error(runErr.Error())
		return runErr
	}

	return events.Done(DoneEvent{
		ItemFiles:  itemFiles(targets),
		PropFile:   opts.OutProps,
		Scanned:    stats.Scanned,
		Items:      stats.Items,
		Properties: stats.Properties,
		Skipped:    stats.Skipped,
		Malformed:  stats.Malformed,
	})
}

// closeAll closes a list of files.
//
// It is called when the run fails before the files are all opened, so
// that the ones that are open do not leak.
func closeAll(files []*os.File) {
	for _, file := range files {
		_ = file.Close()
	}
}

// itemFiles returns the file of every target.
//
// It is the list the done event carries, so that the driver knows what
// to import without having to guess.
func itemFiles(targets []*Target) []string {
	files := make([]string, 0, len(targets))
	for _, target := range targets {
		files = append(files, target.ItemsPath)
	}
	return files
}

// validateOptions checks the options of the command.
//
// The checks are the ones that save the user from a run that produces
// nothing: a target that is not an identifier, a language list that is
// empty, and an output file that already exists.
func validateOptions(opts CommandOptions) error {
	for _, qid := range opts.TargetQIDs {
		if !IsQID(qid) {
			return fmt.Errorf(
				"etl: the target %q is not a Wikidata identifier, such as Q90",
				qid)
		}
	}

	for _, language := range opts.Languages {
		if !IsLanguageCode(language) {
			return fmt.Errorf(
				"etl: the language %q is not a language code, such as en or zh",
				language)
		}
	}

	if opts.ItemsOnly && opts.PropertiesOnly {
		return fmt.Errorf(
			"etl: --items-only and --properties-only cannot be used together: " +
				"the run would write nothing")
	}

	if opts.WritesItems() && opts.OutItems == "" && len(opts.OutItemsPerTarget) == 0 {
		return fmt.Errorf("etl: the items output is required")
	}
	if opts.WritesProperties() && opts.OutProps == "" {
		return fmt.Errorf("etl: the properties output is required")
	}
	if opts.OutItems != "" && opts.OutItems == opts.OutProps {
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
