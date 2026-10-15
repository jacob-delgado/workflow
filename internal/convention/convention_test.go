// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package convention_test

import (
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/convention"
)

// Fixtures the tests share.
const (
	projKey    = "PROJ-412"
	taskType   = "Task"
	fixType    = "fix"
	featType   = "feat"
	choreType  = "chore"
	hotfixType = "hotfix"
	// redactTokens is a description that satisfies every rule, and
	// redactSubject the subject it makes.
	redactTokens  = "redact tokens"
	redactSubject = "fix(config): redact tokens"
	// issueSummary is the summary of the issue projKey names.
	issueSummary = "Fix token redaction"
	// project is the Jira project projKey belongs to.
	project = "PROJ"
)

func TestBranchNameReadsAsTheIssue(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		issueType, key, summary string
		want                    string
	}{
		"a bug is a fix": {
			issueType: "Bug", key: projKey, summary: issueSummary,
			want: "fix/PROJ-412-fix-token-redaction",
		},
		"anything else is a feature": {
			issueType: "Story", key: "PROJ-7", summary: "Add retries to the webhook client",
			want: "feat/PROJ-7-add-retries-to-the-webhook-client",
		},
		"the type is matched without regard to case": {
			issueType: "bug", key: "OPS-1", summary: "x", want: "fix/OPS-1-x",
		},
		"punctuation collapses to one hyphen": {
			issueType: taskType, key: "OPS-2", summary: "  Don't crash --- on: empty (nil) input!  ",
			want: "feat/OPS-2-don-t-crash-on-empty-nil-input",
		},
		"accents fold to ASCII": {
			issueType: taskType, key: "OPS-3", summary: "Café menü für Zoë",
			want: "feat/OPS-3-cafe-menu-fur-zoe",
		},
		"a letter with no ASCII form ends a word": {
			issueType: taskType, key: "OPS-6", summary: "Fix the λ parser",
			want: "feat/OPS-6-fix-the-parser",
		},
		"a digit stays in its word": {
			issueType: taskType, key: "OPS-8", summary: "Retry 3 times on 5xx",
			want: "feat/OPS-8-retry-3-times-on-5xx",
		},
		"a summary with no letters leaves just the key": {
			issueType: taskType, key: "OPS-4", summary: "!!! ???", want: "feat/OPS-4",
		},
		"a long summary stops at a word": {
			issueType: taskType, key: "OPS-5",
			summary: "Replace the hand rolled retry loop in the webhook client with exponential backoff",
			want:    "feat/OPS-5-replace-the-hand-rolled-retry-loop-in-the",
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			got := convention.DefaultBranchNaming().Name(tt.issueType, tt.key, tt.summary)

			// Assert
			if got != tt.want {
				t.Errorf("Name = %q, want %q", got, tt.want)
			}

			err := convention.ValidateBranchName(got)
			if err != nil {
				t.Errorf("Name proposed %q, which git would refuse: %v", got, err)
			}
		})
	}
}

func TestIssueKeyReadsAForgeNumberFromABranch(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		text    string
		want    string
		tracker convention.Tracker
	}{
		"a forge branch":           {text: "fix/42-fix-typo", want: "42", tracker: convention.TrackerForge},
		"no prefix":                {text: "7-add-retries", want: "7", tracker: convention.TrackerForge},
		"an empty slug":            {text: "feat/13", want: "13", tracker: convention.TrackerForge},
		"a Jira key still wins":    {text: "fix/PROJ-1-thing", want: "PROJ-1", tracker: convention.TrackerJira},
		"a slug number is no key":  {text: "fix/add-256-colors", want: ""},
		"a hash is no forge key":   {text: "chore/bump-sha-256", want: ""},
		"a leading zero is no key": {text: "fix/0-nope", want: ""},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			got, ok := convention.IssueKey(tt.text, "")

			// Assert
			if got.Key != tt.want || ok != (tt.want != "") || (ok && got.Tracker != tt.tracker) {
				t.Errorf("IssueKey(%q) = %+v, %v, want %q of tracker %v", tt.text, got, ok, tt.want, tt.tracker)
			}
		})
	}
}

