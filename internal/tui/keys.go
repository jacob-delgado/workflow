// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"strconv"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/seams"
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
//
// The reverse never holds: a verb that several panes share keeps one key on
// all of them — / searches, f opens the filter checklist, O sorts, r refreshes,
// o opens and y copies — so a key learned on one pane works on the next.
type keyMap struct {
	// Moving around.
	next, previous, jump  key.Binding
	up, down, first, last key.Binding
	scrollUp, scrollDown  key.Binding
	confirm, closeOverlay key.Binding
	nextField, prevField  key.Binding
	cycleLeft, cycleRight key.Binding
	refresh, retry        key.Binding

	// Issues.
	changeStatus, comment, assign, logWork, startWork, nextView, loadMore, trackIssue key.Binding
	searchIssues, filterIssues                                                        key.Binding

	// Branch and Commits.
	newBranch, switchBranch, linkIssue, rebase, push key.Binding
	stage, stageAll, unstageAll, discard             key.Binding
	commit, amend, fixup, runHooks, hookConfig       key.Binding

	// Review and messaging.
	newPullRequest, checks, rerun, merge, finish, compose, peopleAndGroups key.Binding

	// Tasks.
	startStop, markDone, addTask, annotateTask, modifyTask, undoTask, syncTasks key.Binding
	searchTasks, filterTasks, sortTasks                                         key.Binding

	// Summary.
	earlier, later, today, calendar, copySummary, postSummary key.Binding
	// Repositories.
	favoriteDir, goToDir, settings, localData key.Binding

	// Reviews.
	sortReviews, filterReviews key.Binding

	// Opening and copying a link, on the Issues, Review, Reviews and Tasks panes.
	openLink, copyLink key.Binding

	// Writing a comment, in the composer's normal mode.
	insert, appendAfter, appendLine, openLine, cursorLeft, cursorRight key.Binding

	// In composers and previews.
	edit, editBody, nextTemplate, toggleDraft, toggleBreaking, verbatim, fullOutput, showLog key.Binding

	// In the branch creator, the messaging preview and the link form.
	worktree, postWhenGreen, unlinkIssue key.Binding

	// In Local data, and in Settings.
	removeCache, removeAll, saveSettings, removeEntry key.Binding

	// Tagging in the messaging preview, and in People and groups.
	linkToSlack, notOnSlack, forgetOwner key.Binding

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
	groupReviews
	groupTasks
	groupSummary
	groupRepositories
	groupComposer
	groupWriting
	groupRunning
	groupEverywhere
)

// The shared list actions are handled on more than one pane, so each is named
// once here and referenced both from its binding and from every context it is
// live in — the conflict check reads them everywhere they answer a press.
const (
	actionRefresh  = "refresh"
	actionUp       = "up"
	actionDown     = "down"
	actionFirst    = "first"
	actionLast     = "last"
	actionOpenLink = "open-link"
	actionCopyLink = "copy-link"
)

// actionJumpToPane is the one action CheckKeys refuses to move: its keys are the
// pane numbers, one per pane, and Model.handleGlobalKey reads the pane from the
// digit pressed.
const actionJumpToPane = "jump-to-pane"

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
// key — an arrow drawn in the terminal's glyph, a "1-7" range, a "pgup/K" pair.
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
	reviewKeys(&builder, &keys)
	taskKeys(&builder, &keys)
	summaryKeys(&builder, &keys)
	repositoryKeys(&builder, &keys)
	composerKeys(&builder, &keys, marks)
	writingKeys(&builder, &keys, marks)
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
	into.jump = builder.bindShown(groupMoving, actionJumpToPane,
		"1-"+strconv.Itoa(paneCount), "jump to pane", paneNumbers()...)
	into.up = builder.bindShown(groupMoving, actionUp, marks.upKey+"/k", "up", "up", "k")
	into.down = builder.bindShown(groupMoving, actionDown, marks.downKey+"/j", "down", "down", "j")
	into.first = builder.bind(groupMoving, actionFirst, "first", "home")
	into.last = builder.bindShown(groupMoving, actionLast, "end/G", "last", "end", "G")
	into.scrollUp = builder.bindShown(groupMoving, "scroll-up", "pgup/K", "scroll up", "pgup", "K")
	into.scrollDown = builder.bindShown(groupMoving, "scroll-down", "pgdn/J", "scroll down", "pgdown", "J")
}

