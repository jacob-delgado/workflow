// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"context"

	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/tui/latest"
)

// searchIssues is the command that makes the search of the Issues pane's
// first page that New began, as Init asks.
func (m Model) searchIssues() tea.Cmd {
	return m.searchPage(0, m.issues.read.Round())
}

// readingIssues begins a search of the Issues list's first page, superseding
// any still out and any further page asked of the list as it stood, and is
// the command that searches; with no tracker to search, it begins nothing.
func (m Model) readingIssues() (Model, tea.Cmd) {
	if m.deps.Jira.Search == nil {
		return m, nil
	}

	read, round := m.issues.read.Begin()
	m.issues.read = read

	return m, m.searchPage(0, round)
}

// relistingIssues begins a search of the Issues pane's first page, as
// readingIssues does, and is the command that searches with the branches
// that mark its issues in flight, which may have changed since.
func (m Model) relistingIssues() (Model, tea.Cmd) {
	m, search := m.readingIssues()

	return m, tea.Batch(search, listIssueBranches(m.session, m.cfg, m.deps))
}

// searchPage is the command that reads one page of issues, from startAt, for
// the list searched in round.
func (m Model) searchPage(startAt int, round latest.Round) tea.Cmd {
	search := m.deps.Jira.Search
	if search == nil {
		return nil
	}

	ctx, jql := m.session, m.activeView().JQL

	return func() tea.Msg {
		found, err := search(ctx, jql, startAt)

		return issuesLoaded{found: found, err: err, startAt: startAt, jql: jql, round: round}
	}
}

// loadMoreIssues reads the next page when the list can be paged.
func (m Model) loadMoreIssues() (Model, tea.Cmd) {
	if !m.issues.canPage() {
		return m, nil
	}

	m.issues.paging = true

	return m, m.searchPage(len(m.issues.found().Issues), m.issues.read.Round())
}

// pageIfAtEnd reads the next page once the selection reaches the last loaded
// issue of a truncated list that the filter and the places admit.
func (m Model) pageIfAtEnd() (Model, tea.Cmd) {
	if m.issues.selected < len(m.issues.visible())-1 {
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
	// jql is the query this answers, which a first page is cached under.
	jql string
	// round is the search of a first page the list was read from: a page of
	// a list read afresh since, by a refresh or by its view left and come
	// back to, reaches nothing.
	round latest.Round
}

var _ applier = issuesLoaded{}

// apply records a first page the latest search found, caches it, and asks
// for the selected issue in full; the answer of a search a later one
// superseded is dropped, and cached no more than shown. A further page goes
// to pageAnswered.
func (msg issuesLoaded) apply(m Model) (Model, tea.Cmd) {
	if msg.startAt > 0 {
		return m.pageAnswered(msg), nil
	}

	read, taken := m.issues.read.Answered(msg.round, msg.found, msg.err)
	if !taken {
		return m, nil
	}

	m.issues = m.issues.settled(read)
	m, detail := m.resumeIssue().followDetail()

	return m, tea.Batch(cacheIssues(m.session, m.deps, msg), detail)
}

// pageAnswered appends a further page to the list it was asked of, which is
// then no longer paging; a page that failed leaves what is listed rather than
// replacing the pane with an error. A page of a list searched afresh since
// reaches nothing: that search's answer ends the paging.
func (m Model) pageAnswered(msg issuesLoaded) Model {
	if msg.round != m.issues.read.Round() {
		return m
	}

	m.issues.paging = false

	if msg.err == nil {
		m.issues = m.issues.appended(msg.found)
	}

	return m
}

// cacheIssues is the command that stores a freshly-loaded first page, so a
// later session opens on it before the tracker answers, off the update loop,
// and says so when the store could not keep it. Only a first page is cached; a
// failure and further pages are not. It is stored under ctx.
func cacheIssues(ctx context.Context, deps Deps, msg issuesLoaded) tea.Cmd {
	write := deps.Store.CacheIssues
	if msg.err != nil || msg.startAt != 0 || write == nil {
		return nil
	}

	return func() tea.Msg {
		err := write(ctx, msg.jql, msg.found.Issues)
		if err != nil {
			return storeNotKept{words: "the issue list was not kept for the next session: " + err.Error()}
		}

		return nil
	}
}

// searchingFromCache begins the search of the Issues list's first page that
// Init makes, and seeds it with the last issue list cached for the active
// view, shown at once so a session opens on it before the tracker answers.
// It reads the store, so it is for the interface being made, before any key;
// a view switched to reads it with readCachedIssues.
func (m Model) searchingFromCache() Model {
	if m.deps.Jira.Search == nil {
		return m
	}

	read, round := m.issues.read.Begin()
	m.issues.read = read

	if m.deps.Store.CachedIssues == nil {
		return m
	}

	cached, held := m.deps.Store.CachedIssues(m.session, m.activeView().JQL)
	m, _ = cachedIssuesRead{round: round, issues: cached, held: held}.apply(m)

	return m
}

// readCachedIssues is the command that reads the cache for the active view off
// the update loop, so a view switched to shows its last list while its search,
// begun in the Issues list's round, is out.
func (m Model) readCachedIssues() tea.Cmd {
	ctx, read, jql, round := m.session, m.deps.Store.CachedIssues, m.activeView().JQL, m.issues.read.Round()
	if read == nil {
		return nil
	}

	return func() tea.Msg {
		cached, held := read(ctx, jql)

		return cachedIssuesRead{round: round, issues: cached, held: held}
	}
}

// cachedIssuesRead is the list the cache held, if it held one, for the view
// searched in round.
type cachedIssuesRead struct {
	round  latest.Round
	issues []jira.Issue
	held   bool
}

var _ applier = cachedIssuesRead{}

// apply shows the cached list in place of the answer of the search it was
// read for, while that search is the latest and has shown nothing of its
// own: still out, or failed, the cache then listed beside the failure. A list
// the cache did not hold leaves the pane as it is.
func (read cachedIssuesRead) apply(m Model) (Model, tea.Cmd) {
	if !read.held {
		return m, nil
	}

	seeded, taken := m.issues.read.Seeded(read.round, jira.SearchResult{Issues: read.issues, Total: len(read.issues)})
	if taken {
		m.issues.read = seeded
	}

	return m, nil
}
