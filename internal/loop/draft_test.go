// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package loop_test

import (
	"testing"

	"github.com/jacob-delgado/workflow/internal/convention"
	"github.com/jacob-delgado/workflow/internal/loop"
)

// jiraLine is how a drafted body names the issue the branch is for.
const jiraLine = "Jira: [" + issueKey + "](" + browseURL + ")\n"

func TestADraftProposesTheTitleAndBodyFromWhatItIsGiven(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		source    convention.TitleSource
		template  string
		wantTitle string
		wantBody  string
	}{
		"titled from the issue, the body from a template": {
			source: convention.TitleFromIssue, template: "## What\n\nIt redacts.\n",
			wantTitle: issueKey + ": " + summary, wantBody: "## What\n\nIt redacts.\n\n" + jiraLine,
		},
		"titled from the commit, the body listing the commits": {
			source:    convention.TitleFromCommit,
			wantTitle: subject, wantBody: "## Commits\n\n- " + subject + "\n\n" + jiraLine,
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			input := loop.DraftInput{
				Subjects: []string{subject}, IssueKey: issueKey, IssueSummary: summary, IssueURL: browseURL,
				Template: tt.template, TitleSource: tt.source,
			}

			// Act
			title, body := loop.Draft(input)

			// Assert
			if title != tt.wantTitle || body != tt.wantBody {
				t.Errorf("Draft = %q, %q; want %q, %q", title, body, tt.wantTitle, tt.wantBody)
			}
		})
	}
}
