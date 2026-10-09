// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"fmt"
	"maps"
	"slices"
	"unicode"
	"unicode/utf8"

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

// keyContexts are the sets of bindings live together, one per keyboard surface:
// each pane's, with the actions its handler answers, and the overlays'.
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
	return []keyContext{runningContext(), composingContext(), commentingContext()}
}

// runningContext is the keys live while a command runs.
func runningContext() keyContext {
	return keyContext{"a running command", []int{groupMoving, groupEverywhere, groupRunning}, nil}
}

// composingContext is the keys live in a composer or preview.
func composingContext() keyContext {
	return keyContext{
		"a composer or preview",
		[]int{groupEverywhere, groupComposer},
		[]string{actionUp, actionDown, actionFirst, actionLast, actionRefresh},
	}
}

// commentingContext is the keys live in the comment composer.
func commentingContext() keyContext {
	return keyContext{
		"the comment composer", []int{groupEverywhere, groupWriting}, []string{actionUp, actionDown, "edit-body"},
	}
}

// overlayKind is which overlay one is: every overlay names its own, and its
// kind names the key context CheckKeys reads for it.
type overlayKind int

const (
	overlayAmendPreview overlayKind = iota
	overlayBranchCreator
	overlayBranchLinker
	overlayBranchPicker
	overlayCalendar
	overlayChecklist
	overlayChecks
	overlayCommandRun
	overlayCommentComposer
	overlayCommentPreview
	overlayCommitComposer
	overlayDirPrompt
	overlayFinishPreview
	overlayFixupPicker
	overlayHelp
	overlayHookgenOffer
	overlayIssueLinker
	overlayIssueWrite
	overlayJobLog
	overlayLastLook
	overlayLocalData
	overlayMergePicker
	overlayMessagingPreview
	overlayOwnerPicker
	overlayPeople
	overlayPRComposer
	overlayPREditor
	overlayQuitGuard
	overlaySettings
	overlaySetupForm
	overlayStatusPicker
	overlaySummaryPost
	overlayTaskLine
)

// keyContext is the keys live while an overlay of the kind is open. The
// table is a map keyed by kind so that exhaustive refuses a kind left out.
func (k overlayKind) keyContext() keyContext {
	running, composing, commenting := runningContext(), composingContext(), commentingContext()

	return map[overlayKind]keyContext{
		overlayAmendPreview: composing, overlayBranchCreator: composing, overlayBranchLinker: composing,
		overlayBranchPicker: composing, overlayCalendar: composing, overlayChecklist: composing,
		overlayChecks: composing, overlayCommandRun: running, overlayCommentComposer: commenting,
		overlayCommentPreview: composing, overlayCommitComposer: composing, overlayDirPrompt: composing,
		overlayFinishPreview: composing, overlayFixupPicker: composing, overlayHelp: composing,
		overlayHookgenOffer: composing, overlayIssueLinker: composing, overlayIssueWrite: composing,
		overlayJobLog: composing, overlayLastLook: composing, overlayLocalData: composing,
		overlayMergePicker: composing, overlayMessagingPreview: composing, overlayOwnerPicker: composing,
		overlayPeople: composing, overlayPRComposer: composing, overlayPREditor: composing,
		overlayQuitGuard: composing, overlaySettings: composing, overlaySetupForm: composing,
		overlayStatusPicker: composing, overlaySummaryPost: composing, overlayTaskLine: composing,
	}[k]
}

// KeyContext is one keyboard surface — a pane, or an overlay while it is
// open — by name, and every action live on it at once. CheckKeys refuses a
// ui.keys map that binds two of its actions to one key.
type KeyContext struct {
	Name    string
	Actions []string
}

// OverlayKeyContexts is the key context of every overlay, as CheckKeys
// reads them.
func OverlayKeyContexts() []KeyContext {
	contexts := overlayContexts()
	listed := make([]KeyContext, 0, len(contexts))

	for _, context := range contexts {
		listed = append(listed, context.listed())
	}

	return listed
}

// OverlayKeyContext is the open overlay's key context, as CheckKeys reads
// it, and false when no overlay is open.
func (m Model) OverlayKeyContext() (KeyContext, bool) {
	if m.overlay == nil {
		return KeyContext{}, false
	}

	return m.overlay.which().keyContext().listed(), true
}

// listed is the context by name, with every action live in it in the order
// the keymap defines them.
func (c keyContext) listed() KeyContext {
	_, builder := compileKeys(unicodeGlyphs(), "pull request", "Slack", nil)
	listed := KeyContext{Name: c.name}

	for _, placed := range builder.placements {
		if c.covers(placed) {
			listed.Actions = append(listed.Actions, placed.action)
		}
	}

	return listed
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
