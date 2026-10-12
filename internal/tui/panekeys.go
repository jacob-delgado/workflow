// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

// issueKeys are the Issues pane's bindings, including opening and copying a
// link, and tracking the issue in Taskwarrior.
func issueKeys(builder *helpBuilder, into *keyMap) {
	group := paneIssues.helpGroup()

	into.changeStatus = builder.bind(group, "change-status", "change status", "t")
	into.comment = builder.bind(group, "comment", "comment", "c")
	into.assign = builder.bind(group, "assign", "assign", "a")
	into.logWork = builder.bind(group, "log-work", "log work", "w")
	into.startWork = builder.bind(group, "start-work", "start work", "b")
	into.searchIssues = builder.bind(group, "search-issues", "search", "/")
	into.filterIssues = builder.bind(group, "filter-issues", "filter", "f")
	into.nextView = builder.bind(group, "switch-view", "switch view", "v")
	into.loadMore = builder.bind(group, "load-more", "load more", "ctrl+n")
	into.openLink = builder.bind(group, actionOpenLink, "open", "o")
	into.copyLink = builder.bind(group, actionCopyLink, "copy url", "y")
	into.refresh = builder.bind(group, actionRefresh, "refresh", "r")
	into.trackIssue = builder.bind(group, "track-issue", "track in Taskwarrior", "T")
}

// Actions that ask first and that the web page binds, named once for their
// panes and for CheckKeys, which keeps them off the key that confirms them
// and reads the page's list: the Branch pane's push and rebase, the Review
// pane's re-run, and the Tasks pane's mark done, undo and sync.
const (
	actionPush        = "push"
	actionRebase      = "rebase"
	actionRerunChecks = "rerun-checks"
	actionMarkDone    = "mark-done"
	actionUndoTask    = "undo-task"
	actionSyncTasks   = "sync-tasks"
)

// branchKeys are the Branch pane's bindings.
func branchKeys(builder *helpBuilder, into *keyMap) {
	group := paneBranch.helpGroup()

	into.newBranch = builder.bind(group, "new-branch", "new branch", "b")
	into.switchBranch = builder.bind(group, "switch-branch", "switch branch", "s")
	into.linkIssue = builder.bind(group, "link-issue", "link issue", "i")
	into.rebase = builder.bind(group, actionRebase, "rebase onto base", "u")
	into.push = builder.bind(group, actionPush, "push", "P")
}

// commitKeys are the Commits pane's bindings.
func commitKeys(builder *helpBuilder, into *keyMap) {
	group := paneCommits.helpGroup()

	into.stage = builder.bind(group, "stage", "stage/unstage", "space")
	into.stageAll = builder.bind(group, "stage-all", "stage all", "a")
	into.unstageAll = builder.bind(group, "unstage-all", "unstage all", "U")
	into.discard = builder.bind(group, "discard-change", "discard", "x")
	into.commit = builder.bind(group, "commit", "commit", "c")
	into.amend = builder.bind(group, "amend", "amend", "A")
	into.fixup = builder.bind(group, "fixup", "fix up", "f")
	into.runHooks = builder.bind(group, "run-pre-commit", "run pre-commit", "h")
	into.hookConfig = builder.bind(group, "set-up-lefthook", "set up lefthook", "g")
}

// reviewKeys are the Review pane's bindings, opening the pull request named
// for the forge.
func reviewKeys(builder *helpBuilder, into *keyMap, reviewNoun string) {
	group := paneReview.helpGroup()

	into.newPullRequest = builder.bind(group, "open-pull-request", "open "+reviewNoun, "n")
	into.checks = builder.bind(group, "checks", "checks", "c")
	into.rerun = builder.bind(group, actionRerunChecks, "re-run checks", "R")
	into.merge = builder.bind(group, "merge", "merge", "M")
	into.finish = builder.bind(group, "finish-branch", "finish branch", "F")
}

// messagingKeys are the messaging pane's bindings, announcing to the service
// in use.
func messagingKeys(builder *helpBuilder, into *keyMap, messagingService string) {
	group := paneMessaging.helpGroup()

	into.compose = builder.bind(group, "post", "announce to "+messagingService, "p")
	into.peopleAndGroups = builder.bind(group, "people-and-groups", "people and groups", "P")
}

// reviewQueueKeys are the Reviews pane's bindings.
func reviewQueueKeys(builder *helpBuilder, into *keyMap) {
	group := paneReviews.helpGroup()

	into.sortReviews = builder.bind(group, "sort-reviews", "sort", "O")
	into.filterReviews = builder.bind(group, "filter-reviews", "filter", "f")
}

// taskKeys are the Tasks pane's bindings. m toggles the mouse on every pane,
// so modify is e.
func taskKeys(builder *helpBuilder, into *keyMap) {
	group := paneTasks.helpGroup()

	into.startStop = builder.bind(group, "start-stop", "start/stop", "s")
	into.markDone = builder.bind(group, actionMarkDone, "mark done", "d")
	into.addTask = builder.bind(group, "add-task", "add", "a")
	into.annotateTask = builder.bind(group, "annotate-task", "annotate", "A")
	into.modifyTask = builder.bind(group, "modify-task", "modify", "e")
	into.undoTask = builder.bind(group, actionUndoTask, "undo", "u")
	into.syncTasks = builder.bind(group, actionSyncTasks, "sync", "S")
	into.searchTasks = builder.bind(group, "search-tasks", "search", "/")
	into.filterTasks = builder.bind(group, "filter-tasks", "filter", "f")
	into.sortTasks = builder.bind(group, "sort-tasks", "sort", "O")
}

// summaryKeys are the Summary pane's bindings: the period before and after the
// one shown, today, and the summary copied as text or posted.
func summaryKeys(builder *helpBuilder, into *keyMap) {
	group := paneSummary.helpGroup()

	into.earlier = builder.bind(group, "earlier", "earlier", "[")
	into.later = builder.bind(group, "later", "later", "]")
	into.today = builder.bind(group, "today", "today", "t")
	into.calendar = builder.bind(group, "calendar", "calendar", "c")
	into.copySummary = builder.bind(group, "copy-summary", "copy as Markdown", "Y")
	into.postSummary = builder.bind(group, "post-summary", "post", "p")
}

// repositoryKeys are the Repositories pane's bindings: marking the directory
// the cursor is on a favorite, or forgetting it, typing a directory to go to,
// and the settings and local data.
func repositoryKeys(builder *helpBuilder, into *keyMap) {
	group := paneRepositories.helpGroup()

	into.favoriteDir = builder.bind(group, "favorite-directory", "favorite", "f")
	into.goToDir = builder.bind(group, "go-to-directory", "go to", "g")
	into.settings = builder.bind(group, "settings", "settings", "S")
	into.localData = builder.bind(group, "local-data", "local data", "L")
}
