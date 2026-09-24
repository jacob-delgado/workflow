// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
)

// keyMap satisfies help.KeyMap, which is what renders it in the footer and the
// help overlay from one definition.
var _ help.KeyMap = keyMap{}

// Errors CheckKeys returns for a ui.keys map that cannot be used. Callers
// distinguish them with errors.Is.
var (
	// ErrUnknownKeyAction reports a ui.keys entry naming an action that does not
	// exist — a typo in the action id.
	ErrUnknownKeyAction = errors.New("ui.keys names a key action that does not exist")
	// ErrKeyConflict reports two actions bound to the same key where both are
	// live at once, so a press would be ambiguous.
	ErrKeyConflict = errors.New("ui.keys binds two actions to one key in the same context")
)

// keyMap is every key the interface answers to. It is built by a function and
// carried on the model rather than held in a package variable, which
// gochecknoglobals forbids.
//
// A key may mean different things in different panes — c comments on an issue
// and commits on the Commits pane — so each meaning is its own binding, with
// its own help, and a pane only ever matches its own.
type keyMap struct {
	// Moving around.
	next, previous, jump  key.Binding
	up, down              key.Binding
	scrollUp, scrollDown  key.Binding
	confirm, closeOverlay key.Binding
	nextField, prevField  key.Binding
	cycleLeft, cycleRight key.Binding
	refresh, retry        key.Binding

	// Issues.
	changeStatus, comment, assign, logWork, branchForIssue, filter, nextView, loadMore key.Binding

	// Branch and Commits.
	newBranch, switchTask, rebase, push, stage, stageAll, commit, amend, fixup, runHooks, hookConfig key.Binding

	// Review and messaging.
	newPullRequest, checks, rerun, merge, finish, compose key.Binding

	// Opening and copying a link, on the Issues and Review panes.
	openLink, copyLink key.Binding

	// In composers and previews.
	edit, editBody, nextTemplate, toggleDraft, toggleBreaking, verbatim, fullOutput key.Binding

	// In the branch creator, and in the messaging preview.
	worktree, postWhenGreen key.Binding

	// In a field form.
	toggleOption key.Binding

	// While a command runs.
	stopRun key.Binding

	// Everywhere.
	toggleMouse, toggleHelp, quit, interrupt key.Binding

	// full is every binding grouped for the help, accumulated as the bindings
	// are created so it cannot omit one. FullHelp returns it verbatim.
	full [][]key.Binding
}

// The help groups, in the order helpGroups names them. Every binding is created
// into one of these, so the help cannot leave a binding out.
const (
	groupMoving = iota
	groupIssues
	groupBranchCommits
	groupReviewMessaging
	groupComposer
	groupRunning
	groupEverywhere
)

// The shared list actions are handled on more than one pane, so each is named
// once here and referenced both from its binding and from every context it is
// live in — the conflict check reads them everywhere they answer a press.
const (
	actionRefresh  = "refresh"
	actionOpenLink = "open-link"
	actionCopyLink = "copy-link"
)

// placement is one binding as it was defined: the action it answers to, the help
// group it belongs to, and the binding itself. The set of placements is what
// FullHelp and CheckKeys both read.
type placement struct {
	group   int
	action  string
	binding key.Binding
}

// helpBuilder collects each binding as it is defined: into its help group so the
// help is built from the same bindings the keyMap holds, and by action so a
// ui.keys override can be applied to it and CheckKeys can reject an override that
// names no action. A binding cannot be created without landing in a group, which
// is what keeps FullHelp complete.
type helpBuilder struct {
	overrides  map[string]string
	groups     [][]key.Binding
	byAction   map[string]key.Binding
	placements []placement
}

// bind defines a binding for action whose shown help key is its first bound key,
// the common case.
func (b *helpBuilder) bind(group int, action, help string, keys ...string) key.Binding {
	return b.place(group, action, keys[0], help, keys...)
}

// bindShown defines a binding whose shown help key differs from its first bound
// key — an arrow drawn in the terminal's glyph, a "1-6" range, a "pgup/K" pair.
func (b *helpBuilder) bindShown(group int, action, shown, help string, keys ...string) key.Binding {
	return b.place(group, action, shown, help, keys...)
}

