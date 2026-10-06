// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"unicode"
	"unicode/utf8"
)

// Errors CheckKeys returns for a ui.keys map that cannot be used. Callers
// distinguish them with errors.Is.
var (
	// ErrUnknownKeyAction reports a ui.keys entry naming an action that does not
	// exist — a typo in the action id.
	ErrUnknownKeyAction = errors.New("ui.keys names a key action that does not exist")
	// ErrKeyConflict reports two actions bound to the same key where both are
	// live at once, so a press would be ambiguous.
	ErrKeyConflict = errors.New("ui.keys binds two actions to one key in the same context")
	// ErrKeyNotRebindable reports a ui.keys entry moving an action whose keys
	// cannot be one key: jump-to-pane answers the pane numbers, one per pane.
	ErrKeyNotRebindable = errors.New("ui.keys moves an action whose keys cannot be rebound")
	// ErrInterruptEdits reports interrupt moved onto a key that types or edits
	// text. Interrupt is answered before any filter, prompt or text box, so on
	// such a key it would quit mid-sentence and lose what was written.
	ErrInterruptEdits = errors.New("ui.keys moves interrupt onto a key that types or edits text")
	// ErrTextFieldKey reports an overlay's key moved onto one its focused text
	// field edits with — a character, or a readline key such as ctrl+w. The
	// overlay answers the key first, so the field would lose that edit.
	ErrTextFieldKey = errors.New("ui.keys moves an overlay's key onto one its text field edits with")
)

// actionInterrupt is the action every context answers first, even while text
// is typed.
const actionInterrupt = "interrupt"

// CheckKeys reports whether a ui.keys override map is usable: every entry names a
// real action that one key can trigger, and no two actions that are live at the
// same time are bound to one key. It builds the keymap the overrides produce and
// inspects it, so it checks exactly what the interface would run — the default
// set, with no overrides, passes. The glyphs only name the arrow keys in the
// help, which key collisions do not depend on, so a default set stands in for
// them.
func CheckKeys(overrides map[string]string) error {
	_, builder := compileKeys(unicodeGlyphs(), "pull request", "Slack", overrides)

	err := builder.unknownActions(overrides)
	if err != nil {
		return err
	}

	if _, moved := overrides[actionJumpToPane]; moved {
		return fmt.Errorf("%w: %q", ErrKeyNotRebindable, actionJumpToPane)
	}

	if moved, ok := overrides[actionInterrupt]; ok && editsText(moved) {
		return fmt.Errorf("%w: %s on %q", ErrInterruptEdits, actionInterrupt, moved)
	}

	err = builder.conflicts()
	if err != nil {
		return err
	}

	return builder.textFieldClash()
}

// textFieldActions are the actions an overlay answers before its focused text
// field sees the key: the branch creator's worktree, the commit composer's
// breaking, the pull request composer's draft and template, the body handed to
// the editor, and moving between fields.
func textFieldActions() []string {
	return []string{
		"worktree", "toggle-breaking", "toggle-draft", "next-template", "edit-body", "next-field", "previous-field",
	}
}

// textFieldClash rejects a text field's action bound to a key the field edits
// with, so the overlay would take the edit for its own.
func (b *helpBuilder) textFieldClash() error {
	for _, placed := range b.placements {
		if !slices.Contains(textFieldActions(), placed.action) {
			continue
		}

		for _, boundKey := range placed.binding.Keys() {
			if editsText(boundKey) || slices.Contains(readlineKeys(), boundKey) {
				return fmt.Errorf("%w: %s on %q", ErrTextFieldKey, placed.action, boundKey)
			}
		}
	}

	return nil
}

// readlineKeys are the control keys bubbles' text input edits or moves with,
// as a terminal's readline does — ctrl+w deletes a word, ctrl+b moves back —
// and ctrl+n and ctrl+p, which step through a field's suggestions.
func readlineKeys() []string {
	return []string{
		"ctrl+a", "ctrl+b", "ctrl+d", "ctrl+e", "ctrl+f", "ctrl+h", "ctrl+k", "ctrl+u", "ctrl+v", "ctrl+w",
		"ctrl+n", "ctrl+p",
	}
}

// unknownActions rejects an override naming an action the keymap does not define,
// reporting them in a stable order so the message does not depend on map order.
func (b *helpBuilder) unknownActions(overrides map[string]string) error {
	for _, action := range slices.Sorted(maps.Keys(overrides)) {
		if _, ok := b.byAction[action]; !ok {
			return fmt.Errorf("%w: %q", ErrUnknownKeyAction, action)
		}
	}

	return nil
}

