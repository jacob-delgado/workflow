// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

// Package listsearch holds the search a list is narrowed by as it is typed:
// whether keys are typing it, and the text it narrows the list to once it is
// kept. Every list of the terminal that narrows as you type answers its keys
// the same way, so its search is one machine.
package listsearch

import "unicode/utf8"

// Stage is where a list's search stands.
type Stage int

const (
	// StageOff narrows nothing: the list is whole. It is the zero stage.
	StageOff Stage = iota
	// StageTyping takes the keys typed into it, narrowing the list as it
	// goes; its text may still be empty.
	StageTyping
	// StageKept takes no keys, and still narrows the list by the text it was
	// kept with, which is never empty.
	StageKept
)

// Typed is a list's search: off, being typed, or kept. The zero Typed is off.
type Typed struct {
	stage Stage
	text  string
}

// Stage is where the search stands.
func (t Typed) Stage() Stage {
	return t.stage
}

// Text is what the search narrows the list to; empty while it is off.
func (t Typed) Text() string {
	return t.text
}

// Begun is a search being typed afresh, from no text, whatever stood before.
func (Typed) Begun() Typed {
	return Typed{stage: StageTyping, text: ""}
}

// Typed is the search with text typed onto its end while it is being typed,
// and unchanged otherwise: a search off or kept takes no keys.
func (t Typed) Typed(text string) Typed {
	if t.stage != StageTyping {
		return t
	}

	t.text += text

	return t
}

// Trimmed is the search with its last character taken off while it is being
// typed, and unchanged otherwise, or when there is no character to take.
func (t Typed) Trimmed() Typed {
	if t.stage != StageTyping || t.text == "" {
		return t
	}

	_, size := utf8.DecodeLastRuneInString(t.text)
	t.text = t.text[:len(t.text)-size]

	return t
}

// Kept is the search typed no more, still narrowing the list by its text; a
// search kept with no text narrows nothing, and is off.
func (t Typed) Kept() Typed {
	if t.text == "" {
		return Typed{stage: StageOff, text: ""}
	}

	return Typed{stage: StageKept, text: t.text}
}

// Cleared is the search off, the list whole again.
func (Typed) Cleared() Typed {
	return Typed{stage: StageOff, text: ""}
}