// place builds the binding for action, applying any ui.keys override, records it
// by action, and files it in its help group.
func (b *helpBuilder) place(group int, action, shown, help string, keys ...string) key.Binding {
	bind := b.bindingFor(action, shown, help, keys)
	b.byAction[action] = bind
	b.placements = append(b.placements, placement{group: group, action: action, binding: bind})

	return b.in(group, bind)
}

// bindingFor is the binding an action gets: the override's key, shown and bound,
// when ui.keys names one, and the defaults otherwise.
func (b *helpBuilder) bindingFor(action, shown, help string, keys []string) key.Binding {
	if override, ok := b.overrides[action]; ok {
		return key.NewBinding(key.WithKeys(override), key.WithHelp(override, help))
	}

	return key.NewBinding(key.WithKeys(keys...), key.WithHelp(shown, help))
}

// in files bind into group and returns it, so a field is assigned and grouped in
// one expression.
func (b *helpBuilder) in(group int, bind key.Binding) key.Binding {
	for len(b.groups) <= group {
		b.groups = append(b.groups, nil)
	}

	b.groups[group] = append(b.groups[group], bind)

	return bind
}

// compileKeys builds the key bindings under a set of ui.keys overrides, returning
// both the keyMap the interface runs and the builder CheckKeys inspects. Each
// group is defined through helpBuilder.place, which places every binding in a
// help group as it is created, so a new binding is shown in the help by
// construction — and an override changes the key shown as well as the key bound.
func compileKeys(marks glyphs, reviewNoun, messagingService string, overrides map[string]string) (keyMap, helpBuilder) {
	builder := helpBuilder{overrides: overrides, byAction: map[string]key.Binding{}}

	keys := keyMap{}
	movingKeys(&builder, &keys, marks)
	issueKeys(&builder, &keys)
	branchAndCommitKeys(&builder, &keys)
	reviewAndMessagingKeys(&builder, &keys, reviewNoun, messagingService)
	composerKeys(&builder, &keys, marks)
	runningKeys(&builder, &keys)
	everywhereKeys(&builder, &keys)

	keys.full = builder.groups

	return keys, builder
}

// newKeyMap builds the key bindings, naming the arrow keys in the glyphs in use
// and applying the ui.keys overrides.
func newKeyMap(marks glyphs, reviewNoun, messagingService string, overrides map[string]string) keyMap {
	keys, _ := compileKeys(marks, reviewNoun, messagingService, overrides)

	return keys
}

// movingKeys are the pane and cursor movement bindings.
func movingKeys(builder *helpBuilder, into *keyMap, marks glyphs) {
	into.next = builder.bind(groupMoving, "next-pane", "next pane", "tab")
	into.previous = builder.bind(groupMoving, "previous-pane", "previous pane", "shift+tab")
	into.jump = builder.bindShown(groupMoving, "jump-to-pane",
		"1-"+strconv.Itoa(paneCount), "jump to pane", paneNumbers()...)
	into.up = builder.bindShown(groupMoving, "up", marks.upKey+"/k", "up", "up", "k")
	into.down = builder.bindShown(groupMoving, "down", marks.downKey+"/j", "down", "down", "j")
	into.scrollUp = builder.bindShown(groupMoving, "scroll-up", "pgup/K", "scroll up", "pgup", "K")
	into.scrollDown = builder.bindShown(groupMoving, "scroll-down", "pgdn/J", "scroll down", "pgdown", "J")
}

// issueKeys are the Issues pane's bindings, including opening and copying a link.
func issueKeys(builder *helpBuilder, into *keyMap) {
	into.changeStatus = builder.bind(groupIssues, "change-status", "change status", "t")
	into.comment = builder.bind(groupIssues, "comment", "comment", "c")
	into.assign = builder.bind(groupIssues, "assign", "assign", "a")
	into.logWork = builder.bind(groupIssues, "log-work", "log work", "w")
	into.branchForIssue = builder.bind(groupIssues, "branch-for-issue", "branch for issue", "b")
	into.filter = builder.bind(groupIssues, "filter", "filter", "/")
	into.nextView = builder.bind(groupIssues, "switch-view", "switch view", "v")
	into.loadMore = builder.bind(groupIssues, "load-more", "load more", "ctrl+n")
	into.openLink = builder.bind(groupIssues, actionOpenLink, "open", "o")
	into.copyLink = builder.bind(groupIssues, actionCopyLink, "copy url", "y")
	into.refresh = builder.bind(groupIssues, actionRefresh, "refresh", "r")
}

