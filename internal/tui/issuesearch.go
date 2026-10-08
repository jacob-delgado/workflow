// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/jira"
)

// searchIssues is the command that fills, or refreshes, the Issues pane with its
// first page.
func (m Model) searchIssues() tea.Cmd {
	return m.searchPage(0)
}

// searching marks the Issues list in flight for a search of its first page
// about to start, when there is a tracker to search.
func (m Model) searching() Model {
	m.issues.loading = m.deps.Jira.Search != nil

	return m
}

// relistIssues is the command that reads the Issues pane's first page again,
// with the branches that mark its issues in flight, which may have changed
// since.
func (m Model) relistIssues() tea.Cmd {
	return tea.Batch(m.searchIssues(), m.listIssueBranches())
}

// searchPage is the command that reads one page of issues, from startAt.
func (m Model) searchPage(startAt int) tea.Cmd {
	search := m.deps.Jira.Search
	if search == nil {
		return nil
	}

	jql := m.activeView().jql

	return func() tea.Msg {
		found, err := search(jql, startAt)

		return issuesLoaded{found: found, err: err, startAt: startAt, jql: jql}
	}
}

// loadMoreIssues reads the next page when the list is truncated and one is not
// already on its way.
func (m Model) loadMoreIssues() (Model, tea.Cmd) {
	if m.issues.loading || !m.issues.hasMore() || m.deps.Jira.Search == nil {
		return m, nil
	}

	m.issues.loading = true

	return m, m.searchPage(len(m.issues.found.Issues))
}

// pageIfAtEnd reads the next page once the selection reaches the last loaded
// issue of a truncated list.
func (m Model) pageIfAtEnd() (Model, tea.Cmd) {
	if m.issues.selected < len(m.issues.found.Issues)-1 {
		return m, nil
	}

	return m.loadMoreIssues()
}

// issuesLoaded carries the search's answer back into the update loop. startAt is
// where the page began: past zero, it is a further page to append rather than a
// fresh list to replace.
type issuesLoaded struct {
	found   jira.SearchResult
	err     error
	startAt int
	// jql is the query this answers. A first page is cached under it whichever
	// view is active, but the answer reaches the list only while jql is still the
	// active view's: a reader who has moved on is waiting for another answer.
	jql string
}

var _ applier = issuesLoaded{}

// apply caches a fresh first page, then, when the answer is for the view on
// screen, records it and asks for the selected issue in full.
func (msg issuesLoaded) apply(m Model) (Model, tea.Cmd) {
	cache := cacheIssues(m.deps, msg)

	if msg.jql != m.activeView().jql {
		return m, cache
	}

	m.issues = m.issues.settle(msg)
	m, detail := m.resumeIssue().loadDetail()

	return m, tea.Batch(cache, detail)
}

// cacheIssues is the command that stores a freshly-loaded first page, so a
// later session opens on it before the tracker answers, off the update loop,
// and says so when the store could not keep it. Only a first page is cached; a
// failure and further pages are not.
func cacheIssues(deps Deps, msg issuesLoaded) tea.Cmd {
	write := deps.Store.CacheIssues
	if msg.err != nil || msg.startAt != 0 || write == nil {
		return nil
	}

	return func() tea.Msg {
		err := write(msg.jql, msg.found.Issues)
		if err != nil {
			return storeNotKept{notice: "the issue list was not kept for the next session: " + err.Error()}
		}

		return nil
	}
}

// seededIssues is the last issue list cached for the active view, settled and
// shown at once so a session opens on it before the tracker answers, or an empty
// list when the store has nothing for it. It reads the store, so it is for the
// interface being made, before any key; a view switched to reads it with
// readCachedIssues.
func (m Model) seededIssues() issueList {
	if m.deps.Store.CachedIssues == nil {
		return issueList{}
	}

	return seeded(m.deps.Store.CachedIssues(m.activeView().jql))
}

// seeded is an issue list settled on what the cache held, or an empty one when
// it held nothing.
func seeded(cached []jira.Issue, held bool) issueList {
	if !held {
		return issueList{}
	}

	return issueList{found: jira.SearchResult{Issues: cached, Total: len(cached)}, settled: true}
}

// readCachedIssues is the command that reads the cache for the active view off
// the update loop, so a view switched to shows its last list while its search
// is out.
func (m Model) readCachedIssues() tea.Cmd {
	read, jql := m.deps.Store.CachedIssues, m.activeView().jql
	if read == nil {
		return nil
	}

	return func() tea.Msg {
		cached, held := read(jql)

		return cachedIssuesRead{jql: jql, list: seeded(cached, held)}
	}
}

// cachedIssuesRead is the list the cache held for the view searched with jql.
type cachedIssuesRead struct {
	jql  string
	list issueList
}

var _ applier = cachedIssuesRead{}

// apply shows the cached list while the view it was read for is still shown
// and its search has not answered; an answer, or a list the cache did not hold,
// leaves the pane as it is.
func (read cachedIssuesRead) apply(m Model) (Model, tea.Cmd) {
	if read.jql != m.activeView().jql || m.issues.settled || !read.list.settled {
		return m, nil
	}

	m.issues.found, m.issues.settled = read.list.found, true

	return m, nil
}
