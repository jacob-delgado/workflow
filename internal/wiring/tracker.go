// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package wiring

import (
	"errors"
	"fmt"
	"sync"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/convention"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/sanitize"
	"github.com/jacob-delgado/workflow/internal/seams"
	"github.com/jacob-delgado/workflow/internal/store"
)

// errJiraOnly is something only a Jira issue takes, asked of a forge issue.
var errJiraOnly = errors.New("only a Jira issue takes this; a forge issue has it on its own page")

// forgeIssuesLabel names the forge's issues where a list says it could not
// read them.
const forgeIssuesLabel = "the forge's issues"

// combinedTracker is Jira and the repository's forge issues as one tracker:
// the forge's issues lead the first view's list, and every key goes to the
// tracker its shape names, so a forge number never reaches Jira, which would
// read it as the id of an unrelated issue.
//
// Trade-off TRADE-24: the first view and whether the forge joins in are read
// here, once; a Settings save that changes either applies after a restart.
func combinedTracker(settings config.Jira, jiraSide, forgeSide seams.Jira) seams.Jira {
	first := firstView(settings)
	listed := &forgeRows{}
	asked := &forgeRows{}

	return seams.Jira{
		Search: func(jql string, startAt int) (jira.SearchResult, error) {
			return listed.search(jiraSide.Search, forgeSide.Search, jql == first, jql, startAt)
		},
		SearchLenient: func(jql string, startAt int) (jira.SearchResult, error) {
			return asked.search(jiraSide.SearchLenient, forgeSide.Search, true, jql, startAt)
		},
		Issue:       byTracker(jiraSide.Issue, forgeSide.Issue),
		Transitions: byTracker(jiraSide.Transitions, forgeSide.Transitions),
		Transition: func(issueKey jira.Key, to jira.Transition, values []jira.FieldValue) error {
			return pick(issueKey, jiraSide.Transition, forgeSide.Transition)(issueKey, to, values)
		},
		Comment: func(issueKey jira.Key, text string) (jira.Comment, error) {
			return jiraOnly(issueKey, func() (jira.Comment, error) { return jiraSide.Comment(issueKey, text) })
		},
		Assign: func(issueKey jira.Key, assignee string) error {
			return pick(issueKey, jiraSide.Assign, forgeSide.Assign)(issueKey, assignee)
		},
		AddWorklog: func(issueKey jira.Key, timeSpent, comment string) (jira.Worklog, error) {
			return jiraOnly(issueKey, func() (jira.Worklog, error) { return jiraSide.AddWorklog(issueKey, timeSpent, comment) })
		},
		LinkPullRequest: func(issueKey jira.Key, pullURL, title string) error {
			_, err := jiraOnly(issueKey, func() (struct{}, error) {
				return struct{}{}, jiraSide.LinkPullRequest(issueKey, pullURL, title)
			})

			return err
		},
		BrowseURL: func(issueKey jira.Key) string {
			return pick(issueKey, jiraSide.BrowseURL, forgeSide.BrowseURL)(issueKey)
		},
	}
}

// firstView is the JQL of the list the Issues pane opens on: the first view
// configured, or the issues assigned to you when there is none.
func firstView(settings config.Jira) string {
	if len(settings.Views) == 0 {
		return jira.AssignedToMe
	}

	return jira.ScopedToMe(settings.Views[0].JQL)
}

// isForgeKey reports a key shaped like a forge issue number.
func isForgeKey(issueKey jira.Key) bool {
	ref, known := convention.RefOf(string(issueKey))

	return known && ref.Tracker == convention.TrackerForge
}

// pick is onForge for a forge issue's key and onJira for any other.
func pick[F any](issueKey jira.Key, onJira, onForge F) F {
	if isForgeKey(issueKey) {
		return onForge
	}

	return onJira
}

// byTracker is a one-key read that goes to the tracker the key belongs to.
func byTracker[T any](onJira, onForge func(jira.Key) (T, error)) func(jira.Key) (T, error) {
	return func(issueKey jira.Key) (T, error) { return pick(issueKey, onJira, onForge)(issueKey) }
}

