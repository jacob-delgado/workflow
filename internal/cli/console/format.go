// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package console

import (
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"

	"charm.land/lipgloss/v2"
	"charm.land/log/v2"
	"github.com/charmbracelet/colorprofile"
)

// Format is how the notes are written: as text, the sentences a person
// reads, or as JSON or logfmt, an entry a line for a log collector.
type Format int

const (
	// Text writes a note as its sentence, and a warning and an error after
	// their tags.
	Text Format = iota
	// JSON writes each entry as one JSON object, stamped with its time.
	JSON
	// Logfmt writes each entry as one logfmt record, stamped with its time.
	Logfmt
)

// errUnknownFormat is a --log-format that names no format.
var errUnknownFormat = errors.New("the log format is text, json or logfmt")

// formats are every format, in the order --log-format's help names them.
func formats() []Format {
	return []Format{Text, JSON, Logfmt}
}

// ParseFormat is the format name names, as --log-format takes it.
func ParseFormat(name string) (Format, error) {
	for _, format := range formats() {
		if format.String() == name {
			return format, nil
		}
	}

	return Text, fmt.Errorf("%w, not %q", errUnknownFormat, name)
}

// String is the format's name, as --log-format takes it.
func (f Format) String() string {
	return map[Format]string{Text: "text", JSON: "json", Logfmt: "logfmt"}[f]
}

// formatter is the log's own name for the format.
func (f Format) formatter() log.Formatter {
	return map[Format]log.Formatter{Text: log.TextFormatter, JSON: log.JSONFormatter, Logfmt: log.LogfmtFormatter}[f]
}

// stamped reports whether the format carries each entry's time: a log
// collector's formats do, and the sentences a person reads do not.
func (f Format) stamped() bool {
	return f != Text
}

// styles are the log's styles in the format: text's tags, or, for a
// structured format, which styles nothing, every level, so each entry names
// its own.
func (f Format) styles() *log.Styles {
	return map[Format]*log.Styles{Text: tagged(), JSON: log.DefaultStyles(), Logfmt: log.DefaultStyles()}[f]
}

// exitStatusKey names the exit status an error carries in a structured
// format.
const exitStatusKey = "exit_status"

// statusFields are the fields an error the command exits with status on
// carries in the format: its status, in a structured format, and none in
// text, where the shell already says it.
func (f Format) statusFields(status int) []any {
	if !f.stamped() {
		return nil
	}

	return []any{exitStatusKey, status}
}

// Verbosity is how much the notes say.
type Verbosity int

const (
	// Normal says notes, warnings and errors.
	Normal Verbosity = iota
	// Quiet says warnings and errors alone.
	Quiet
	// Verbose says debug lines too.
	Verbose
)

// level is the least level said at the verbosity.
func (v Verbosity) level() log.Level {
	return map[Verbosity]log.Level{Normal: log.InfoLevel, Quiet: log.WarnLevel, Verbose: log.DebugLevel}[v]
}

// tagged are the log's styles for text: a note carries no tag, a warning WARN
// in ANSI 3 and an error ERROR in ANSI 1, each bold, so the tags are the one
// thing colored, and a debug line DEBUG, faint. No width pads them, so the
// sentence follows a tag after one space. A note's level has no style at
// all, which is what keeps the log from writing a level, or the space after
// one, before it.
func tagged() *log.Styles {
	styles := log.DefaultStyles()
	delete(styles.Levels, log.InfoLevel)
	styles.Levels[log.DebugLevel] = lipgloss.NewStyle().SetString("DEBUG").Faint(true)
	styles.Levels[log.WarnLevel] = lipgloss.NewStyle().SetString("WARN").Bold(true).Foreground(lipgloss.Yellow)
	styles.Levels[log.ErrorLevel] = lipgloss.NewStyle().SetString("ERROR").Bold(true).Foreground(lipgloss.Red)

	return styles
}

// colorVariables are the variables colorprofile reads to decide what a
// stream can draw.
func colorVariables() []string {
	return []string{"TERM", "COLORTERM", "CLICOLOR", "CLICOLOR_FORCE", "NO_COLOR", "TMUX", "WT_SESSION"}
}

// colorFor is what stderr can draw, as colorprofile reads the stream and the
// variables getenv reads. NO_COLOR set to anything turns the hues off, bold
// kept, as the interface reads it, where colorprofile would take only a true
// value and only on a terminal.
func colorFor(stderr io.Writer, getenv func(string) string) colorprofile.Profile {
	environ := environOf(getenv, colorVariables())

	profile := colorprofile.Detect(Underlying(stderr), environ)
	if slices.ContainsFunc(environ, func(variable string) bool { return strings.HasPrefix(variable, "NO_COLOR=") }) {
		profile = min(profile, colorprofile.ASCII)
	}

	return profile
}

// environOf is the variables names name that getenv holds, as a program's
// environment lists them: each set to something, and none from a nil getenv.
func environOf(getenv func(string) string, names []string) []string {
	if getenv == nil {
		return nil
	}

	var environ []string

	for _, name := range names {
		if value := getenv(name); value != "" {
			environ = append(environ, name+"="+value)
		}
	}

	return environ
}
