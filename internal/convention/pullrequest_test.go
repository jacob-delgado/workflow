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

			// Act & Assert
			if got := convention.PullRequestTitle(tt.subjects, tt.key, tt.summary); got != tt.want {
				t.Errorf("PullRequestTitle = %q, want %q", got, tt.want)
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
		"nothing to say": {template: "", subjects: nil, key: "", url: "", want: ""},
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
