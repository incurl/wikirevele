package etl

import (
	"encoding/json"
	"strings"

	"github.com/incurl/wikirevele/internal/wikidata/schema"
)

// The languages the trimmed entities keep.
//
// The application shows an English interface, and French is the other
// language of the data it is about. Every other language is dropped:
// the dump holds a label in more than three hundred languages, and
// keeping them all would multiply the size of the output by a factor
// that no part of the application needs.
const (
	// LabelLanguageEnglish is the language of the labels and of the
	// descriptions.
	LabelLanguageEnglish = "en"
	// LabelLanguageFrench is the second language of the labels.
	LabelLanguageFrench = "fr"
)

// TrimOptions says which languages to keep.
//
// The three lists are separate because the panel shows the labels, the
// descriptions and the aliases differently: a fallback label in
// another language is useful, a description in another language is
// noise.
type TrimOptions struct {
	// LabelLanguages are the languages of the labels to keep.
	LabelLanguages []string
	// DescriptionLanguages are the languages of the descriptions to
	// keep.
	DescriptionLanguages []string
	// AliasLanguages are the languages of the aliases to keep.
	AliasLanguages []string
}

// DefaultItemTrim is the trimming of an item.
//
// An item keeps its English and its French labels, its English
// description and its English aliases.
func DefaultItemTrim() TrimOptions {
	return TrimOptions{
		LabelLanguages:       []string{LabelLanguageEnglish, LabelLanguageFrench},
		DescriptionLanguages: []string{LabelLanguageEnglish},
		AliasLanguages:       []string{LabelLanguageEnglish},
	}
}

// DefaultPropertyTrim is the trimming of a property.
//
// A property keeps its English label and its English description. It
// has no aliases in the dump, and the field is left out when it is
// empty.
func DefaultPropertyTrim() TrimOptions {
	return TrimOptions{
		LabelLanguages:       []string{LabelLanguageEnglish},
		DescriptionLanguages: []string{LabelLanguageEnglish},
	}
}

// TrimOptionsFor returns the trimming that keeps the given languages.
//
// The languages are kept for the labels, the descriptions and the
// aliases alike: the user asked for those languages, and the panel
// falls back to another language when a term is missing.
//
// The list is cleaned first: the codes are trimmed, the empty ones are
// dropped and the duplicates are removed. A list that names the same
// language twice would keep the same term twice, which is harmless but
// wasteful.
//
// An empty list means the defaults: the English and the French.
func TrimOptionsFor(languages []string) TrimOptions {
	cleaned := cleanLanguages(languages)
	if len(cleaned) == 0 {
		return DefaultItemTrim()
	}

	return TrimOptions{
		LabelLanguages:       cleaned,
		DescriptionLanguages: cleaned,
		AliasLanguages:       cleaned,
	}
}

// cleanLanguages trims the codes, drops the empty ones and removes the
// duplicates.
//
// The order is kept: it is the order the user gave, and the first
// language is the one the panel shows first.
func cleanLanguages(languages []string) []string {
	seen := make(map[string]struct{}, len(languages))
	out := make([]string, 0, len(languages))

	for _, language := range languages {
		code := strings.TrimSpace(language)
		if code == "" {
			continue
		}
		if _, ok := seen[code]; ok {
			continue
		}
		seen[code] = struct{}{}
		out = append(out, code)
	}

	return out
}

// TrimEntity trims a raw entity down to the fields that are kept.
//
// The claims are kept whole. A statement carries a rank, its
// qualifiers and sometimes its references, and the application reads
// all of them, so there is nothing to trim inside a claim.
//
// The sitelinks, the references and the other languages are not
// declared in the raw entity, so they never reach this point.
func TrimEntity(raw *rawEntity, opts TrimOptions) Entity {
	trimmed := Entity{
		ID:           raw.ID,
		Type:         raw.Type,
		Labels:       trimTerms(raw.Labels, opts.LabelLanguages),
		Descriptions: trimTerms(raw.Descriptions, opts.DescriptionLanguages),
		Claims:       raw.Claims,
	}

	if len(trimmed.Claims) == 0 {
		trimmed.Claims = map[string]json.RawMessage{}
	}

	aliases := trimAliases(raw.Aliases, opts.AliasLanguages)
	if len(aliases) > 0 {
		trimmed.Aliases = aliases
	}

	return trimmed
}

// trimTerms keeps the terms of the given languages.
//
// A term without a language is dropped: it is a damaged record, and a
// label with no language cannot be shown.
func trimTerms(terms map[string]schema.Term, languages []string) map[string]schema.Term {
	out := make(map[string]schema.Term, len(languages))

	for _, language := range languages {
		term, ok := terms[language]
		if !ok {
			continue
		}
		out[language] = term
	}

	return out
}

// trimAliases keeps the aliases of the given languages.
func trimAliases(aliases map[string][]schema.Term, languages []string) map[string][]schema.Term {
	out := make(map[string][]schema.Term, len(languages))

	for _, language := range languages {
		terms, ok := aliases[language]
		if !ok || len(terms) == 0 {
			continue
		}
		out[language] = terms
	}

	return out
}
