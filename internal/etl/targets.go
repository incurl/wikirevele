package etl

import (
	"fmt"
	"path/filepath"
	"strings"
)

// TargetSpec is one target of a run, with the file its items go to.
//
// It is the resolved form of a target: the identifier, the name and
// the path, with the defaults filled in. The pipeline works on it.
type TargetSpec struct {
	// QID is the identifier of the entity, such as Q90.
	QID string
	// Label is the name of the target, such as "shanghai".
	//
	// It is empty when the target has no name, and the identifier is
	// used instead.
	Label string
	// ItemsPath is the file the items are written to.
	ItemsPath string
}

// Name returns the name of the target.
//
// The label is used when there is one, and the identifier otherwise.
func (s TargetSpec) Name() string {
	if s.Label != "" {
		return s.Label
	}
	return s.QID
}

// Slug returns the name of the target as it is used in a file name.
//
// It is the label when there is one, and the identifier otherwise,
// lowercased. A label that holds characters a file name should not
// hold is reduced to the ones that are safe.
func (s TargetSpec) Slug() string {
	source := s.Label
	if source == "" {
		source = s.QID
	}

	return slugify(source)
}

// slugify turns a name into something a file name can hold.
//
// The letters, the digits, the dashes and the underscores are kept, and
// everything else becomes a dash. The result never starts or ends with
// a dash, so that the name reads well in a path.
func slugify(value string) string {
	var out strings.Builder

	lastDash := true
	for _, r := range strings.ToLower(value) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			out.WriteRune(r)
			lastDash = false
		case r == '-' || r == '_':
			out.WriteRune(r)
			lastDash = false
		default:
			if !lastDash {
				out.WriteRune('-')
				lastDash = true
			}
		}
	}

	return strings.Trim(out.String(), "-")
}

// ResolveTargets works out the targets of a run and the file of each.
//
// The rules are:
//   - the identifiers are the ones the user gave, in order;
//   - the names are the ones the user gave, in order, and the
//     identifier is used for the ones that have no name;
//   - the file of a target is the one the user gave for it, and the
//     file of the first target is the one the user gave in general;
//   - the file of the other targets is derived from that one, with the
//     name of the target in it.
//
// The derivation matters because a run over several targets writes
// several files, and the user should not have to name every one of
// them. "paris-wikidata-subset.ndjson" with a target named "shanghai"
// becomes "shanghai-wikidata-subset.ndjson": the suffix of the name is
// kept, and the target takes its place.
func ResolveTargets(opts CommandOptions) ([]TargetSpec, error) {
	qids := cleanQIDs(opts.TargetQIDs)
	if len(qids) == 0 {
		return nil, fmt.Errorf("etl: at least one target is required")
	}

	labels := opts.TargetLabels
	if len(labels) > len(qids) {
		return nil, fmt.Errorf(
			"etl: %d names were given for %d targets",
			len(labels), len(qids))
	}

	// The files the user named for the targets, if any.
	paths := opts.OutItemsPerTarget
	if len(paths) > len(qids) {
		return nil, fmt.Errorf(
			"etl: %d item files were given for %d targets",
			len(paths), len(qids))
	}

	base := opts.OutItems
	if base == "" && len(paths) > 0 {
		base = paths[0]
	}

	targets := make([]TargetSpec, 0, len(qids))
	for i, qid := range qids {
		spec := TargetSpec{QID: qid}

		if i < len(labels) {
			spec.Label = strings.TrimSpace(labels[i])
		}

		// The items of a target have no file when the run writes the
		// property dictionary only: there is nothing to write them
		// to.
		if opts.WritesItems() {
			switch {
			case i < len(paths) && paths[i] != "":
				spec.ItemsPath = paths[i]
			case len(qids) == 1:
				// A single target keeps the file the user named, as
				// it always did.
				spec.ItemsPath = base
			default:
				spec.ItemsPath = DeriveItemsPath(base, spec)
			}
		}

		targets = append(targets, spec)
	}

	return targets, nil
}

// DeriveItemsPath builds the file of a target from the file of the
// first one.
//
// The extension is kept, and the name of the target replaces the name
// of the first target: "paris-wikidata-subset.ndjson" becomes
// "shanghai-wikidata-subset.ndjson" for the target named "shanghai".
//
// The stem of the file is split on its first dash: the part before it
// is the name of the first target, and the part after it is the suffix
// that every target shares. A file that cannot be split keeps its
// whole stem, and the name of the target is put in front of it.
func DeriveItemsPath(base string, target TargetSpec) string {
	if base == "" {
		return ""
	}

	extension := filepath.Ext(base)
	stem := strings.TrimSuffix(base, extension)

	prefix, suffix, split := strings.Cut(stem, "-")
	if !split || prefix == "" {
		// The stem holds no name to replace. The target is put
		// in front of it, so that the files of two targets do
		// not collide.
		if stem == "" {
			return target.Slug() + extension
		}
		return target.Slug() + "-" + stem + extension
	}

	return target.Slug() + "-" + suffix + extension
}

// cleanQIDs trims the identifiers and drops the empty ones.
//
// The order is kept, and the duplicates are dropped: the same target
// twice would write the same file twice.
func cleanQIDs(qids []string) []string {
	seen := make(map[string]struct{}, len(qids))
	out := make([]string, 0, len(qids))

	for _, qid := range qids {
		cleaned := strings.TrimSpace(qid)
		if cleaned == "" {
			continue
		}
		if _, ok := seen[cleaned]; ok {
			continue
		}
		seen[cleaned] = struct{}{}
		out = append(out, cleaned)
	}

	return out
}

// WritesItems tells whether the run writes the items.
//
// The run writes the items unless it was asked for the property
// dictionary only.
func (o CommandOptions) WritesItems() bool {
	return !o.PropertiesOnly
}

// WritesProperties tells whether the run writes the property
// dictionary.
//
// The run writes the dictionary unless it was asked for the items
// only.
func (o CommandOptions) WritesProperties() bool {
	return !o.ItemsOnly
}