func TestIssueKeyIsFoundWhereJiraWouldFindIt(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		text, project, want string
	}{
		"a generated branch":      {text: "fix/PROJ-412-token-redaction", want: projKey},
		"a bare key":              {text: "OPS-7", want: "OPS-7"},
		"digits in the project":   {text: "feat/AB2C-10-thing", want: "AB2C-10"},
		"the first of two":        {text: "OPS-1-and-OPS-2", want: "OPS-1"},
		"a key inside a word":     {text: "xOPS-1", want: ""},
		"lowercase is not a key":  {text: "fix/utf-8-handling", want: ""},
		"a version is not a key":  {text: "release/V-0", want: ""},
		"one letter is not a key": {text: "A-1", want: ""},
		"nothing":                 {text: "main", want: ""},
		"a key followed by digit": {text: "OPS-12x", want: "OPS-12"},
		// Common standards read like a key but are not one.
		"an encoding is not a key": {text: "fix/UTF-8-decoding", want: ""},
		"a hash is not a key":      {text: "feat/SHA-256-support", want: ""},
		"a CVE is not a key":       {text: "fix/CVE-2024-1-patch", want: ""},
		// A real key after a standard or another project's key is still
		// found, even with only a separator between them.
		"a standard then a key":                 {text: "fix/UTF-8-and-PROJ-412", want: projKey},
		"a standard directly before a key":      {text: "fix/UTF-8-PROJ-412", want: projKey},
		"another project's key directly before": {text: "feat/ABC-1-PROJ-9", project: project, want: "PROJ-9"},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			got, ok := convention.IssueKey(tt.text, tt.project)

			// Assert
			if got.Key != tt.want || ok != (tt.want != "") {
				t.Errorf("IssueKey(%q, %q) = %q, %v, want %q", tt.text, tt.project, got.Key, ok, tt.want)
			}
		})
	}
}

// "@" is refused, but not because it is empty — it is git's shorthand for the
// current branch — so the reason must not misreport it as blank.
func TestTheAtSignIsRefusedForWhatItIs(t *testing.T) {
	t.Parallel()

	// Act
	err := convention.ValidateBranchName("@")

	// Assert
	if err == nil || strings.Contains(err.Error(), "empty") {
		t.Errorf("ValidateBranchName(%q) = %v, want a reason that is not about emptiness", "@", err)
	}
}

func TestBranchTypeReadsThePrefixWhenItIsACommitType(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		branch string
		want   string
	}{
		"a fix branch":              {branch: "fix/PROJ-1-thing", want: fixType},
		"a feat branch":             {branch: "feat/OPS-2-add", want: featType},
		"a docs branch":             {branch: "docs/readme", want: "docs"},
		"a prefix that is no type":  {branch: "feature/OPS-3", want: ""},
		"no slash at all":           {branch: "main", want: ""},
		"a bare type is not a type": {branch: fixType, want: ""},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			got, ok := convention.DefaultCommitConvention().BranchType(tt.branch)

			// Assert
			if got != tt.want || ok != (tt.want != "") {
				t.Errorf("BranchType(%q) = %q, %v, want %q", tt.branch, got, ok, tt.want)
			}
		})
	}
}

func TestValidateBranchNameAcceptsWhatGitAccepts(t *testing.T) {
	t.Parallel()

	for _, branch := range []string{"feat/PROJ-1-x", "a", "fix/nested/deeper", "v1.2"} {
		t.Run(strconv.Quote(branch), func(t *testing.T) {
			t.Parallel()

			// Act
			err := convention.ValidateBranchName(branch)
			// Assert
			if err != nil {
				t.Errorf("ValidateBranchName(%q) = %v, want nil", branch, err)
			}
		})
	}
}

func TestValidateBranchNameRefusesWhatGitRefuses(t *testing.T) {
	t.Parallel()

	// Each is a rule of git check-ref-format --branch.
	for _, branch := range []string{
		"", "has space", "a..b", "a/.hidden", ".start", "end.", "end/", "/start", "a//b",
		"x.lock", "a/b.lock/c", "a@{b", "@", "-start", "tilde~", "caret^", "colon:",
		"question?", "star*", "bracket[", "back\\slash", "tab\tname", "del\x7f",
	} {
		t.Run(strconv.Quote(branch), func(t *testing.T) {
			t.Parallel()

			// Act
			err := convention.ValidateBranchName(branch)

			// Assert
			if !errors.Is(err, convention.ErrInvalidBranchName) {
				t.Errorf("ValidateBranchName(%q) = %v, want ErrInvalidBranchName", branch, err)
			}
		})
	}
}

