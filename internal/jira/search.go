// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package jira

import (
	"context"
	"encoding/json"
	"fmt"
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

// Issue is one row of a search.
type Issue struct {
	Key            string
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

// wireIssue is an issue as Jira sends it, in a search or on its own.
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
		Type        named        `json:"issuetype"`
		Priority    named        `json:"priority"`
		Description string       `json:"description"`
		Reporter    person       `json:"reporter"`
		Comment     wireComments `json:"comment"`
	} `json:"fields"`
}

// issue flattens the parts of a wire issue every row carries.
func (w wireIssue) issue() Issue {
	return Issue{
		Key:            w.Key,
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
	if err != nil {
		return SearchResult{}, err
	}

	body, err := c.exchange(request)
	if err != nil {
		return SearchResult{}, err
	}

	var answer searchAnswer

	err = json.Unmarshal(body, &answer)
	if err != nil {
		return SearchResult{}, fmt.Errorf("reading the answer from %s: %w", c.settings.BaseURL, err)
	}

	return answer.result(), nil
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
