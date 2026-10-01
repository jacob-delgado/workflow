// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package convention

import (
	"regexp"
	"strings"
)

// TitleSource decides where a pull request's title comes from.
type TitleSource string

const (
	// TitleFromCommit takes the title from the branch's oldest commit, which on a
	// branch of Conventional Commits already reads as a title. It is the default.
	TitleFromCommit TitleSource = "commit"
	// TitleFromIssue takes the title from the issue the branch names.
	TitleFromIssue TitleSource = "issue"
)

// PullRequestTitleFrom proposes a pull request's title from the chosen source,
// falling back to the other when the chosen one has nothing to offer.
func PullRequestTitleFrom(source TitleSource, subjects []string, issueKey, summary string) string {
	if source == TitleFromIssue {
		return titleFromIssue(subjects, issueKey, summary)
	}

	return titleFromCommit(subjects, issueKey, summary)
}

// titleFromCommit prefers the oldest commit, then the issue, then the summary.
func titleFromCommit(subjects []string, issueKey, summary string) string {
	switch {
	case len(subjects) > 0:
		return subjects[0]
	case issueKey != "":
		return issueKey + ": " + summary
	default:
		return summary
	}
}

// titleFromIssue prefers the issue, then the oldest commit, then the summary —
// falling back when the branch names no issue or its summary is unknown.
func titleFromIssue(subjects []string, issueKey, summary string) string {
	switch {
	case issueKey != "" && summary != "":
		return issueKey + ": " + summary
	case len(subjects) > 0:
		return subjects[0]
	default:
		return summary
	}
}

// PullRequestBody proposes a pull request's description: the repository's
// template, or the branch's commits when it has none, then a link to the issue
// unless the text already names it.
func PullRequestBody(template string, subjects []string, issueKey, issueURL string) string {
	body := strings.TrimRight(template, "\n")

	if body == "" && len(subjects) > 0 {
		body = "## Commits\n\n- " + strings.Join(subjects, "\n- ")
	}

	if issueKey != "" && !references(body, issueKey) {
		body = strings.TrimLeft(body+"\n\n"+issueLine(issueKey, issueURL), "\n")
	}

	if body == "" {
		return ""
	}

	return body + "\n"
}

// WithIssueLine is body with the line naming the issue added, as a pull
// request workflow opens carries it, and whether it needed adding: a pull
// request opened outside workflow, then linked to its issue, gains the same
// line, and the forge closes a forge issue named so when it merges.
func WithIssueLine(body, issueKey, issueURL string) (string, bool) {
	if references(body, issueKey) {
		return body, false
	}

	return strings.TrimLeft(strings.TrimRight(body, "\n")+"\n\n"+issueLine(issueKey, issueURL), "\n") + "\n", true
}

// references reports whether text names the issue key as a whole token, so a
// longer key that only contains its text does not count as naming it. A forge
// number names its issue only as #42 or in the issue's URL: a bare 42 is as
// likely a count, a version or another key's number.
func references(text, issueKey string) bool {
	quoted := regexp.QuoteMeta(issueKey)
	if forgeIssueNumber(issueKey) {
		return regexp.MustCompile(`(?:(?:^|[^\w&])#|/issues/)` + quoted + `\b`).MatchString(text)
	}

	return regexp.MustCompile(`\b` + quoted + `\b`).MatchString(text)
}

// issueLine names the issue in the pull request body. A forge issue is named
// the way its forge auto-closes it on merge; a Jira issue is named and linked in
// Markdown, which both forges render.
func issueLine(issueKey, issueURL string) string {
	if forgeIssueNumber(issueKey) {
		return "Closes #" + issueKey
	}

	if issueURL == "" {
		return "Jira: " + issueKey
	}

	return "Jira: [" + issueKey + "](" + issueURL + ")"
}

// forgeIssueNumber reports that the key is a forge issue number — all digits —
// rather than a Jira key, which always carries a letter.
func forgeIssueNumber(issueKey string) bool {
	for _, character := range issueKey {
		if character < '0' || character > '9' {
			return false
		}
	}

	return issueKey != ""
}
