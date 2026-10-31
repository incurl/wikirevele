package etl

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"time"
)

// The sizes of the buffer the scanner reads into.
//
// The dump has lines of a few kilobytes, but a handful of items carry
// very long statements, so the buffer is allowed to grow.
const (
	// ScanBufferSize is the initial size of the scan buffer.
	ScanBufferSize = 256 << 10
	// MaxScanBufferSize is the largest line the scanner accepts.
	//
	// A line longer than this is a broken dump rather than a real
	// entity.
	MaxScanBufferSize = 16 << 20
)

// ProgressInterval is the number of scanned lines between two progress
// reports.
//
// A hundred thousand lines is frequent enough to show that the run is
// alive, and rare enough not to drown the log.
const ProgressInterval = 100_000

// Stats counts what a run has seen so far.
type Stats struct {
	// Scanned is the number of lines read from the input.
	Scanned int64
	// Items is the number of items that were kept.
	Items int64
	// Properties is the number of properties that were written.
	Properties int64
	// Skipped is the number of lines that were not kept.
	Skipped int64
	// Malformed is the number of lines that could not be parsed.
	Malformed int64
	// Bytes is the number of bytes read from the input.
	//
	// It is what the speed is computed from.
	Bytes int64
}

// Reporter is told about the progress of a run.
//
// The interface exists so that the progress can be reported to a user
// interface as well as to the standard error of a command line run.
type Reporter interface {
	// Report is called with the statistics of the run.
	Report(stats Stats)
}

// ReporterFunc adapts a plain function to the Reporter interface.
type ReporterFunc func(stats Stats)

// Report calls the wrapped function.
func (f ReporterFunc) Report(stats Stats) {
	f(stats)
}

// TextReporter writes the progress to a writer, one line per report.
//
// It is the reporter of the command line run: the progress goes to the
// standard error, so that it never mixes with the data.
type TextReporter struct {
	// out is where the lines go.
	out io.Writer
}

// NewTextReporter creates a reporter that writes to out.
func NewTextReporter(out io.Writer) *TextReporter {
	return &TextReporter{out: out}
}

// Report writes the statistics of the run.
//
// It implements Reporter.
func (r *TextReporter) Report(stats Stats) {
	fmt.Fprintf(r.out,
		"scanned=%d items=%d properties=%d skipped=%d malformed=%d\n",
		stats.Scanned, stats.Items, stats.Properties, stats.Skipped, stats.Malformed)
}

// Pipeline reads a dump and writes the trimmed datasets.
//
// It holds one entity in memory at a time. The scanner reads a line,
// the line is decoded, and the line is dropped: nothing accumulates
// except the counters and, in the recursive mode, the edges of the
// administrative hierarchies.
//
// A run may extract several targets at once. The dump is read once,
// and every entity is checked against every target: an item that is
// linked to Shanghai and to New York is written to both files. Reading
// the dump once is the whole point: the dump is more than a hundred
// gigabytes, and a run per target would read it once per target.
type Pipeline struct {
	// targets are the entities the items are extracted for.
	//
	// The pipeline holds one target per extraction, and the run of a
	// single target is the degenerate case of one target.
	targets []*Target
	// targetPatterns are the patterns of every target, for the fast
	// path.
	targetPatterns [][]byte
	// properties writes the property dictionary.
	properties *EntityWriter
	// writeProperties tells whether the dictionary is written at all.
	//
	// A run that was asked for the items only does not write it.
	writeProperties bool
	// reporter is told about the progress. It may be nil.
	reporter Reporter
	// log receives the lines that could not be read. It may be nil.
	log *slog.Logger
	// trim says which languages are kept.
	trim TrimOptions
	// recursive tells whether the administrative hierarchy is walked.
	recursive bool
	// events writes the machine readable events of the run. It may be
	// nil, in which case the events are not written.
	events *EventWriter
	// started is when the run began, for the speed.
	started time.Time
}