// jiraOnly runs ask for a Jira issue, and refuses a forge issue's key without
// asking anything.
func jiraOnly[T any](issueKey jira.Key, ask func() (T, error)) (T, error) {
	if isForgeKey(issueKey) {
		var none T

		return none, fmt.Errorf("%w: %s", errJiraOnly, issueKey)
	}

	return ask()
}

// forgeRows remembers how many forge issues led a list's first page, so a
// later page asks Jira for its own rows from where its share of the list
// left off.
type forgeRows struct {
	lock  sync.Mutex
	count int
}

// search is one page of a list: Jira's alone, or, with withForge, the forge's
// issues ahead of Jira's first page and Jira's own rows after it. A forge that
// cannot be read leaves Jira's list, naming the forge unavailable; a Jira that
// cannot be read fails the list, as it always has.
func (f *forgeRows) search(
	jiraSearch, forgeList func(string, int) (jira.SearchResult, error), withForge bool, jql string, startAt int,
) (jira.SearchResult, error) {
	if !withForge {
		return jiraSearch(jql, startAt)
	}

	if startAt > 0 {
		return f.laterPage(jiraSearch, jql, startAt)
	}

	page, err := jiraSearch(jql, 0)
	if err != nil {
		return jira.SearchResult{}, err
	}

	forge, forgeErr := forgeList("", 0)
	if forgeErr != nil {
		forge = jira.SearchResult{Unavailable: []string{forgeIssuesLabel}}
	}

	f.lock.Lock()
	f.count = len(forge.Issues)
	f.lock.Unlock()

	return jira.SearchResult{
		Issues: append(forge.Issues, page.Issues...), Total: page.Total + len(forge.Issues),
		Unavailable: forge.Unavailable,
	}, nil
}

// laterPage is Jira's page past the forge rows the first page led with.
func (f *forgeRows) laterPage(
	jiraSearch func(string, int) (jira.SearchResult, error), jql string, startAt int,
) (jira.SearchResult, error) {
	f.lock.Lock()
	count := f.count
	f.lock.Unlock()

	page, err := jiraSearch(jql, max(0, startAt-count))
	if err != nil {
		return jira.SearchResult{}, err
	}

	page.Total += count

	return page, nil
}

// jiraRows is a list without the forge's issues. The issue cache is keyed by
// the Jira instance and the view, and a forge issue's number means an issue
// only within its repository, which that key does not hold: kept there, one
// repository's #42 would open another's list as an issue it does not have.
func jiraRows(issues []jira.Issue) []jira.Issue {
	kept := make([]jira.Issue, 0, len(issues))
	for _, issue := range issues {
		if !isForgeKey(issue.Key) {
			kept = append(kept, issue)
		}
	}

	return kept
}

// toCachedIssues reduces the tracker's issues to the store's shape, neutralizing
// terminal control in each field so nothing hostile is written to the file.
func toCachedIssues(issues []jira.Issue) []store.CachedIssue {
	cached := make([]store.CachedIssue, len(issues))
	for index, issue := range issues {
		cached[index] = store.CachedIssue{
			Key: sanitize.Line(string(issue.Key)), Summary: sanitize.Line(issue.Summary),
			Status: sanitize.Line(issue.Status), StatusCategory: sanitize.Line(string(issue.StatusCategory)),
			Type: sanitize.Line(issue.Type), Priority: sanitize.Line(issue.Priority),
		}
	}

	return cached
}

// fromCachedIssues rebuilds the tracker's issues from the store, sanitizing each
// field again: the store is a file on disk, so what it reads back is untrusted
// and must not reach the terminal as a control sequence.
func fromCachedIssues(cached []store.CachedIssue) []jira.Issue {
	issues := make([]jira.Issue, len(cached))
	for index, issue := range cached {
		issues[index] = jira.Issue{
			Key: jira.Key(sanitize.Line(issue.Key)), Summary: sanitize.Line(issue.Summary),
			Status: sanitize.Line(issue.Status), StatusCategory: jira.StatusCategory(sanitize.Line(issue.StatusCategory)),
			Type: sanitize.Line(issue.Type), Priority: sanitize.Line(issue.Priority),
		}
	}

	return issues
}
