// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package convention

import (
	"regexp"
	"strings"
)

// PullRequestTitle proposes a pull request's title: the oldest commit on the
// branch, which on a branch of Conventional Commits already reads as one, or the
// issue when there are no commits yet.
func PullRequestTitle(subjects []string, issueKey, summary string) string {
	switch {
	case len(subjects) > 0:
		return subjects[0]
	case issueKey != "":
		return issueKey + ": " + summary
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

// references reports whether text names the issue key as a whole token, so a
// longer key that only contains its text does not count as naming it.
func references(text, issueKey string) bool {
	return regexp.MustCompile(`\b` + regexp.QuoteMeta(issueKey) + `\b`).MatchString(text)
}

// issueLine names the issue, linked when there is a link. Markdown, which both
// forges render.
func issueLine(issueKey, issueURL string) string {
	if issueURL == "" {
		return "Jira: " + issueKey
	}

	return "Jira: [" + issueKey + "](" + issueURL + ")"
}