// issueKeys are the Issues pane's bindings, including opening and copying a
// link, and tracking the issue in Taskwarrior.
func issueKeys(builder *helpBuilder, into *keyMap) {
	into.changeStatus = builder.bind(groupIssues, "change-status", "change status", "t")
	into.comment = builder.bind(groupIssues, "comment", "comment", "c")
	into.assign = builder.bind(groupIssues, "assign", "assign", "a")
	into.logWork = builder.bind(groupIssues, "log-work", "log work", "w")
	into.startWork = builder.bind(groupIssues, "start-work", "start work", "b")
	into.searchIssues = builder.bind(groupIssues, "search-issues", "search", "/")
	into.filterIssues = builder.bind(groupIssues, "filter-issues", "filter", "f")
	into.nextView = builder.bind(groupIssues, "switch-view", "switch view", "v")
	into.loadMore = builder.bind(groupIssues, "load-more", "load more", "ctrl+n")
	into.openLink = builder.bind(groupIssues, actionOpenLink, "open", "o")
	into.copyLink = builder.bind(groupIssues, actionCopyLink, "copy url", "y")
	into.refresh = builder.bind(groupIssues, actionRefresh, "refresh", "r")
	into.trackIssue = builder.bind(groupIssues, "track-issue", "track in Taskwarrior", "T")
}

// branchAndCommitKeys are the Branch and Commits panes' bindings.
func branchAndCommitKeys(builder *helpBuilder, into *keyMap) {
	into.newBranch = builder.bind(groupBranchCommits, "new-branch", "new branch", "b")
	into.switchBranch = builder.bind(groupBranchCommits, "switch-branch", "switch branch", "s")
	into.linkIssue = builder.bind(groupBranchCommits, "link-issue", "link issue", "i")
	into.rebase = builder.bind(groupBranchCommits, "rebase", "rebase onto base", "u")
	into.push = builder.bind(groupBranchCommits, "push", "push", "P")
	into.stage = builder.bind(groupBranchCommits, "stage", "stage/unstage", "space")
	into.stageAll = builder.bind(groupBranchCommits, "stage-all", "stage all", "a")
	into.unstageAll = builder.bind(groupBranchCommits, "unstage-all", "unstage all", "U")
	into.discard = builder.bind(groupBranchCommits, "discard-change", "discard", "x")
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
	into.compose = builder.bind(groupReviewMessaging, "post", "announce to "+messagingService, "p")
	into.peopleAndGroups = builder.bind(groupReviewMessaging, "people-and-groups", "people and groups", "P")
}

// reviewKeys are the Reviews pane's bindings.
func reviewKeys(builder *helpBuilder, into *keyMap) {
	into.sortReviews = builder.bind(groupReviews, "sort-reviews", "sort", "O")
	into.filterReviews = builder.bind(groupReviews, "filter-reviews", "filter", "f")
}

// taskKeys are the Tasks pane's bindings. m toggles the mouse everywhere, so
// modify is e.
func taskKeys(builder *helpBuilder, into *keyMap) {
	into.startStop = builder.bind(groupTasks, "start-stop", "start/stop", "s")
	into.markDone = builder.bind(groupTasks, "mark-done", "mark done", "d")
	into.addTask = builder.bind(groupTasks, "add-task", "add", "a")
	into.annotateTask = builder.bind(groupTasks, "annotate-task", "annotate", "A")
	into.modifyTask = builder.bind(groupTasks, "modify-task", "modify", "e")
	into.undoTask = builder.bind(groupTasks, "undo-task", "undo", "u")
	into.syncTasks = builder.bind(groupTasks, "sync-tasks", "sync", "S")
	into.searchTasks = builder.bind(groupTasks, "search-tasks", "search", "/")
	into.filterTasks = builder.bind(groupTasks, "filter-tasks", "filter", "f")
	into.sortTasks = builder.bind(groupTasks, "sort-tasks", "sort", "O")
}

