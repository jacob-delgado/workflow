// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package codeowners

import "strings"

// This file is Ruby's File.fnmatch? with FNM_PATHNAME | FNM_DOTMATCH, which
// GitLab matches CODEOWNERS patterns with, ported from Ruby's dir.c (fnmatch,
// fnmatch_helper and bracket) step for step rather than mapped onto
// path.Match: the two disagree on a doubled slash, on classes such as [a-],
// [!] and one spanning a slash, and on a trailing backslash, and GitLab's
// answer is Ruby's. It works on runes, as Ruby does on a UTF-8 string, and
// reads a NUL as the end of either string, as C does.

// fnmatch reports whether path matches pattern, segment by segment. A **/
// starting a segment remembers where it was; when the rest fails, it takes one
// more of the path's segments and tries again.
func fnmatch(pattern, path string) bool {
	text, name := []rune(pattern), []rune(path)
	cursor, from, globAt, globFrom := 0, 0, -1, -1

	for {
		if after := skipGlobstars(text, cursor); after != cursor {
			cursor, globAt, globFrom = after, after, from
		}

		end, stop, matched := matchSegment(text, cursor, name, from)
		if stop = segmentEnd(name, stop); matched && end < len(text) && stop < len(name) {
			cursor, from = end+1, stop+1

			continue
		}

		if matched && end == len(text) && stop == len(name) {
			return true
		}

		var more bool
		if globFrom, more = nextSegment(name, globFrom); !more {
			return false
		}

		cursor, from = globAt, globFrom
	}
}

// globstarSegment starts a segment that matches any number of whole
// segments, none included.
const globstarSegment = "**/"

// skipGlobstars is past the **/ segments starting at cursor, which fnmatch
// reads as one.
func skipGlobstars(text []rune, cursor int) int {
	for cursor < len(text) && strings.HasPrefix(string(text[cursor:]), globstarSegment) {
		cursor += len(globstarSegment)
	}

	return cursor
}

// nextSegment is where the segment after the one holding index starts, if
// there is one; a negative index, no globstar yet, has none.
func nextSegment(name []rune, index int) (int, bool) {
	if index < 0 {
		return index, false
	}

	index = segmentEnd(name, index)

	return index + 1, index < len(name)
}

// segmentEnd is where the segment holding index ends: at its slash, or at the
// end of the string.
func segmentEnd(name []rune, index int) int {
	for index < len(name) && name[index] != '/' {
		index++
	}

	return index
}

// matchSegment is fnmatch_helper: whether the pattern from at matches the
// path's segment from from, and if so where each stopped. A * that the rest
// fails after takes one more character and tries again. A pattern that
// matches up to a * ending its segment stops the path wherever it got to.
func matchSegment(text []rune, cursor int, name []rune, from int) (int, int, bool) {
	starAt, starFrom := -1, -1

	for {
		if runeAt(text, cursor) == '*' {
			cursor = skipStars(text, cursor)
			if after := unescape(text, cursor); endOf(text, after) {
				return after, from, true
			}

			if endOf(name, from) {
				return cursor, from, false
			}

			starAt, starFrom = cursor, from

			continue
		}

		next, stop, found := matchOne(text, cursor, name, from)
		if found != failed {
			cursor, from = next, stop

			if found == advanced {
				continue
			}

			return cursor, from, found == segmentMatched
		}

		if starAt < 0 {
			return cursor, from, false
		}

		starFrom++
		cursor, from = starAt, starFrom
	}
}

// step is what matchOne found.
type step int

const (
	// advanced is one character of each matched; go on.
	advanced step = iota
	// failed is a mismatch a * before it may yet undo.
	failed
	// segmentMatched is both at the end of their segments.
	segmentMatched
	// segmentFailed is the path's segment ended before the pattern's.
	segmentFailed
)

// matchOne matches the pattern's next ?, class or character, which may be
// escaped, against the path's next character.
func matchOne(text []rune, cursor int, name []rune, from int) (int, int, step) {
	pathEnded := endOf(name, from)

	switch runeAt(text, cursor) {
	case '?':
		if pathEnded {
			return cursor, from, segmentFailed
		}

		return cursor + 1, from + 1, advanced
	case '[':
		if pathEnded {
			return cursor, from, segmentFailed
		}

		if next, ok := bracket(text, cursor+1, name[from]); ok {
			return next, from + 1, advanced
		}

		return cursor, from, failed
	}

	return matchLiteral(text, unescape(text, cursor), name, from)
}

func matchLiteral(text []rune, cursor int, name []rune, from int) (int, int, step) {
	if endOf(name, from) {
		return cursor, from, map[bool]step{true: segmentMatched, false: segmentFailed}[endOf(text, cursor)]
	}

	if !endOf(text, cursor) && text[cursor] == name[from] {
		return cursor + 1, from + 1, advanced
	}

	return cursor, from, failed
}

// bracket is Ruby's bracket: whether character is in the class starting at
// at, just after its [, and where the class ends. Unlike path.Match it takes
// a leading ! or ^ and then nothing as any character, a - before the ] as
// itself, and a slash as a member; a class with no ] matches nothing.
func bracket(text []rune, cursor int, character rune) (int, bool) {
	negated := runeAt(text, cursor) == '!' || runeAt(text, cursor) == '^'
	if negated {
		cursor++
	}

	member := false

	for runeAt(text, cursor) != ']' {
		low := unescape(text, cursor)
		if runeAt(text, low) == 0 {
			return 0, false
		}

		high := low
		if cursor = low + 1; runeAt(text, cursor) == '-' && runeAt(text, cursor+1) != ']' {
			if high = unescape(text, cursor+1); runeAt(text, high) == 0 {
				return 0, false
			}

			cursor = high + 1
		}

		member = member || inRange(character, text[low], text[high])
	}

	return cursor + 1, member != negated
}

// inRange is Ruby's test of a class member low-high: either end itself, or a
// character between them.
func inRange(character, low, high rune) bool {
	return character == low || character == high || (low <= character && character <= high)
}

func skipStars(text []rune, cursor int) int {
	for runeAt(text, cursor) == '*' {
		cursor++
	}

	return cursor
}

// unescape is past a backslash: the character it escapes, or the end of the
// pattern when it is the last.
func unescape(text []rune, cursor int) int {
	if runeAt(text, cursor) == '\\' {
		return cursor + 1
	}

	return cursor
}

// endOf reports the end of a segment: a slash, or the end of the string.
func endOf(text []rune, cursor int) bool {
	character := runeAt(text, cursor)

	return character == 0 || character == '/'
}

// runeAt is the character at index, or NUL past the end.
func runeAt(text []rune, index int) rune {
	if index >= len(text) {
		return 0
	}

	return text[index]
}
