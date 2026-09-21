// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package jira

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
)

// AssignedToMe is the working list: open issues assigned to whoever the
// credential belongs to, most recently touched first.
//
// statusCategory rather than resolution, because a workflow can close an issue
// without ever setting a resolution — and such an issue would otherwise sit in
// the list forever.
const AssignedToMe = "assignee = currentUser() AND statusCategory != done ORDER BY updated DESC"

// searchPath is Data Center's search. Cloud replaced it with /search/jql in
// 2025; Data Center still serves this one and answers 404 to the other.
const searchPath = "/rest/api/2/search"

// searchFields is everything a row shows, and what naming a branch for the issue
// needs, and nothing else. Each field requested is a field that can be null.
const searchFields = "summary,status,issuetype,priority"

// searchLimit is how many issues one page holds. The server caps a page at 1000
// regardless; Total says how many matched in all, so the interface can page
// through them.
const searchLimit = 50

// StatusCategory is Jira's coarse grouping of a status — where it sits in the
// to-do / in-progress / done progression — named the same on every instance
// whatever the status itself is called.
type StatusCategory string

const (
	// CategoryNew is a status not yet started.
	CategoryNew StatusCategory = "new"
	// CategoryIndeterminate is a status in progress.
	CategoryIndeterminate StatusCategory = "indeterminate"
	// CategoryDone is a status finished.
	CategoryDone StatusCategory = "done"
)

// Key identifies an issue, such as "PROJ-412". It is a named type rather than a
// bare string so a key and the text beside it — a comment's body, a pull
// request's URL and title — cannot be passed in each other's place: AddComment
// and LinkPullRequest took adjacent strings the compiler was content to see
// swapped.
type Key string

// Issue is one row of a search.
type Issue struct {
	Key            Key
	Summary        string
	Status         string
	StatusCategory StatusCategory
	// Type is the issue type's name, such as Bug or Story.
	Type string
	// Priority is the priority's name, or empty on an instance that has turned
	// priorities off — Jira then sends null.
	Priority string
}

// SearchResult is a page of issues and how many matched in all.
type SearchResult struct {
	Issues []Issue
	Total  int
}

// searchAnswer is the wire shape, decoded and then flattened into Issues.
type searchAnswer struct {
	Total  int         `json:"total"`
	Issues []wireIssue `json:"issues"`
}

// named is anything Jira describes by name: an issue type, a priority.
type named struct {
	Name string `json:"name"`
}

// names is the names of a list of named things, in order.
func names(items []named) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, item.Name)
	}

	return out
}

// wireLinked is another issue referred to from this one — a parent, a subtask,
// or the far side of an issue link — as Jira nests it.
type wireLinked struct {
	Key    string `json:"key"`
	Fields struct {
		Summary string `json:"summary"`
		Status  named  `json:"status"`
	} `json:"fields"`
}

// linked flattens a nested issue reference.
func (w wireLinked) linked() LinkedIssue {
	return LinkedIssue{Key: w.Key, Summary: w.Fields.Summary, Status: w.Fields.Status.Name}
}

// wireIssueLink is one entry of an issue's issuelinks: the relationship and the
// issue on whichever side of it Jira filled in.
type wireIssueLink struct {
	Type struct {
		Inward  string `json:"inward"`
		Outward string `json:"outward"`
	} `json:"type"`
	InwardIssue  *wireLinked `json:"inwardIssue"`  //nolint:tagliatelle // Jira's field name on the wire, not ours to pick
	OutwardIssue *wireLinked `json:"outwardIssue"` //nolint:tagliatelle // Jira's field name on the wire, not ours to pick
}

// link flattens an issue link to the relation and the issue it points at,
// reporting false for a link with neither side filled in.
func (w wireIssueLink) link() (IssueLink, bool) {
	switch {
	case w.InwardIssue != nil:
		return IssueLink{Relation: w.Type.Inward, Issue: w.InwardIssue.linked()}, true
	case w.OutwardIssue != nil:
		return IssueLink{Relation: w.Type.Outward, Issue: w.OutwardIssue.linked()}, true
	default:
		return IssueLink{}, false
	}
}

// wireIssue is an issue as Jira sends it, in a search or on its own. The fields
// past Reporter ride only on the detail request; a search leaves them zero.
type wireIssue struct {
	Key    string `json:"key"`
	Fields struct {
		Summary string `json:"summary"`
		Status  struct {
			Name     string `json:"name"`
			Category struct {
				Key string `json:"key"`
			} `json:"statusCategory"` //nolint:tagliatelle // Jira's field name on the wire, not ours to pick
		} `json:"status"`
		Type        named           `json:"issuetype"`
		Priority    named           `json:"priority"`
		Description string          `json:"description"`
		Reporter    person          `json:"reporter"`
		Assignee    person          `json:"assignee"`
		Labels      []string        `json:"labels"`
		Components  []named         `json:"components"`
		FixVersions []named         `json:"fixVersions"` //nolint:tagliatelle // Jira's wire name
		Parent      wireLinked      `json:"parent"`
		Subtasks    []wireLinked    `json:"subtasks"`
		IssueLinks  []wireIssueLink `json:"issuelinks"`
		Comment     wireComments    `json:"comment"`
	} `json:"fields"`
}

// issue flattens the parts of a wire issue every row carries.
func (w wireIssue) issue() Issue {
	return Issue{
		Key:            Key(w.Key),
		Summary:        w.Fields.Summary,
		Status:         w.Fields.Status.Name,
		StatusCategory: StatusCategory(w.Fields.Status.Category.Key),
		Type:           w.Fields.Type.Name,
		Priority:       w.Fields.Priority.Name,
	}
}

// Search runs a JQL query, returning one page of results from startAt. The
// result's Total says how many matched, so a caller can page to the end.
func (c Client) Search(ctx context.Context, jql string, startAt int) (SearchResult, error) {
	request, err := c.newRequest(ctx, http.MethodGet, searchPath+"?"+searchQuery(jql, startAt), nil)

	answer, err := decode[searchAnswer](c, request, err)

	return answer.result(), err
}

// searchQuery encodes a search's query string for the page starting at startAt.
func searchQuery(jql string, startAt int) string {
	return url.Values{
		"jql":        {jql},
		"fields":     {searchFields},
		"maxResults": {strconv.Itoa(searchLimit)},
		"startAt":    {strconv.Itoa(startAt)},
	}.Encode()
}

// result flattens the wire shape.
func (a searchAnswer) result() SearchResult {
	issues := make([]Issue, 0, len(a.Issues))

	for _, wire := range a.Issues {
		issues = append(issues, wire.issue())
	}

	return SearchResult{Issues: issues, Total: a.Total}
}
