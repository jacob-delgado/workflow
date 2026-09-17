// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
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
	changeStatus, comment, branchForIssue, filter key.Binding

	// Branch and Commits.
	newBranch, push, stage, stageAll, commit, runHooks, hookConfig key.Binding

	// Review and Slack.
	newPullRequest, compose, postWhenGreen key.Binding

	// In composers and previews.
	edit, editBody, nextTemplate, toggleDraft, verbatim, fullOutput key.Binding

	// Everywhere.
	toggleMouse, toggleHelp, quit, interrupt key.Binding
}

// binding is a key binding with its help.
func binding(help string, keys ...string) key.Binding {
	return key.NewBinding(key.WithKeys(keys...), key.WithHelp(keys[0], help))
}

// newKeyMap builds the key bindings, naming the arrow keys in the glyphs in use.
func newKeyMap(marks glyphs, reviewNoun string) keyMap {
	return keyMap{
		next:           binding("next pane", "tab"),
		previous:       binding("previous pane", "shift+tab"),
		jump:           key.NewBinding(key.WithKeys("1", "2", "3", "4", "5"), key.WithHelp("1-5", "jump to pane")),
		up:             key.NewBinding(key.WithKeys("up", "k"), key.WithHelp(marks.upKey+"/k", "up")),
		down:           key.NewBinding(key.WithKeys("down", "j"), key.WithHelp(marks.downKey+"/j", "down")),
		scrollUp:       key.NewBinding(key.WithKeys("pgup", "K"), key.WithHelp("pgup/K", "scroll up")),
		scrollDown:     key.NewBinding(key.WithKeys("pgdown", "J"), key.WithHelp("pgdn/J", "scroll down")),
		confirm:        binding("apply", "enter"),
		closeOverlay:   binding("close", "esc"),
		nextField:      binding("next field", "tab"),
		prevField:      binding("previous field", "shift+tab"),
		cycleLeft:      key.NewBinding(key.WithKeys("left"), key.WithHelp(marks.sideways, "change type")),
		cycleRight:     key.NewBinding(key.WithKeys("right")),
		refresh:        binding("refresh", "r"),
		retry:          binding("run again", "r"),
		changeStatus:   binding("change status", "t"),
		comment:        binding("comment", "c"),
		branchForIssue: binding("branch for issue", "b"),
		filter:         binding("filter", "/"),
		newBranch:      binding("new branch", "b"),
		push:           binding("push", "P"),
		stage:          key.NewBinding(key.WithKeys(" "), key.WithHelp("space", "stage/unstage")),
		stageAll:       binding("stage all", "a"),
		commit:         binding("commit", "c"),
		runHooks:       binding("run pre-commit", "h"),
		hookConfig:     binding("set up lefthook", "g"),
		newPullRequest: binding("open "+reviewNoun, "n"),
		compose:        binding("post to slack", "p"),
		postWhenGreen:  binding("post when CI passes", "w"),
		edit:           binding("edit", "e"),
		editBody:       binding("edit body", "ctrl+o"),
		nextTemplate:   binding("next template", "ctrl+t"),
		toggleDraft:    binding("draft", "ctrl+r"),
		verbatim:       binding("keep scripts whole", "v"),
		fullOutput:     binding("full output", "o"),
		toggleMouse:    binding("toggle mouse", "m"),
		toggleHelp:     binding("keys", "?"),
		quit:           binding("quit", "q"),
		interrupt:      binding("quit", "ctrl+c"),
	}
}

// ShortHelp is the footer's tail: enough to move around and to find the rest.
// The way to every other key comes first, so a narrow footer keeps it.
func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.toggleHelp, k.next, k.jump, k.quit}
}

// FullHelp is every key, in groups by where it works. Every binding the keyMap
// holds belongs to exactly one group, so the help is complete by construction.
func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.next, k.previous, k.jump, k.up, k.down, k.scrollUp, k.scrollDown},
		{k.changeStatus, k.comment, k.branchForIssue, k.filter, k.refresh},
		{k.newBranch, k.push, k.stage, k.stageAll, k.commit, k.runHooks, k.hookConfig},
		{k.newPullRequest, k.compose, k.postWhenGreen},
		{
			k.edit, k.editBody, k.nextTemplate, k.toggleDraft, k.verbatim,
			k.nextField, k.prevField, k.cycleLeft, k.cycleRight,
		},
		{k.retry, k.fullOutput},
		{k.confirm, k.closeOverlay, k.toggleMouse, k.toggleHelp, k.quit, k.interrupt},
	}
}

// everyBinding is every key the keyMap holds, drawn from the same groups the
// help shows, so a field that is not grouped is caught by the count.
func (k keyMap) everyBinding() []key.Binding {
	var all []key.Binding
	for _, group := range k.FullHelp() {
		all = append(all, group...)
	}

	return all
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
