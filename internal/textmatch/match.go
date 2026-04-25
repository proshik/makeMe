// Package textmatch provides normalization and matching utilities for typing answers.
//
// It is used by both /tasks (typing vocab subtypes) and /words (typing modes) to
// validate user-typed translations. Validation is forgiving on case, whitespace,
// trailing punctuation, the Russian ё/е fold, and supports ";"- or ","-separated
// alternatives in the expected string (e.g. "история, рассказ").
package textmatch

import (
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// Normalize folds a string to its canonical comparable form:
//   - Unicode NFC
//   - lowercase
//   - leading/trailing whitespace trimmed
//   - internal whitespace collapsed to a single space
//   - trailing sentence punctuation (.,!?;:) stripped
//   - Russian ё → е (extremely common typo, safer to fold)
func Normalize(s string) string {
	s = norm.NFC.String(s)
	s = strings.ToLower(s)
	s = strings.TrimSpace(s)
	s = collapseWhitespace(s)
	s = strings.TrimRight(s, ".,!?;:")
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "ё", "е")
	return s
}

func collapseWhitespace(s string) string {
	var b strings.Builder
	prevSpace := false
	for _, r := range s {
		if unicode.IsSpace(r) {
			if !prevSpace {
				b.WriteRune(' ')
			}
			prevSpace = true
			continue
		}
		b.WriteRune(r)
		prevSpace = false
	}
	return b.String()
}

// Match reports whether `input` is an acceptable answer for `expected`.
//
// `expected` may contain multiple acceptable answers separated by either
// ";" or "," (e.g. "история, рассказ" or "house; home"). Both `input` and
// each alternative are normalized before comparison.
//
// As a fallback the whole expected string is also tried, so a user who
// types the full multi-translation phrase ("история, рассказ") is still
// counted correct.
func Match(input, expected string) bool {
	n := Normalize(input)
	if n == "" {
		return false
	}
	if Normalize(expected) == n {
		return true
	}
	for _, alt := range splitAlternatives(expected) {
		if Normalize(alt) == n {
			return true
		}
	}
	return false
}

// splitAlternatives splits expected on either ";" or "," and drops empty pieces.
func splitAlternatives(s string) []string {
	return strings.FieldsFunc(s, func(r rune) bool {
		return r == ';' || r == ','
	})
}
