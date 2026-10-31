package etl

import (
	"bytes"
	"fmt"
	"log/slog"
)

// Target is one entity the items are extracted for.
//
// A target owns everything that is specific to it: the identifier, the
// file the items are written to, the pattern that finds it in the dump,
// and the administrative hierarchy when the recursive mode is on.
//
// The pipeline holds one target per extraction. A run over one target
// and a run over several are the same code: a single target is the
// degenerate case of several, and there is no separate path to keep
// honest.
type Target struct {
	// QID is the identifier of the entity, such as Q90.
	QID string
	// Label is the name of the target, used in the messages and in
	// the name of the file when the file is derived from the target.
	//
	// It is empty when the target has no name, and the identifier is
	// used instead.
	Label string
	// ItemsPath is the file the items are written to.
	//
	// It is empty when the run writes no items for this target.
	ItemsPath string
	// items writes the items that are linked to the target.
	items *EntityWriter
	// pattern is the target as it is written in the dump, with the
	// quotes of the JSON string.
	pattern []byte
	// recursive tells whether the administrative hierarchy is walked
	// from this target.
	recursive bool
	// hierarchy holds the edges of the administrative hierarchy.
	//
	// It is only built in the recursive mode.
	hierarchy *Hierarchy
	// kept is the number of items that were written.
	kept int64
}

// NewTarget creates a target.
//
// The writer is created over the given one, which may be nil when the
// run writes no items for this target.
func NewTarget(qid, label, itemsPath string, items *EntityWriter, recursive bool) *Target {
	target := &Target{
		QID:       qid,
		Label:     label,
		ItemsPath: itemsPath,
		items:     items,
		pattern:   []byte(`"` + qid + `"`),
		recursive: recursive,
	}

	if recursive {
		target.hierarchy = NewHierarchy()
	}

	return target
}

// Name returns the name of the target.
//
// The label is used when there is one, and the identifier otherwise. It
// is what the messages and the file names carry.
func (t *Target) Name() string {
	if t.Label != "" {
		return t.Label
	}
	return t.QID
}

// Matches tells whether the entity is linked to the target.
//
// It is the plain rule: the entity carries a statement that points at
// the target.
func (t *Target) Matches(raw *rawEntity) bool {
	return LinksToTarget(raw, t.QID)
}

// Collect records an item that was written.
//
// In the recursive mode the edges of the hierarchy are collected as
// well, so that the entities inside the target can be found after the
// pass.
func (t *Target) Collect(entity Entity) {
	t.kept++

	if t.hierarchy != nil {
		t.hierarchy.Add(entity)
	}
}

// Write writes an item to the file of the target.
//
// It does nothing when the target writes no items: the run may have
// been asked for the properties only.
func (t *Target) Write(entity Entity) error {
	if t.items == nil {
		return nil
	}

	if err := t.items.Write(entity); err != nil {
		return fmt.Errorf("write the item %s: %w", entity.ID, err)
	}

	return nil
}

// Flush writes the buffered items of the target.
func (t *Target) Flush() error {
	if t.items == nil {
		return nil
	}
	return t.items.Flush()
}

// Kept returns the number of items that were written for the target.
func (t *Target) Kept() int64 {
	return t.kept
}

// Recursive tells whether the administrative hierarchy is walked from
// this target.
func (t *Target) Recursive() bool {
	return t.recursive
}

// Hierarchy returns the hierarchy of the target.
//
// It is nil when the recursive mode is off.
func (t *Target) Hierarchy() *Hierarchy {
	return t.hierarchy
}

// containsAny tells whether the line holds one of the patterns.
//
// It is the fast path of a run over several targets: the line is
// skipped when it mentions none of them, and the search stops at the
// first one that it does.
func containsAny(line []byte, patterns [][]byte) bool {
	for _, pattern := range patterns {
		if bytes.Contains(line, pattern) {
			return true
		}
	}
	return false
}

// logTarget reports what a target produced.
//
// It is the per-target line of the summary of a run over several
// targets.
func logTarget(log *slog.Logger, target *Target) {
	if log == nil {
		return
	}

	log.Info("the target was extracted",
		"target", target.Name(),
		"qid", target.QID,
		"items", target.Kept(),
	)
}
