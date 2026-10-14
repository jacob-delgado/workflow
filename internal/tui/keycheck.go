// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"fmt"
	"maps"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/seams"
)

// actionInterrupt is the action every context answers first, even while text
// is typed.
const actionInterrupt = "interrupt"

// CheckKeys reports whether a ui.keys override map is usable. It refuses an
// entry naming no action (config.ErrUnknownKeyAction); a move of jump-to-pane,
// which answers a digit per pane that no one key stands in for
// (config.ErrKeyNotRebindable); interrupt on a key that types or edits text
// (config.ErrInterruptEdits); two actions live at the same time bound to one
// key, on a pane, in a section of the web page that shows several panes, or in
// an overlay (config.ErrKeyConflict); an action a text field's overlay answers
// on a key the field edits with (config.ErrTextFieldKey); and an action whose
// press opens a last look on a key apply has, which would confirm it
// (config.ErrKeyConfirmsItself). It builds the keymap the overrides produce and
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

	err = builder.confirmClash()
	if err != nil {
		return err
	}

	err = builder.conflicts()
	if err != nil {
		return err
	}

	return builder.textFieldClash()
}

// askingActions are the actions whose press opens a last look, which apply
// confirms. Where apply is live beside one, sharing its key would be a
// conflict as well, but this check comes first so the refusal says why the
// key cannot be shared; Local data's removals are answered where apply is
// not, and only this check keeps them off its key.
func askingActions() []string {
	return []string{
		actionPush, actionRebase, "discard-change", actionRerunChecks, actionMarkDone, actionUndoTask, actionSyncTasks,
		"forget-owner", "remove-entry", "remove-cache", "remove-everything", actionSwitchDirectory,
	}
}

// confirmClash rejects an action that asks first on a key apply has: the
// press that opened its last look, made again, would confirm it.
func (b *helpBuilder) confirmClash() error {
	confirming := b.byAction["apply"].Keys()

	for _, placed := range b.placements {
		if !slices.Contains(askingActions(), placed.action) {
			continue
		}

		for _, boundKey := range placed.binding.Keys() {
			if slices.Contains(confirming, boundKey) {
				return fmt.Errorf("%w: %s on %q, which confirms its last look as apply",
					config.ErrKeyConfirmsItself, placed.action, boundKey)
			}
		}
	}

	return nil
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
			if editsText(boundKey) || slices.Contains(textFieldKeys(), boundKey) {
				return fmt.Errorf("%w: %s on %q", config.ErrTextFieldKey, placed.action, boundKey)
			}
		}
	}

	return nil
}