// repositoryKeys are the Repositories pane's bindings: marking the directory
// the cursor is on a favorite, or forgetting it, typing a directory to go to,
// and the settings and local data.
func repositoryKeys(builder *helpBuilder, into *keyMap) {
	into.favoriteDir = builder.bind(groupRepositories, "favorite-directory", "favorite", "f")
	into.goToDir = builder.bind(groupRepositories, "go-to-directory", "go to", "g")
	into.settings = builder.bind(groupRepositories, "settings", "settings", "S")
	into.localData = builder.bind(groupRepositories, "local-data", "local data", "L")
}

// summaryKeys are the Summary pane's bindings: the period before and after the
// one shown, today, and the summary copied as text or posted.
func summaryKeys(builder *helpBuilder, into *keyMap) {
	into.earlier = builder.bind(groupSummary, "earlier", "earlier", "[")
	into.later = builder.bind(groupSummary, "later", "later", "]")
	into.today = builder.bind(groupSummary, "today", "today", "t")
	into.calendar = builder.bind(groupSummary, "calendar", "calendar", "c")
	into.copySummary = builder.bind(groupSummary, "copy-summary", "copy as Markdown", "Y")
	into.postSummary = builder.bind(groupSummary, "post-summary", "post", "p")
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
	into.toggleBreaking = builder.bind(groupComposer, "toggle-breaking", "breaking", "ctrl+x")
	into.verbatim = builder.bind(groupComposer, "verbatim", "keep scripts whole", "v")
	into.showLog = builder.bind(groupComposer, "show-log", "show log", "l")
	into.nextField = builder.bind(groupComposer, "next-field", "next field", "tab")
	into.prevField = builder.bind(groupComposer, "previous-field", "previous field", "shift+tab")
	into.cycleLeft = builder.bindShown(groupComposer, "cycle-type-left", marks.sideways, "change type", "left")
	into.cycleRight = builder.bindShown(groupComposer, "cycle-type-right", "", "", "right")
	into.toggleOption = builder.bind(groupComposer, "toggle-option", "select", "space")
	into.worktree = builder.bind(groupComposer, "worktree", "worktree", "ctrl+g")
	into.postWhenGreen = builder.bind(groupComposer, "post-when-green", "announce when CI passes", "w")
	into.unlinkIssue = builder.bind(groupComposer, "unlink-issue", "unlink", "u")
	into.removeCache = builder.bind(groupComposer, "remove-cache", "remove the cache", "c")
	into.removeAll = builder.bind(groupComposer, "remove-everything", "remove everything", "C")
	into.saveSettings = builder.bind(groupComposer, "save-settings", "save", "ctrl+s")
	into.removeEntry = builder.bind(groupComposer, "remove-entry", "remove", "D")
	into.linkToSlack = builder.bind(groupComposer, "link-to-slack", "link to Slack", "a")
	into.notOnSlack = builder.bind(groupComposer, "not-on-slack", "not on Slack", "x")
	into.forgetOwner = builder.bind(groupComposer, "forget-owner", "forget", "d")
}

// writingKeys are the comment composer's normal-mode commands, vim's: those
// that start typing, and moving across a line. Up and down are the shared
// movement keys, k and j among them.
func writingKeys(builder *helpBuilder, into *keyMap, marks glyphs) {
	into.insert = builder.bind(groupWriting, "insert", "insert", "i")
	into.appendAfter = builder.bind(groupWriting, "append", "append", "a")
	into.appendLine = builder.bind(groupWriting, "append-line", "append at line end", "A")
	into.openLine = builder.bind(groupWriting, "open-line", "new line below", "o")
	into.cursorLeft = builder.bindShown(groupWriting, "cursor-left", marks.leftKey+"/h", "left", "left", "h")
	into.cursorRight = builder.bindShown(groupWriting, "cursor-right", marks.rightKey+"/l", "right", "right", "l")
}

