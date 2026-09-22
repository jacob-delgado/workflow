// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"strconv"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
)

// keyMap satisfies help.KeyMap, which is what renders it in the footer and the
// help overlay from one definition.
var _ help.KeyMap = keyMap{}

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
	newBranch, switchTask, worktree, rebase, push, stage, stageAll, commit, amend, fixup, runHooks, hookConfig key.Binding

	// Review and Slack.
	newPullRequest, checks, rerun, merge, finish, compose, postWhenGreen key.Binding

	// Opening and copying a link, on the Issues and Review panes.
	openLink, copyLink key.Binding

	// In composers and previews.
	edit, editBody, nextTemplate, toggleDraft, toggleBreaking, verbatim, fullOutput key.Binding
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
	groupReviewSlack
	groupComposer
	groupRunning
	groupEverywhere
)

// binding is a key binding with its help.
func binding(help string, keys ...string) key.Binding {
	return key.NewBinding(key.WithKeys(keys...), key.WithHelp(keys[0], help))
}

// keyHelp is a binding whose shown key differs from the first bound key — an
// arrow drawn in the terminal's glyph, a "1-6" range, a "pgup/K" pair.
func keyHelp(shown, help string, keys ...string) key.Binding {
	return key.NewBinding(key.WithKeys(keys...), key.WithHelp(shown, help))
}

