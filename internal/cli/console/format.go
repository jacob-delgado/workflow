// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package console

import (
	"io"

	"charm.land/lipgloss/v2"
	"charm.land/log/v2"
	"github.com/charmbracelet/colorprofile"
)

// tagged are the log's styles: a note carries no tag, a warning WARN in ANSI
// 3 and an error ERROR in ANSI 1, each bold, so the tags are the one thing
// colored. No width pads them, so the sentence follows a tag after one space.
// A note's level has no style at all, which is what keeps the log from
// writing a level, or the space after one, before it.
func tagged() *log.Styles {
	styles := log.DefaultStyles()
	delete(styles.Levels, log.InfoLevel)
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
	if getenv == nil {
		getenv = func(string) string { return "" }
	}

	var environ []string

	for _, name := range colorVariables() {
		if value := getenv(name); value != "" {
			environ = append(environ, name+"="+value)
		}
	}

	profile := colorprofile.Detect(stderr, environ)
	if getenv("NO_COLOR") != "" {
		profile = min(profile, colorprofile.ASCII)
	}

	return profile
}