// Options configures a Pipeline.
type Options struct {
	// Targets are the entities the items are extracted for.
	//
	// It is empty when the run writes the property dictionary only.
	Targets []*Target
	// Properties receives the property dictionary.
	Properties io.Writer
	// WriteProperties tells whether the dictionary is written.
	//
	// It is false when the run was asked for the items only. The
	// dictionary is the same for every target, so it is written once
	// per run and not once per target.
	WriteProperties bool
	// Languages are the languages of the labels, descriptions and
	// aliases that are kept. It is empty when the defaults are
	// wanted.
	Languages []string
	// Recursive tells whether the administrative hierarchy is walked
	// from the targets, so that everything inside them is kept.
	Recursive bool
	// Reporter is told about the progress every ProgressInterval lines.
	// It may be nil, in which case there is no progress output.
	Reporter Reporter
	// Logger receives the lines that could not be parsed. It may be
	// nil, in which case the damaged lines are only counted.
	Logger *slog.Logger
	// Events writes the machine readable events of the run. It may be
	// nil, in which case the events are not written.
	Events *EventWriter
}

// NewPipeline creates a pipeline that writes to the given writers.
func NewPipeline(opts Options) *Pipeline {
	pipeline := &Pipeline{
		targets:         opts.Targets,
		properties:      NewEntityWriter(opts.Properties),
		writeProperties: opts.WriteProperties,
		reporter:        opts.Reporter,
		log:             opts.Logger,
		trim:            TrimOptionsFor(opts.Languages),
		recursive:       opts.Recursive,
		events:          opts.Events,
		started:         time.Now(),
	}

	for _, target := range pipeline.targets {
		pipeline.targetPatterns = append(pipeline.targetPatterns, target.pattern)
	}

	return pipeline
}

// locationPattern is the field that marks a statement of the
// administrative hierarchy.
//
// It is what the recursive mode looks for: an item that carries a P131
// statement is part of the hierarchy, whatever it points at.
var locationPattern = []byte(`"P131"`)

// typeField is the field that says what an entity is.
//
// The value of the field is what tells a property from an item, and it
// is looked for in a way that does not care about the spaces around
// the colon: the official dump writes it without any, and a mirror
// that writes it with some must not be skipped.
const typeField = `"type"`

// propertyType is the value of the type field for a property.
const propertyType = "property"

// looksLikeObject tells whether a line looks like a JSON object.
//
// It checks the first and the last byte, which is what tells a line of
// the dump from a line of something else. The content is not checked:
// that is the job of the decoder.
func looksLikeObject(line []byte) bool {
	trimmed := bytes.TrimSpace(line)

	if len(trimmed) < 2 {
		return false
	}

	return trimmed[0] == '{' && trimmed[len(trimmed)-1] == '}'
}

// canContribute tells whether a line of the dump is worth reading.
//
// A line contributes when it mentions one of the targets, or when it
// declares a property, or when it carries a statement of the hierarchy
// and the recursive mode is on.
//
// The search is a byte search: the identifiers are written verbatim in
// the dump, so the answer does not need the line to be decoded.
func (p *Pipeline) canContribute(line []byte) bool {
	if containsAny(line, p.targetPatterns) {
		return true
	}

	if p.writeProperties && hasTypeValue(line, propertyType) {
		return true
	}

	return p.recursive && bytes.Contains(line, locationPattern)
}

// hasTypeValue tells whether the line holds the type field with the
// given value.
//
// The field is looked for by its name, and the value that follows it
// is read. The spaces around the colon do not matter: the official
// dump writes the field without any, and a mirror that writes it with
// some must not be skipped.
//
// The search is a byte search, and the field is read a few bytes at a
// time. It is much cheaper than decoding the line, and it is what
// keeps the pass fast over a dump of a hundred gigabytes.
func hasTypeValue(line []byte, value string) bool {
	needle := []byte(typeField)

	for offset := 0; offset < len(line); {
		index := bytes.Index(line[offset:], needle)
		if index < 0 {
			return false
		}

		// The position after the name of the field.
		position := offset + index + len(needle)
		offset = position

		// The colon, with the spaces that may be around it.
		position = skipSpaces(line, position)
		if position >= len(line) || line[position] != ':' {
			continue
		}
		position = skipSpaces(line, position+1)

		// The value, which is a string.
		if position >= len(line) || line[position] != '"' {
			continue
		}
		position++

		end := position
		for end < len(line) && line[end] != '"' {
			end++
		}
		if end >= len(line) {
			return false
		}

		if string(line[position:end]) == value {
			return true
		}
	}

	return false
}