// helpBuilder collects each binding into its help group as it is defined, so the
// help is built from the same bindings the keyMap holds — a binding cannot be
// created without landing in a group, which is what keeps FullHelp complete.
type helpBuilder struct {
	groups [][]key.Binding
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

// newKeyMap builds the key bindings, naming the arrow keys in the glyphs in use.
// Each group is defined through helpBuilder.in, which places every binding in a
// help group as it is created, so a new binding is shown in the help by
// construction — FullHelp returns exactly what was built here.
func newKeyMap(marks glyphs, reviewNoun string) keyMap {
	var builder helpBuilder

	keys := keyMap{}
	movingKeys(&builder, &keys, marks)
	issueKeys(&builder, &keys)
	branchAndCommitKeys(&builder, &keys)
	reviewAndSlackKeys(&builder, &keys, reviewNoun)
	composerKeys(&builder, &keys, marks)
	runningKeys(&builder, &keys)
	everywhereKeys(&builder, &keys)

	keys.full = builder.groups

	return keys
}

// movingKeys are the pane and cursor movement bindings.
func movingKeys(builder *helpBuilder, into *keyMap, marks glyphs) {
	into.next = builder.in(groupMoving, binding("next pane", "tab"))
	into.previous = builder.in(groupMoving, binding("previous pane", "shift+tab"))
	into.jump = builder.in(groupMoving, keyHelp("1-"+strconv.Itoa(paneCount), "jump to pane", paneNumbers()...))
	into.up = builder.in(groupMoving, keyHelp(marks.upKey+"/k", "up", "up", "k"))
	into.down = builder.in(groupMoving, keyHelp(marks.downKey+"/j", "down", "down", "j"))
	into.scrollUp = builder.in(groupMoving, keyHelp("pgup/K", "scroll up", "pgup", "K"))
	into.scrollDown = builder.in(groupMoving, keyHelp("pgdn/J", "scroll down", "pgdown", "J"))
}

// issueKeys are the Issues pane's bindings, including opening and copying a link.
func issueKeys(builder *helpBuilder, into *keyMap) {
	into.changeStatus = builder.in(groupIssues, binding("change status", "t"))
	into.comment = builder.in(groupIssues, binding("comment", "c"))
	into.assign = builder.in(groupIssues, binding("assign", "a"))
	into.logWork = builder.in(groupIssues, binding("log work", "w"))
	into.branchForIssue = builder.in(groupIssues, binding("branch for issue", "b"))
	into.filter = builder.in(groupIssues, binding("filter", "/"))
	into.nextView = builder.in(groupIssues, binding("switch view", "v"))
	into.loadMore = builder.in(groupIssues, binding("load more", "ctrl+n"))
	into.openLink = builder.in(groupIssues, binding("open", "o"))
	into.copyLink = builder.in(groupIssues, binding("copy url", "y"))
	into.refresh = builder.in(groupIssues, binding("refresh", "r"))
}

// branchAndCommitKeys are the Branch and Commits panes' bindings.
func branchAndCommitKeys(builder *helpBuilder, into *keyMap) {
	into.newBranch = builder.in(groupBranchCommits, binding("new branch", "b"))
	into.switchTask = builder.in(groupBranchCommits, binding("switch task", "s"))
	into.worktree = builder.in(groupBranchCommits, binding("worktree", "ctrl+w"))
	into.rebase = builder.in(groupBranchCommits, binding("rebase onto base", "u"))
	into.push = builder.in(groupBranchCommits, binding("push", "P"))
	into.stage = builder.in(groupBranchCommits, keyHelp("space", "stage/unstage", "space"))
	into.stageAll = builder.in(groupBranchCommits, binding("stage all", "a"))
	into.commit = builder.in(groupBranchCommits, binding("commit", "c"))
	into.amend = builder.in(groupBranchCommits, binding("amend", "A"))
	into.fixup = builder.in(groupBranchCommits, binding("fix up", "f"))
	into.runHooks = builder.in(groupBranchCommits, binding("run pre-commit", "h"))
	into.hookConfig = builder.in(groupBranchCommits, binding("set up lefthook", "g"))
}

// reviewAndSlackKeys are the Review and Slack panes' bindings.
func reviewAndSlackKeys(builder *helpBuilder, into *keyMap, reviewNoun string) {
	into.newPullRequest = builder.in(groupReviewSlack, binding("open "+reviewNoun, "n"))
	into.checks = builder.in(groupReviewSlack, binding("checks", "c"))
	into.rerun = builder.in(groupReviewSlack, binding("re-run checks", "R"))
	into.merge = builder.in(groupReviewSlack, binding("merge", "M"))
	into.finish = builder.in(groupReviewSlack, binding("finish branch", "F"))
	into.compose = builder.in(groupReviewSlack, binding("post to slack", "p"))
	into.postWhenGreen = builder.in(groupReviewSlack, binding("post when CI passes", "w"))
}

// composerKeys are the composer, preview and field-form bindings.
func composerKeys(builder *helpBuilder, into *keyMap, marks glyphs) {
	into.edit = builder.in(groupComposer, binding("edit", "e"))
	into.editBody = builder.in(groupComposer, binding("edit body", "ctrl+o"))
	into.nextTemplate = builder.in(groupComposer, binding("next template", "ctrl+t"))
	into.toggleDraft = builder.in(groupComposer, binding("draft", "ctrl+r"))
	into.toggleBreaking = builder.in(groupComposer, binding("breaking", "ctrl+b"))
	into.verbatim = builder.in(groupComposer, binding("keep scripts whole", "v"))
	into.nextField = builder.in(groupComposer, binding("next field", "tab"))
	into.prevField = builder.in(groupComposer, binding("previous field", "shift+tab"))
	into.cycleLeft = builder.in(groupComposer, keyHelp(marks.sideways, "change type", "left"))
	into.cycleRight = builder.in(groupComposer, key.NewBinding(key.WithKeys("right")))
	into.toggleOption = builder.in(groupComposer, binding("select", "space"))
}

// runningKeys are the bindings available while a command runs.
func runningKeys(builder *helpBuilder, into *keyMap) {
	into.stopRun = builder.in(groupRunning, binding("stop", "s"))
	into.retry = builder.in(groupRunning, binding("run again", "r"))
	into.fullOutput = builder.in(groupRunning, binding("full output", "o"))
}

// everywhereKeys are the bindings every context answers to.
func everywhereKeys(builder *helpBuilder, into *keyMap) {
	into.confirm = builder.in(groupEverywhere, binding("apply", "enter"))
	into.closeOverlay = builder.in(groupEverywhere, binding("close", "esc"))
	into.toggleMouse = builder.in(groupEverywhere, binding("toggle mouse", "m"))
	into.toggleHelp = builder.in(groupEverywhere, binding("keys", "?"))
	into.quit = builder.in(groupEverywhere, binding("quit", "q"))
	into.interrupt = builder.in(groupEverywhere, binding("quit", "ctrl+c"))
}

// ShortHelp is the footer's tail: enough to move around and to find the rest,
// with the way to every other key first among them. It follows the focused
// pane's own verbs, and a footer too narrow for all of them drops from the end,
// so an action-dense pane can push the tail off a very narrow terminal.
func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.toggleHelp, k.next, k.jump, k.quit}
}

// FullHelp is every key, in groups by where it works. It is the set built in
// newKeyMap, so every binding the keyMap holds is shown and the help cannot
// silently drop one.
func (k keyMap) FullHelp() [][]key.Binding {
	return k.full
}

// helpGroups names the groups FullHelp returns, in the same order.
func helpGroups() []string {
	return []string{
		"Moving around", "Issues", "Branch and Commits", "Review and Slack",
		"In a composer", "While a command runs", "Everywhere",
	}
}

// listKeys is the footer of an overlay that is a list to choose from.
func (k keyMap) listKeys() []key.Binding {
	return []key.Binding{k.up, k.down, k.confirm, k.closeOverlay}
}
