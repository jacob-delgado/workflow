// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package config

// UI is how the terminal interface behaves.
type UI struct {
	// Mouse captures the mouse, so a click focuses a pane or selects a row.
	// Capturing it takes away the terminal's own click-and-drag selection,
	// which is why it can be turned off here, and toggled with m in a session.
	Mouse bool `json:"mouse"`
	// ASCII draws borders and glyphs in plain ASCII, for a terminal or font
	// without box-drawing characters. There is no reliable way to detect that,
	// so it is a setting rather than a guess.
	ASCII bool `json:"ascii"`
	// Color is whether to draw the system hues: empty (auto) draws them when the
	// terminal allows, "never" turns them off while keeping bold, faint and the
	// reverse-video cursor, which carry meaning without color.
	Color string `json:"color"`
	// Notify rings the terminal, and sends a desktop notification where the
	// terminal relays one, when CI finishes — so a developer who stepped away is
	// told rather than having to check back. Off unless set.
	Notify bool `json:"notify"`
	// CommentsShown is how many of an issue's most recent comments the detail
	// pane draws. Zero keeps the built-in default.
	CommentsShown int `json:"comments_shown"`
	// Keys rebinds the interface's keys. Each entry maps an action to the single
	// key that should trigger it, e.g. {"commit": "C", "comment": "ctrl+e"}; the
	// help then shows the new key. An action left out keeps its default. The
	// interface refuses to start on a map that names an action it does not know,
	// or that binds two actions in the same context to one key. The known
	// actions, by where they work, are:
	//
	//   Moving:  next-pane, previous-pane, jump-to-pane, up, down, scroll-up,
	//            scroll-down
	//   Issues:  change-status, comment, assign, log-work, branch-for-issue,
	//            filter, switch-view, load-more, open-link, copy-link, refresh
	//   Branch:  new-branch, switch-task, rebase, push, stage, stage-all,
	//            commit, amend, fixup, run-pre-commit, set-up-lefthook
	//   Review:  open-pull-request, checks, rerun-checks, merge, finish-branch,
	//            post
	//   Composer or preview: edit, edit-body, next-template, toggle-draft,
	//            toggle-breaking, verbatim, next-field, previous-field,
	//            cycle-type-left, cycle-type-right, toggle-option, worktree,
	//            post-when-green
	//   Running: stop, run-again, full-output
	//   Everywhere: apply, close, toggle-mouse, toggle-help, quit, interrupt
	Keys map[string]string `json:"keys"`
}

// DrawColor reports whether the system hues should be drawn. NO_COLOR (set to
// any value) and ui.color "never" both turn them off; bold, faint and reverse
// stay.
func (u UI) DrawColor(noColorEnv string) bool {
	return noColorEnv == "" && u.Color != "never"
}