// skipSpaces returns the position of the first byte that is not a
// space.
func skipSpaces(line []byte, position int) int {
	for position < len(line) {
		switch line[position] {
		case ' ', '\t', '\r', '\n':
			position++
		default:
			return position
		}
	}

	return position
}

// Run reads the dump from r and writes the results.
//
// It returns when the input is exhausted, when the context is cancelled
// or when the output fails. A damaged line is not a failure: it is
// logged, counted and skipped, and the run carries on.
//
// In the recursive mode the items file is written twice: once with the
// items that point at the target, and once with the items that are
// inside the target. The second file is the one that is kept.
func (p *Pipeline) Run(ctx context.Context, r io.Reader) (Stats, error) {
	var stats Stats
	var isArray bool

	// A compressed stream is refused before anything is read: the
	// decompressor is another process, and the dump is piped in.
	//
	// Without this, every line of a compressed stream is reported as
	// malformed, and the run ends with two empty files and a count
	// that looks like a damaged dump rather than a wrong command.
	reader, err := DetectCompression(r)
	if err != nil {
		return stats, err
	}

	// The dump may be written as one JSON array instead of JSON Lines.
	// The framing is removed here, so that the scanner sees the same
	// lines in both forms.
	reader, isArray = StripArrayIfNeeded(reader)

	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, ScanBufferSize), MaxScanBufferSize)

	for scanner.Scan() {
		// The context is honoured on every line, so that a cancelled
		// run stops promptly instead of reading the whole dump.
		if err := ctx.Err(); err != nil {
			return stats, err
		}

		line := scanner.Bytes()

		// The closing bracket of a JSON array is not an entity, and
		// it is not counted: the two forms of the dump produce the
		// same statistics for the same entities.
		if isArray && IsArrayClose(line) {
			continue
		}

		stats.Scanned++
		stats.Bytes += int64(len(line))

		// A trailing comma is not valid JSON, so it is removed
		// whatever the form of the dump is: the entities of a JSON
		// array are separated by commas, and a line that ends with
		// one is an entity and not a value.
		line = TrimEntityComma(line)

		if err := p.processLine(line, &stats); err != nil {
			return stats, err
		}

		if stats.Scanned%ProgressInterval == 0 {
			p.report(stats)
		}
	}

	if err := scanner.Err(); err != nil {
		// A line that does not fit in the buffer stops the scan. The
		// run has still produced everything that came before it.
		return stats, fmt.Errorf("read the dump: %w", err)
	}

	return stats, nil
}

// Finish completes the run.
//
// In the recursive mode it walks the administrative hierarchy from the
// target and keeps only the items that are inside it. In the plain
// mode it does nothing: the items were written as they were read.
//
// It is called after Run. It flushes the items on its own, because it
// reads the items files back.
//
// The recursive mode writes the items file twice. The first pass writes
// the items that point at the target, and the second one writes the
// items that are inside it. The second file replaces the first one.
//
// The filter is a stream: the items file is read a line at a time and
// the lines that are not reachable are dropped. Holding the items in
// memory would cost more than the hierarchy does, and the items file
// of a full run is large.
//
// Every target is filtered on its own. The hierarchies of two targets
// are separate: Shanghai and New York have nothing in common, and an
// item that is inside one of them is not inside the other.
func (p *Pipeline) Finish() error {
	if !p.recursive {
		return nil
	}

	for _, target := range p.targets {
		if !target.Recursive() {
			continue
		}

		if err := p.finishTarget(target); err != nil {
			return err
		}
	}

	return nil
}

// finishTarget walks the hierarchy of one target and filters its file.
func (p *Pipeline) finishTarget(target *Target) error {
	hierarchy := target.Hierarchy()
	if hierarchy == nil || target.ItemsPath == "" {
		return nil
	}

	// The items are read back from the file, so they have to reach it
	// first.
	if err := target.Flush(); err != nil {
		return err
	}

	reachable := hierarchy.Reachable(target.QID)

	// The items are filtered even when nothing was reached: the pass
	// kept every item that is located somewhere, and the ones that
	// are not inside the target have to go.
	kept, err := p.filterItems(target.ItemsPath, reachable)
	if err != nil {
		return err
	}

	if p.log != nil {
		p.log.Info("the administrative hierarchy was walked",
			"target", target.Name(),
			"entities", hierarchy.Len(),
			"edges", hierarchy.Edges(),
			"reachable", len(reachable),
			"kept", kept)
	}

	return nil
}