// runningKeys are the bindings available while a command runs.
func runningKeys(builder *helpBuilder, into *keyMap) {
	into.stopRun = builder.bind(groupRunning, "stop", "stop", "s")
	into.retry = builder.bind(groupRunning, "run-again", "run again", "r")
	into.fullOutput = builder.bind(groupRunning, "full-output", "full output", "o")
}

// The words esc wears, one rule for every overlay: discard when it drops what
// was written; close when what was written is kept — saying so in a notice —
// or there was nothing to keep; cancel for a form or a last look not yet sent;
// back for a step nested in another; skip for an offer that follows a done
// act; stay at a guard.
const (
	escDiscard = "discard"
	escClose   = "close"
	escCancel  = "cancel"
	escBack    = "back"
	escSkip    = "skip"
	escStay    = "stay"
)

// everywhereKeys are the bindings every context answers to.
func everywhereKeys(builder *helpBuilder, into *keyMap) {
	into.confirm = builder.bind(groupEverywhere, "apply", "apply", "enter")
	into.closeOverlay = builder.bind(groupEverywhere, "close", escClose, "esc")
	into.toggleMouse = builder.bind(groupEverywhere, "toggle-mouse", "toggle mouse", "m")
	into.toggleHelp = builder.bind(groupEverywhere, "toggle-help", "keys", "?")
	into.quit = builder.bind(groupEverywhere, "quit", "quit", "q")
	into.interrupt = builder.bind(groupEverywhere, actionInterrupt, "quit", "ctrl+c")
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
		"Moving around", "Issues", "Branch and Commits", "Review and " + messagingService, "Reviews", "Tasks",
		"Summary", "Repositories", "In a composer or preview", "Writing a comment", "While a command runs", "Everywhere",
	}
}

// KeyActions is every action the help lists, in its order, with the ui.keys
// overrides applied: what another surface offers of the interface's keys, read
// from the same bindings the interface runs, so a rebinding reaches both. The
// review noun names open-pull-request for the forge, and the messaging service
// names its group, as the help does; a binding the help draws on another's
// line, with no words of its own, is left out, as the help leaves it. Each
// names the key it has with no override, too.
func KeyActions(reviewNoun, messagingService string, overrides map[string]string) []seams.KeyAction {
	_, builder := compileKeys(unicodeGlyphs(), reviewNoun, messagingService, overrides)
	_, defaults := compileKeys(unicodeGlyphs(), reviewNoun, messagingService, nil)
	groups := helpGroups(messagingService)
	listed := make([]seams.KeyAction, 0, len(builder.placements))

	for _, placed := range builder.placements {
		words := placed.binding.Help()
		if words.Desc == "" {
			continue
		}

		listed = append(listed, seams.KeyAction{
			Action: placed.action, Help: words.Desc, Group: groups[placed.group],
			Shown: words.Key, Keys: placed.binding.Keys(), Default: defaults.byAction[placed.action].Help().Key,
		})
	}

	return listed
}

// farthest is a step longer than any list or detail runs, which every list
// and every scroll clamps to its first or last row: the step home and end
// take.
const farthest = 1 << 20

// cursorKeys are the keys that move a list's cursor: a row at a time, or to
// either end.
func (k keyMap) cursorKeys() []key.Binding {
	return []key.Binding{k.up, k.down, k.first, k.last}
}

// stepOf is how far msg moves a list's cursor, down for a positive step: one
// row for up and down, to the end for first and last, and nowhere for any
// other key.
func (k keyMap) stepOf(msg tea.KeyPressMsg) int {
	switch {
	case key.Matches(msg, k.down):
		return 1
	case key.Matches(msg, k.up):
		return -1
	case key.Matches(msg, k.last):
		return farthest
	case key.Matches(msg, k.first):
		return -farthest
	}

	return 0
}

// listKeys is the footer of an overlay that is a list to choose from.
func (k keyMap) listKeys() []key.Binding {
	return []key.Binding{k.up, k.down, k.confirm, k.closeOverlay}
}