// textFieldKeys are the keys bubbles' text input moves its cursor, edits and
// steps its suggestions with, read from the key map every field uses: the
// arrows, home and end, the readline keys such as ctrl+w, the word keys such
// as alt+left, and up and down, which the mouse wheel sends an overlay too.
// Tab, which accepts a suggestion, is left out: next-field takes it first.
func textFieldKeys() []string {
	fieldKeys := textinput.DefaultKeyMap()
	bindings := []key.Binding{
		fieldKeys.CharacterForward, fieldKeys.CharacterBackward, fieldKeys.WordForward, fieldKeys.WordBackward,
		fieldKeys.DeleteWordBackward, fieldKeys.DeleteWordForward, fieldKeys.DeleteAfterCursor,
		fieldKeys.DeleteBeforeCursor, fieldKeys.DeleteCharacterBackward, fieldKeys.DeleteCharacterForward,
		fieldKeys.LineStart, fieldKeys.LineEnd, fieldKeys.Paste, fieldKeys.NextSuggestion, fieldKeys.PrevSuggestion,
	}

	keys := make([]string, 0, len(bindings))
	for _, binding := range bindings {
		keys = append(keys, binding.Keys()...)
	}

	return keys
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
// each pane's, with the actions its handler answers; each section of the web
// page that shows more than one pane; and the overlays'.
func keyContexts() []keyContext {
	contexts := make([]keyContext, 0, paneCount)
	for each := range pane(paneCount) {
		contexts = append(contexts, each.keyContext())
	}

	contexts = append(contexts, sharedSectionContexts()...)

	return append(contexts, overlayContexts()...)
}

// sharedSectionContexts are the key contexts of the web page's sections that
// show more than one pane, in the order the sections are named. The page
// answers a key with the first control on screen bound to it, so there every
// action it binds of each pane shown is live at once.
func sharedSectionContexts() []keyContext {
	panesIn := map[seams.PageSection][]pane{}
	for each := range pane(paneCount) {
		panesIn[each.webSection()] = append(panesIn[each.webSection()], each)
	}

	var contexts []keyContext

	for _, section := range slices.Sorted(maps.Keys(panesIn)) {
		if shown := panesIn[section]; len(shown) > 1 {
			contexts = append(contexts, shownTogether(shown))
		}
	}

	return contexts
}

// shownTogether is the key context of panes the web page shows in one
// section: the actions of each pane's own the page binds, all at once, named
// for the panes. An action only the terminal answers has no control there, so
// it may share a key with the other pane's.
func shownTogether(panes []pane) keyContext {
	titles := make([]string, 0, len(panes))
	together := keyContext{name: "", groups: nil, actions: nil}
	webBound := WebBoundActions()

	for _, each := range panes {
		titles = append(titles, each.title("messaging"))
		together.actions = append(together.actions, slices.DeleteFunc(each.keyContext().listed().Actions,
			func(action string) bool { return !slices.Contains(webBound, action) })...)
	}

	together.name = "the " + strings.Join(titles, " and ") + " panes, which the web page shows together"

	return together
}

// WebBoundActions is the actions the --web page has a control for, by the
// names ui.keys gives them, which are the only ones live in a section of the
// page. web/src/features/keyboard/boundActions.ts is the page's own copy, and
// a test holds the two to one list.
func WebBoundActions() []string {
	return []string{
		actionJumpToPane, "change-status", "comment", "assign", "log-work", "search-issues", "switch-view",
		"load-more", actionOpenLink, "track-issue", "link-issue", actionRebase, actionPush, "stage-all",
		"unstage-all", "commit", "amend", "fixup", "run-hook", "set-up-lefthook", "open-pull-request",
		actionRerunChecks, "merge", "finish-branch", "post", "sort-reviews", "start-stop", actionMarkDone, "add-task",
		"annotate-task", "modify-task", actionUndoTask, actionSyncTasks, "search-tasks", "sort-tasks", "earlier", "later",
		"today", "copy-summary", "post-summary", "favorite-directory", "go-to-directory", "toggle-help",
	}
}

// overlayContexts are the keyboard surfaces an overlay opens, one for each
// kind of overlay, in the order the kinds are named.
func overlayContexts() []keyContext {
	contexts := overlayKeyContexts()
	ordered := make([]keyContext, 0, len(contexts))

	for _, kind := range slices.Sorted(maps.Keys(contexts)) {
		ordered = append(ordered, contexts[kind])
	}

	return ordered
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
	overlayDirPicker
	overlayFinishPreview
	overlayFixupPicker
	overlayHelp
	overlayHookgenOffer
	overlayHookPicker
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

// keyContext is the keys live while an overlay of the kind is open.
func (k overlayKind) keyContext() keyContext {
	return overlayKeyContexts()[k]
}

// overlayKeyContexts is each kind of overlay's key context: the actions its
// handleKey answers in any of its states, an overlay taking every other key
// as typing or as nothing. The table is a map keyed by kind so that
// exhaustive refuses a kind left out.
func overlayKeyContexts() map[overlayKind]keyContext {
	return map[overlayKind]keyContext{
		overlayAmendPreview:  answering("the amend preview", "apply", "close"),
		overlayBranchCreator: answering("the branch creator", "worktree", "apply", "close"),
		overlayBranchLinker:  answering("the link form", "unlink-issue", "apply", "close"),
		overlayBranchPicker:  listAnswering("the branch picker", "apply", "close"),
		overlayCalendar: answering("the calendar", actionUp, actionDown, "cycle-type-left", "cycle-type-right",
			"next-field", "previous-field", "toggle-option", "apply", "close"),
		overlayChecklist:  listAnswering("a checklist", "toggle-option", "apply", "close"),
		overlayChecks:     listAnswering("the checks list", "show-log", "apply", "close"),
		overlayCommandRun: listAnswering("a running command", "stop", "run-again", "full-output", "apply", "close"),
		overlayCommentComposer: answering("the comment composer", actionUp, actionDown, "cursor-left", "cursor-right",
			"insert", "append", "append-line", "open-line", "edit-body", "apply", "close"),
		overlayCommentPreview: answering("the comment preview", "apply", "close"),
		overlayCommitComposer: answering("the commit composer", "next-field", "previous-field", "cycle-type-left",
			"cycle-type-right", "toggle-breaking", "edit-body", "apply", "close"),
		overlayDirPicker: listAnswering("the directory picker", "next-field", "cursor-left", "cursor-right",
			"switch-directory", "apply", "close"),
		overlayFinishPreview: answering("the finish preview", "apply", "close"),
		overlayFixupPicker:   listAnswering("the fixup picker", "apply", "close"),
		overlayHelp:          listAnswering("the help", "scroll-up", "scroll-down", "toggle-help", "quit", "close"),
		overlayHookgenOffer:  answering("the lefthook offer", "verbatim", "apply", "close"),
		overlayHookPicker:    listAnswering("the hook picker", "apply", "close"),
		overlayIssueLinker:   answering("the Jira link offer", "apply", "close"),
		overlayIssueWrite:    answering("the assign or log-work form", "apply", "close"),
		overlayJobLog:        listAnswering("a job's log", "scroll-up", "scroll-down", "close"),
		overlayLastLook:      answering("a last look", "apply", "close"),
		overlayLocalData:     answering("Local data", actionRefresh, "remove-cache", "remove-everything", "close"),
		overlayMergePicker:   listAnswering("the merge picker", "apply", "close"),
		overlayMessagingPreview: listAnswering("the announcement preview", "cycle-type-left", "cycle-type-right",
			"edit", "post-when-green", "toggle-option", "link-to-slack", "not-on-slack", "apply", "close"),
		overlayOwnerPicker: answering("the owner picker"),
		overlayPeople: listAnswering("People and groups", "next-field", "toggle-option", actionRefresh,
			"not-on-slack", "forget-owner", "apply", "close"),
		overlayPRComposer: answering("the pull request composer", "next-field", "previous-field", "next-template",
			"toggle-draft", "edit-body", "apply", "close"),
		overlayPREditor:  answering("the pull request editor", "edit-body", "apply", "close"),
		overlayQuitGuard: answering("the quit guard", "apply", "close"),
		overlaySettings: listAnswering("Settings", "cycle-type-left", "cycle-type-right", "toggle-option",
			actionRefresh, "save-settings", "remove-entry", "apply", "close"),
		overlaySetupForm: listAnswering("the setup form", "apply", "close"),
		overlayStatusPicker: listAnswering("the status picker", "next-field", "previous-field", "toggle-option",
			"apply", "close"),
		overlaySummaryPost: answering("the summary's post preview", "cycle-type-left", "cycle-type-right", "edit",
			"apply", "close"),
		overlayTaskLine: answering("a task form", "apply", "close"),
	}
}

// answering is an overlay's key context: interrupt, which Update reads
// before an open overlay sees the key, and the actions the overlay answers.
func answering(name string, actions ...string) keyContext {
	return keyContext{name: name, groups: nil, actions: append([]string{actionInterrupt}, actions...)}
}

// listAnswering is the key context of an overlay that is a list: the cursor
// keys, which step it, and the actions it answers.
func listAnswering(name string, actions ...string) keyContext {
	return answering(name, append([]string{actionUp, actionDown, actionFirst, actionLast}, actions...)...)
}

// ListedKeyContext is one keyboard surface — a pane, or an overlay while it is
// open — by name, and every action live on it at once. CheckKeys refuses a
// ui.keys map that binds two of its actions to one key.
type ListedKeyContext struct {
	Name    string
	Actions []string
}

// OverlayKeyContexts is the key context of every overlay, as CheckKeys
// reads them.
func OverlayKeyContexts() []ListedKeyContext {
	contexts := overlayContexts()
	listed := make([]ListedKeyContext, 0, len(contexts))

	for _, context := range contexts {
		listed = append(listed, context.listed())
	}

	return listed
}

// OverlayKeyContext is the open overlay's key context, as CheckKeys reads
// it, and false when no overlay is open.
func (m Model) OverlayKeyContext() (ListedKeyContext, bool) {
	if m.overlay == nil {
		return ListedKeyContext{}, false
	}

	return m.overlay.which().keyContext().listed(), true
}

// listed is the context by name, with every action live in it in the order
// the keymap defines them.
func (c keyContext) listed() ListedKeyContext {
	_, builder := compileKeys(unicodeGlyphs(), "pull request", "Slack", nil)
	listed := ListedKeyContext{Name: c.name}

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