// filterItems rewrites the items file, keeping the reachable ones.
//
// The file is written to a temporary name and then put in place of the
// original, so that a failure in the middle leaves the original file
// alone.
func (p *Pipeline) filterItems(itemsPath string, reachable map[string]struct{}) (int, error) {
	source, err := os.Open(itemsPath)
	if err != nil {
		return 0, fmt.Errorf("read the items: %w", err)
	}
	defer source.Close()

	temporary := itemsPath + ".tmp"
	target, err := os.Create(temporary)
	if err != nil {
		return 0, fmt.Errorf("write the items: %w", err)
	}
	defer target.Close()

	writer := NewEntityWriter(target)
	scanner := bufio.NewScanner(source)
	scanner.Buffer(make([]byte, 0, ScanBufferSize), MaxScanBufferSize)

	kept := 0
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}

		var entity Entity
		if err := json.Unmarshal(line, &entity); err != nil {
			// A line that was written by this run is valid JSON.
			// One that is not is a damaged file, and it is
			// dropped rather than stopping the filter.
			continue
		}

		if _, ok := reachable[entity.ID]; !ok {
			continue
		}

		if err := writer.Write(entity); err != nil {
			return 0, fmt.Errorf("write the item %s: %w", entity.ID, err)
		}
		kept++
	}

	if err := scanner.Err(); err != nil {
		return 0, fmt.Errorf("read the items: %w", err)
	}
	if err := writer.Flush(); err != nil {
		return 0, err
	}
	if err := target.Close(); err != nil {
		return 0, fmt.Errorf("write the items: %w", err)
	}

	if err := os.Rename(temporary, itemsPath); err != nil {
		return 0, fmt.Errorf("replace the items: %w", err)
	}

	return kept, nil
}

// report tells the reporter and the event writer about the progress.
func (p *Pipeline) report(stats Stats) {
	if p.reporter != nil {
		p.reporter.Report(stats)
	}

	if p.events == nil || !p.events.Enabled() {
		return
	}

	_ = p.events.Progress(ProgressEvent{
		ProcessedLines:      stats.Scanned,
		MatchedItems:        stats.Items,
		CollectedProperties: stats.Properties,
		SpeedMBs:            p.speed(stats),
	})
}

// speed returns the rate of the run, in megabytes per second.
//
// It is zero before the first second has passed: a rate computed over
// a few milliseconds is noise, and a bar that jumps about is worse
// than a bar that moves slowly.
func (p *Pipeline) speed(stats Stats) float64 {
	elapsed := time.Since(p.started).Seconds()
	if elapsed < 1 {
		return 0
	}

	return float64(stats.Bytes) / (1024 * 1024) / elapsed
}

// Flush writes the buffered records to the underlying writers.
//
// It is called at the end of the run, so that the last batch reaches
// the files as well.
func (p *Pipeline) Flush() error {
	for _, target := range p.targets {
		if err := target.Flush(); err != nil {
			return err
		}
	}

	if !p.writeProperties {
		return nil
	}

	return p.properties.Flush()
}

// processLine processes one line of the dump.
func (p *Pipeline) processLine(line []byte, stats *Stats) error {
	if len(line) == 0 {
		return nil
	}

	// An entity is a JSON object. A line that does not even look like
	// one is damaged, and it is reported: a dump that arrived
	// truncated or that is not a dump at all has to be seen.
	//
	// The check is two bytes, and it is what keeps the filter below
	// from hiding the damage.
	if !looksLikeObject(line) {
		p.logMalformed(stats, "the line is not a JSON object")
		stats.Malformed++
		return nil
	}

	// The fast path.
	//
	// Decoding a line costs about ten microseconds, and most of what
	// it decodes is thrown away: an item is only kept when one of its
	// statements points at the target, and a property is always kept.
	//
	// The identifiers are written verbatim in the dump, so a byte
	// search tells whether a line can contribute at all. A line that
	// mentions neither the target nor a property declaration is not
	// read, which is most of the dump.
	//
	// In the recursive mode the search is wider: an item that is
	// located somewhere contributes too, and such an item does not
	// have to mention the target. The P131 statement is what marks
	// it, and the property is in the line.
	if !p.canContribute(line) {
		stats.Skipped++
		return nil
	}

	var raw rawEntity
	if err := json.Unmarshal(line, &raw); err != nil {
		p.logMalformed(stats, "the line is not valid JSON")
		stats.Malformed++
		return nil
	}

	switch raw.Type {
	case TypeItem:
		return p.processItem(&raw, stats)
	case TypeProperty:
		return p.processProperty(&raw, stats)
	default:
		// A lexeme, a media information record and anything else
		// the dump holds is not an entity of this dataset.
		stats.Skipped++
		return nil
	}
}