// branchAndCommitKeys are the Branch and Commits panes' bindings.
func branchAndCommitKeys(builder *helpBuilder, into *keyMap) {
	into.newBranch = builder.bind(groupBranchCommits, "new-branch", "new branch", "b")
	into.switchTask = builder.bind(groupBranchCommits, "switch-task", "switch task", "s")
	into.rebase = builder.bind(groupBranchCommits, "rebase", "rebase onto base", "u")
	into.push = builder.bind(groupBranchCommits, "push", "push", "P")
	into.stage = builder.bind(groupBranchCommits, "stage", "stage/unstage", "space")
	into.stageAll = builder.bind(groupBranchCommits, "stage-all", "stage all", "a")
	into.commit = builder.bind(groupBranchCommits, "commit", "commit", "c")
	into.amend = builder.bind(groupBranchCommits, "amend", "amend", "A")
	into.fixup = builder.bind(groupBranchCommits, "fixup", "fix up", "f")
	into.runHooks = builder.bind(groupBranchCommits, "run-pre-commit", "run pre-commit", "h")
	into.hookConfig = builder.bind(groupBranchCommits, "set-up-lefthook", "set up lefthook", "g")
}

// reviewAndMessagingKeys are the Review and messaging panes' bindings.
func reviewAndMessagingKeys(builder *helpBuilder, into *keyMap, reviewNoun, messagingService string) {
	into.newPullRequest = builder.bind(groupReviewMessaging, "open-pull-request", "open "+reviewNoun, "n")
	into.checks = builder.bind(groupReviewMessaging, "checks", "checks", "c")
	into.rerun = builder.bind(groupReviewMessaging, "rerun-checks", "re-run checks", "R")
	into.merge = builder.bind(groupReviewMessaging, "merge", "merge", "M")
	into.finish = builder.bind(groupReviewMessaging, "finish-branch", "finish branch", "F")
	into.compose = builder.bind(groupReviewMessaging, "post", "announce to "+strings.ToLower(messagingService), "p")
}

// composerKeys are the composer, preview and field-form bindings, the branch
// creator's and the messaging preview's own keys among them: only those overlays
// answer them, so they are listed where they work rather than under the pane
// that opens the overlay.
func composerKeys(builder *helpBuilder, into *keyMap, marks glyphs) {
	into.edit = builder.bind(groupComposer, "edit", "edit", "e")
	into.editBody = builder.bind(groupComposer, "edit-body", "edit body", "ctrl+o")
	into.nextTemplate = builder.bind(groupComposer, "next-template", "next template", "ctrl+t")
	into.toggleDraft = builder.bind(groupComposer, "toggle-draft", "draft", "ctrl+r")
	into.toggleBreaking = builder.bind(groupComposer, "toggle-breaking", "breaking", "ctrl+b")
	into.verbatim = builder.bind(groupComposer, "verbatim", "keep scripts whole", "v")
	into.nextField = builder.bind(groupComposer, "next-field", "next field", "tab")
	into.prevField = builder.bind(groupComposer, "previous-field", "previous field", "shift+tab")
	into.cycleLeft = builder.bindShown(groupComposer, "cycle-type-left", marks.sideways, "change type", "left")
	into.cycleRight = builder.bindShown(groupComposer, "cycle-type-right", "", "", "right")
	into.toggleOption = builder.bind(groupComposer, "toggle-option", "select", "space")
	into.worktree = builder.bind(groupComposer, "worktree", "worktree", "ctrl+w")
	into.postWhenGreen = builder.bind(groupComposer, "post-when-green", "announce when CI passes", "w")
}

