// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

// Package sanitize keeps text that a server or another program chose from
// reaching the terminal as control sequences.
//
// Everything workflow shows from outside itself — an issue summary, Jira's
// reason for a refusal, a commit subject, a hook's output — is written to a
// terminal that obeys escape sequences: clear the screen, draw over what is
// already there, set the window title, write the clipboard. Anyone able to edit
// a Jira summary could otherwise do those things to everyone who reads it.
package sanitize

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/charmbracelet/x/ansi"
)

// Code points with a meaning to a terminal, or to the order text is drawn in.
const (
	// firstPrintable is the first code point after the C0 controls.
	firstPrintable = 0x20
	// deleteControl is DEL, and c1Last ends the C1 controls it begins.
	deleteControl = 0x7f
	c1Last        = 0x9f
	// The bidirectional embeddings, overrides and isolates: they reorder what
	// is drawn, so a line can read differently from what it says.
	bidiEmbeddingFirst = 0x202a
	bidiEmbeddingLast  = 0x202e
	bidiIsolateFirst   = 0x2066
	bidiIsolateLast    = 0x2069
	// carriageReturn returns the cursor over what the line already shows.
	carriageReturn = 0x0d
)

// escapeLength is the length of a JSON \u escape: the backslash, the u, and
// four hex digits. shortEscapeLength is every other escape: \n, \", \\ and
// the rest.
const (
	escapeLength      = 6
	shortEscapeLength = 2
)

// hexBase is the base a JSON \u escape's digits are written in.
const hexBase = 16

// replacementEscape is U+FFFD written as a JSON escape, the same length as the
// escape it stands in for.
func replacementEscape() []byte {
	return []byte{'\\', 'u', 'f', 'f', 'f', 'd'}
}

// JSON neutralizes control characters in a JSON document before it is decoded.
//
// It works on the encoded form because that is the one place every field of
// every answer passes through: a field added to a struct next year is covered
// without anyone remembering to. JSON can carry a control character only as an
// escape, so the escapes are what change; raw DEL, C1 and bidirectional
// characters, which JSON allows unescaped, are replaced as well, as is invalid
// UTF-8. A carriage return is dropped rather than replaced, so a Windows line
// ending leaves a plain newline. A malformed escape is passed through for the
// decoder to reject.
func JSON(raw []byte) []byte {
	clean := make([]byte, 0, len(raw))

	for index := 0; index < len(raw); {
		if raw[index] == '\\' {
			consumed, written := escape(raw[index:])
			clean = append(clean, written...)
			index += consumed

			continue
		}

		// An invalid byte becomes U+FFFD too, which is what the decoder would
		// make of it anyway. Copied as it is, two invalid bytes either side of
		// a dropped \r would join into a valid control: the fuzzer found
		// 0xC2, \r, 0x97 decoding to U+0097.
		character, size := utf8.DecodeRune(raw[index:])
		if character == utf8.RuneError || (character >= deleteControl && neutralized(character)) {
			clean = utf8.AppendRune(clean, utf8.RuneError)
		} else {
			clean = append(clean, raw[index:index+size]...)
		}

		index += size
	}

	return clean
}

// escape rewrites the escape at the start of encoded, reporting how many bytes
// it consumed and what to write in their place.
func escape(encoded []byte) (int, []byte) {
	if len(encoded) < shortEscapeLength {
		return len(encoded), encoded
	}

	switch encoded[1] {
	case 'u':
		return unicodeEscape(encoded)
	case 'r':
		return shortEscapeLength, nil
	case 'b', 'f':
		return shortEscapeLength, replacementEscape()
	default:
		// \" \\ \/ \n \t, or something malformed. Copying both bytes keeps an
		// escaped backslash from being read as the start of the next escape.
		return shortEscapeLength, encoded[:shortEscapeLength]
	}
}

// unicodeEscape rewrites a \u escape.
func unicodeEscape(encoded []byte) (int, []byte) {
	code, ok := hexDigits(encoded)
	if !ok {
		return shortEscapeLength, encoded[:shortEscapeLength]
	}

	switch {
	case code == carriageReturn:
		return escapeLength, nil
	case neutralized(code):
		return escapeLength, replacementEscape()
	default:
		return escapeLength, encoded[:escapeLength]
	}
}

// hexDigits reads the four digits of a \u escape.
func hexDigits(encoded []byte) (rune, bool) {
	if len(encoded) < escapeLength {
		return 0, false
	}

	var code rune

	for _, digit := range encoded[shortEscapeLength:escapeLength] {
		value, ok := hexValue(digit)
		if !ok {
			return 0, false
		}

		code = code*hexBase + value
	}

	return code, true
}

// hexValue is the value of one hex digit.
func hexValue(digit byte) (rune, bool) {
	const decimalDigits = 10

	switch {
	case digit >= '0' && digit <= '9':
		return rune(digit - '0'), true
	case digit >= 'a' && digit <= 'f':
		return rune(digit-'a') + decimalDigits, true
	case digit >= 'A' && digit <= 'F':
		return rune(digit-'A') + decimalDigits, true
	default:
		return 0, false
	}
}

// Text neutralizes control characters in text a program wrote, such as a hook's
// output or a commit subject. Terminal sequences are removed whole, so colored
// output stays readable; what is left of a control becomes U+FFFD, and a
// carriage return is dropped.
func Text(text string) string {
	stripped := ansi.Strip(strings.ToValidUTF8(text, string(utf8.RuneError)))

	var clean strings.Builder

	for _, character := range stripped {
		switch {
		case character == carriageReturn:
		case neutralized(character):
			clean.WriteRune(utf8.RuneError)
		default:
			clean.WriteRune(character)
		}
	}

	return clean.String()
}

// Line is Text for a name: a file, a branch, anything drawn on one line that has
// to mean what it shows. On top of what Text takes out, a line break or a tab
// becomes U+FFFD, and so does every character that has no shape of its own: the
// zero-width ones, the direction marks, the tag characters.
//
// Prose keeps all of those, which is why Text leaves them alone: they join an
// emoji, spell a flag, and hold the letters of some scripts apart. In a name
// they can only make it read as something it is not.
func Line(text string) string {
	var clean strings.Builder

	for _, character := range Text(text) {
		if unseen(character) {
			clean.WriteRune(utf8.RuneError)

			continue
		}

		clean.WriteRune(character)
	}

	return clean.String()
}

// unseen reports a character that takes no space of its own on a line, or that
// ends the line: Unicode's format characters and its two separators, and the
// newline and tab that Text lets through.
func unseen(character rune) bool {
	return character == '\n' || character == '\t' || unicode.In(character, unicode.Cf, unicode.Zl, unicode.Zp)
}

// neutralized reports a code point that must not reach the terminal. Newline and
// tab are text; every other control, and every bidirectional override, is not.
func neutralized(character rune) bool {
	switch {
	case character == '\n', character == '\t':
		return false
	case character < firstPrintable, character >= deleteControl && character <= c1Last:
		return true
	case character >= bidiEmbeddingFirst && character <= bidiEmbeddingLast,
		character >= bidiIsolateFirst && character <= bidiIsolateLast:
		return true
	default:
		return false
	}
}
