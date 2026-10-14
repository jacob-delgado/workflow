// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"context"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"
)

// WithContext is the interface asking every system outside it under ctx, its
// session's, which New leaves unending; Run gives it one that ends when the
// program does. Its end stops a read in flight, never a write.
func (m Model) WithContext(ctx context.Context) Model {
	// Trade-off TRADE-38: the session ends on quit, stopping its reads;
	// leaving a view cancels nothing it asked for.
	m.session = ctx

	return m
}

// WithLifetime is the interface ending every write it sends once ctx ends, and
// not before, and counting it in flight, by name, in the lifetime.InFlight ctx
// carries until it lands. A write outlasts its session, so a quit or a switch
// never stops git midway with its index locked. ctx is the program's lifetime,
// the context Run is given: the command line's ends on SIGINT or SIGTERM, as a
// signal stops every command. New leaves its writes unending and uncounted.
func (m Model) WithLifetime(ctx context.Context) Model {
	// Trade-off TRADE-38: a quit or a switch stops reads alone; the end of the
	// program's lifetime stops writes too.
	m.deps = outlasting(endingWith(ctx), uncountedEndingWith(ctx), m.given, m.vocab.noun)

	return m
}

// WithDryRun is the interface holding back every write — to Jira, the forge,
// the messaging service, Taskwarrior, git and files — and saying instead what
// it would have done. Reads stay live, so what it says is about the real state
// of things. Of the store it keeps only the kept reads — whom a post would tag,
// and the favorites — which --dry-run binds to kept.db, read read-only, and it
// reads nothing from the cache.
func (m Model) WithDryRun() Model {
	m.deps, m.given = heldBack(m.deps), heldBack(m.given)
	m.dryRun = true

	return m
}

// WithoutColor draws no hue while keeping the bold, faint and reverse-video
// cursor that carry meaning without it: its styles hold no hue, and Run forces
// the ANSI profile, which still draws those attributes.
func (m Model) WithoutColor() Model {
	m.styles = newStyles(false)
	m.programOptions = []tea.ProgramOption{tea.WithColorProfile(colorprofile.ANSI)}

	return m
}

// WithAccessibleForms is the interface asking each multi-field form's
// questions as plain prompts that take the whole terminal, one at a time, for
// a screen reader, rather than drawing the form over the screen.
func (m Model) WithAccessibleForms() Model {
	m.accessibleForms = true

	return m
}
