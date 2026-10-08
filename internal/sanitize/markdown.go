// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package sanitize

import "strings"

// markdownMarkup is every character Markdown reads as markup inside a line. It
// is the one list both directions are built from, so an escape can never be
// left that the unescape does not undo.
const markdownMarkup = "\\`[]()*_~|#>"

// EscapeMarkdown backslash-escapes every character Markdown reads as markup
// inside a line, so text anyone can write — a pull request's title, an issue's
// summary — is shown as written rather than read as emphasis, code or a link.
func EscapeMarkdown(text string) string {
	return markdownReplacer(func(character string) (string, string) { return character, `\` + character }).
		Replace(text)
}

// UnescapeMarkdown undoes EscapeMarkdown, for a reader that shows no Markdown
// and would show its backslashes.
func UnescapeMarkdown(text string) string {
	return markdownReplacer(func(character string) (string, string) { return `\` + character, character }).
		Replace(text)
}

// replacementPair is the length of one replacement strings.NewReplacer takes:
// what to find, and what to put in its place.
const replacementPair = 2

// markdownReplacer replaces each of markdownMarkup's characters as pair says.
func markdownReplacer(pair func(character string) (string, string)) *strings.Replacer {
	pairs := make([]string, 0, replacementPair*len(markdownMarkup))

	for _, character := range markdownMarkup {
		found, replaced := pair(string(character))
		pairs = append(pairs, found, replaced)
	}

	return strings.NewReplacer(pairs...)
}
