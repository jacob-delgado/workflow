// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package workdirs

import "unicode"

// Match is an entry whose name holds what was typed, letter by letter in
// order: where in the name each letter matched, and whether the name starts
// with them.
type Match struct {
	Entry Entry
	// Letters is the place of each matched letter among the name's letters,
	// counted from zero.
	Letters []int
	Prefix  bool
}

// Matching is the entries whose names hold typed's letters in order,
// whatever their case: those starting with typed first, then the rest, each
// kept in the order given. Every entry matches "".
func Matching(entries []Entry, typed string) []Match {
	var starting, within []Match

	for _, entry := range entries {
		letters, found := inOrder([]rune(entry.Name), []rune(typed))
		if !found {
			continue
		}

		match := Match{Entry: entry, Letters: letters, Prefix: fromTheStart(letters)}
		if match.Prefix {
			starting = append(starting, match)
		} else {
			within = append(within, match)
		}
	}

	return append(starting, within...)
}

// inOrder is the place in name of each of typed's letters, each the first
// after the last that serves, and false when name runs out before typed does.
func inOrder(name, typed []rune) ([]int, bool) {
	letters := make([]int, 0, len(typed))

	for place, letter := range name {
		if len(letters) < len(typed) && unicode.ToLower(letter) == unicode.ToLower(typed[len(letters)]) {
			letters = append(letters, place)
		}
	}

	return letters, len(letters) == len(typed)
}

// fromTheStart reports letters that are a name's first, one after another:
// the first match after each is the next letter whenever the name starts
// with what was typed.
func fromTheStart(letters []int) bool {
	for count, place := range letters {
		if place != count {
			return false
		}
	}

	return true
}
