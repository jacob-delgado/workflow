// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package jira

import (
	"context"
	"encoding/json"
	"errors"
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

// searchFields is everything a row and the detail pane display, and nothing
// else. Each field requested is a field that can be null or renamed.
const searchFields = "summary,status"

// searchLimit caps the list. A working list does not need paging, and the server
// silently caps it at 1000 anyway; Total says how many there were.
const searchLimit = 50

// Issue is one row of a search.
type Issue struct {
	Key            string
	Summary        string
	Status         string
	StatusCategory string
}

// SearchResult is a page of issues and how many matched in all.
type SearchResult struct {
	Issues []Issue
	Total  int
}

// searchAnswer is the wire shape, decoded and then flattened into Issues.
type searchAnswer struct {
	Total  int `json:"total"`
	Issues []struct {
		Key    string `json:"key"`
		Fields struct {
			Summary string `json:"summary"`
			Status  struct {
				Name     string `json:"name"`
				Category struct {
					Key string `json:"key"`
				} `json:"statusCategory"` //nolint:tagliatelle // Jira's field name on the wire, not ours to pick
			} `json:"status"`
		} `json:"fields"`
	} `json:"issues"`
}

// Search runs a JQL query.
func (c Client) Search(ctx context.Context, jql string) (SearchResult, error) {
	request, err := c.newRequest(ctx, http.MethodGet, searchPath+"?"+searchQuery(jql), nil)
	if err != nil {
		return SearchResult{}, err
	}

	response, err := c.exchange(request)
	if err != nil {
		return SearchResult{}, err
	}
	defer func() { _ = response.Body.Close() }()

	var answer searchAnswer

	err = json.NewDecoder(response.Body).Decode(&answer)
	if err != nil {
		return SearchResult{}, fmt.Errorf("reading the answer from %s: %w", c.settings.BaseURL, err)
	}

	return answer.result(), nil
}

// searchQuery encodes a search's query string.
func searchQuery(jql string) string {
	return url.Values{
		"jql":        {jql},
		"fields":     {searchFields},
		"maxResults": {strconv.Itoa(searchLimit)},
	}.Encode()
}

// cause strips net/http's *url.Error down to what actually went wrong. Its
// message quotes the whole request URL first — for a search, some 180
// characters of encoded JQL — and the pane clips each line, so "no such host"
// would be lost behind the query.
func cause(err error) error {
	transportErr, ok := errors.AsType[*url.Error](err)
	if ok {
		return transportErr.Err
	}

	return err
}

// result flattens the wire shape.
func (a searchAnswer) result() SearchResult {
	issues := make([]Issue, 0, len(a.Issues))

	for _, wire := range a.Issues {
		issues = append(issues, Issue{
			Key:            wire.Key,
			Summary:        wire.Fields.Summary,
			Status:         wire.Fields.Status.Name,
			StatusCategory: wire.Fields.Status.Category.Key,
		})
	}

	return SearchResult{Issues: issues, Total: a.Total}
}
