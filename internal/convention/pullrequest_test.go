// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package convention_test

import (
	"testing"

	"github.com/jacob-delgado/workflow/internal/convention"
)

func TestPullRequestTitleIsTheOldestCommit(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		subjects     []string
		key, summary string
		want         string
	}{
		"the first commit on the branch": {
			subjects: []string{redactSubject, "test: cover the empty token"},
			key:      projKey, summary: issueSummary,
			want: redactSubject,
		},
		"no commits yet, so the issue": {
			subjects: nil, key: projKey, summary: issueSummary, want: "PROJ-412: Fix token redaction",
		},
		"no issue either": {subjects: nil, key: "", summary: "", want: ""},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			got := convention.PullRequestTitleFrom(convention.TitleFromCommit, tt.subjects, tt.key, tt.summary)

			// Assert
			if got != tt.want {
				t.Errorf("PullRequestTitleFrom(TitleFromCommit) = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPullRequestTitleFromTheChosenSource(t *testing.T) {
	t.Parallel()

	subjects := []string{redactSubject, "test: cover the empty token"}

	cases := map[string]struct {
		source       convention.TitleSource
		key, summary string
		want         string
	}{
		"the commit source takes the oldest commit": {
			source: convention.TitleFromCommit, key: projKey, summary: issueSummary, want: redactSubject,
		},
		"the issue source takes the issue over the commit": {
			source: convention.TitleFromIssue, key: projKey, summary: issueSummary,
			want: "PROJ-412: Fix token redaction",
		},
		"the issue source falls back to the commit without an issue": {
			source: convention.TitleFromIssue, key: "", summary: "", want: redactSubject,
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act & Assert
			if got := convention.PullRequestTitleFrom(tt.source, subjects, tt.key, tt.summary); got != tt.want {
				t.Errorf("PullRequestTitleFrom(%q) = %q, want %q", tt.source, got, tt.want)
			}
		})
	}
}

func TestPullRequestBodyStartsFromTheTemplate(t *testing.T) {
	t.Parallel()

	const jiraLink = "https://jira.example.com/browse/PROJ-412"

	cases := map[string]struct {
		template string
		subjects []string
		key, url string
		want     string
	}{
		"a template, and the issue linked under it": {
			template: "## What this changes\n\n## Related issue\n\n", subjects: []string{"fix: a"},
			key: projKey, url: jiraLink,
			want: "## What this changes\n\n## Related issue\n\nJira: [PROJ-412](" + jiraLink + ")\n",
		},
		"no template, so the commits": {
			template: "", subjects: []string{"fix: a", "test: b"}, key: projKey, url: "",
			want: "## Commits\n\n- fix: a\n- test: b\n\nJira: PROJ-412\n",
		},
		"a template already naming the issue is left alone": {
			template: "Fixes PROJ-412\n", subjects: nil, key: projKey, url: jiraLink,
			want: "Fixes PROJ-412\n",
		},
		// A longer key that merely contains this one does not count as naming it.
		"a longer key in the template is not this issue": {
			template: "Depends on PROJ-4120\n", subjects: nil, key: projKey, url: jiraLink,
			want: "Depends on PROJ-4120\n\nJira: [PROJ-412](" + jiraLink + ")\n",
		},
		"a forge issue is closed by the body": {
			template: "", subjects: []string{"chore: bump"}, key: "42", url: "",
			want: "## Commits\n\n- chore: bump\n\nCloses #42\n",
		},
		"nothing to say": {template: "", subjects: nil, key: "", url: "", want: ""},
		// Neither tracker writes a key so: a forge issue's number has no leading
		// zero, and a Jira key has a project.
		"a key no tracker writes is not linked": {
			template: "", subjects: []string{"chore: bump"}, key: "0", url: "",
			want: "## Commits\n\n- chore: bump\n",
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act & Assert
			if got := convention.PullRequestBody(tt.template, tt.subjects, tt.key, tt.url); got != tt.want {
				t.Errorf("PullRequestBody =\n%q\nwant\n%q", got, tt.want)
			}
		})
	}
}

// linkedKey is the Jira issue a pull request is linked to below.
const linkedKey = "OPS-5"

func TestWithIssueLineAddsTheIssueToAPullRequestThatDoesNotNameIt(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		body, key, url, want string
		changed              bool
	}{
		"a Jira issue": {
			body: "Speeds up search.\n", key: linkedKey, url: "https://jira/browse/OPS-5",
			want: "Speeds up search.\n\nJira: [OPS-5](https://jira/browse/OPS-5)\n", changed: true,
		},
		"a forge issue":   {body: "Speeds up search.", key: "42", want: "Speeds up search.\n\nCloses #42\n", changed: true},
		"an empty body":   {body: "", key: "42", want: "Closes #42\n", changed: true},
		"one it names":    {body: "Fixes OPS-5 at last.\n", key: linkedKey, want: "Fixes OPS-5 at last.\n", changed: false},
		"a longer number": {body: "Closes #420\n", key: "42", want: "Closes #420\n\nCloses #42\n", changed: true},
		"a count":         {body: "ran 42 tests", key: "42", want: "ran 42 tests\n\nCloses #42\n", changed: true},
		"a version":       {body: "v1.42", key: "42", want: "v1.42\n\nCloses #42\n", changed: true},
		"a Jira-like key": {body: "PR-42", key: "42", want: "PR-42\n\nCloses #42\n", changed: true},
		"an anchor":       {body: "docs#42", key: "42", want: "docs#42\n\nCloses #42\n", changed: true},
		"a mention":       {body: "Follows (#42).", key: "42", want: "Follows (#42).", changed: false},
		"the issue's URL": {
			body: "See https://github.com/o/r/issues/42", key: "42", want: "See https://github.com/o/r/issues/42",
			changed: false,
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			got, changed := convention.WithIssueLine(tt.body, tt.key, tt.url)

			// Assert
			if got != tt.want || changed != tt.changed {
				t.Errorf("WithIssueLine = %q, %v; want %q, %v", got, changed, tt.want, tt.changed)
			}
		})
	}
}