// keyContext is a set of bindings all live at once — one keyboard surface — and a
// name for it. A key bound twice within one context is ambiguous; the same key
// meaning different things across two contexts is not. A binding belongs to a
// context by help group, or by being named in alsoLive: a few bindings are
// handled on a surface whose help group they are not filed under, and modeling
// the context by group alone would miss the conflicts they cause there.
type keyContext struct {
	name     string
	groups   []int
	alsoLive []string
}

// covers reports that a placement's binding is live in the context: its help
// group is one of the context's, or its action is one the context also runs.
func (c keyContext) covers(placed placement) bool {
	return slices.Contains(c.groups, placed.group) || slices.Contains(c.alsoLive, placed.action)
}

// keyContexts are the sets of bindings live together, one per keyboard surface.
// Most of a surface's keys come from its help groups, but some bindings are
// handled outside the group they are filed under, so a context also names those:
// the list actions refresh, open-link and copy-link are filed under Issues, yet
// refresh acts on the Branch, Commits, Review, messaging, review-requests and
// Tasks panes, and open-link and copy-link on the Review, review-requests and
// Tasks panes; edit acts on the Review pane as well as in a preview; and an
// overlay's list reads up and down, first and last, which a composer otherwise
// excludes so that its tab can mean next-field rather than next-pane, and
// People and groups reads refresh to read the Slack directory again. An overlay's own keys, the branch creator's
// worktree and the messaging preview's wait for CI and channel among them, are
// filed with the composer's, so they are live there and not on the pane behind
// it.
func keyContexts() []keyContext {
	return []keyContext{
		{"the Issues pane", []int{groupMoving, groupEverywhere, groupIssues}, nil},
		{"the Branch and Commits panes", []int{groupMoving, groupEverywhere, groupBranchCommits}, []string{actionRefresh}},
		{
			"the Review and messaging panes",
			[]int{groupMoving, groupEverywhere, groupReviewMessaging},
			[]string{actionOpenLink, actionCopyLink, actionRefresh, "edit"},
		},
		{
			"the review-requests pane",
			[]int{groupMoving, groupEverywhere, groupReviews},
			[]string{actionOpenLink, actionCopyLink, actionRefresh},
		},
		{
			"the Tasks pane",
			[]int{groupMoving, groupEverywhere, groupTasks},
			[]string{actionOpenLink, actionCopyLink, actionRefresh},
		},
		{
			"the Summary pane",
			[]int{groupMoving, groupEverywhere, groupSummary},
			[]string{actionOpenLink, actionCopyLink, actionRefresh},
		},
		{
			"the Repositories pane",
			[]int{groupMoving, groupEverywhere, groupRepositories},
			[]string{actionUp, actionDown, actionRefresh},
		},
		{"a running command", []int{groupMoving, groupEverywhere, groupRunning}, nil},
		{
			"a composer or preview",
			[]int{groupEverywhere, groupComposer},
			[]string{actionUp, actionDown, actionFirst, actionLast, actionRefresh},
		},
		{"the comment composer", []int{groupEverywhere, groupWriting}, []string{actionUp, actionDown, "edit-body"}},
	}
}

// conflicts rejects the first context in which two actions share a key.
func (b *helpBuilder) conflicts() error {
	for _, context := range keyContexts() {
		err := b.conflictIn(context)
		if err != nil {
			return err
		}
	}

	return nil
}

// conflictIn reports two actions in one context bound to the same key, naming
// both actions, the key and the context.
func (b *helpBuilder) conflictIn(context keyContext) error {
	boundBy := map[string]string{}

	for _, placed := range b.placements {
		if !context.covers(placed) {
			continue
		}

		for _, boundKey := range placed.binding.Keys() {
			if other, taken := boundBy[boundKey]; taken && other != placed.action {
				return fmt.Errorf("%w: %s and %s both bind %q in %s",
					ErrKeyConflict, other, placed.action, boundKey, context.name)
			}

			boundBy[boundKey] = placed.action
		}
	}

	return nil
}

// editsText reports a key name a text field takes as editing: space,
// backspace or delete, or one character as typed — a letter, digit or symbol,
// with any accent or variation selector that rides on it.
func editsText(name string) bool {
	switch name {
	case "space", "backspace", "delete":
		return true
	case "":
		return false
	}

	first, size := utf8.DecodeRuneInString(name)
	if !unicode.IsPrint(first) {
		return false
	}

	for _, riding := range name[size:] {
		if !unicode.In(riding, unicode.Mn, unicode.Me, unicode.Variation_Selector) {
			return false
		}
	}

	return true
}
