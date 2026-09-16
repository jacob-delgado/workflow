// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package jira

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// detailFields is what the detail pane shows of one issue. The comments ride
// along in the same request rather than costing a second one.
const detailFields = "summary,status,issuetype,priority,description,reporter,comment"

// timeLayout is how Data Center writes a date: milliseconds and a numeric zone
// with no colon, which is not RFC 3339.
const timeLayout = "2006-01-02T15:04:05.000-0700"

// Comment is one comment on an issue.
type Comment struct {
	Author string
	Body   string
	// Created is when it was written, or the zero time when Jira wrote the date
	// in a form this package does not read.
	Created time.Time
}

// IssueDetail is one issue in full.
type IssueDetail struct {
	Issue       Issue
	Description string
	Reporter    string
	// Comments are the comments Jira sent, oldest first; CommentTotal is how
	// many there are in all.
	Comments     []Comment
	CommentTotal int
}

// person is a user as Jira names one.
type person struct {
	DisplayName string `json:"displayName"` //nolint:tagliatelle // Jira's field name on the wire, not ours to pick
}

// wireComment is a comment as Jira sends it.
type wireComment struct {
	Author  person `json:"author"`
	Body    string `json:"body"`
	Created string `json:"created"`
}

// wireComments is the comment field of an issue.
type wireComments struct {
	Total    int           `json:"total"`
	Comments []wireComment `json:"comments"`
}

// comment flattens a wire comment. A date in another form is left as the zero
// time: the comment is still worth showing without one.
func (w wireComment) comment() Comment {
	created, err := time.Parse(timeLayout, w.Created)
	if err != nil {
		created = time.Time{}
	}

	return Comment{Author: w.Author.DisplayName, Body: w.Body, Created: created.UTC()}
}

// Issue reads one issue in full, with its comments.
func (c Client) Issue(ctx context.Context, issueKey string) (IssueDetail, error) {
	query := url.Values{"fields": {detailFields}}.Encode()

	request, err := c.newRequest(ctx, http.MethodGet, issuePath(issueKey)+"?"+query, nil)
	if err != nil {
		return IssueDetail{}, err
	}

	body, err := c.exchange(request)
	if err != nil {
		return IssueDetail{}, err
	}

	var wire wireIssue

	err = json.Unmarshal(body, &wire)
	if err != nil {
		return IssueDetail{}, fmt.Errorf("reading the answer from %s: %w", c.settings.BaseURL, err)
	}

	comments := make([]Comment, 0, len(wire.Fields.Comment.Comments))
	for _, each := range wire.Fields.Comment.Comments {
		comments = append(comments, each.comment())
	}

	return IssueDetail{
		Issue:        wire.issue(),
		Description:  wire.Fields.Description,
		Reporter:     wire.Fields.Reporter.DisplayName,
		Comments:     comments,
		CommentTotal: wire.Fields.Comment.Total,
	}, nil
}

// AddComment posts a comment on an issue and returns it as Jira stored it.
func (c Client) AddComment(ctx context.Context, issueKey, text string) (Comment, error) {
	payload, err := json.Marshal(struct {
		Body string `json:"body"`
	}{Body: text})
	if err != nil {
		return Comment{}, fmt.Errorf("encoding the comment: %w", err)
	}

	request, err := c.newRequest(ctx, http.MethodPost, issuePath(issueKey)+"/comment", bytes.NewReader(payload))
	if err != nil {
		return Comment{}, err
	}

	request.Header.Set("Content-Type", "application/json")

	body, err := c.exchange(request)
	if err != nil {
		return Comment{}, err
	}

	var wire wireComment

	err = json.Unmarshal(body, &wire)
	if err != nil {
		return Comment{}, fmt.Errorf("reading the answer from %s: %w", c.settings.BaseURL, err)
	}

	return wire.comment(), nil
}

// BrowseURL is the address of an issue in Jira's web interface, for a link
// someone will click — in a Slack message, say. Any username and password in
// the base URL are left out: the link is shared, and so would they be. It is
// empty when the base URL cannot be read.
func (c Client) BrowseURL(issueKey string) string {
	base, err := url.Parse(c.settings.BaseURL)
	if err != nil || base.Host == "" {
		return ""
	}

	base.User = nil

	return base.String() + "/browse/" + url.PathEscape(issueKey)
}

// issuePath is where an issue lives in the API. The key is escaped because it is
// text a server supplied: it must stay one path segment.
func issuePath(issueKey string) string {
	return "/rest/api/2/issue/" + url.PathEscape(issueKey)
}
