// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/hooks"
	"github.com/jacob-delgado/workflow/internal/seams"
)

// Deps is everything the interface asks of the world outside the terminal,
// grouped by what it asks. Each is a function rather than a client, so the model
// never holds a context or a credential, and a test can hand it canned answers
// without a network, a repository or a subprocess.
//
// A load the interface starts by itself is skipped when its function is nil, so
// a test need only supply what it exercises. An action is only offered once the
// model has what it acts on.
//
// The groups every surface shares are declared in package seams. Editor is
// declared here, because it speaks Bubble Tea's messages and commands.
type Deps struct {
	Jira      seams.Jira
	Git       seams.Git
	Forge     seams.Forge
	Messaging seams.Messaging
	Hooks     seams.Hooks
	Editor    EditorDeps
	Store     seams.Store
	// Clock tells the time, for how long ago a comment was written. Nil means
	// time.Now.
	Clock func() time.Time
	// CIInterval is how often CI is asked about while it runs, or while a post
	// waits for it to pass. Zero means every twenty seconds.
	CIInterval time.Duration
	// After is the timer every wait goes through — the selection resting before
	// an issue is read, the gap between CI checks: a command that delivers fire's
	// message once the wait has passed. Nil means tea.Tick.
	After func(wait time.Duration, fire func(time.Time) tea.Msg) tea.Cmd
	// Notify rings the terminal and sends a desktop notification, for when CI
	// finishes while the developer is looking elsewhere. Nil where the interface
	// cannot reach the terminal to ring it.
	Notify func()
	// OpenURL opens a page in the user's browser, for a check whose detail lives
	// on the forge. Nil where there is no opener to reach.
	OpenURL func(url string) error
	// Copy puts text on the system clipboard. In production it is tea.SetClipboard,
	// which writes it through the terminal's own OSC 52 sequence — no program, and
	// it works over SSH. Nil where the interface cannot reach the terminal.
	Copy func(text string) tea.Cmd
}

// EditorDeps hands text and files to the user's editor, which takes the
// terminal while it is open.
type EditorDeps struct {
	Edit func(text, help string, done func(string, error) tea.Msg) tea.Cmd
	Open func(file string, line int, done func(error) tea.Msg) tea.Cmd
	// Resolve turns the places a tool printed into the files that open them,
	// keyed by the place as printed and leaving out any that names none. A place
	// relative to a package, not the root, is the common miss. It can walk the
	// whole checkout, so it runs in a command, never in Update.
	Resolve func(places []string) map[string]string
}

// resolvedFailures keeps only the places that resolve to a file, rewriting each
// to the path that opens it, so a place a tool printed relative to its package
// is either found below the root or dropped rather than offered as a jump that
// opens nothing. Every place is resolved in one call, so a run's places cost
// one walk of the checkout between them. With no Resolve seam the places are
// left as they came.
func (d Deps) resolvedFailures(found []hooks.Location) []hooks.Location {
	if d.Editor.Resolve == nil {
		return found
	}

	printed := make([]string, 0, len(found))
	for _, place := range found {
		printed = append(printed, place.File)
	}

	files := d.Editor.Resolve(printed)
	kept := make([]hooks.Location, 0, len(found))

	for _, place := range found {
		file, ok := files[place.File]
		if ok {
			place.File = file
			kept = append(kept, place)
		}
	}

	return kept
}

// defaultCIInterval is how often CI is asked about when nothing says otherwise:
// often enough to post soon after a pass, rarely enough not to spend a forge's
// rate limit on it.
const defaultCIInterval = 20 * time.Second

// ciInterval is how often CI is asked about.
func (d Deps) ciInterval() time.Duration {
	if d.CIInterval <= 0 {
		return defaultCIInterval
	}

	return d.CIInterval
}

// now is the time by the clock the interface was given.
func (d Deps) now() time.Time {
	if d.Clock == nil {
		return time.Now()
	}

	return d.Clock()
}

// after is a command that delivers fire's message once wait has passed, on the
// timer the interface was given.
func (d Deps) after(wait time.Duration, fire func(time.Time) tea.Msg) tea.Cmd {
	if d.After == nil {
		return tea.Tick(wait, fire)
	}

	return d.After(wait, fire)
}
