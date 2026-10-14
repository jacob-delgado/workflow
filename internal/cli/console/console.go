// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

// Package console is what a command says on stderr: its notes, warnings and
// errors, each an entry of one log, and, with --verbose, a debug line for
// each request. A note is the bare sentence, as a script reading stderr has
// always seen it; a warning and an error carry their tags. --log-format says
// them as JSON or logfmt instead, an entry a line, each stamped with its time.
package console

import (
	"io"
	"strings"
	"time"

	"charm.land/log/v2"
)

// Options are how a command's notes are said: in which format, how much of
// them, and Getenv, which reads the variables that decide color — NO_COLOR,
// CLICOLOR_FORCE, TERM and the like — which no note reads from the process
// itself. A nil Getenv reads none.
type Options struct {
	Format    Format
	Verbosity Verbosity
	Getenv    func(name string) string
}

// Notes is what a command says on stderr, through one log: a note about what
// it did, a warning about what it could not, the error it ended with, and,
// when verbose, a debug line about how it went. Copies share the log they
// were made with.
type Notes struct {
	log    *log.Logger
	format Format
}

// New is the notes said on stderr as opts asks, colored as the terminal there
// and the variables opts reads allow.
func New(stderr io.Writer, opts Options) Notes {
	logger := log.NewWithOptions(stderr, log.Options{
		Level: opts.Verbosity.level(), Formatter: opts.Format.formatter(),
		ReportTimestamp: opts.Format.stamped(), TimeFormat: time.RFC3339, TimeFunction: log.NowUTC,
	})
	logger.SetStyles(opts.Format.styles())
	logger.SetColorProfile(colorFor(stderr, opts.Getenv))

	return Notes{log: logger, format: opts.Format}
}

// To is the notes said to stream instead, tagged and colored as they were:
// for a stream that passes on to the one they were made for, such as one that
// clears a line kept there first.
func (n Notes) To(stream io.Writer) Notes {
	logger := n.log.With()
	logger.SetOutput(stream)

	return Notes{log: logger, format: n.format}
}

// Note says sentence, a note about what the command did or found: on a pipe,
// the sentence alone. --quiet drops it.
func (n Notes) Note(sentence string) {
	n.log.Info(sentence)
}

// Warn says sentence as a warning: the command goes on, but something it was
// asked for did not happen.
func (n Notes) Warn(sentence string) {
	n.log.Warn(sentence)
}

// Fail says err, the error the command ended with, and, in a structured
// format, the status it exits with, for a script reading the log to act on.
func (n Notes) Fail(err error, status int) {
	n.log.Error(err.Error(), n.format.statusFields(status)...)
}

// Debug says sentence as a debug line, which only --verbose shows.
func (n Notes) Debug(sentence string) {
	n.log.Debug(sentence)
}

// Writer is the notes as a stream, for prose a command says on stderr beside
// an artifact on stdout, such as a preview printed beside the JSON a script
// reads: each line written to it is a note, a blank one too.
func (n Notes) Writer() io.Writer {
	return lineWriter{say: n.Note}
}

// DebugWriter is the debug lines as a stream, for the request log's outlines:
// each line written to it is a debug line, its runs of spaces closed up and a
// leading RFC 3339 time dropped, since the log stamps its own entries where
// its format carries a time.
func (n Notes) DebugWriter() io.Writer {
	return lineWriter{say: func(line string) { n.Debug(withoutStamp(line)) }}
}

// lineWriter is a stream that says each of its lines. A write is taken as
// whole lines, as every writer of prose here writes them.
type lineWriter struct {
	say func(line string)
}

var _ io.Writer = lineWriter{}

// Write says each line of p.
func (w lineWriter) Write(p []byte) (int, error) {
	for line := range strings.SplitSeq(strings.TrimSuffix(string(p), "\n"), "\n") {
		w.say(line)
	}

	return len(p), nil
}

// withoutStamp is line's words, one space apart, without a first that is an
// RFC 3339 time.
func withoutStamp(line string) string {
	words := strings.Fields(line)
	if len(words) == 0 {
		return ""
	}

	_, err := time.Parse(time.RFC3339, words[0])
	if err == nil {
		words = words[1:]
	}

	return strings.Join(words, " ")
}
