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
	"strconv"
	"time"
)

// detailFields is what the detail pane shows of one issue. The comments ride
// along in the same request rather than costing a second one.
const detailFields = "summary,status,issuetype,priority,description,reporter,assignee," +
	"labels,components,fixVersions,parent,subtasks,issuelinks,comment"

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

// LinkedIssue is another issue referred to from this one — a parent or a
// subtask — by key, summary and status. Its zero value (an empty Key) means
// there is none.
type LinkedIssue struct {
	Key     string
	Summary string
	Status  string
}

// IssueLink is a relationship to another issue: what it is ("blocks", "is
// blocked by") and the issue on the other end.
type IssueLink struct {
	Relation string
	Issue    LinkedIssue
}

// IssueDetail is one issue in full.
type IssueDetail struct {
	Issue       Issue
	Description string
	Reporter    string
	Assignee    string
	Labels      []string
	Components  []string
	FixVersions []string
	// Parent is the issue this one is a subtask of, or the zero LinkedIssue when
	// it has none.
	Parent     LinkedIssue
	Subtasks   []LinkedIssue
	IssueLinks []IssueLink
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
func (c Client) Issue(ctx context.Context, issueKey Key) (IssueDetail, error) {
	query := url.Values{"fields": {detailFields}}.Encode()

	request, err := c.newRequest(ctx, http.MethodGet, issuePath(issueKey)+"?"+query, nil)

	wire, err := decode[wireIssue](c, request, err)
	if err != nil {
		return IssueDetail{}, err
	}

	return c.withAllComments(ctx, issueKey, wire.detail()), nil
}

// commentLoadCap bounds how many comments a single read pages in, so an issue
// with an enormous history — or a server that never advances — cannot drive an
// unbounded run of requests. Past it the "N of Total" heading shows the rest are
// there without loading them, which no one scrolls to in a terminal anyway.
const commentLoadCap = 1000

// withAllComments fetches any comments past the first page Jira folded into the
// issue, so the list does not trail CommentTotal. It is best effort: a page that
// fails to load, or the cap, stops the paging and leaves what arrived, and the
// "N of Total" heading still says how many of them are shown.
func (c Client) withAllComments(ctx context.Context, issueKey Key, detail IssueDetail) IssueDetail {
	for len(detail.Comments) < detail.CommentTotal && len(detail.Comments) < commentLoadCap {
		page, err := c.commentPage(ctx, issueKey, len(detail.Comments))
		if err != nil || len(page) == 0 {
			break
		}

		detail.Comments = append(detail.Comments, page...)
	}

	return detail
}

// commentPage reads one page of an issue's comments, starting at startAt.
func (c Client) commentPage(ctx context.Context, issueKey Key, startAt int) ([]Comment, error) {
	query := url.Values{"startAt": {strconv.Itoa(startAt)}}.Encode()

	request, err := c.newRequest(ctx, http.MethodGet, issuePath(issueKey)+"/comment?"+query, nil)

	wire, err := decode[wireComments](c, request, err)
	if err != nil {
		return nil, err
	}

	page := make([]Comment, 0, len(wire.Comments))
	for _, each := range wire.Comments {
		page = append(page, each.comment())
	}

	return page, nil
}

// detail flattens a whole issue: its fields, the issues it relates to, and its
// comments.
func (w wireIssue) detail() IssueDetail {
	fields := w.Fields

	comments := make([]Comment, 0, len(fields.Comment.Comments))
	for _, each := range fields.Comment.Comments {
		comments = append(comments, each.comment())
	}

	subtasks := make([]LinkedIssue, 0, len(fields.Subtasks))
	for _, each := range fields.Subtasks {
		subtasks = append(subtasks, each.linked())
	}

	links := make([]IssueLink, 0, len(fields.IssueLinks))
	for _, each := range fields.IssueLinks {
		if link, ok := each.link(); ok {
			links = append(links, link)
		}
	}

	return IssueDetail{
		Issue:        w.issue(),
		Description:  fields.Description,
		Reporter:     fields.Reporter.DisplayName,
		Assignee:     fields.Assignee.DisplayName,
		Labels:       fields.Labels,
		Components:   names(fields.Components),
		FixVersions:  names(fields.FixVersions),
		Parent:       fields.Parent.linked(),
		Subtasks:     subtasks,
		IssueLinks:   links,
		Comments:     comments,
		CommentTotal: fields.Comment.Total,
	}
}

// AddComment posts a comment on an issue and returns it as Jira stored it. When
// the instance is configured for Markdown comments, the text is rewritten as
// Jira's wiki markup first.
func (c Client) AddComment(ctx context.Context, issueKey Key, text string) (Comment, error) {
	if c.settings.MarkdownComments {
		text = WikiFromMarkdown(text)
	}

	payload, err := json.Marshal(struct {
		Body string `json:"body"`
	}{Body: text})
	if err != nil {
		return Comment{}, fmt.Errorf("encoding the comment: %w", err)
	}

	request, err := c.newRequest(ctx, http.MethodPost, issuePath(issueKey)+"/comment", bytes.NewReader(payload))
	if err == nil {
		request.Header.Set("Content-Type", "application/json")
	}

	wire, err := decode[wireComment](c, request, err)
	if err != nil {
		return Comment{}, err
	}

	return wire.comment(), nil
}

// remoteLink is the body that adds a web link to an issue.
type remoteLink struct {
	Object remoteLinkObject `json:"object"`
}

// remoteLinkObject is the link itself, as Jira's remote-link API names it.
type remoteLinkObject struct {
	URL   string `json:"url"`
	Title string `json:"title"`
}

// LinkPullRequest records a pull request as a web link on an issue, so the team
// that looks at Jira sees the work without an integration installed on the
// server. Jira folds a repeat POST of the same URL into the existing link
// rather than adding a second, so confirming twice is harmless.
func (c Client) LinkPullRequest(ctx context.Context, issueKey Key, pullURL, title string) error {
	payload, err := json.Marshal(remoteLink{Object: remoteLinkObject{URL: pullURL, Title: title}})
	if err != nil {
		return fmt.Errorf("encoding the link: %w", err)
	}

	request, err := c.newRequest(ctx, http.MethodPost, issuePath(issueKey)+"/remotelink", bytes.NewReader(payload))
	if err != nil {
		return err
	}

	request.Header.Set("Content-Type", "application/json")

	_, err = c.exchange(request)

	return err
}

// BrowseURL is the address of an issue in Jira's web interface, for a link
// someone will click — in a Slack message, say. Any username and password in
// the base URL are left out: the link is shared, and so would they be. It is
// empty when the base URL cannot be read.
func (c Client) BrowseURL(issueKey Key) string {
	base, err := url.Parse(c.settings.BaseURL)
	if err != nil || base.Host == "" {
		return ""
	}

	base.User = nil

	return base.String() + "/browse/" + url.PathEscape(string(issueKey))
}

// issuePath is where an issue lives in the API. The key is escaped because it is
// text a server supplied: it must stay one path segment.
func issuePath(issueKey Key) string {
	return "/rest/api/2/issue/" + url.PathEscape(string(issueKey))
}
