// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

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

// branchKeys are the Branch pane's bindings.
func branchKeys(builder *helpBuilder, into *keyMap) {
	into.newBranch = builder.bind(groupBranch, "new-branch", "new branch", "b")
	into.switchBranch = builder.bind(groupBranch, "switch-branch", "switch branch", "s")
	into.linkIssue = builder.bind(groupBranch, "link-issue", "link issue", "i")
	into.rebase = builder.bind(groupBranch, "rebase", "rebase onto base", "u")
	into.push = builder.bind(groupBranch, "push", "push", "P")
}

// commitKeys are the Commits pane's bindings.
func commitKeys(builder *helpBuilder, into *keyMap) {
	into.stage = builder.bind(groupCommits, "stage", "stage/unstage", "space")
	into.stageAll = builder.bind(groupCommits, "stage-all", "stage all", "a")
	into.unstageAll = builder.bind(groupCommits, "unstage-all", "unstage all", "U")
	into.discard = builder.bind(groupCommits, "discard-change", "discard", "x")
	into.commit = builder.bind(groupCommits, "commit", "commit", "c")
	into.amend = builder.bind(groupCommits, "amend", "amend", "A")
	into.fixup = builder.bind(groupCommits, "fixup", "fix up", "f")
	into.runHooks = builder.bind(groupCommits, "run-pre-commit", "run pre-commit", "h")
	into.hookConfig = builder.bind(groupCommits, "set-up-lefthook", "set up lefthook", "g")
}

// reviewKeys are the Review pane's bindings, opening the pull request named
// for the forge.
func reviewKeys(builder *helpBuilder, into *keyMap, reviewNoun string) {
	into.newPullRequest = builder.bind(groupReview, "open-pull-request", "open "+reviewNoun, "n")
	into.checks = builder.bind(groupReview, "checks", "checks", "c")
	into.rerun = builder.bind(groupReview, "rerun-checks", "re-run checks", "R")
	into.merge = builder.bind(groupReview, "merge", "merge", "M")
	into.finish = builder.bind(groupReview, "finish-branch", "finish branch", "F")
}

// messagingKeys are the messaging pane's bindings, announcing to the service
// in use.
func messagingKeys(builder *helpBuilder, into *keyMap, messagingService string) {
	into.compose = builder.bind(groupMessaging, "post", "announce to "+messagingService, "p")
	into.peopleAndGroups = builder.bind(groupMessaging, "people-and-groups", "people and groups", "P")
}

// reviewQueueKeys are the Reviews pane's bindings.
func reviewQueueKeys(builder *helpBuilder, into *keyMap) {
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

// repositoryKeys are the Repositories pane's bindings: marking the directory
// the cursor is on a favorite, or forgetting it, typing a directory to go to,
// and the settings and local data.
func repositoryKeys(builder *helpBuilder, into *keyMap) {
	into.favoriteDir = builder.bind(groupRepositories, "favorite-directory", "favorite", "f")
	into.goToDir = builder.bind(groupRepositories, "go-to-directory", "go to", "g")
	into.settings = builder.bind(groupRepositories, "settings", "settings", "S")
	into.localData = builder.bind(groupRepositories, "local-data", "local data", "L")
}
