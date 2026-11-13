// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package jira

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// AssignedToMe is the working list: open issues assigned to whoever the
// credential belongs to, most recently touched first.
//
// statusCategory rather than resolution, because a workflow can close an issue
// without ever setting a resolution — and such an issue would otherwise sit in
// the list forever.
const AssignedToMe = assigneeIsMe + " AND statusCategory != done ORDER BY updated DESC"

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

// cursorSeparator parts a cursor's key from its offset in the cursor's text.
const cursorSeparator = "@"

// ErrNotACursor is text that is no cursor a page of issues gave.
var ErrNotACursor = errors.New("not a cursor a page of issues gave, such as PROJ-9@49")

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
	// Unavailable names each tracker whose issues a list drawing on more than
	// one could not read, so what is listed is not taken for all there is.
	Unavailable []string
	// Next is where the page after this one starts, which a page that ends
	// the list does not name.
	Next Cursor
}

// Cursor is where a later page of a list starts: after the issue After, which
// the list held at offset At when its page was read. The zero Cursor names
// no issue, and starts a list at its first page.
type Cursor struct {
	After Key
	At    int
}

// More reports that the cursor names an issue for a page to follow, which a
// page that ends its list leaves unnamed.
func (c Cursor) More() bool {
	return c.After != ""
}

// String is the cursor as text, its issue's key and offset as PROJ-9@49,
// which ParseCursor reads back.
func (c Cursor) String() string {
	return string(c.After) + cursorSeparator + strconv.Itoa(c.At)
}

// ParseCursor reads a cursor back from the text String made of it, refusing
// any other text with ErrNotACursor. A key holds no @, but the offset after it
// is the cursor's last part, written as String writes it: no sign and no
// leading zero.
func ParseCursor(text string) (Cursor, error) {
	key, offset, found := strings.CutLast(text, cursorSeparator)

	at, err := strconv.Atoi(offset)
	if !found || key == "" || err != nil || at < 0 || strconv.Itoa(at) != offset {
		return Cursor{}, fmt.Errorf("%w: %q", ErrNotACursor, text)
	}

	return Cursor{After: Key(key), At: at}, nil
}

// rest is what of issues follows the cursor's issue: everything after it when
// the page holds it, and the whole page when it does not, since an issue gone
// from the list leaves what followed it moved up into its place.
func (c Cursor) rest(issues []Issue) []Issue {
	for index, issue := range issues {
		if issue.Key == c.After {
			return issues[index+1:]
		}
	}

	return issues
}

// SearchAfter is the page of a JQL query that follows the issue after names,
// or the first page for the zero Cursor. It reads the list from where the
// issue was, and starts after it wherever in that page it is now, so issues
// joining or leaving the list ahead of it since neither skip nor repeat a
// row.
//
// Trade-off TRADE-66: Jira pages only by offset, so the page is read from the
// offset the issue had. Once it, or one issue ahead of it, has left the list,
// the page starts where it was, where what followed it moved to, but each
// further issue ahead of it that left skips a row; and one moved more than a
// page further down is not found, its page starting where it was.
func (c Client) SearchAfter(ctx context.Context, jql string, after Cursor) (SearchResult, error) {
	page, err := c.Search(ctx, jql, after.At)
	if err != nil {
		return SearchResult{}, err
	}

	page.Issues = after.rest(page.Issues)

	return page, nil
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
	InwardIssue  *wireLinked `json:"inwardIssue"`
	OutwardIssue *wireLinked `json:"outwardIssue"`
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
// past Priority ride only on the detail request; a search leaves them zero.
type wireIssue struct {
	Key    string `json:"key"`
	Fields struct {
		Summary string `json:"summary"`
		Status  struct {
			Name     string `json:"name"`
			Category struct {
				Key string `json:"key"`
			} `json:"statusCategory"`
		} `json:"status"`
		Type        named           `json:"issuetype"`
		Priority    named           `json:"priority"`
		Description string          `json:"description"`
		Reporter    person          `json:"reporter"`
		Assignee    person          `json:"assignee"`
		Labels      []string        `json:"labels"`
		Components  []named         `json:"components"`
		FixVersions []named         `json:"fixVersions"`
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
	return c.search(ctx, searchQuery(jql, startAt), startAt)
}

// SearchLenient is Search for a query naming values Jira need not know: an
// issue key whose issue is gone or hidden is skipped, where Search answers 400
// for the whole query. Only a key check asks this way; a view's own mistake
// should still be refused. NoKeys names no key, so it asks nothing: Jira
// would read the empty query as every issue.
func (c Client) SearchLenient(ctx context.Context, jql string, startAt int) (SearchResult, error) {
	if jql == NoKeys {
		return SearchResult{}, nil
	}

	query := searchQuery(jql, startAt)
	query.Set("validateQuery", "false")

	return c.search(ctx, query, startAt)
}

// search asks for the page query names, which starts at startAt.
func (c Client) search(ctx context.Context, query url.Values, startAt int) (SearchResult, error) {
	request, err := c.newRequest(ctx, http.MethodGet, searchPath+"?"+query.Encode(), nil)

	answer, err := decode[searchAnswer](c, request, err)

	return answer.result(startAt), err
}

// searchQuery is a search's query for the page starting at startAt.
func searchQuery(jql string, startAt int) url.Values {
	return url.Values{
		"jql":        {jql},
		"fields":     {searchFields},
		"maxResults": {strconv.Itoa(searchLimit)},
		"startAt":    {strconv.Itoa(startAt)},
	}
}

// result flattens the wire shape of the page that starts at startAt, naming
// its last issue as the cursor of the page after it unless it ends the list.
func (a searchAnswer) result(startAt int) SearchResult {
	issues := make([]Issue, 0, len(a.Issues))

	for _, wire := range a.Issues {
		issues = append(issues, wire.issue())
	}

	result := SearchResult{Issues: issues, Total: a.Total}

	if last := startAt + len(issues); len(issues) > 0 && last < a.Total {
		result.Next = Cursor{After: issues[len(issues)-1].Key, At: last - 1}
	}

	return result
}