func TestIssueKeyRestrictsToTheConfiguredProject(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		text, project, want string
		found               bool
	}{
		"the configured project matches":     {text: "feat/PROJ-42-thing", project: project, want: "PROJ-42", found: true},
		"another project is not read as one": {text: "feat/ABC-123-thing", project: project, want: "", found: false},
		"no project falls back to the guard": {text: "fix/UTF-8-decoding", project: "", want: "", found: false},
		"no project still reads a real key":  {text: "feat/PROJ-42-thing", project: "", want: "PROJ-42", found: true},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			got, found := convention.IssueKey(tt.text, tt.project)

			// Assert
			if got.Key != tt.want || found != tt.found {
				t.Errorf("IssueKey(%q, %q) = %q, %v; want %q, %v", tt.text, tt.project, got.Key, found, tt.want, tt.found)
			}
		})
	}
}

func TestRefOfTellsAKeysTracker(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		key     string
		tracker convention.Tracker
		known   bool
	}{
		"a Jira key":           {key: projKey, tracker: convention.TrackerJira, known: true},
		"a forge number":       {key: "42", tracker: convention.TrackerForge, known: true},
		"a hash-marked number": {key: "#42", tracker: convention.TrackerForge, known: true},
		"a leading zero":       {key: "042", known: false},
		"a word":               {key: "develop", known: false},
		"nothing":              {key: "", known: false},
		"a slug with a hyphen": {key: "fix-typo", known: false},
		"a lone hyphen":        {key: "-", known: false},
		"a lowercase key":      {key: "proj-7", known: false},
		"a zero number":        {key: "PROJ-0", known: false},
		"no number":            {key: "PROJ-", known: false},
		"a one-letter project": {key: "P-7", known: false},
		"a URL":                {key: "https://jira.example.com/browse/PROJ-7", known: false},
		"a terminal control":   {key: "\x1b]8;;http://x\aPROJ-7", known: false},
		"a key with a tail":    {key: "PROJ-7 and more", known: false},
		"an underscore key":    {key: "MY_PROJ2-7", tracker: convention.TrackerJira, known: true},
		"a doubled hash":       {key: "##42", known: false},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			ref, known := convention.RefOf(tt.key)

			// Assert
			if known != tt.known || (known && ref.Tracker != tt.tracker) {
				t.Errorf("RefOf(%q) = %+v, %v; want tracker %v, %v", tt.key, ref, known, tt.tracker, tt.known)
			}
		})
	}
}

func TestRefOfDropsTheHashFromAForgeNumber(t *testing.T) {
	t.Parallel()

	// Act
	ref, _ := convention.RefOf("#42")

	// Assert
	if ref.Key != "42" {
		t.Errorf("RefOf(#42).Key = %q, want 42", ref.Key)
	}
}

// mentionedKey is the Jira key the pull request texts below mention.
const mentionedKey = "OPS-31"

func TestIssueInTextReadsAPullRequestsIssue(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		text    string
		want    string
		tracker convention.Tracker
	}{
		"a Jira key in the title": {text: "OPS-31: speed up search", want: mentionedKey, tracker: convention.TrackerJira},
		"the Jira line workflow writes": {
			text: "Fix it\n\nJira: [OPS-31](https://jira/browse/OPS-31)", want: mentionedKey, tracker: convention.TrackerJira,
		},
		"a closing keyword":     {text: "Speeds it up.\n\nCloses #42", want: "42", tracker: convention.TrackerForge},
		"a bare mention":        {text: "Follows up on #9.", want: "9", tracker: convention.TrackerForge},
		"a Jira key wins":       {text: "OPS-31, see #42", want: mentionedKey, tracker: convention.TrackerJira},
		"an anchor is no issue": {text: "see docs#42 and a&#42;", want: ""},
		"no issue at all":       {text: "Speed up search", want: ""},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			got, ok := convention.IssueInText(tt.text, "")

			// Assert
			if got.Key != tt.want || ok != (tt.want != "") || (ok && got.Tracker != tt.tracker) {
				t.Errorf("IssueInText(%q) = %+v, %v; want %q of tracker %v", tt.text, got, ok, tt.want, tt.tracker)
			}
		})
	}
}
