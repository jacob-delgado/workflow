// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

// Package loop composes the developer loop once, for every surface: the pull
// request a branch proposes and the push it needs first. The command line, the
// terminal interface and the web server each hand it their seams — plain
// functions over the domain clients — and word its refusals in their own terms.
//
// It sits below the three surfaces and above the domain packages: it imports
// config, convention, forge, gitrepo, jira, messaging and proc, and nothing that
// imports it back. A depguard rule in .golangci.yml holds that direction.
package loop

import (
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/jira"
)

// Subjects is the subject line of each commit, oldest first, which is what a
// pull request's title and body are proposed from.
func Subjects(commits []gitrepo.Commit) []string {
	subjects := make([]string, 0, len(commits))
	for _, commit := range commits {
		subjects = append(subjects, commit.Subject)
	}

	return subjects
}

// issueSummary is the issue's summary, or empty when there is no issue, no way
// to read one, or the tracker cannot say — what it describes reads without it.
func issueSummary(read func(jira.Key) (jira.IssueDetail, error), key jira.Key) string {
	if read == nil || key == "" {
		return ""
	}

	detail, err := read(key)
	if err != nil {
		return ""
	}

	return detail.Issue.Summary
}

// issueURL links the issue, or empty when there is no issue or the tracker has
// no web address for one — issues read from the forge have none.
func issueURL(browse func(jira.Key) string, key jira.Key) string {
	if browse == nil || key == "" {
		return ""
	}

	return browse(key)
}
