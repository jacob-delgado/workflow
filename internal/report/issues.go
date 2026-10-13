// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package report

import (
	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/convention"
	"github.com/jacob-delgado/workflow/internal/jira"
)

// Issue is a tracker issue in the API's shape.
func Issue(issue jira.Issue) api.Issue {
	return api.Issue{
		Key:            string(issue.Key),
		Tracker:        TrackerOf(issue.Key),
		Summary:        issue.Summary,
		Status:         issue.Status,
		StatusCategory: StatusCategory(issue.StatusCategory),
		Type:           issue.Type,
		Priority:       optional(issue.Priority),
	}
}

// StatusCategory is a status's category in the API's shape. The contract
// names Jira's three categories; any other — Jira's No Category, keyed
// "undefined", which a status no one sorted into a category is filed under,
// or a row read back from a store someone changed — is carried as not yet
// started, which is what such a status is to the loop: not done. Carried as
// it came, it would make the page refuse the whole frame it is in.
func StatusCategory(category jira.StatusCategory) api.StatusCategory {
	if category == jira.CategoryIndeterminate || category == jira.CategoryDone {
		return api.StatusCategory(category)
	}

	return api.StatusCategoryNew
}

// TrackerOf is where the issue a key names lives, by the key's shape: a
// number is the forge's, anything else Jira's.
func TrackerOf(issueKey jira.Key) api.IssueTracker {
	ref, known := convention.RefOf(string(issueKey))
	if known && ref.Tracker == convention.TrackerForge {
		return api.IssueTrackerForge
	}

	return api.IssueTrackerJira
}

// IssuesPage is a page of search results in the API's shape, keeping the
// caller's start index. Its issues and its unavailable trackers are empty
// lists, never null.
func IssuesPage(result jira.SearchResult, startAt int) api.IssuesPage {
	issues := make([]api.Issue, 0, len(result.Issues))
	for _, issue := range result.Issues {
		issues = append(issues, Issue(issue))
	}

	unavailable := result.Unavailable
	if unavailable == nil {
		unavailable = []string{}
	}

	return api.IssuesPage{Issues: issues, Total: result.Total, StartAt: startAt, Unavailable: unavailable}
}

// Comment is a comment on an issue in the API's shape.
func Comment(comment jira.Comment) api.Comment {
	return api.Comment{Author: comment.Author, Body: comment.Body, Created: optionalTime(comment.Created)}
}

// IssueDetail is an issue read in full in the API's shape, with its comments
// oldest first and link, the issue's page in the tracker ("" when there is
// none to give).
func IssueDetail(detail jira.IssueDetail, link string) api.IssueDetail {
	comments := make([]api.Comment, 0, len(detail.Comments))
	for _, comment := range detail.Comments {
		comments = append(comments, Comment(comment))
	}

	return api.IssueDetail{
		Key:            string(detail.Issue.Key),
		Tracker:        TrackerOf(detail.Issue.Key),
		Summary:        detail.Issue.Summary,
		Status:         detail.Issue.Status,
		StatusCategory: StatusCategory(detail.Issue.StatusCategory),
		Type:           detail.Issue.Type,
		Priority:       optional(detail.Issue.Priority),
		Reporter:       detail.Reporter,
		Assignee:       optional(detail.Assignee),
		Description:    detail.Description,
		Comments:       comments,
		CommentTotal:   detail.CommentTotal,
		URL:            link,
	}
}

// optional carries an empty string to the wire as an absent field rather than
// an empty one, for the properties the contract marks optional.
func optional(text string) *string {
	if text == "" {
		return nil
	}

	return &text
}
