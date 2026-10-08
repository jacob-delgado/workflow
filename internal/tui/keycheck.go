// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"fmt"
	"maps"
	"slices"
	"unicode"
	"unicode/utf8"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/config"
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
		return fmt.Errorf("%w: %q", config.ErrKeyNotRebindable, actionJumpToPane)
	}

	if moved, ok := overrides[actionInterrupt]; ok && editsText(moved) {
		return fmt.Errorf("%w: %s on %q", config.ErrInterruptEdits, actionInterrupt, moved)
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
				return fmt.Errorf("%w: %s on %q", config.ErrTextFieldKey, placed.action, boundKey)
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
			return fmt.Errorf("%w: %q", config.ErrUnknownKeyAction, action)
		}
	}

	return nil
}

// keyContext is a set of bindings all live at once — one keyboard surface — and a
// name for it. A key bound twice within one context is ambiguous; the same key
// meaning different things across two contexts is not. A binding belongs to a
// context by its help group, or by its action being one the context names.
type keyContext struct {
	name    string
	groups  []int
	actions []string
}

// covers reports that a placement's binding is live in the context: its help
// group is one of the context's, or its action is one the context names.
func (c keyContext) covers(placed placement) bool {
	return slices.Contains(c.groups, placed.group) || slices.Contains(c.actions, placed.action)
}

// liveIn reports whether msg presses a key live in context.
func (k keyMap) liveIn(context keyContext, msg tea.KeyPressMsg) bool {
	for _, group := range context.groups {
		if key.Matches(msg, k.full[group]...) {
			return true
		}
	}

	for _, action := range context.actions {
		if key.Matches(msg, k.byAction[action]) {
			return true
		}
	}

	return false
}

// keyContexts are the sets of bindings live together, one per keyboard surface:
// each pane's, which its handler obeys, and the overlays'.
func keyContexts() []keyContext {
	contexts := make([]keyContext, 0, paneCount)
	for each := range pane(paneCount) {
		contexts = append(contexts, each.keyContext())
	}

	return append(contexts, overlayContexts()...)
}

// overlayContexts are the keyboard surfaces an overlay opens. A running
// command answers its own keys and the moving ones. A composer or preview
// reads its own keys, the branch creator's worktree and the messaging
// preview's wait for CI and channel among them, and an overlay's list reads
// up and down, first and last, which a composer otherwise leaves out so its
// tab can mean next-field rather than next-pane; People and groups reads
// refresh to read the Slack directory again. The comment composer moves with
// up and down and hands its body to the editor.
func overlayContexts() []keyContext {
	return []keyContext{
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
					config.ErrKeyConflict, other, placed.action, boundKey, context.name)
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