// processItem writes an item to the file of every target it belongs to.
//
// In the recursive mode the rule is wider: an item that is located
// somewhere is kept as well, whatever it points at. The hierarchy is
// walked after the pass, and the walk needs the whole chain: an item
// that is located in a city that is located in the target is inside
// the target, and the item that links to the target is not enough to
// find it.
//
// The items that are not reachable are dropped when the pass is over,
// so the wider rule does not make the output bigger.
//
// An item may belong to several targets: it is written to each of
// their files, and it is counted once.
func (p *Pipeline) processItem(raw *rawEntity, stats *Stats) error {
	matched := false

	for _, target := range p.targets {
		keep := target.Matches(raw)
		if !keep && target.Recursive() {
			// The target itself is kept, and so is every item
			// that is located somewhere: the walk needs the
			// whole chain.
			keep = raw.ID == target.QID || hasLocationClaim(raw)
		}

		if !keep {
			continue
		}

		trimmed := TrimEntity(raw, p.trim)

		if err := target.Write(trimmed); err != nil {
			return err
		}

		// The edges of the hierarchy are collected as well, so
		// that the entities inside the target can be found after
		// the pass.
		target.Collect(trimmed)

		matched = true
	}

	if !matched {
		// The item was read and it does not point at any target: it
		// is not kept, and it is counted with the lines that were
		// passed over.
		stats.Skipped++
		return nil
	}

	stats.Items++

	return nil
}

// hasLocationClaim tells whether an item is located somewhere.
//
// It is the P131 statement that makes an item part of the
// administrative hierarchy.
func hasLocationClaim(raw *rawEntity) bool {
	_, ok := raw.Claims[PropertyLocatedIn]
	return ok
}

// processProperty writes a property.
//
// Every property of the dump is kept: the property dictionary is the
// catalogue the panel reads the names of the properties from, so it
// holds all of them and not only the ones on the location allowlist.
//
// The dictionary is the same for every target, so it is written once
// per run and not once per target.
func (p *Pipeline) processProperty(raw *rawEntity, stats *Stats) error {
	if !p.writeProperties {
		// The run was asked for the items only. The property is
		// not written, and it is not counted: it was not
		// produced.
		stats.Skipped++
		return nil
	}

	trimmed := TrimEntity(raw, p.trim)
	if err := p.properties.Write(trimmed); err != nil {
		return fmt.Errorf("write the property %s: %w", raw.ID, err)
	}
	stats.Properties++

	return nil
}

// MaxMalformedReports is the number of damaged lines that are reported
// individually.
//
// A dump with thousands of damaged lines is a wrong input, not a few
// bad records. Reporting every one of them buries the summary in
// warnings, so the reports stop and the count carries the rest.
const MaxMalformedReports = 10

// logMalformed reports a line that could not be processed.
//
// The line itself is not logged: a damaged line can be a whole entity,
// and the dump is full of them. The number of the line is enough to
// find it again in the dump.
//
// Only the first few are reported. After that, one line says that the
// rest are being counted and not reported.
func (p *Pipeline) logMalformed(stats *Stats, reason string) {
	if p.log == nil {
		return
	}

	if stats.Malformed > MaxMalformedReports {
		if stats.Malformed == MaxMalformedReports+1 {
			p.log.Warn("the damaged lines are counted and not reported any more",
				"reason", reason,
				"reported", MaxMalformedReports)
		}
		return
	}

	p.log.Warn("the line was skipped",
		"reason", reason,
		"line", stats.Scanned)
}
