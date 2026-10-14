// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

// Package console is what a command says on stderr: its notes, warnings and
// errors, each an entry of one log. A note is the bare sentence, as a script
// reading stderr has always seen it; a warning and an error carry their tags.
package console

import (
	"io"
	"strings"

	"charm.land/log/v2"
)

// Options are how a command's notes are said: Getenv reads the variables
// that decide color — NO_COLOR, CLICOLOR_FORCE, TERM and the like — which no
// note reads from the process itself. A nil Getenv reads none.
type Options struct {
	Getenv func(name string) string
}

// Notes is what a command says on stderr, through one log: a note about what
// it did, a warning about what it could not, and the error it ended with.
// Copies share the log they were made with.
type Notes struct {
	log *log.Logger
}

// New is the notes said on stderr, colored as the terminal there and the
// variables opts reads allow.
func New(stderr io.Writer, opts Options) Notes {
	logger := log.NewWithOptions(stderr, log.Options{Level: log.InfoLevel})
	logger.SetStyles(tagged())
	logger.SetColorProfile(colorFor(stderr, opts.Getenv))

	return Notes{log: logger}
}

// To is the notes said to stream instead, tagged and colored as they were:
// for a stream that passes on to the one they were made for, such as one that
// clears a line kept there first.
func (n Notes) To(stream io.Writer) Notes {
	logger := n.log.With()
	logger.SetOutput(stream)

	return Notes{log: logger}
}

// Note says sentence, a note about what the command did or found: on a pipe,
// the sentence alone.
func (n Notes) Note(sentence string) {
	n.log.Info(sentence)
}

// Warn says sentence as a warning: the command goes on, but something it was
// asked for did not happen.
func (n Notes) Warn(sentence string) {
	n.log.Warn(sentence)
}

// Fail says err, the error the command ended with.
func (n Notes) Fail(err error) {
	n.log.Error(err.Error())
}

// Writer is the notes as a stream, for prose a command says on stderr beside
// an artifact on stdout, such as a preview printed beside the JSON a script
// reads: each line written to it is a note, a blank one too.
func (n Notes) Writer() io.Writer {
	return noteLines{notes: n}
}

// noteLines is a stream whose every line is a note. A write is taken as
// whole lines, as every writer of prose here writes them.
type noteLines struct {
	notes Notes
}

var _ io.Writer = noteLines{}

// Write says each line of p as a note.
func (w noteLines) Write(p []byte) (int, error) {
	for line := range strings.SplitSeq(strings.TrimSuffix(string(p), "\n"), "\n") {
		w.notes.Note(line)
	}

	return len(p), nil
}