// runningKeys are the bindings available while a command runs.
func runningKeys(builder *helpBuilder, into *keyMap) {
	into.stopRun = builder.bind(groupRunning, "stop", "stop", "s")
	into.retry = builder.bind(groupRunning, "run-again", "run again", "r")
	into.fullOutput = builder.bind(groupRunning, "full-output", "full output", "o")
}

// everywhereKeys are the bindings every context answers to.
func everywhereKeys(builder *helpBuilder, into *keyMap) {
	into.confirm = builder.bind(groupEverywhere, "apply", "apply", "enter")
	into.closeOverlay = builder.bind(groupEverywhere, "close", "close", "esc")
	into.toggleMouse = builder.bind(groupEverywhere, "toggle-mouse", "toggle mouse", "m")
	into.toggleHelp = builder.bind(groupEverywhere, "toggle-help", "keys", "?")
	into.quit = builder.bind(groupEverywhere, "quit", "quit", "q")
	into.interrupt = builder.bind(groupEverywhere, "interrupt", "quit", "ctrl+c")
}

// CheckKeys reports whether a ui.keys override map is usable: every entry names a
// real action, and no two actions that are live at the same time are bound to one
// key. It builds the keymap the overrides produce and inspects it, so it checks
// exactly what the interface would run — the default set, with no overrides,
// passes. The glyphs only name the arrow keys in the help, which key collisions
// do not depend on, so a default set stands in for them.
func CheckKeys(overrides map[string]string) error {
	_, builder := compileKeys(unicodeGlyphs(), "pull request", "Slack", overrides)

	err := builder.unknownActions(overrides)
	if err != nil {
		return err
	}

	return builder.conflicts()
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
// the list actions refresh, open-link and copy-link act on the Branch, Commits,
// Review and review-requests panes though they are filed under Issues; edit acts
// on the Review pane as well as in a preview; and a field form reads up and
// down, which a composer otherwise excludes so that its tab can mean next-field
// rather than next-pane. An overlay's own keys, the branch creator's worktree
// and the messaging preview's wait for CI and channel among them, are filed
// with the composer's, so they are live there and not on the pane behind it.
func keyContexts() []keyContext {
	return []keyContext{
		{"the Issues pane", []int{groupMoving, groupEverywhere, groupIssues}, nil},
		{"the Branch and Commits panes", []int{groupMoving, groupEverywhere, groupBranchCommits}, []string{actionRefresh}},
		{
			"the Review and Slack panes",
			[]int{groupMoving, groupEverywhere, groupReviewMessaging},
			[]string{actionOpenLink, actionCopyLink, actionRefresh, "edit"},
		},
		{
			"the review-requests pane",
			[]int{groupMoving, groupEverywhere},
			[]string{actionOpenLink, actionCopyLink, actionRefresh},
		},
		{"a running command", []int{groupMoving, groupEverywhere, groupRunning}, nil},
		{"a composer or preview", []int{groupEverywhere, groupComposer}, []string{"up", "down"}},
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

// ShortHelp is the footer's tail: enough to move around and to find the rest,
// with the way to every other key first among them. It follows the focused
// pane's own verbs, and a footer too narrow for all of them drops from the end —
// all but ?, which the pane's last verbs give way to instead.
func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.toggleHelp, k.next, k.jump, k.quit}
}

// FullHelp is every key, in groups by where it works. It is the set built in
// compileKeys, so every binding the keyMap holds is shown and the help cannot
// silently drop one.
func (k keyMap) FullHelp() [][]key.Binding {
	return k.full
}

// helpGroups names the groups FullHelp returns, in the same order. The messaging
// group is named for the service in use rather than a fixed "Slack".
func helpGroups(messagingService string) []string {
	return []string{
		"Moving around", "Issues", "Branch and Commits", "Review and " + messagingService,
		"In a composer or preview", "While a command runs", "Everywhere",
	}
}

// listKeys is the footer of an overlay that is a list to choose from.
func (k keyMap) listKeys() []key.Binding {
	return []key.Binding{k.up, k.down, k.confirm, k.closeOverlay}
}
